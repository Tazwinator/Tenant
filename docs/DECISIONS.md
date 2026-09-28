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

**D18. No dependencies at all; Landlock through raw syscalls (supersedes part of D1).**
`go-landlock` and `x/sys` turned out to be unnecessary. The sandbox is about 100
lines of `syscall` calls, the binary depends only on the standard library, and CI
checks that `go.mod` stays empty.

**D19. `prompt.glyph` shifts a vowel instead of using a homoglyph.**
Cyrillic look-alikes are invisible in most monospace fonts, which wastes the beat.
`rarepulls` → `rarepolls` is visible, deniable and pure ASCII, so the hook only ever
handles lowercase letters.

**D20. Optional beats wait for the chapter's required beats; stalled required beats
fire at their next chance.**
Simulation showed optional beats spending the budget and delaying the story. Now the
story comes first, and colour fills the wait between chapters.

**D21. The prototype phase was skipped (the maintainer's call).**
The full MVP was built straight away and verified by simulation and pty tests instead.
Living with it on Zireael moves to the pre-teaser week.

**D22. Prompts are changed through variable references, not text.**
In bash (and zsh with `PROMPT_SUBST`), the cwd token becomes `${_tenant_pwd}`, which
the shell expands without re-evaluating it. Hostile directory names can't reach
prompt expansion, so no escaping rules have to be got right.

**D23. The finale is only saved as finished when it reaches the end.**
Ctrl-C always exits at once with the terminal restored, and `tenant` starts the
conversation again.

**D24. Developer commands are gated behind `TENANT_DEV=1`, and forced beats never
count as story progress.**
`_force <mechanic> [text]` and `_invite` exist so the teaser can be recorded from the
real binary.

## Open questions

**Q2. What does Zireael's bash setup look like?**
v0.1 supports a `PS1` with `\w` or `\W`, readline's history functions on Up, GNU
`ls` or `eza` without icons, and any not-found handler. With starship, ble.sh or an
atuin Up key, the affected beats fall back to other mechanics. `tenant doctor`
reports what your shell can show. If Zireael needs an adapter, it goes here.

**Q6 (minor). Module path casing.**
The repo is `Tazwinator/Tenant`. Go module paths are case-sensitive, and a
lowercase `tenant` repo would make `go install` URLs nicer. It's cheap to rename now
and annoying later.

## Answered

Q1 was answered by D11, Q3 by D12, Q4 by D13 and Q5 by D14. Q2 was partly answered
by D15.
