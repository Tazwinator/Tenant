# Architecture

One static, CGO-free Go binary (`tenant`) and one short zsh hook script that the
binary prints (`tenant init zsh`). The hook does as little as possible: it hands
the binary a few facts on each prompt and carries out a small, fixed set of actions
the binary asks for. All decisions (whether anything happens, what, and when) are
made in the binary, inside a Landlock sandbox.

```
 zsh ── precmd / preexec / zle widget / command_not_found_handler
  │
  ▼
 hook.zsh  (shell builtins only)
  │  fast path: no ~/.local/state/tenant/active → return, no fork
  │
  ▼
 tenant _hook precmd --status N --cmd ARGV0 --shape SHAPE      (one fork per prompt)
  │  sandboxed: Landlock FS + net rules
  ├── state      lock, load, save        (~/.local/state/tenant)
  ├── observe    record cwd + argv0      (names only)
  ├── scheduler  seeded: fire a beat on this tick?
  ├── story      which beat, which text
  └── mechanics  carry out the beat
        ├── text for the player  → written straight to stderr (the terminal)
        └── shell actions        → stdout: "verb int int" lines, fixed vocabulary
  │
  ▼
 hook.zsh reads the action lines, dispatches with `case` (never `eval`)
```

## Components

Planned repo layout:

```
cmd/tenant/         CLI entry point and command dispatch
internal/hook/      the embedded zsh script and the hook protocol (verbs and args)
internal/sched/     seeded scheduler, sittings, chapter unlocks, haunting budget
internal/story/     story loader, beats, text templates
internal/mech/      one file per mechanic (ls.phantom, prompt.glyph, ...)
internal/observe/   what tenant learns: cwd, argv0, directory names
internal/audit/     the only package allowed to touch the filesystem; confess
internal/state/     state dir, flock, atomic writes, schema version
internal/sandbox/   Landlock setup and status reporting
internal/term/      output sanitising, colours (LS_COLORS), OSC title
story/              the Act 1 script (spoilers)
teaser/             tape file for the teaser clip
packaging/aur/      PKGBUILD
docs/
```

## The zsh hook

`eval "$(tenant init zsh)"` installs:

| Hook | Does |
|---|---|
| `preexec` | Classifies the command line in shell and keeps only its first word (`argv0`) and a shape token (for example `ls:plain`, `ls:long`, `other`). The full command line never leaves the shell. |
| `precmd` | Captures `$?`. If the `active` sentinel is missing, it unloads anything it changed and returns. Otherwise it runs `tenant _hook precmd`, then dispatches the returned actions. It always ends with `return $status` so `$?` is preserved for other hooks and the prompt. |
| zle widget | Wraps whatever Up-arrow is already bound to. It only does anything when a history ghost is armed (see [EVENTS](EVENTS.md#historyghost)). |
| `command_not_found_handler` | Only installed if the notfound mechanic is enabled. It wraps any existing handler (for example pkgfile on Arch) and always delegates to it. |

`precmd` is appended last in `precmd_functions`, so it runs after prompt frameworks
have built `PROMPT`.

## Hook protocol

The binary talks to the shell in two channels:

- **stderr** carries text for the player. The binary writes it straight to the
  terminal (it is not captured by `$(...)`). Every user-derived string is sanitised
  first (see [SECURITY](SECURITY.md#terminal-output)).
- **stdout** carries action lines. Each line is a verb from a fixed list, followed
  by integers only:

| Verb | Args | Shell does |
|---|---|---|
| `prompt-glyph` | `index codepoint` | Saves `PROMPT`, swaps one character of the cwd shown in it, escapes it, and restores it on the next precmd |
| `rprompt` | `slot` | Sets `RPROMPT=$(tenant _text rprompt SLOT)` for one render, escaped |
| `ghost-arm` | none | Arms the Up-arrow widget. The widget fills the buffer with `BUFFER=$(tenant _ghost)` |
| `unload` | none | Restores everything it saved, unbinds its widget and removes its hooks |

Text that the shell has to hold (the ghost command, RPROMPT text) goes into a
variable by command-substitution assignment. Assignment does not evaluate its
value, so there is never an `eval` of anything the binary prints at runtime.

## State

Everything lives under `$XDG_STATE_HOME/tenant` (default `~/.local/state/tenant`):

| File | Holds |
|---|---|
| `active` | Sentinel. Hooks are inert unless it exists. `evict` deletes it first. |
| `state.json` | Schema version, seed, mode (normal/compressed), start time, sittings, chapter, beats fired, budget counters |
| `observations.jsonl` | Timestamp, cwd, argv0, exit status. Pruned after the story ends. |
| `seen.jsonl` | Directory names listed, per directory. This is what `confess` prints. |
| `audit.log` | Append-only, human-readable log of every read and write |
| `lock` | `flock` target. Several shells (tmux panes) run hooks at once. |

Writes are atomic (temp file plus `rename`). An event is claimed under the lock, so
only one pane fires it, and it fires in the pane you just used.

## Performance budget

- **Inactive:** zero forks. The precmd fast path is a single `[[ -e ... ]]`.
- **Active:** one fork+exec per prompt. The target is p95 of 5 ms or less on
  Zireael. The binary also has an internal deadline of 25 ms: if it overruns, it
  does nothing and exits.
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
| Mechanics | Golden output tests per mechanic |
| zsh integration | A real interactive zsh under a pty in CI (the pty library is a test-only dependency). It checks hook install, unload, `$?` preservation and prompt restore. |
| Adversarial | Directory and file names such as `$(touch pwned)`, backticks, `%F{red}`, ANSI escapes, newlines and very long names, run through every mechanic |
| Sandbox | Under Landlock, attempts to read a file, write outside state, connect and exec all fail |
