# Architecture

One static, CGO-free Go binary (`tenant`) and one short hook script per shell,
which the binary prints (`tenant init bash`, `tenant init zsh`). The hook does as
little as possible: on each prompt it hands the binary a few facts, then carries out
a small, fixed set of actions the binary asks for. All decisions (whether anything
happens, what, and when) are made in the binary, inside a Landlock sandbox.

## Engine and surfaces

The binary is split into two layers:

- **The engine:** scheduler, story, observations, audit log and sandbox. It knows
  nothing about shells.
- **Surfaces:** the places the entity can appear. A surface turns an engine
  decision into something the player sees. Today the only surfaces are the bash
  and zsh hooks.

This split is what lets a future surface (fish, PowerShell, or a desktop surface)
be added to the **same binary**, rather than becoming a second app. See
[PLATFORMS](PLATFORMS.md).

```
 bash: PROMPT_COMMAND, bind -x, command_not_found_handle
 zsh:  precmd, preexec, zle widget, command_not_found_handler
  │
  ▼
 hook script  (shell builtins only)
  │  fast path: no ~/.local/state/tenant/active → return, no exec
  │
  ▼
 tenant _hook prompt --shell bash --status N --cmd ARGV0 --shape SHAPE  (one exec per prompt)
  │  sandboxed: Landlock FS + net rules
  ├── state      lock, load, save        (~/.local/state/tenant)
  ├── observe    record cwd + argv0      (names only)
  ├── scheduler  seeded: fire a beat on this tick?
  ├── story      which beat, which text
  └── mechanics  carry out the beat, through the shell's surface adapter
        ├── text for the player  → written straight to stderr (the terminal)
        └── shell actions        → stdout: "verb int int" lines, fixed vocabulary
  │
  ▼
 hook script reads the action lines, dispatches with `case` (never `eval`)
```

## Components

Planned repo layout:

```
cmd/tenant/           CLI entry point and command dispatch
internal/shell/       hook protocol (verbs and args), shared by all shells
internal/shell/bash/  the embedded bash hook script and its probes
internal/shell/zsh/   the embedded zsh hook script and its probes
internal/sched/       seeded scheduler, sittings, chapter unlocks, haunting budget
internal/story/       story loader, beats, text templates
internal/mech/        one file per mechanic (ls.phantom, prompt.glyph, ...)
internal/observe/     what tenant learns: cwd, argv0, directory names
internal/audit/       the only package allowed to touch the filesystem; confess
internal/state/       state dir, flock, atomic writes, schema version
internal/sandbox/     Landlock setup and status reporting
internal/term/        output sanitising, colours (LS_COLORS), OSC title
story/                the Act 1 script (spoilers)
teaser/               tape file for the teaser clip
packaging/aur/        PKGBUILD
docs/
```

## Shell hooks

Each hook does the same four jobs, using that shell's own mechanisms:

| Job | bash (5.1+) | zsh |
|---|---|---|
| Capture the last command's status | `_tenant_status`, **prepended** to the `PROMPT_COMMAND` array, so it sees `$?` before anything else runs | the first line of the precmd function |
| Learn the last command's first word | in the shell, from the single in-memory entry `history 1` (never `$HISTFILE`). An unchanged `HISTCMD` means no new command. | `preexec` argument `$1` |
| Run each prompt | `_tenant_prompt`, **appended** to `PROMPT_COMMAND`, so it runs after prompt frameworks have built `PS1` | appended last to `precmd_functions` |
| Up-arrow ghost | `bind -x` on Up, only while armed; it restores the previous binding after one press | zle widget wrapping the existing Up widget |
| Typo remark | wraps `command_not_found_handle` | wraps `command_not_found_handler` |

In both shells:

- Only the first word (`argv0`) and a shape token (for example `ls:plain`,
  `ls:long`, `other`) leave the shell. The full command line never reaches the
  binary.
- A command that starts with a space is ignored if the shell is set to keep those
  out of history. On bash this happens naturally, because the command never
  reaches `history 1`. On zsh it applies when `HIST_IGNORE_SPACE` is set.
