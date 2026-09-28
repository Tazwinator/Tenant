# Security

The people most likely to enjoy this are the people least likely to let anything
hook their shell. So the security design is the selling point, not an afterthought.

## The promise

tenant looks at **names**, never **contents**. It writes only to its own state
directory. It never touches the network and never runs another program. It keeps an
honest log of everything it has looked at, and hands it over whenever you ask.

## Invariants

| # | Invariant | Enforced by |
|---|---|---|
| I1 | Never opens a file of yours for reading. It only lists directory names under `$HOME`. | Landlock (`READ_DIR` on `$HOME`, `READ_FILE` only on its own dirs). CI: all FS access goes through `internal/audit`. |
| I2 | Never writes outside `$XDG_STATE_HOME/tenant`. It never edits `.bashrc`, `.zshrc`, `$HISTFILE` or any other file of yours. | Landlock. The history ghost lives only in the line editor. |
| I3 | Never uses the network | No `net` import (CI check). Landlock TCP rules on ABI v4+ (kernel 6.7+). |
| I4 | Never runs other programs | No `os/exec` import (CI check). Landlock blocks `execute`. |
| I5 | The hook never `eval`s runtime output from the binary | Hook protocol: fixed verbs plus integers, `case` dispatch, assignment-only text. Reviewed and pty-tested. |
| I6 | Never wraps, aliases or replaces your commands | Hook review. The only functions it defines under shell-reserved names are the not-found hooks (`command_not_found_handle` in bash, `command_not_found_handler` in zsh), which delegate to your existing handler. |
| I7 | The audit log is complete and truthful | Every read and write goes through `internal/audit`, which logs before acting. `confess` prints it verbatim. |
| I8 | Inert unless started; inert as root; inert with `TENANT_OFF=1`; `evict` takes effect in every shell straight away | Sentinel file checked in pure shell. EUID check in both the hook and the binary. |
| I9 | Anything printed to your terminal that came from your filesystem is sanitised | `internal/term`: control characters and escape sequences are stripped, and length is capped |

## Exactly what it reads

- The **names** of entries in directories under `$HOME` that you `cd` into, only when
  a beat needs them. Each listing is logged with its path and entry count.
- The **first word** of each command you run (`git`, `ls`, `nvim`). The shell
  extracts it, so the binary never receives the full command line, including
  anything like a pasted token.
- Your **cwd**, exit status, `$USER`, `COLUMNS`, `LS_COLORS` and `TERM`, all passed
  in by the hook or the environment.
- Its **own** state and config files.
- The system time zone (`/etc/localtime`), read by the Go runtime before the sandbox
  is applied. `confess` discloses this too.

It does **not** read `$HISTFILE`, ever (D11 in [DECISIONS](DECISIONS.md)). What it
knows about your habits, it learned by watching during the dormancy period. On bash,
the hook finds the command you just ran from the shell's in-memory `history 1`,
inside the shell, and passes on only its first word. Commands you start with a space
are never seen if your shell keeps them out of history.

## Exactly what it writes

Only files under `~/.local/state/tenant` (see
[ARCHITECTURE](ARCHITECTURE.md#state)). `tenant evict` deletes that directory
completely. The one line in your rc file is yours to add and yours to remove. tenant
never touches it.

## Landlock sandbox

At the start of every invocation, the binary loads what it needs (time zone,
config), then restricts itself before it does anything that depends on your files:

| Path | Access |
|---|---|
| `$HOME` (recursive) | `READ_DIR` only |
| `$XDG_STATE_HOME/tenant` | read and write files, make and remove entries |
| `$XDG_CONFIG_HOME/tenant` | read files |
| everything else | nothing |
| TCP (ABI v4+) | no bind, no connect |

- It uses `go-landlock` in best-effort mode. On kernels without Landlock, the
  invariants hold by code and CI checks only, and `tenant doctor` says so plainly:
  `sandbox: unavailable (kernel 5.10), enforced by code only`.
- The **hook script is not sandboxed.** It runs inside your shell with your
  privileges. Its safety comes from being short, using builtins only, `eval`ing
  nothing, and being printable for review with `tenant init bash | less`.

## Shell-side safety

- **No runtime eval.** The one `eval` is in your rc file, and it evaluates the
  static script from `tenant init <shell>`, which doesn't depend on your files or
  state.
- **Integers, not strings.** Actions that change `PROMPT` carry only indices and
  codepoints. The shell computes the text itself from `$PWD`.
- **Escaping.** Anything placed in `PROMPT` or `RPROMPT` has `%`, `$`, backtick and
  `\` escaped. This matters when `PROMPT_SUBST` is set, where an unescaped
  directory called `$(rm -rf ~)` would otherwise run.
- **`$?` preserved.** The prompt hook returns the status it received. On bash, a
  tiny status-capturing function is prepended to `PROMPT_COMMAND` so that other
  frameworks don't clobber `$?` first.
- **Adversarial names** are part of the test suite (see
  [ARCHITECTURE](ARCHITECTURE.md#testing)).

## Terminal output

Names are printed as data. Before printing, tenant strips C0 and C1 control
characters, ESC sequences and bidi overrides, and truncates the result. A file named
`\e]2;pwned\a` is shown as a harmless string and doesn't set your title.

## The audit log

```
2026-10-06 21:14  list      ~/code/rarepulls (names only, 6 entries)
2026-10-06 21:14  observed  cwd ~/code/rarepulls, argv0 ls
2026-10-08 23:41  observed  412 commands (names only)
2026-10-08 23:41  wrote     nothing
```

- It is append-only, and written by `internal/audit` **before** each access.
- `confess` prints every entry, plus every name in `seen.jsonl`. `--summary` groups
  them.
- `wrote nothing` is computed, not written in by hand: it summarises the log of
  writes outside the state directory, which should always be empty. If it ever
  weren't, the entries would be listed.
- The story can add a closing passage **after** the log, separated by a rule. It
  can never add, change or hide entries in the log itself.

## Threat model

| Threat | Mitigation |
|---|---|
| Hostile file or directory names (shell or terminal injection) | I5, I9, escaping, adversarial tests |
| A hostile story pack | Story packs are data: they can only reference audited mechanics and a fixed set of template variables. No templates that read paths. |
| Shared or multi-user machines | Per-user state. Inert as root. Never follows `sudo -s`. |
| tenant confused with malware by AV tools or packagers | Open source, no obfuscation, reproducible builds (`-trimpath`, pinned toolchain), AUR builds from source, no persistence tricks: the hook is a line you add yourself |
| A compromised binary on `$PATH` | Out of scope: that attacker already owns your shell. Verify checksums. |

## Verify it yourself

```sh
tenant init bash | less                    # read the whole hook
tenant doctor                              # sandbox status, probes, latency
strace -f -e trace=openat,connect,execve \
  tenant _hook prompt --shell bash --status 0 --cmd ls --shape ls:plain
```

## Reporting a vulnerability

Open a GitHub security advisory on this repository. Please don't file a public
issue.
