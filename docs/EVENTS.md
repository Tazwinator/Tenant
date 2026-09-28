# Events

A **mechanic** is a kind of wrong thing the engine knows how to do, written in Go
and audited. A **beat** is a moment in the story that uses a mechanic with specific
text. Story files can only use the mechanics listed here, so a story is data and
never code.

Every mechanic declares:

- a **trigger**: what has to have just happened
- a **probe**: whether it can work in this shell, given the prompt framework, key
  bindings and terminal
- a **cleanup**: what gets restored, and when

If the probe fails, the scheduler never picks that mechanic. It uses the beat's
fallback instead. `tenant doctor` lists the probe results without spoilers.

## Catalogue

| # | Mechanic | The player sees | Phase |
|---|---|---|---|
| 1 | `ls.phantom` | An extra entry in `ls` output that isn't there next time | Weekend |
| 2 | `prompt.glyph` | The prompt's path is one character off, once | Weekend |
| 3 | `history.ghost` | Pressing Up shows a command you never typed | Weekend |
| 4 | `motd.lastlogin` | A new shell greets you with a "Last login" line that knows where you were last night | MVP |
| 5 | `clear.residue` | One line survives `clear` | MVP |
| 6 | `notfound.remark` | A typo gets a slightly-too-personal "command not found" | MVP |
| 7 | `title.whisper` | The window title changes for one prompt | MVP |
| 8 | `rprompt.time` | A time appears on the right of the prompt, once | MVP |
| 9 | `finale.summon` | Typing `tenant` at the end opens a conversation | MVP |
| 10 | `confess.epilogue` | A closing passage after the audit log, once the story is done | MVP |

## Details

### ls.phantom

- **Trigger:** the last command was `ls` with no path arguments (shape `ls:plain`
  or `ls:long`), and cwd is under `$HOME`.
- **How:** tenant never wraps `ls`. When `ls` has finished, precmd prints one more
  line that looks like more `ls` output. It uses the right `LS_COLORS` colour for
  the name. In long format it's a believable row: `$USER`, a size of 0, and a time
  that matters to the story.
- **Name:** either written into the story (`still_here.txt`), or a real name from
  a different directory you visited, which is creepier.
- **Probe:** `ls` resolves to GNU `ls`, or to `eza` without icons. It skips `lsd`
  and icon themes, where a phantom without an icon would give the game away.
- **Cleanup:** none. It was only ever output.
- **Later:** append the phantom to the end of `ls`'s last line, as in the README
  illustration. This means reproducing GNU `ls` column layout from the names and
  `COLUMNS`.

### prompt.glyph

- **Trigger:** any prompt.
- **How:** the binary sends `prompt-glyph <index> <codepoint>`, two integers. The
  shell takes `${(D)PWD}`, replaces the character at that index with a homoglyph,
  escapes `%`, `$`, backtick and `\`, and substitutes it for the cwd token in
  `PROMPT` for a single render.
- **Glyphs:** single-cell only, so the prompt width is unchanged: `a→а` (U+0430),
  `e→е` (U+0435), `o→ο` (U+03BF), `c→ϲ` (U+03F2), `/→∕` (U+2215).
- **Probe:** `PROMPT` contains a cwd token (`%~`, `%/`, `%d`, `%c`, `%1~` and so
  on). This covers plain prompts, grml-zsh-config, oh-my-zsh themes and pure. Starship
  and powerlevel10k need their own adapters (see open question Q2 in
  [DECISIONS](DECISIONS.md)).
- **Cleanup:** the original `PROMPT` is restored on the next precmd, and by `unload`.

### history.ghost

- **Trigger:** any prompt. It is armed on this prompt and consumed by the next press
  of Up.
- **How:** tenant wraps the existing Up binding (`$terminfo[kcuu1]` and `^[[A`). On
  the first press after arming, it fills `BUFFER=$(tenant _ghost)`: a command in
  your vocabulary, written in a style that isn't yours. After that, Up behaves
  normally.
- **Never persisted:** it doesn't use `print -s`, because zsh would write that to
  `$HISTFILE`. The ghost only ever exists in the line editor. If you press Enter,
  it is your command.
- **Probe:** Up is bound to a widget tenant knows how to wrap:
  `up-line-or-history`, `up-line-or-beginning-search`,
  `history-substring-search-up`, or the equivalent with zsh-autosuggestions.
- **Cleanup:** it disarms after one press, and `unload` restores the original
  binding.

### motd.lastlogin

- **Trigger:** the first precmd of a new shell. It never prints during `.zshrc`, so
  it doesn't clash with the p10k instant prompt.
- **How:** it prints `Last login: Thu Oct  8 23:41:07 2026 on pts/3 from ~/code/rarepulls`.
  The format is real; the "from" field isn't.

### clear.residue

- **Trigger:** the last command was `clear` (or `^L` via a widget, later).
- **How:** precmd prints one faint line at the top of the now-empty screen before
  the prompt draws.

### notfound.remark

- **Trigger:** zsh calls `command_not_found_handler`.
- **How:** it prints zsh's usual message with one extra clause, then delegates to
  any handler that was already defined (such as pkgfile), unchanged.
- **Probe:** the handler is wrappable, meaning it isn't a read-only function.

### title.whisper

- **Trigger:** any prompt.
- **How:** the binary writes OSC 2 (`\e]2;…\a`) to the terminal. The next precmd,
  or your prompt framework, sets it back.
- **Probe:** `TERM` supports titles. It is skipped inside tmux unless
  `set-titles on`.

### rprompt.time

- **Trigger:** any prompt.
- **How:** `RPROMPT` is set to an escaped story time (for example `23:41`) for one
  render. The same time turns up later in the story.
- **Probe:** `RPROMPT` is empty, or only shows a clock.

### finale.summon

- **Trigger:** chapter 5 is unlocked and the invitation beat has fired.
- **How:** before the finale, `tenant` with no arguments prints help. After the
  invitation, it opens the confrontation: an interactive, typed conversation on
  stdin/stdout, typewriter-paced and skippable. Ctrl-C always exits. See
  [STORY](STORY.md#the-finale).

### confess.epilogue

- **Trigger:** the finale is complete.
- **How:** `tenant confess` prints the real audit log unchanged, then a horizontal
  rule, then the closing passage. The log is never edited, dramatised or reordered.
  See [SECURITY](SECURITY.md#the-audit-log).

## Writing a new mechanic

Implement the `Mechanic` interface in `internal/mech/`:

```go
type Mechanic interface {
    ID() string                       // "ls.phantom"
    Probe(env Env) Capability         // can it work in this shell?
    Triggered(t Tick) bool            // did the right thing just happen?
    Fire(t Tick, b Beat, out Out) error
}
```

`Out` only offers `Say(text)` (stderr, sanitised) and `Act(verb, ints...)`
(protocol actions). A mechanic can't write shell code, and it can't touch the
filesystem. It asks `observe` for names.
