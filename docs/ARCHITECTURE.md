# Architecture

One static, CGO-free Go binary (`tenant`) with no third-party dependencies, and one
short hook script per shell, which the binary prints (`tenant init bash`, `tenant
init zsh`). The hook does as little as possible: on each prompt it hands the binary
a few facts, then carries out a small, fixed set of actions the binary asks for.
All decisions (whether anything happens, what, and when) are made in the binary,
inside a Landlock sandbox.

## Engine and surfaces

- **The engine** is the scheduler, story, observations, audit log and sandbox. It
  knows nothing about shells.
- **Surfaces** are the places the entity can appear. Today the only surfaces are the
  bash and zsh hooks. A future surface (fish, PowerShell, a desktop) goes into
  **the same binary**. See [PLATFORMS](PLATFORMS.md).

```
 bash: PROMPT_COMMAND, bind -x, command_not_found_handle
 zsh:  precmd, preexec, zle widget, command_not_found_handler
  │
  ▼
 hook script (shell builtins only)
  │  fast path: no ~/.local/state/tenant/active → nothing, no exec
  │
  ▼
 tenant _hook prompt --shell bash --status N --cmd ARGV0 --shape SHAPE --first 0|1 --caps BITS
  │  Landlock: read names under $HOME, use its own directory, nothing else
  ├── state     lock, load, save          (~/.local/state/tenant)
  ├── engine    observe cwd + argv0, move the story clock on, pick a beat
  ├── sched     seeded pacing: should anything fire on this prompt?
  ├── script    which beat, which text (story/act1)
  └── mech      carry the beat out
        ├── text for the player  → stderr, straight to the terminal
        └── actions for the hook → stdout: "glyph 3 111 117", "time", "ghost"
  │
  ▼
 hook reads the action lines and dispatches with `case` (never `eval`)
```

## Repo layout

```
cmd/tenant/          CLI: init, start, evict, confess, doctor, the finale, _hook, dev tools
internal/audit/      the only package that touches the filesystem; the audit log; seen names
internal/state/      state.json: story clock, observations, hand-offs
internal/sandbox/    Landlock through raw syscalls
internal/sched/      pace presets, sittings, chapter unlocks, guards, hazard, seeded rolls
internal/engine/     one tick: observe, advance, choose, fire; template variables; simulator
internal/mech/       the eight mechanics
internal/script/     parsers for beats, the finale dialogue and the epilogue
internal/shell/      the hook scripts (bash.sh, zsh.zsh) and the protocol encoder
internal/finale/     the typed conversation
internal/term/       sanitising, LS_COLORS, window titles, raw mode for the typewriter
internal/policy/     tests that hold the source to the security invariants
story/act1/          the script: beats.txt, finale.txt, epilogue.txt (spoilers)
teaser/              the VHS tape for the teaser clip
packaging/aur/       PKGBUILD
```

## Shell hooks

Each hook does the same jobs with its own shell's mechanisms:

| Job | bash (5.1+) | zsh |
|---|---|---|
| Keep `$?` and undo last prompt's change | `_tenant_pre`, **prepended** to the `PROMPT_COMMAND` array | `_tenant_status`, first in `precmd_functions` |
| Learn the last command's first word | `_tenant_pre`, from the in-memory `history 1` (never `$HISTFILE`), only when `HISTCMD` moved | `_tenant_preexec`, from its first argument |
| Run each prompt | `_tenant_post`, **appended**, so it runs after prompt frameworks build `PS1` | `_tenant_precmd`, last in `precmd_functions` |
| Up-arrow ghost | `bind -x` on Up while armed, then restores the previous readline function | a zle widget bound while armed, then restores the previous widget |
| Typo remark | wraps `command_not_found_handle` | wraps `command_not_found_handler` |

In both shells:

- Only the first word and a **shape** token leave the shell. Shapes are `ls:p`,
  `ls:l`, plus `c` (colour) and `a` (all), or `ls:x` (paths, pipes, icons, `-R`),
  and `clear`, `cd` or `-`. One level of alias is expanded in the shell to work the
  shape out.
- A command started with a space is invisible when the shell keeps such commands
  out of history (`HISTCONTROL=ignorespace`, `HIST_IGNORE_SPACE`). That includes
  the typo hook.
- The hook returns the status it received, so `$?` is preserved.
- Each shell passes its PID (`--pid $$`). Hand-offs (the ghost, the time, a pushed
  window title) are kept per shell, so one terminal's prompt never consumes another
  terminal's ghost or pops its title.
- **Robust to your settings.** bash functions always return 0 or the status they
  were given, so `set -e` and `set -u` are safe. zsh functions set `localoptions`, so
  `ksharrays`, `globsubst`, `nounset`, `errexit` and friends don't reach them.
- **Re-sourcing is safe.** Reading your rc file again keeps tenant's state and puts
  its two entries back at the front and end of `PROMPT_COMMAND` or `precmd_functions`,
  without duplicates. In bash, `_tenant_post` runs at most once per `_tenant_pre`.
