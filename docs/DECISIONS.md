# Decisions

A short log of decisions and why they were made. When something changes, add a new
entry rather than rewriting an old one.

## Decided

**D1. Go, CGO-free, with minimal dependencies.**
Standard library, `go-landlock` and `x/sys` only. A single static binary like
RarePulls, which someone can audit in an afternoon.

**D2. Nothing happens without `tenant start`.**
Installing the hook isn't consent. Running `start` is. It also means evict can
simply delete state and leave the hook inert.

**D3. The hook never evals runtime output.**
The protocol is fixed verbs plus integers. Text reaches the shell only by assignment.
This removes the whole class of injection bugs that come from hostile filenames.

**D4. The history ghost uses a zle widget, not `print -s`.**
`print -s` gets persisted to `$HISTFILE`, which would break "wrote nothing". A
widget that pre-fills one press of Up gives the same scare and writes nothing.
(D15 extends this to bash: `bind -x` and `READLINE_LINE`, never `history -s`.)

**D5. Never wrap your commands.**
Effects are added around commands (before the prompt, in the prompt, in the title),
never in place of them. `ls.phantom` prints after `ls` has finished. This keeps the
effects removable and trustworthy, and means aliases and exotic `ls` setups can't
break.

**D6. The story advances by sittings, not calendar days.**
This way it can't stall for occasional users and can't be binged by heavy ones.
Details are in [SCHEDULER](SCHEDULER.md).

**D7. You summon the finale by typing `tenant`.**
It happens when you choose, not in the middle of a `git push`. It's still a
confrontation you type out.

**D8. `evict` is out of character and deletes everything.**
A safeword that gets played for a scare isn't a safeword. If you want the log, run
`confess` first.

**D9. `confess` is never dramatised.**
The story may only add text after the log. The log's honesty is both the security
feature and the ending, so it can't bend.

**D10. State lives in `$XDG_STATE_HOME/tenant`, and the story is embedded.**
The binary carries Act 1. Story packs (later) are data files that can only reference
audited mechanics.

**D11. Never read the history file (answers Q1).**
It learns only by watching during dormancy. "Never opens a file of yours" is absolute
and kernel-enforced. We give up "it knows your past" beats. Commands started with a
space are invisible to it where the shell honours `ignorespace`.

**D12. Claude drafts Act 1, and the maintainer edits (answers Q3).**
The draft goes in `story/`, following the rules in [STORY](STORY.md#writing-rules).

**D13. Aim for Halloween 2026 (answers Q4).**
The dates in the [roadmap](ROADMAP.md) hold. If the teaser kill check fails, we park
it until October 2027.

**D14. MIT for the whole repo, including Act 1 (answers Q5).**
Paid story content will be distributed separately in the future, under its own terms,
and never lives in this repo.

**D15. Bash first, then zsh, both before launch (partly answers Q2).**
Zireael runs bash, and the weekend prototype must be something the maintainer lives
with. zsh follows in the MVP phase, for the r/unixporn crowd and macOS. The minimum is
bash 5.1, for array `PROMPT_COMMAND`.

**D16. Terminal first; no GUI before launch, and never a second app.**
The reasoning is in [PLATFORMS](PLATFORMS.md). The engine and surface split keeps
the door open: any future desktop, macOS or Windows support is a new surface in the
same binary.

**D17. Five terminal-using testers, recruited from outside the friend group.**
The maintainer's friends mostly don't use terminals, so the kill test recruits from
the teaser's "want to test" issue and Linux communities instead.

## Open questions

**Q2. What does Zireael's bash setup look like?**
Is it a plain `PS1`, starship, oh-my-bash or bash-it? Does it use ble.sh, atuin,
fzf's Ctrl-R or history bindings, or bash-preexec? Is `ls` aliased to `eza`? This
decides which adapters the weekend prototype needs first.

**Q6 (minor). Module path casing.**
The repo is `Tazwinator/Tenant`. Go module paths are case-sensitive, and a
lowercase `tenant` repo would make `go install` URLs nicer. It's cheap to rename now
and annoying later.

## Answered

Q1 was answered by D11, Q3 by D12, Q4 by D13 and Q5 by D14. Q2 was partly answered
by D15.
