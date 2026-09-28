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

## Open questions

**Q1. Should it read `$HISTFILE`?**
The recommendation is **no**: it learns only by watching during dormancy. That makes
"never opens a file of yours" absolute and enforceable by the kernel. The cost is
losing "it knows your past" beats, such as "you haven't run `make` since March".
The docs currently assume no.

**Q2. Which prompt and plugins does Zireael run?**
This decides which `prompt.glyph` and `history.ghost` adapters the weekend
prototype needs first: plain `%~`, grml, oh-my-zsh, pure, starship or p10k, and
whether zsh-autosuggestions or history-substring-search is in use. It also matters
whether `ls` is aliased to `eza` or `lsd`.

**Q3. Who writes the story?**
The writing is the moat. Either you write the script and I build the engine and the
tooling, or I draft Act 1 from the premise in [STORY](STORY.md) for you to rewrite.

**Q4. Are we aiming for Halloween 2026?**
It's 33 days away, and the [roadmap](ROADMAP.md) dates assume yes. It's tight but
realistic for the MVP cut. The alternative is to build without a deadline and launch
in October 2027.

**Q5. What licence, and how do we split paid content?**
The suggestion is GPL-3.0 or MIT for the engine, and Act 1 under CC BY-NC-SA, all in
this repo. The paid full story would live outside the repo as a data-only pack.

**Q6 (minor). Module path casing.**
The repo is `Tazwinator/Tenant`. Go module paths are case-sensitive, and a
lowercase `tenant` repo would make `go install` URLs nicer. It's cheap to rename now
and annoying later.
