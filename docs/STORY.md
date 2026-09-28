# Story

> Light spoilers: this is the structure and the rules, not the script. The script
> is in `story/act1/`. Don't read it if you want to be got.

## Premise

**The Previous Tenant.** Claude drafted Act 1, and the maintainer edits it (D12).

Something lived in this home directory before you. It can't open anything. It can't
write. It can't leave. All it can do is read names: your directories, your files,
the first word of the things you type. It has been very lonely, and it has been
paying attention.

The twist is the security model. The ending is `tenant confess`: a complete and true
list of everything it ever looked at, ending in `wrote nothing`. At the start, that
line is reassurance. By the end, it's sad. It never left a mark.

## Structure

| Chapter | Player feels | Required beats | Optional beats (after the required ones) |
|---|---|---|---|
| 0 Dormancy | Nothing | (it watches) | |
| 1 Off | "Did I misread that?" | `prompt.glyph`, then `ls.phantom` with a real name from elsewhere | a stale window title |
| 2 It knows | "That's… where I was." | a login line from last night's directory, then `cd <that directory>` left after `clear` | `still_here.txt`, another glyph |
| 3 It speaks | "Something is talking to me." | a ghost command with a comment, then a window title that has been counting | a typo remark, a residue about a rare command |
| 4 It wants | "It wants something." | `i_can_only_read_the_names`, a story time at the right of the prompt | a login "from the next room over", "neither am i" |
| 5 The name | "I have to deal with this." | "you know what i'm called." after `clear`, then Up shows `tenant` (the invitation) | "waiting" |
| Finale | | typing `tenant` opens the conversation | |
| Epilogue | | after the finale, `confess` ends with a closing passage | |

That's eight wrong-thing mechanics, about 13 required and optional beats a daily user
will see, the finale and the epilogue: roughly 20 minutes of content across a week.

## Writing rules

The register is dry. Creepy comes from precision, not volume.

1. **Specific beats spooky.** A real directory name at 23:41 beats any amount of
   "I see you".
2. **Deniable early.** In chapters 1 and 2, every event has a boring explanation.
3. **Short.** One sentence per event until chapter 4. Lowercase. No exclamation marks.
4. **One wrong thing at a time.** The engine never fires two beats on one prompt.
5. **Never threaten** the player, their files or their machine.
6. **Never claim a power it doesn't have.** It can't read your files, and it says so.
   That limitation is the heart of the story.
7. **No cheap horror.** No skulls, no zalgo text, no screaming caps, no fake `rm -rf`,
   no fake errors that make someone check their disk.
8. **Ghost commands are harmless if run**: a `cd`, or an `ls` with a comment. The
   engine enforces this (`mech.HarmlessCommand`), so a text that fails is skipped.
   Keep comments to plain words, without apostrophes.
9. **Always have a way out.** A required beat's last text alternative should need no
   variables (or `{{first_seen.dir}}`, which is almost always known), so a missing
   fact can't block the story.
10. **Out of character** only in `help`, `doctor`, `start` and `evict`.

## Beat format

```
beat      off-phantom
chapter   1
required  yes              # required beats fire in file order within a chapter
mech      ls.phantom
text      {{names.elsewhere}}
text      untitled.txt     # alternatives, tried in order until one renders
fallback  motd.lastlogin   # if this shell can't show the mechanic, or the chapter stalls
ftext     {{first_seen.dir}}
```

Other keys: `invite yes` (firing it lets `tenant` open the finale) and `after <id>`
(an optional beat that waits for another beat).

## What the writer can reference

These are the only facts the engine offers. A text that uses one it doesn't know
yet isn't used.

| Variable | Example |
|---|---|
| `{{user}}`, `{{host}}` | `sam`, `zireael` |
| `{{cwd}}`, `{{cwd.off}}` | `~/code/rarepulls`, `~/code/rarepolls` |
| `{{names.here}}`, `{{names.elsewhere}}` | a file name here, or from a directory you visited |
| `{{last_night.dir}}`, `{{last_night.time}}` | your last directory between 21:00 and 04:00 before this sitting |
| `{{first_seen.dir}}` | the first directory it saw you in |
| `{{cmd.top}}`, `{{cmd.rare}}` | your most-used and least-used command (after 20 commands) |
| `{{days}}` | days since `tenant start` |
| `{{typo}}`, `{{typo.meant}}` | in typo remarks only |
| `{{cmds.total}}`, `{{dirs.count}}`, `{{seen.count}}` | counts, for the finale and the epilogue |

## The finale

After the invitation, `tenant` with no arguments opens the conversation
(`story/act1/finale.txt`). It's a small graph: each node says its lines, then either
jumps or waits for input. Input is routed on keywords (a keyword of three or more
letters matches as a prefix, so `evicted` matches `evict`), with a `*` fallback. After
seven answers it winds itself up. The text is typewritten at 40 characters a second,
and any key finishes the line. Ctrl-C always exits, and typing `tenant` again starts
over. It's only saved as finished when it reaches the end. Lines whose variables
aren't known are skipped.

After the finale, the hooks go permanently quiet, `confess` ends with the epilogue,
and `doctor` points at `evict`.

## Acts and money

Act 1 is free and MIT-licensed, like everything in this repo. A longer paid story
(about $5 on itch.io) would ship later as a **data-only story pack** that you can
read in full before installing. It is separate from this repo and has its own terms
(D14).

One possible shape for it: **it gets out.** Act 1 lives in the terminal. A later act
follows you onto the desktop (notifications, window titles) through the same binary.
See [PLATFORMS](PLATFORMS.md).