- **An exported `PROMPT_COMMAND`** (bash) can't become an array without child
  processes losing it. In that case tenant doesn't attach, and `doctor` explains how
  to un-export it.
- On bash, the command-number marker is recorded at the end of each prompt, after
  your own `PROMPT_COMMAND` entries, so lines that `history -n` pulls in from other
  terminals are never mistaken for yours.
- When the `active` sentinel disappears, the hook unloads itself at the next prompt:
  prompt, bindings and the not-found handler go back (only if they are still
  tenant's), and its hooks and variables are removed. In bash, `_tenant_pre` and
  `_tenant_post` are left as do-nothing stand-ins that keep `$?`, because tools like
  bash-preexec fold `PROMPT_COMMAND` entries into their own. If the binary vanishes
  (exit status 126 or 127), the hook unloads too.

## Hook protocol

| Verb | Args | The hook |
|---|---|---|
| `glyph` | `offset from to` | Checks that the letter `offset` places from the end of the displayed cwd is `from`, swaps it for `to` (both lowercase ASCII), and points the prompt's cwd token at a variable holding the result. Restored at the next prompt if the prompt is unchanged. |
| `time` | none | Sets a variable from `$(tenant _text time)` and shows it right-aligned for one prompt. bash: `\[${_tenant_rtime}\]` prefix. zsh: `RPROMPT`. |
| `ghost` | none | Arms each Up key (`\e[A`, `\eOA`) whose own history function it can restore. The first press fills the line from `$(tenant _ghost --pid $$)`. |

`shell.Encode` drops any verb it doesn't know or any action with the wrong number of
arguments, so nothing unexpected can reach the hook. Text only ever reaches shell
variables by command-substitution assignment, which isn't evaluated.

## State

Everything lives in `$XDG_STATE_HOME/tenant` (default `~/.local/state/tenant`):

| File | Holds |
|---|---|
| `active` | Sentinel. Hooks are inert unless it exists. `evict` deletes it first. |
| `state.json` | Seed, pace, story clock, beats fired, command-name counts, directories visited (times only), and one-prompt hand-offs per shell PID (ghost, time, title) |
| `seen.json` | Every name tenant has read, by directory. `confess` prints it. |
| `audit.log` | Append-only, tab-separated log of every listing, written before it happens |
| `lock` | `flock` target, so several shells can't interleave. It gives up after about 40 ms rather than stall a prompt. |

Writes are atomic (temp file then `rename`). Directories and command counts are
capped (400 and 300).

## Performance

- **Inactive:** no exec. The fast path is `[[ -e .../active ]]`.
- **Active:** one exec per prompt (two when a `time` action runs). bash also forks
  once for `history 1` after a command.
- **Measured** in a slow CI-class VM, where `/bin/true` takes 1.4 ms: p50 3.6 ms and
  p95 4.8 ms per prompt, of which about 1.2 ms is tenant's own work. On a desktop,
  expect about half that.
- **Deadline:** any hook call that runs past 25 ms exits silently, before it saves
  anything, so a beat is never recorded without being shown. The prompt matters
  more than the story.
- `tenant doctor` reports the engine's own time.

## Failure policy

The shell must never break. The hook swallows every error. The binary does nothing
if it can't lock, load, parse or save. A missing binary makes the hook unload itself.

## Build and dependencies

```sh
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o tenant ./cmd/tenant
go test ./...          # needs bash and zsh for the integration tests; -short skips them
TENANT_DEV=1 ./tenant _sim --profile=daily   # the whole story on a fake clock (spoilers)
```

The binary depends only on the Go standard library. CI checks that `go list -deps`
contains no `net`, `os/exec`, `os/user`, `plugin` or `runtime/cgo`, that the binary
is statically linked, and that `go.mod` has no requirements.

## Testing

| Layer | How |
|---|---|
| Pacing | `engine.Simulate` drives the engine with synthetic usage on a fake clock. Tests assert the [tuning targets](SCHEDULER.md#tuning-targets) across 12 seeds per profile, plus dormancy, minimum gaps and daily caps. |
| Story | Act 1 parses, uses only known mechanics and variables, has a required beat per chapter and exactly one invitation. The finale reaches its end, and the turn limit works. |
| Mechanics | Output for each mechanic, and hostile text through every mechanic: escapes, newlines, bidi controls, `$(...)`, `%F{}` |
| Shells | Real interactive bash and zsh on a pseudo-terminal (as `nobody` when the tests run as root). They force every mechanic, then check `$?`, prompt and Up-key restore, hostile directory names, a rejected hostile ghost, ignorespace, confess, evict and unload, and that nothing reached the history file. Two more sessions run bash under `set -eu` with a folded `PROMPT_COMMAND`, and zsh with `ksharrays nounset globsubst warncreateglobal errexit`, a `%/` prompt and the user's own precmd hooks. |
| Sandbox | A child process under Landlock tries to read a file in `$HOME`, write outside state, read `/etc`, exec and open TCP, and all of them must fail |
| Policy | The source is parsed: no forbidden imports, `os` file functions only in `internal/audit`, `unsafe` only in `sandbox` and `term`, and the hook scripts contain no `eval` of output and no history writes |
