package mech

import (
	"strings"
	"testing"
	"time"
)

func env() *Env {
	return &Env{
		Now: time.Date(2026, 10, 8, 23, 41, 0, 0, time.UTC), Shell: "bash", Kind: "prompt",
		Shape: ParseShape("ls:p"), Caps: Caps(63), Home: "/home/sam", Cwd: "/home/sam/code/rarepulls",
		CwdShown: "~/code/rarepulls", UnderHome: true, User: "sam", Styled: false,
		Roll: func(string) uint64 { return 0 },
	}
}

func TestShapes(t *testing.T) {
	for in, want := range map[string]Shape{
		"ls:p":   {Kind: "ls"},
		"ls:lca": {Kind: "ls", Long: true, Color: true, All: true},
		"ls:x":   {Kind: "ls", Bad: true},
		"clear":  {Kind: "clear"},
		"-":      {Kind: "other"},
		"rm -rf": {Kind: "other"},
	} {
		if got := ParseShape(in); got != want {
			t.Errorf("ParseShape(%q) = %+v, want %+v", in, got, want)
		}
	}
}

func TestGlyph(t *testing.T) {
	m, _ := Get("prompt.glyph")
	e := env()
	eff, ok := m.Fire(e, "")
	if !ok || len(eff.Acts) != 1 || eff.Acts[0].Verb != "glyph" {
		t.Fatalf("glyph: %+v %v", eff, ok)
	}
	a := eff.Acts[0].Args
	shown := []rune(e.CwdShown)
	i := len(shown) - a[0]
	if shown[i] != rune(a[1]) || vowelShift[rune(a[1])] != rune(a[2]) {
		t.Fatalf("glyph args %v don't match %q", a, e.CwdShown)
	}
	if i <= strings.LastIndex(e.CwdShown, "/")+1 {
		t.Fatal("glyph changed the first letter of the directory")
	}
	if off, ok := ShiftVowel("~/code/rarepulls", 0); !ok || off == "~/code/rarepulls" || len(off) != len("~/code/rarepulls") {
		t.Fatalf("ShiftVowel = %q", off)
	}
	// Nothing to swap: no fire.
	e.CwdShown = "~/x/zzz"
	if _, ok := m.Fire(e, ""); ok {
		t.Fatal("glyph fired with no vowel to shift")
	}
}

func TestPhantom(t *testing.T) {
	m, _ := Get("ls.phantom")
	e := env()
	if eff, ok := m.Fire(e, "still_here.txt"); !ok || eff.Say != "still_here.txt\n" {
		t.Fatalf("plain phantom: %q", eff.Say)
	}
	e.Shape = ParseShape("ls:l")
	eff, ok := m.Fire(e, "still_here.txt")
	if !ok || !strings.HasPrefix(eff.Say, "-rw-r--r-- 1 sam sam 0 ") || !strings.HasSuffix(eff.Say, " still_here.txt\n") {
		t.Fatalf("long phantom: %q", eff.Say)
	}
	for _, shape := range []string{"ls:x", "clear", "-"} {
		e.Shape = ParseShape(shape)
		if m.Triggered(e) {
			t.Errorf("phantom triggered after %s", shape)
		}
	}
	e.Shape = ParseShape("ls:p")
	e.Status = 2
	if m.Triggered(e) {
		t.Error("phantom triggered after a failed ls")
	}
}

// Hostile names must never become escape sequences or break lines.
func TestHostileText(t *testing.T) {
	hostile := []string{
		"\x1b]2;pwned\a", "a\nb", "\x1b[31mred", "$(touch pwned)", "`id`", "%F{red}x",
		"\u202eevil", "\x9b31m", "ok\x00nul",
	}
	for _, id := range IDs() {
		m, _ := Get(id)
		for _, h := range hostile {
			e := env()
			e.Styled = true
			if id == "notfound.remark" {
				e.Kind = "notfound"
			}
			eff, ok := m.Fire(e, h)
			if !ok {
				continue
			}
			body := strings.NewReplacer(TitlePush, "", TitlePop, "", "\x1b[2m", "", "\x1b[0m", "", "\x1b]2;", "", "\a", "").Replace(eff.Say)
			body = strings.TrimSuffix(body, "\n")
			for _, r := range body + eff.Ghost + eff.Time {
				if r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0) || r == '\u202e' {
					t.Errorf("%s with %q printed control character %U", id, h, r)
				}
			}
			if strings.Contains(body, "\n") {
				t.Errorf("%s with %q printed an extra line", id, h)
			}
			for _, a := range eff.Acts {
				if a.Verb != "glyph" && len(a.Args) > 0 {
					t.Errorf("%s: unexpected args %v", id, a.Args)
				}
			}
		}
	}
}
