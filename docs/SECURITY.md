# Security

The people most likely to enjoy this are the people least likely to let anything
hook their shell. So the security design is the selling point, and every promise
below is enforced by the kernel, by a test, or both.

## The promise

tenant looks at **names**, never **contents**. It writes only to its own state
directory. It never touches the network and never runs another program. It keeps an
honest log of everything it has looked at, and hands it over whenever you ask.

## Invariants

| # | Invariant | Enforced by |
|---|---|---|
| I1 | Never opens a file of yours for reading, including `$HISTFILE`. It only lists directory names under `$HOME`. | Landlock (`READ_DIR` on `$HOME`, `READ_FILE` only in its own directory). A policy test allows `os` file calls only in `internal/audit`, and the sandbox test checks that reads are denied. |
| I2 | Never writes outside `$XDG_STATE_HOME/tenant`, and never edits your rc files or history | Landlock; `audit` only writes a fixed list of its own file names; the policy test bans `print -s`, `history -s` and history writes in the hooks; the integration tests check the history file |
| I3 | Never uses the network | No `net` in the dependency graph (CI). Landlock denies TCP bind and connect on ABI 4+; the sandbox test checks this. |
| I4 | Never runs other programs | No `os/exec` or `syscall.Exec` (policy test). Landlock denies execute; the sandbox test checks this. |
| I5 | The hooks never `eval` anything the binary prints | Protocol: fixed verbs and integers, `case` dispatch, assignment-only text. The policy test allows exactly two `eval`s, which rename your own not-found handler. |
| I6 | Never wraps, aliases or replaces your commands | The hooks only add output around commands. The not-found hook runs your handler first. |
| I7 | The audit log is complete and truthful | Every listing goes through `audit.ListDir`, which writes the log entry **before** reading. `confess` prints the log verbatim; the story can only add an epilogue after it. |
| I8 | Inert unless started, as root, with `TENANT_OFF=1`, outside `$HOME`, and during a git rebase, merge or bisect; `evict` works in every shell at the next prompt | A sentinel file checked in pure shell; EUID checks in both hook and binary; guards in the engine; integration tests |
| I9 | Anything from your filesystem that is printed is sanitised | `term.Clean` strips C0 and C1 controls, escape sequences, bidi and zero-width characters, and caps the length. Tested for every mechanic. |
| I10 | A ghost command left on your line is harmless if you press Enter | `mech.HarmlessCommand`: only `cd`, `ls`, `pwd`, `true` or `tenant`, arguments of plain characters only (so a directory called `x$(touch pwned)` can't be one), and a comment in plain words. Anything else isn't shown. Unit and pty tests. |

## Exactly what it reads

- The **names** of entries (and whether each is a directory) in directories under
  `$HOME`, only when a beat that's about to fire needs them, and at most 256 per
  directory. Each listing is logged, and the names go into `seen.json`. A symlink
  that leads out of `$HOME` is refused even without Landlock.
- The **first word** of each command you run, worked out in the shell. The full
  command line never reaches the binary. On bash it comes from the shell's in-memory
  `history 1`; the history file is never read. Commands started with a space are
  invisible under `ignorespace` or `HIST_IGNORE_SPACE`, typos included.
- Your **cwd**, the exit status, `$USER`, `COLUMNS`, `LS_COLORS`, `TERM` and the host
  name (`uname`).
- Whether git marker names (`.git/rebase-merge`, `rebase-apply`, `MERGE_HEAD`,
  `BISECT_LOG`) exist in the repository you're in. It checks existence only.
- Which terminal it runs on (`/proc/self/fd/0` → `pts/3`), for the login line.
- The system time zone (`/etc/localtime`), loaded by the Go runtime **before** the
  sandbox goes up.

`confess` lists all of these as "standing behaviour" at the top.

## Exactly what it writes

Only `active`, `state.json`, `seen.json`, `audit.log`, `lock` and temporary files
for atomic writes, all in `~/.local/state/tenant`. `evict` deletes them and then the
directory. The hook exports one environment variable, `TENANT_HOOK`, which `doctor`
uses to tell whether the hook is loaded (`bash`, `zsh`, or
`bash-exported-prompt-command` when it chose not to attach; see
[ARCHITECTURE](ARCHITECTURE.md#shell-hooks)).

## Landlock sandbox

`internal/sandbox` calls `landlock_create_ruleset`, `landlock_add_rule` and
`landlock_restrict_self` directly. It uses no library, and it reaches every thread
through `AllThreadsSyscall`, after setting `no_new_privs`. It handles every access
right the kernel knows about (ABI 1 to 7), so anything not granted is denied:

| Path | Access | When |
|---|---|---|
| `$HOME` (recursive) | `READ_DIR` only | the prompt hook and the finale; every other command gets no access to `$HOME` at all |
| `~/.local/state/tenant` | read and write files, make and remove entries | always |
| `~/.local/state` | `REMOVE_DIR` only | `evict` only, so it can remove its own directory |
| everything else | nothing | |
| TCP (ABI 4+) | no bind, no connect | always |
| signals, abstract sockets (ABI 6+) | scoped to itself | always |

- On kernels without Landlock, the invariants hold by code and tests only, and
  `tenant doctor` says so: `sandbox: unavailable (kernel has no Landlock), enforced
  by code only`.
- **The hook scripts are not sandboxed.** They run inside your shell with your
  privileges. They are short, use builtins only, `eval` nothing the binary prints, and
  are covered by the integration tests. Read one with `tenant init bash | less`.

## Shell-side safety

- **No runtime eval.** The only `eval` of output is in your rc file, and it evaluates
  the static script from `tenant init`.
- **Integers, not strings.** `glyph` carries a position and two lowercase letters as
  numbers. The shell builds the path itself and puts it in the prompt as a **variable
  reference** (bash, and zsh with `PROMPT_SUBST`) or as `%`-escaped text (zsh
  otherwise), so a directory called `` x$(touch pwned)`touch pwned2`%F{red}a `` shows
  up as itself. The integration tests use exactly that name.
- **Assignment only.** Ghost and time text reach the shell as `var=$(tenant …)` and
  are rejected if they contain control characters. Ghost text must also pass I10.
- **Your shell options can't bend it.** Every bash function returns 0 or the status
  it was given, so `set -e` never exits your shell because of tenant, and `set -u`
  finds nothing unset. Every zsh function sets its own options locally, so
  `ksharrays`, `globsubst`, `nounset`, `errexit` and `warncreateglobal` don't change
  what it does. Tested in both shells.
- **`$?` preserved**, checked before start, while active and after evict.

## The audit log

```
log
2026-10-06 21:14  started   normal pace
2026-10-06 21:14  list      ~/code/rarepulls (names only)
...
2026-10-08 23:41  observed 412 commands (names only)
2026-10-08 23:41  wrote nothing
```

- It is append-only. Each entry is written before the access it describes.
- `confess` prints the standing behaviour, the log, every name in `seen.json`, and the
  command-name counts. `--summary` skips the names.
- `wrote nothing` is computed from the log, which would list any write outside the
  state directory. There is no code path that makes one.
- Once the story is finished, a closing passage follows the log after a rule. It is
  never mixed into it.

## Developer commands

`_force`, `_invite`, `_schedule` and `_sim` need `TENANT_DEV=1`. They only change
tenant's own state, run under the same sandbox, and forced beats never count as story
progress. `TENANT_DEADLINE_MS` can raise the 25 ms hook deadline (the tests use it).

## Threat model

| Threat | Mitigation |
|---|---|
| Hostile file or directory names (shell or terminal injection) | I5, I9, variable-reference prompts, adversarial tests in both shells |
| A hostile story pack | Story files are parsed as data: unknown keys, mechanics and variables are rejected. Templates can't name paths. |
| Shared or multi-user machines | Per-user state. Inert as root. |
| Mistaken for malware by AV tools or packagers | Open source, no dependencies, no obfuscation, static reproducible builds (`-trimpath`), an AUR package built from source, and a hook you add yourself |
| A compromised binary on `$PATH` | Out of scope: that attacker already owns your shell. Verify `SHA256SUMS`. |

## Verify it yourself

```sh
tenant init bash | less          # the whole hook
tenant doctor                    # sandbox status, capabilities, latency
strace -f -e trace=openat,connect,execve,landlock_restrict_self \
  tenant _hook prompt --shell bash --cmd ls --shape ls:p
go test ./internal/sandbox ./internal/policy   # the kernel and source checks
```

## Reporting a vulnerability

Open a GitHub security advisory on this repository. Please don't file a public
issue.
