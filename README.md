# tenant

> Something has been living in your shell. It can only read names.

`tenant` is a slow-burn horror story that plays out over about a week inside your
real shell prompt and login message. It uses your real directory names and the
commands you actually run, which is what makes it land. It runs in bash and zsh on
Linux.

It never reads the contents of your files, including your shell history. It never
writes anywhere except its own state directory. It never touches the network. At
any time, `tenant confess` prints everything it has ever looked at, and `tenant
evict` removes it cleanly.

```
~/code/rarepulls $ ls
cmd/  internal/  web/  go.mod  go.sum  still_here.txt
~/code/rarepulls $ ls
cmd/  internal/  web/  go.mod  go.sum
~/code/rarepulls $ tenant confess | tail -2
2026-10-08 23:41  observed 412 commands (names only)
2026-10-08 23:41  wrote nothing
```
<sub>An illustration of the mechanic, not a real transcript.</sub>

**Status: planning.** Nothing here runs yet. It is aiming for Halloween 2026. See
[the roadmap](docs/ROADMAP.md).

## How it plays

1. Install it, add one line to your shell's rc file, and run `tenant start`.
2. For a day or two, nothing happens.
3. Then, rarely and on a seeded schedule, the terminal starts doing small wrong
   things. A prompt that's one character off. An `ls` entry that isn't there when
   you look again. A login message that knows where you were last night.
4. Over five sittings the story escalates, and it ends in a conversation you type out.
5. The last act is `tenant confess`: the true and complete audit log.

## The two commands to remember

| Command | What it does |
|---|---|
| `tenant evict` | The safeword. Stops everything in every open shell straight away and deletes all of its state. Always out of character. |
| `tenant confess` | Prints every directory name it listed, every command name it observed, and every write it made outside its own state directory (there should never be any). |

## Install (planned)

```sh
# Arch (AUR, builds from source)
yay -S tenant

# anywhere with Go 1.24+
CGO_ENABLED=0 go install github.com/Tazwinator/Tenant/cmd/tenant@latest
```

Add one line as the last line of your rc file. You can read what it does first with
`tenant init bash | less`.

```sh
eval "$(tenant init bash)"   # ~/.bashrc  (bash 5.1+)
eval "$(tenant init zsh)"    # ~/.zshrc
```

```sh
tenant start               # nothing happens until you run this
tenant start --compressed  # the whole arc in about 30 minutes (streamers, reviewers)
```

## Commands

| Command | What it does |
|---|---|
| `tenant init bash\|zsh` | Prints the hook script. It is short, uses only shell builtins, and does nothing until you run `tenant start`. |
| `tenant start [--compressed]` | Your consent. Creates the seed and state, then goes quiet. |
| `tenant evict` | Safeword. See above. |
| `tenant confess [--summary]` | The audit log. See above. |
| `tenant doctor` | Spoiler-free checks: hook installed, prompt compatibility, sandbox status, hook latency. |
| `tenant help` | Always prints help. |

## What it can and can't do

**Can:** list the names in directories under `$HOME` that you `cd` into; note the
current directory and the first word of each command you run; briefly change your
prompt or window title; print a line before your prompt; pre-fill a single press
of the Up arrow.

**Can't:** read file contents (it never opens your history file either), write
outside `~/.local/state/tenant`, open network connections, run other programs, or
wrap, alias or replace your commands. On Linux with Landlock (kernel 5.13+) the file
and network rules are enforced by the kernel, not just promised. The details are in
[SECURITY.md](docs/SECURITY.md).

**Doesn't see:** commands you start with a space, if your shell is set to leave
those out of history (`HISTCONTROL=ignorespace` or `setopt HIST_IGNORE_SPACE`).

**Stays quiet:** in root shells, in any shell with `TENANT_OFF=1`, outside `$HOME`,
and in the middle of a git rebase.

## Documentation

| Doc | Covers |
|---|---|
| [ARCHITECTURE](docs/ARCHITECTURE.md) | The engine, the shell hooks, the hook protocol, state, performance |
| [SCHEDULER](docs/SCHEDULER.md) | Seeded pacing, sittings, the haunting budget, compressed mode |
| [EVENTS](docs/EVENTS.md) | Every "wrong thing", and exactly how each one is done in bash and zsh |
| [SECURITY](docs/SECURITY.md) | Threat model, invariants, Landlock sandbox, how to verify it yourself |
| [STORY](docs/STORY.md) | Story structure and writing rules. Light spoilers; the script lives in `story/` |
| [PLATFORMS](docs/PLATFORMS.md) | Why terminal first, and how macOS, Windows or a desktop version would fit later |
| [ROADMAP](docs/ROADMAP.md) | Phases, the teaser test, kill criteria, launch, money |
| [DECISIONS](docs/DECISIONS.md) | Decision log and open questions |

## Licence

[MIT](LICENSE), including the free Act 1 story. Future paid story content will be
distributed separately under its own terms.
