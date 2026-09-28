# Story

> Light spoilers: this is the structure and the rules, not the script. The script
> lives in `story/`. Don't read it if you want to be got.

## Premise (draft)

The working title is **The Previous Tenant**. Claude drafts Act 1 from this premise,
and the maintainer edits it (D12 in [DECISIONS](DECISIONS.md)).

Something lived in this home directory before you. It can't open anything. It can't
write. It can't leave. All it can do is read names: your directories, your files,
the first word of the things you type. It has been very lonely, and it has been
paying attention.

The twist is in the security model. The ending is `tenant confess`: a complete and
true list of everything it ever looked at, with one line at the bottom, `wrote
nothing`. At the start, that line is reassurance. By the end, it's sad. It never
left a mark.

## Structure

About 20 minutes of actual content, spread over dormancy plus five sittings.

| Chapter | Player feels | Beats (mechanic) |
|---|---|---|
| 0 Dormancy | Nothing. The install is forgotten. | none. It watches. |
| 1 Off | "Did I misread that?" | `prompt.glyph`, `ls.phantom` (a name that could plausibly be yours) |
| 2 It knows | "That's… where I was." | `motd.lastlogin`, `clear.residue` |
| 3 It speaks | "Something is talking to me." | `history.ghost`, `notfound.remark`, `title.whisper` |
| 4 It wants | "It wants something from me." | `ls.phantom` (real names, from elsewhere), `prompt.time`, the invitation: "you know what i'm called." |
| 5 Confrontation | "I have to deal with this." | `finale.summon`, then `confess.epilogue` |

That's eight wrong-thing mechanics, the finale and the epilogue. Required beats are
marked in the script. Optional beats add colour and are shuffled by the seed (see
[SCHEDULER](SCHEDULER.md#beat-selection)).

## Writing rules

The register is dry. Creepy comes from precision, not volume.

1. **Specific beats spooky.** A real directory name at 23:41 beats any amount of
   "I see you".
2. **Deniable early.** In chapters 1 and 2, every event should have a boring
   explanation available.
3. **Short.** No more than one sentence per event until chapter 4. Lowercase. No
   exclamation marks.
4. **One wrong thing at a time.** Never stack two mechanics in one prompt.
5. **Never threaten.** It never threatens you, your files or your machine.
6. **Never lie about its powers.** It can't read your files, and at some point it
   says so. That limitation is the heart of the story.
7. **No cheap horror.** No skulls, no zalgo text, no screaming caps, no fake `rm
   -rf`, no fake errors that make someone check their disk.
8. **Out of character** only in `help`, `doctor`, `start` and `evict`. The safeword
   is never played for a scare.

## What the writer can reference

These are the only facts the engine knows, exposed as template variables:

| Variable | Example |
|---|---|
| `{{user}}` | `sam` |
| `{{host}}` | `zireael` |
| `{{cwd}}` | `~/code/rarepulls` |
| `{{names.here}}` | names in cwd |
| `{{names.elsewhere}}` | a name from another directory you visited |
| `{{last_night.dir}}`, `{{last_night.time}}` | `~/code/rarepulls`, `23:41` |
| `{{first_seen.dir}}` | the first directory it saw you in |
| `{{cmd.top}}`, `{{cmd.rare}}` | your most-used and a rarely-used command name |
| `{{days}}` | days since `tenant start` |

## Story file format (proposed)

Plain text, one beat per block, embedded in the binary at build time:

```
beat      phantom-first
chapter   1
required  yes
mech      ls.phantom
fallback  motd.lastlogin
text      {{names.elsewhere}}
```

## The finale

- After the invitation, `tenant` with no arguments opens the conversation.
- It's a small dialogue graph. Your input is matched on intent keywords (who, what,
  why, leave, stay, sorry, and so on), with graceful fallbacks, so anything you type
  gets an answer.
- The text is typewriter-paced at about 40 characters a second, and any key skips
  it. Ctrl-C always exits, and you can come back to it later.
- MVP: one ending. It ends by asking you to run `tenant confess`.
- After the epilogue, the story is finished. tenant goes permanently quiet, and
  `doctor` reminds you that `evict` is there.

## Acts and money

Act 1 (this five-sitting story) is free and open source, because it has to be for
trust. A longer paid story (about $5 on itch.io) would ship later as a **data-only
story pack** that you can read in full before installing. It is separate from this
repo and has its own terms (D14 in [DECISIONS](DECISIONS.md)).

One possible shape for the paid story: **it gets out.** Act 1 lives in the terminal.
A later act follows you onto the desktop (notifications, window titles), through
the same binary. See [PLATFORMS](PLATFORMS.md).
