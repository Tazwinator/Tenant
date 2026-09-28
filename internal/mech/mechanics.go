package mech

import (
	"fmt"
	"strings"
	"time"

	"github.com/Tazwinator/Tenant/internal/term"
)

func init() {
	register(lsPhantom{})
	register(promptGlyph{})
	register(historyGhost{})
	register(motdLastLogin{})
	register(clearResidue{})
	register(notfoundRemark{})
	register(titleWhisper{})
	register(promptTime{})
}

func (e *Env) atPrompt() bool { return e.Kind == "prompt" }

// ls.phantom: one more entry after `ls` has finished. tenant never wraps
// ls; this is printed before the prompt and just looks like more output.
type lsPhantom struct{}

func (lsPhantom) ID() string        { return "ls.phantom" }
func (lsPhantom) Weight() float64   { return 4 }
func (lsPhantom) NeedsText() bool   { return true }
func (lsPhantom) Probe(e *Env) bool { return true }
func (lsPhantom) Triggered(e *Env) bool {
	return e.atPrompt() && e.Shape.Kind == "ls" && !e.Shape.Bad && e.UnderHome && e.Status == 0
}
func (lsPhantom) Fire(e *Env, text string) (Effect, bool) {
	name := term.Clean(text, 60)
	if name == "" || strings.ContainsAny(name, " /") {
		return Effect{}, false
	}
	shown := name
	if e.Shape.Color && e.Styled {
		shown = term.Paint(term.LSColor(e.LSColors, name, false), name)
	}
	if !e.Shape.Long {
		return Effect{Say: shown + "\n"}, true
	}
	stamp := e.LateStamp
	if stamp.IsZero() {
		stamp = e.Now.Add(-22 * time.Hour)
	}
	user := term.Clean(e.User, 32)
	if user == "" {
		user = "user"
	}
	return Effect{Say: fmt.Sprintf("-rw-r--r-- 1 %s %s 0 %s %s\n", user, user, stamp.Format("Jan _2 15:04"), shown)}, true
}

// prompt.glyph: for one prompt, a vowel in the directory name shifts to a
// neighbouring vowel. The hook gets integers only and does the swap itself.
type promptGlyph struct{}

var vowelShift = map[rune]rune{'a': 'e', 'e': 'a', 'o': 'u', 'u': 'o', 'i': 'e'}

func (promptGlyph) ID() string        { return "prompt.glyph" }
func (promptGlyph) Weight() float64   { return 1 }
func (promptGlyph) NeedsText() bool   { return false }
func (promptGlyph) Probe(e *Env) bool { return e.Caps&CapCwdPrompt != 0 }
func (promptGlyph) Triggered(e *Env) bool {
	return e.atPrompt() && e.UnderHome && e.CwdShown != "~"
}
func (promptGlyph) Fire(e *Env, _ string) (Effect, bool) {
	i, from, to, ok := shiftSpot(e.CwdShown, e.Roll("glyph"))
	if !ok {
		return Effect{}, false
	}
	n := len([]rune(e.CwdShown))
	return Effect{Acts: []Action{{Verb: "glyph", Args: []int{n - i, int(from), int(to)}}}}, true
}

// shiftSpot picks a vowel in the last path component (never its first
// letter) and the vowel it shifts to. i is a rune index into shown.
func shiftSpot(shown string, roll uint64) (i int, from, to rune, ok bool) {
	runes := []rune(shown)
	baseStart := len([]rune(shown[:strings.LastIndex(shown, "/")+1]))
	var spots []int
	for j := baseStart + 1; j < len(runes); j++ {
		if _, ok := vowelShift[runes[j]]; ok {
			spots = append(spots, j)
		}
	}
	if len(spots) == 0 {
		return 0, 0, 0, false
	}
	i = spots[roll%uint64(len(spots))]
	return i, runes[i], vowelShift[runes[i]], true
}

// ShiftVowel returns shown with one vowel of its last component shifted,
// the same change prompt.glyph makes.
func ShiftVowel(shown string, roll uint64) (string, bool) {
	i, _, to, ok := shiftSpot(shown, roll)
	if !ok {
		return "", false
	}
	runes := []rune(shown)
	runes[i] = to
	return string(runes), true
}

// history.ghost: the next press of Up shows a command you never typed. It
// lives only in the line editor and never reaches the history file.
type historyGhost struct{}

