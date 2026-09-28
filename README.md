# tenant

> Something has been living in your shell. It can only read names.

`tenant` is a slow-burn horror story that plays out over about a week inside your
real shell prompt and login message. It uses your real directory names and the
commands you actually run, which is what makes it land. It runs in bash (5.1+) and
zsh on Linux.

It never reads the contents of your files, including your shell history. It never
writes anywhere except its own state directory. It never touches the network, and it
never runs another program. On Linux the kernel enforces all of that, not just a
promise. At any time, `tenant confess` prints everything it has ever looked at, and
`tenant evict` removes it cleanly.

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

**Status: v0.1, feature-complete for Act 1, heading for Halloween 2026.** See
[the roadmap](docs/ROADMAP.md).

## How it plays

1. Install it, add one line to your shell's rc file, and run `tenant start`.
2. For a day or two, nothing happens.
3. Then, rarely, the terminal starts doing small wrong things. A prompt that's one
   letter off. An `ls` entry that isn't there when you look again. A login message
   that knows where you were last night.
4. Over five chapters it escalates, and it ends in a conversation you type out.
5. The last act is `tenant confess`: the true and complete audit log.

## The two commands to remember

| Command | What it does |
|---|---|
| `tenant evict` | The safeword. Every shell switches the hook off at its next prompt, and all of tenant's state is deleted. Always out of character. |
| `tenant confess` | Every directory name it read, every command name it counted, and every write it made outside its own directory (there are none). |

## Install

```sh
# from source, anywhere with Go 1.24+
CGO_ENABLED=0 go install github.com/Tazwinator/Tenant/cmd/tenant@latest

# Arch: AUR package `tenant` (built from source), coming with the first release
```

Add one line as the last line of your rc file. You can read what it does first with
`tenant init bash | less`.

```sh
eval "$(tenant init bash)"   # ~/.bashrc
eval "$(tenant init zsh)"    # ~/.zshrc
```

Then start it:

```sh
tenant start               # nothing happens until you run this
tenant start --compressed  # the whole story in about 30 to 45 minutes (streamers, reviewers)
```

## Commands

| Command | What it does |
|---|---|
| `tenant init bash\|zsh` | Prints the hook. It uses only shell builtins and does nothing until you start. |
| `tenant start [--compressed]` | Your consent. Creates the seed and state, then goes quiet. |
| `tenant evict` | The safeword. |
| `tenant confess [--summary]` | The audit log. |
| `tenant doctor` | Spoiler-free checks: hook loaded, what your prompt can show, sandbox status, latency. |
| `tenant help` | Help. Just `tenant`, near the end, is something else. |

## What it can and can't do

**Can:** list the names in directories under `$HOME` that you visit; note your
current directory and the first word of each command; change your prompt or window
title for one prompt; print a line before your prompt; pre-fill one press of the Up
arrow.

**Can't:** read file contents (your history file included), write outside
`~/.local/state/tenant`, open network connections, run other programs, or wrap,
alias or replace your commands. With Landlock (Linux 5.13+) the kernel enforces the
file, network and exec rules. See [SECURITY](docs/SECURITY.md).

**Doesn't see:** commands you start with a space, if your shell keeps those out of
history (`HISTCONTROL=ignorespace`, `setopt HIST_IGNORE_SPACE`).

**Stays quiet:** as root, in shells with `TENANT_OFF=1`, outside `$HOME`, and in the
middle of a git rebase, merge or bisect.

## Development

```sh
CGO_ENABLED=0 go build -o tenant ./cmd/tenant
go test ./...                                   # integration tests need bash and zsh; -short skips them
TENANT_DEV=1 ./tenant _sim --profile=daily      # the whole story on a fake clock (spoilers)
```

It uses no dependencies beyond the Go standard library. Start with
[ARCHITECTURE](docs/ARCHITECTURE.md). [CLAUDE.md](CLAUDE.md) lists the rules that must
never break.

## Documentation

| Doc | Covers |
|---|---|
| [ARCHITECTURE](docs/ARCHITECTURE.md) | Engine and surfaces, the hooks, the protocol, state, performance, testing |
| [SCHEDULER](docs/SCHEDULER.md) | Sittings, chapters, the haunting budget, paces, tuning results |
| [EVENTS](docs/EVENTS.md) | Every "wrong thing", and exactly how it's done in bash and zsh |
| [SECURITY](docs/SECURITY.md) | Invariants, the Landlock sandbox, the audit log, how to verify it yourself |
| [STORY](docs/STORY.md) | Structure, writing rules, the beat format. Light spoilers; the script is in `story/` |
| [PLATFORMS](docs/PLATFORMS.md) | Why terminal first, and how macOS, Windows or a desktop version would fit |
| [ROADMAP](docs/ROADMAP.md) | What's done, the teaser test, kill criteria, launch, money |
| [DECISIONS](docs/DECISIONS.md) | Decision log and open questions |

## Licence

[MIT](LICENSE), including the Act 1 story. Future paid story content will be
distributed separately under its own terms.