- The prompt hook always ends by returning the status it received, so `$?` is
  preserved.

## Hook protocol

The binary talks to the shell in two channels:

- **stderr** carries text for the player. The binary writes it straight to the
  terminal (it is not captured by `$(...)`). Every user-derived string is sanitised
  first (see [SECURITY](SECURITY.md#terminal-output)).
- **stdout** carries action lines. Each line is a verb from a fixed list, followed
  by integers only:

| Verb | Args | Shell does |
|---|---|---|
| `prompt-glyph` | `index codepoint` | Saves the prompt, swaps one character of the cwd shown in it, escapes it, and restores it on the next prompt |
| `prompt-time` | `slot` | Shows a right-aligned story time for one render. zsh uses `RPROMPT`. bash uses a zero-width right-aligned prefix on `PS1`. The text is fetched with `$(tenant _text time SLOT)` and escaped. |
| `ghost-arm` | none | Arms the Up-arrow ghost. On the next press, the line is filled from `$(tenant _ghost)`. |
| `unload` | none | Restores everything it saved, unbinds its keys and removes its hooks |

Text that the shell has to hold (the ghost command, the time text) goes into a
variable by command-substitution assignment. Assignment does not evaluate its
value, so there is never an `eval` of anything the binary prints at runtime.

## State

Everything lives under `$XDG_STATE_HOME/tenant` (default `~/.local/state/tenant`):

| File | Holds |
|---|---|
| `active` | Sentinel. Hooks are inert unless it exists. `evict` deletes it first. |
| `state.json` | Schema version, seed, mode (normal/compressed/tester), start time, sittings, chapter, beats fired, budget counters |
| `observations.jsonl` | Timestamp, shell, cwd, argv0, exit status. Pruned after the story ends. |
| `seen.jsonl` | Directory names listed, per directory. This is what `confess` prints. |
| `audit.log` | Append-only, human-readable log of every read and write |
| `lock` | `flock` target. Several shells (tmux panes, bash and zsh side by side) run hooks at once. |

Writes are atomic (temp file plus `rename`). An event is claimed under the lock, so
only one pane fires it, and it fires in the pane you just used.

## Performance budget

- **Inactive:** no exec. The prompt fast path is a single `[[ -e ... ]]`.
- **Active:** one exec of the binary per prompt. bash also forks once for `history
  1` (bash 5.3's `${ ...; }` substitution avoids the fork where available). The
  target is p95 of 5 ms or less on Zireael. The binary also has an internal deadline
  of 25 ms: if it overruns, it does nothing and exits.
- `tenant doctor` runs the hook 100 times and reports p50 and p95.

## Failure policy

The shell must never break. Every error means "do nothing, silently" in the hook,
and a line in `audit.log` for the binary. A missing or broken binary makes the hook
unload itself, not print errors.

## Build and dependencies

- `CGO_ENABLED=0 go build -trimpath ./cmd/tenant`, Go 1.24+.
- Runtime dependencies: the standard library, `github.com/landlock-lsm/go-landlock`
  and `golang.org/x/sys`. That's all.
- CI enforces that the binary imports no `net`, `net/http` or `os/exec`, and that
  only `internal/audit` calls the `os` file functions.

## Testing

| Layer | How |
|---|---|
| Scheduler | Pure functions over a fake clock. Simulation profiles (daily, evenings-only, weekend, tmux with six panes) assert the pacing targets in [SCHEDULER](SCHEDULER.md#tuning-targets). |
| Mechanics | Golden output tests per mechanic, per shell |
| Shell integration | Real interactive bash and zsh under a pty in CI (the pty library is a test-only dependency). It checks hook install, unload, `$?` preservation, prompt restore, and coexistence with bash-preexec, starship and oh-my-zsh. |
| Adversarial | Directory and file names such as `$(touch pwned)`, backticks, `\w`, `%F{red}`, ANSI escapes, newlines and very long names, run through every mechanic in both shells |
| Sandbox | Under Landlock, attempts to read a file, write outside state, connect and exec all fail |