func (historyGhost) ID() string            { return "history.ghost" }
func (historyGhost) Weight() float64       { return 1 }
func (historyGhost) NeedsText() bool       { return true }
func (historyGhost) Probe(e *Env) bool     { return e.Caps&CapUp != 0 }
func (historyGhost) Triggered(e *Env) bool { return e.atPrompt() }
func (historyGhost) Fire(e *Env, text string) (Effect, bool) {
	g := term.Clean(text, 120)
	if g == "" {
		return Effect{}, false
	}
	return Effect{Ghost: g, Acts: []Action{{Verb: "ghost"}}}, true
}

// motd.lastlogin: a new shell opens with a "Last login" line whose "from"
// field is somewhere you were.
type motdLastLogin struct{}

func (motdLastLogin) ID() string            { return "motd.lastlogin" }
func (motdLastLogin) Weight() float64       { return 5 }
func (motdLastLogin) NeedsText() bool       { return true }
func (motdLastLogin) Probe(e *Env) bool     { return true }
func (motdLastLogin) Triggered(e *Env) bool { return e.atPrompt() && e.First }
func (motdLastLogin) Fire(e *Env, text string) (Effect, bool) {
	from := term.Clean(text, 80)
	if from == "" {
		return Effect{}, false
	}
	stamp := e.LateStamp
	if stamp.IsZero() {
		stamp = e.Now.Add(-9 * time.Hour)
	}
	tty := e.TTY
	if tty == "" {
		tty = "pts/0"
	}
	return Effect{Say: fmt.Sprintf("Last login: %s on %s from %s\n", stamp.Format("Mon Jan _2 15:04:05 2006"), tty, from)}, true
}

// clear.residue: one faint line survives `clear`.
type clearResidue struct{}

func (clearResidue) ID() string            { return "clear.residue" }
func (clearResidue) Weight() float64       { return 6 }
func (clearResidue) NeedsText() bool       { return true }
func (clearResidue) Probe(e *Env) bool     { return true }
func (clearResidue) Triggered(e *Env) bool { return e.atPrompt() && e.Shape.Kind == "clear" }
func (clearResidue) Fire(e *Env, text string) (Effect, bool) {
	return say(e, text)
}

// notfound.remark: a typo gets one more line than usual.
type notfoundRemark struct{}

func (notfoundRemark) ID() string            { return "notfound.remark" }
func (notfoundRemark) Weight() float64       { return 8 }
func (notfoundRemark) NeedsText() bool       { return true }
func (notfoundRemark) Probe(e *Env) bool     { return true }
func (notfoundRemark) Triggered(e *Env) bool { return e.Kind == "notfound" }
func (notfoundRemark) Fire(e *Env, text string) (Effect, bool) {
	return say(e, text)
}

// title.whisper: the window title changes for one prompt. The old title is
// pushed onto the terminal's title stack and popped at the next prompt.
type titleWhisper struct{}

func (titleWhisper) ID() string            { return "title.whisper" }
func (titleWhisper) Weight() float64       { return 1 }
func (titleWhisper) NeedsText() bool       { return true }
func (titleWhisper) Probe(e *Env) bool     { return e.Caps&CapTitle != 0 && e.Styled }
func (titleWhisper) Triggered(e *Env) bool { return e.atPrompt() }
func (titleWhisper) Fire(e *Env, text string) (Effect, bool) {
	t := term.Clean(text, 80)
	if t == "" {
		return Effect{}, false
	}
	return Effect{Say: TitlePush + term.Title(t), Title: true}, true
}

// Title stack sequences (xterm, and most modern terminals).
const (
	TitlePush = "\x1b[22;0t"
	TitlePop  = "\x1b[23;0t"
)

// prompt.time: a time appears at the right of the prompt, once.
type promptTime struct{}

func (promptTime) ID() string            { return "prompt.time" }
func (promptTime) Weight() float64       { return 1 }
func (promptTime) NeedsText() bool       { return true }
func (promptTime) Probe(e *Env) bool     { return e.Caps&CapRight != 0 }
func (promptTime) Triggered(e *Env) bool { return e.atPrompt() }
func (promptTime) Fire(e *Env, text string) (Effect, bool) {
	t := term.Clean(text, 20)
	if t == "" {
		return Effect{}, false
	}
	return Effect{Time: t, Acts: []Action{{Verb: "time"}}}, true
}

func say(e *Env, text string) (Effect, bool) {
	t := term.Clean(text, 200)
	if t == "" {
		return Effect{}, false
	}
	if e.Styled {
		t = term.Dim(t)
	}
	return Effect{Say: t + "\n"}, true
}
