# Roadmap

Today is 2026-09-28. Halloween is 33 days away. The dates below assume we aim for
it (open question Q4 in [DECISIONS](DECISIONS.md)). If we don't, keep the order and
drop the dates.

## Phase 0: Plan (this week)

- [x] README and design docs
- [ ] Answer the open questions in [DECISIONS](DECISIONS.md)
- [ ] Check the name `tenant` for collisions on the AUR, in Arch repos and on GitHub

## Phase 1: Weekend prototype (Oct 3–4)

The target is the brief's "this weekend": enough to live with.

- [ ] `tenant init zsh`: precmd and preexec hooks, fast path, unload
- [ ] State dir, flock, atomic writes, `active` sentinel
- [ ] Scheduler core: seed, sittings, guards, hazard, fake clock, first simulation test
- [ ] Mechanics: `ls.phantom`, `prompt.glyph`, `history.ghost`
- [ ] `start`, `evict`, and a basic `confess`
- [ ] `_force` (dev only)
- [ ] **Exit:** install on Zireael and live with it for a week without reading the
      schedule

## Phase 1b: Teaser (in parallel, posted about Oct 6)

The cheapest test from the brief.

- [ ] A 90-second clip of "the prompt goes wrong". Record it from the real
      prototype using `_force` and a VHS tape file in `teaser/`. If the prototype
      isn't ready, fall back to a hand-scripted fake session.
- [ ] Post to r/unixporn and r/linux with a repo link and a pinned "notify me" issue
- [ ] **Kill check:** fewer than 50 stars and upvotes combined means we park it
      until next October

## Phase 2: MVP (Oct 5–18)

- [ ] The remaining mechanics: `motd.lastlogin`, `clear.residue`,
      `notfound.remark`, `title.whisper`, `rprompt.time`
- [ ] Story loader and template variables. Act 1 script: five sittings, about 20
      minutes of content.
- [ ] `finale.summon` dialogue and `confess.epilogue`
- [ ] Compressed mode and the `--pace=tester` preset
- [ ] Landlock sandbox, and `doctor` (probes, sandbox, latency)
- [ ] Complete `confess` and the audit log
- [ ] CI: build, tests, zsh-under-pty integration, adversarial names, import checks
- [ ] AUR `PKGBUILD` (source build) and a GitHub release with checksums

## Phase 3: Two-week kill test (Oct 19–25)

- [ ] Five terminal-using friends run it for three days on `--pace=tester`
- [ ] **Pass:** at least 3 of the 5 report at least one moment that actually
      unsettled them. Otherwise the format doesn't work, and we stop.
- [ ] Fix what they hit. Tune pacing from their `_sim`-style traces, if they're
      willing to share them. Nothing is uploaded automatically, ever.

## Phase 4: Launch (Oct 31)

- [ ] v0.1.0 on the AUR and GitHub
- [ ] Posts: r/unixporn, r/linux, r/commandline, Show HN, and horror streamers
      (pitch compressed mode)

## Later

- bash (`PROMPT_COMMAND` plus a DEBUG-trap preexec) and fish (`fish_prompt` and
  `fish_preexec` events)
- A paid full story on itch.io as a data-only story pack
- Branching endings
- An inline `ls.phantom` that reproduces GNU `ls` column layout
- Adapters for starship and powerlevel10k, if they're not already in the MVP

## Risks

| Risk (from the brief) | Mitigation |
|---|---|
| Terminal people refuse to hook their shell | The security design is the pitch: Landlock, `confess`, a short readable hook, `evict`. The README leads with it. |
| A packager or AV tool flags it | Source-only AUR build, reproducible builds, no obfuscation, a hook you add yourself |
| A week-long slow burn loses players | Sittings rather than days, stall protection, and compressed mode for people who want it now |
| The writing is the moat | Keep Act 1 short and very specific. The friends test measures it directly. |
| Scares land as cute | The writing rules in [STORY](STORY.md#writing-rules). If the kill test fails, we stop. |

## Money, honestly

Act 1 is free and open source. The full story is about $5 on itch.io, benchmarked
against KinitoPET at $5.99. The terminal audience is far smaller than the Windows
one, so expect pocket money. Halloween is a free launch hook every year, but it
isn't the reason to build this.
