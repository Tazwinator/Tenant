# Platforms

Where tenant lives now, and how it could spread without turning into two apps.

## Now: the Linux terminal (bash and zsh)

This is the novel part. Meta-horror on the desktop already exists (KinitoPET,
IMSCARED), and it relies on art. A story that happens inside your **real** prompt
doesn't exist yet, runs on timing and text, and is the ground where the kernel-enforced
security story (Landlock) actually holds. We launch here, and only here.

## The question: GUI for Windows and macOS?

The idea: most people, including the maintainer's friends, don't use a terminal,
so maybe tenant should be graphical on Windows and macOS.

**Decision (D16): not before launch, and never as a second app.**

- **A GUI version puts us in KinitoPET's lane,** and its moat is art and design,
  which the brief says to avoid. The terminal version's moat is novelty plus
  engineering.
- **Landlock is Linux-only.** On Windows and macOS, the "names only, enforced by
  the kernel" pitch weakens to "enforced by code", unless we do separate sandbox
  work: AppContainer on Windows, Seatbelt on macOS.
- **Friends who don't use terminals is a tester problem, not a product problem.**
  We recruit terminal-using testers from the teaser audience instead.
- **The format isn't proven yet.** The kill test decides whether it's worth
  expanding at all.

## If the format works: one binary, more surfaces

The engine (scheduler, story, observations, audit, sandbox) doesn't know what a
shell is (see [ARCHITECTURE](ARCHITECTURE.md#engine-and-surfaces)). Every new
platform is a new **surface** compiled into the same binary, sharing the same
engine, `confess` and `evict`:

| Surface | Cost | Sandbox | Notes |
|---|---|---|---|
| bash, zsh on Linux | now | Landlock | launch |
| zsh on macOS (Terminal, iTerm2) | low: same hook | Seatbelt (later); code-enforced until then | the cheapest reach extension |
| fish | low | as the host OS | after launch |
| PowerShell on Windows | medium: `prompt` function, PSReadLine | AppContainer (later) | WSL users already get the Linux version |
| Linux desktop (notifications, window titles) | medium | Landlock still applies to the engine | the natural shape for a paid "it gets out" act |
| Windows or macOS desktop | high: art and design | per OS | only with evidence of demand |

A desktop surface on Linux keeps the Landlock story, which is why the most likely
desktop step is **Linux first**, as the paid continuation: Act 1 happens in your
terminal; in Act 2 it gets out. That's one entity with more rooms, not two apps.

## What would change our mind

Revisit after launch if:

- teaser or launch comments ask for Windows or macOS repeatedly, especially from
  horror players or streamers rather than terminal users, **and**
- the kill test passes, **and**
- the paid story has a first buyer.
