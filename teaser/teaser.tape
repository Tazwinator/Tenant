# The teaser clip, recorded from the real binary with charmbracelet/vhs:
#
#   go build -o /tmp/tenant ./cmd/tenant && PATH=/tmp:$PATH vhs teaser/teaser.tape
#
# It runs in a throwaway $HOME, so nothing of yours is on screen. Hidden
# steps use `tenant _force <mechanic> [text]` to make each moment happen on
# cue. SPOILERS, lightly.

Output teaser/teaser.gif
Output teaser/teaser.mp4

Set Shell "bash"
Set FontSize 20
Set Width 1280
Set Height 640
Set Padding 24
Set Theme "Builtin Dark"
Set TypingSpeed 75ms
Set Framerate 30

Env TENANT_DEV "1"
Env TENANT_DEADLINE_MS "2000"
Env USER "sam"

Hide
Type `export HOME=$(mktemp -d) HISTFILE=/dev/null && cd && mkdir -p code/rarepulls/{cmd,internal,web} notes && touch code/rarepulls/go.{mod,sum} notes/todo.md` Enter
Type `PS1='\w \$ ' && eval "$(tenant init bash)" && tenant start --compressed >/dev/null && cd ~/code/rarepulls && clear` Enter
Sleep 1s
Show

# 1. Nothing is wrong.
Sleep 1.5s
Type "ls" Sleep 300ms Enter
Sleep 2s
Type "git status" Sleep 300ms Enter
Sleep 1.5s

# 2. A file that isn't there.
Hide
Type "tenant _force ls.phantom still_here.txt >/dev/null; clear" Enter
Sleep 500ms
Show
Type "ls" Sleep 300ms Enter
Sleep 2.5s
Type "ls" Sleep 300ms Enter
Sleep 3s

# 3. One letter off.
Hide
Type "clear; tenant _force prompt.glyph >/dev/null" Enter
Sleep 500ms
Show
Sleep 3.5s
Type "true" Sleep 300ms Enter
Sleep 2s

# 4. Something survives clear.
Hide
Type "tenant _force clear.residue 'cd ~/code/rarepulls' >/dev/null" Enter
Sleep 300ms
Show
Type "clear" Sleep 300ms Enter
Sleep 3.5s

# 5. A command you never typed.
Hide
Type "tenant _force history.ghost 'ls  # i only ever read the names' >/dev/null" Enter
Sleep 300ms
Show
Sleep 1s
Up
Sleep 4s
Ctrl+U
Sleep 1s

# 6. It asks for its name.
Hide
Type "tenant _force clear.residue \"you know what i'm called.\" >/dev/null; tenant _invite >/dev/null" Enter
Sleep 300ms
Show
Type "clear" Sleep 300ms Enter
Sleep 3s
Type "tenant" Sleep 600ms Enter
Sleep 7s
Type "who are you" Sleep 400ms Enter
Sleep 9s
Ctrl+C
Sleep 1s

# 7. The receipt.
Type "tenant confess | tail -2" Sleep 300ms Enter
Sleep 5s
