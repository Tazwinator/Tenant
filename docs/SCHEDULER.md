# Scheduler

The scheduler decides, on every prompt, whether something wrong happens. It has to
feel rare and unpredictable to the player, and be fully deterministic for testing.
The code is in `internal/sched` (pure functions) and `internal/engine` (the tick).

## Goals

- **Rare.** Early events are few and deniable.
- **Never clustered.** Two events close together read as a gimmick.
- **About a week** for someone who uses a terminal daily.
- **Never stalls** for someone who only uses it on some evenings or at weekends.
- **Reproducible.** The same seed and the same usage give the same haunting.

## Seed and randomness

`tenant start` draws a 64-bit seed from `crypto/rand`. Every roll is
`splitmix64(fnv(seed, sitting, prompt in sitting, total prompts, purpose))`, so there
is no hidden state beyond `state.json`.

## Story clock: sittings

A **sitting** is a stretch of terminal use. A new one starts at the first prompt
after the idle gap, or after the daily roll hour (05:00 local). Sittings are shared
by every open shell. An **opportunity** is a prompt that follows a command, the
first prompt of a new shell, or a mistyped command. Empty Enter presses don't count.

Chapters unlock only at the start of a sitting:

| Chapter | Unlocks when |
|---|---|
| 0 Dormancy | At `tenant start`. Nothing fires. It is watching. |
| 1 | Dormancy time has passed, **and** more than the dormancy sittings have gone by |
| 2 to 5 | The chapter gap has passed since the previous chapter unlocked, **and** its required beats have all fired |

## Within a chapter

1. **Guards.** Nothing fires unless the chapter is at least 1, the sitting has
   settled, the minimum gap (in time and prompts) has passed, the sitting budget
   isn't spent, and the daily cap isn't reached. It also stays quiet outside `$HOME`
   and during a git rebase, merge or bisect.
2. **Candidates.** While the chapter has required beats left, only the **next
   required beat** can fire: the story comes first. Once they have all fired,
   **optional beats** fill the wait for the next chapter, in a seeded order. A beat is
   a candidate when its mechanic's trigger just happened and its probe passes.
3. **Fallbacks.** A beat switches to its fallback mechanic when this shell can never
   show the primary one (the probe fails), or when a required beat's chapter has
   **stalled** (run for the stall number of sittings).
4. **The roll.** `chance = PBase × (1 + K × quiet prompts) × weight`, capped at
   `PMax`. The weight comes from the rarest trigger among the candidates (ls ×4, a
   new shell ×5, clear ×6, a typo ×8), so rare moments aren't wasted. A stalled
   required beat skips the roll and fires at its next chance.

## Paces

| | normal | tester | compressed |
|---|---|---|---|
| Dormancy | 36 h and 2 sittings | 12 h and 1 sitting | 2 min |
| New sitting | 3 h idle, or 05:00 | 2 h idle, or 05:00 | 3 min idle, or every 3 min |
| Chapter gap | 16 h | none | 2 min |
| Minimum gap | 20 min and 10 prompts | 10 min and 5 prompts | 40 s and 3 prompts |
| Settle | 3 prompts | 2 | 1 |
| Budget per sitting | 2 | 2 | 3 |
| Daily cap | 3 | 6 | none |
| Hazard PBase, K, PMax | 0.03, 0.02, 0.5 | 0.05, 0.03, 0.45 | 0.15, 0.12, 0.75 |
| Stall after | 2 sittings | 2 | 2 |

`tenant start --compressed` or `--pace=tester` picks one. `tenant doctor` shows which
is active.

## Tuning targets

`TestPacingTargets` asserts these across 12 seeds each. The measured ranges come
from 10 seeds.

| Profile | Target | Measured |
|---|---|---|
| Daily (4 sittings a day) | invited within 5 to 8 days | 6.0 to 7.4 days |
| Evenings only (1 sitting a day) | within 14 days | 7.5 to 12.5 days |
| Weekends only | within 21 days (3 weekends) | 16 to 17 days |
| Compressed, steady use | 25 to 45 minutes | 32 to 43 minutes |

The tests also check: nothing during dormancy, never two events within the minimum
gap, never more than the daily cap, every required beat fires in order, and two runs
with one seed are identical.

## Developer tools (spoilers)

These only work with `TENANT_DEV=1`. Forced beats never count as story progress.

| Command | Does |
|---|---|
| `tenant _sim --profile=daily\|evenings\|weekend\|steady [--pace=…] [--seed=N]` | Prints a whole simulated run. It never touches the filesystem. |
| `tenant _schedule` | Shows the chapter, sitting and beats fired |
| `tenant _force <mechanic> [text]` | Fires a mechanic at its next trigger, ignoring guards, optionally with exact text. This is how the teaser is recorded. |
| `tenant _invite` | Lets `tenant` open the finale now |
