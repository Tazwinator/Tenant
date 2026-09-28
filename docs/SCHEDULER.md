# Scheduler

The scheduler decides, on every prompt, whether something wrong happens. It has to
feel rare and unpredictable to the player, but be fully deterministic for testing.

## Goals

- **Rare.** Early events are few and deniable. You should half-believe you misread.
- **Never clustered.** Two events close together read as a gimmick, not a haunting.
- **About a week** for someone who uses a terminal daily.
- **Never stalls** for someone who uses it on some evenings only.
- **Reproducible.** The same seed and the same usage give the same haunting.

## Seed and randomness

`tenant start` draws a 64-bit seed from `crypto/rand` and stores it in `state.json`.
Every random choice is derived from it:

```
roll = PCG(hash(seed, sitting_index, prompt_index_in_sitting, purpose))
```

There is no hidden state beyond `state.json`. A fake clock plus a recorded usage
trace replays any run exactly.

## Story clock: sittings, not days

A **sitting** is a stretch of terminal use. A new sitting starts at the first
prompt after 3 or more idle hours, or the first prompt after 05:00 local time.
Sittings are global across all open shells.

The story moves in chapters. A chapter unlocks at the start of a sitting:

| Chapter | Unlocks when |
|---|---|
| 0 Dormancy | At `tenant start`. Nothing ever fires. It is watching and learning your names. |
| 1 | 36 h or more since start, **and** at least 2 sittings since start |
| 2 to 5 | A new sitting, **and** 16 h or more since the previous chapter unlocked, **and** the previous chapter's required beats have fired |

Using sittings rather than calendar days means a weekend-only user still reaches the
end, and a heavy user can't burn through it in one afternoon.

## Within a sitting

Each prompt, in any shell, is an opportunity. An opportunity can fire only if every guard passes:

| Guard | Value (tunable) |
|---|---|
| Not in the first prompts of a sitting | 3 prompts |
| Minimum gap since the last event | 20 min **and** 10 prompts |
| Per-sitting budget | ch1: 1, ch2: 2, ch3: 2, ch4: 2, ch5: the finale invitation only |
| Daily cap (across sittings) | 3 |
| A beat is eligible | Its trigger matches (for example, the last command was `ls`) and its mechanic's capability probe passes |

If the guards pass, the roll is compared with a hazard that rises with quiet time:

```
p = min(p_max, p_base * (1 + k * prompts_since_last_event))
```

`p_base`, `k` and `p_max` are tuned by simulation, not by feel.

## Beat selection

- Each chapter has **required** beats (story-critical, in order) and **optional**
  beats (colour, shuffled by the seed).
- **Stall protection:** if a chapter hasn't completed after 3 sittings, each
  required beat gets a fallback mechanic with a looser trigger. For example, if you
  never run `ls`, the phantom file turns up in a login message instead.

## Never fires

- As root (`EUID == 0`), in non-interactive shells, or when `TERM=dumb`
- When `TENANT_OFF=1` is set in that shell
- When cwd is outside `$HOME`
- During a git rebase, merge or bisect in cwd. (It checks marker names in `.git/`.
  Names only. We're not monsters.)
- After `tenant evict`, ever

## Compressed mode

`tenant start --compressed` runs the whole arc in about 30 minutes:

| Setting | Normal | Compressed |
|---|---|---|
| Dormancy | 36 h + 2 sittings | 2 min |
| New sitting | 3 h idle or 05:00 | every 5 min of activity |
| Chapter gap | 16 h | 4 min |
| Min gap between events | 20 min / 10 prompts | 1 min / 3 prompts |

`tenant doctor` shows which mode is active.

A third preset, `--pace=tester` (dormancy 12 h, a chapter every sitting), exists for
the three-day friends test in the [roadmap](ROADMAP.md).

## Developer tools (spoilers)

Hidden, and only available with `TENANT_DEV=1`:

| Command | Does |
|---|---|
| `tenant _sim --profile daily` | Prints the full timeline for a synthetic usage profile |
| `tenant _schedule --spoilers` | Shows what's armed for the current state |
| `tenant _force <mechanic>` | Fires a mechanic on the next prompt, ignoring guards. This is how the teaser gets recorded. |

## Tuning targets

The simulation tests assert all of these:

- Daily user (about 4 sittings a day): the arc completes in 5 to 8 days.
- Evenings-only user (1 sitting a day): it completes in 12 days or fewer.
- No events at all in the first 36 h.
- At most 3 events in any calendar day, and never two within 20 minutes.
- Six tmux panes open at once: no double-fires.
- Compressed mode: it completes in 25 to 40 minutes of steady use.
