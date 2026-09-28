# Roadmap

We're aiming to launch on Halloween 2026 (D13). The prototype phase was skipped: the
full MVP was built directly.

## Done: the MVP (v0.1)

- [x] Plan, README and design docs
- [x] bash hook (5.1+) and zsh hook, both with every mechanic, unload and `$?` preserved
- [x] Scheduler: seed, sittings, chapters, guards, hazard, fallbacks, stall protection, three paces
- [x] All eight mechanics: `ls.phantom`, `prompt.glyph`, `history.ghost`, `motd.lastlogin`,
      `clear.residue`, `notfound.remark`, `title.whisper`, `prompt.time`
- [x] Act 1 draft: five chapters, the finale conversation, the epilogue
- [x] `start`, `evict`, `confess`, `doctor`, `init`, and the finale
- [x] Landlock sandbox with raw syscalls (files, TCP, scopes)
- [x] Tests: pacing simulation, story validation, hostile names, bash and zsh on a pty,
      a kernel sandbox check, and source-policy checks
- [x] CI, a release workflow (static amd64 and arm64 binaries plus `SHA256SUMS`), the AUR `PKGBUILD`
- [x] Teaser tape (`teaser/teaser.tape`), recorded from the real binary

## Next: before the teaser (this week)

- [ ] The maintainer reads and edits `story/act1/` (D12)
- [ ] Install on Zireael and live with it at normal pace, without reading the schedule
- [ ] Check the name `tenant` for collisions on the AUR, in Arch repos and on GitHub
- [ ] Decide Q6 (module path casing) before anyone runs `go install`

## Teaser (about Oct 6)

- [ ] Record `teaser/teaser.tape` with VHS. Trim it to 90 seconds.
- [ ] Post to r/unixporn and r/linux with a repo link and a pinned "notify me / want
      to test" issue
- [ ] **Kill check:** fewer than 50 stars and upvotes combined means we park it until
      next October

## Two-week kill test (Oct 19–25)

- [ ] Five **terminal-using** testers, recruited from the teaser issue, run it for three
      days with `tenant start --pace=tester`
- [ ] **Pass:** at least 3 of the 5 report a moment that actually unsettled them.
      Otherwise the format doesn't work, and we stop.
- [ ] Fix what they hit. Tune with `_sim` if their pacing reports disagree with the
      simulation. Nothing is uploaded automatically, ever.

## Launch (Oct 31)

- [ ] Tag `v0.1.0` (the release workflow builds binaries). Fill in `sha256sums` in the
      PKGBUILD, generate `.SRCINFO`, and publish to the AUR.
- [ ] Posts: r/unixporn, r/linux, r/commandline, Show HN, and horror streamers
      (pitch compressed mode)

## After launch

- **The platform question, with evidence** (see [PLATFORMS](PLATFORMS.md))
- fish support
- Prompt adapters for starship and powerlevel10k, so `prompt.glyph` works there too
- An inline `ls.phantom` that reproduces GNU `ls` column layout
- The paid full story on itch.io, as a data-only story pack
- Branching endings

## Risks

| Risk | Mitigation |
|---|---|
| Terminal people refuse to hook their shell | The security design is the pitch: Landlock, `confess`, a short readable hook, `evict`, and tests anyone can run |
| A packager or AV tool flags it | No dependencies, source-only AUR build, reproducible static builds, no obfuscation, a hook you add yourself |
| A week-long slow burn loses players | Sittings rather than days, stall protection, and compressed mode |
| The writing is the moat | Act 1 is short and specific. The kill test measures it directly. |
| Scares land as cute | The writing rules in [STORY](STORY.md#writing-rules). If the kill test fails, we stop. |
| Scope creep into GUI and other platforms | Not before launch (D16) |

## Money, honestly

Act 1 is free and MIT-licensed. The full story is about $5 on itch.io, benchmarked
against KinitoPET at $5.99, and distributed separately. The terminal audience is far
smaller than the Windows one, so expect pocket money. Halloween is a free launch
hook every year, but it isn't the reason to build this.
