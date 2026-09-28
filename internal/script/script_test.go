package script_test

import (
	"strings"
	"testing"

	"github.com/Tazwinator/Tenant/internal/engine"
	"github.com/Tazwinator/Tenant/internal/mech"
	"github.com/Tazwinator/Tenant/internal/script"
	"github.com/Tazwinator/Tenant/story"
)

// The shipped story must parse, use only known mechanics and variables, and
// give every chapter at least one required beat.
func TestAct1(t *testing.T) {
	s, err := script.Load(story.Act1(), mech.Known)
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, v := range engine.VarNames {
		known[v] = true
	}
	invites := 0
	for ch := 1; ch <= script.Chapters; ch++ {
		req := 0
		for _, b := range s.InChapter(ch) {
			if b.Required {
				req++
			}
			if b.Invite {
				invites++
			}
			m, _ := mech.Get(b.Mech)
			if m.NeedsText() && len(b.Text) == 0 {
				t.Errorf("beat %s: %s needs text", b.ID, b.Mech)
			}
			if b.Required && len(b.Text) > 0 && len(script.Variables(b.Text[len(b.Text)-1])) > 0 &&
				!strings.HasPrefix(b.Text[len(b.Text)-1], "cd {{") && b.Mech != "motd.lastlogin" {
				t.Logf("beat %s: last text alternative depends on a variable", b.ID)
			}
			for _, txt := range append(append([]string{}, b.Text...), b.FText...) {
				for _, v := range script.Variables(txt) {
					if !known[v] {
						t.Errorf("beat %s: unknown variable {{%s}}", b.ID, v)
					}
				}
			}
		}
		if req == 0 {
			t.Errorf("chapter %d has no required beat", ch)
		}
	}
	if invites != 1 {
		t.Errorf("want exactly one invitation beat, got %d", invites)
	}
	if s.Finale == nil || len(s.Epilogue) == 0 {
		t.Fatal("missing finale or epilogue")
	}
	for _, n := range s.Finale.Nodes {
		for _, l := range n.Lines {
			for _, v := range script.Variables(l) {
				if !known[v] {
					t.Errorf("finale node %s: unknown variable {{%s}}", n.Name, v)
				}
			}
		}
	}
}

func TestParseErrors(t *testing.T) {
	bad := map[string]string{
		"unknown mechanic": "beat a\nchapter 1\nmech nope\n",
		"bad chapter":      "beat a\nchapter 9\nmech ls.phantom\n",
		"duplicate":        "beat a\nchapter 1\nmech ls.phantom\n\nbeat a\nchapter 1\nmech ls.phantom\n",
		"unknown key":      "beat a\nchapter 1\nmech ls.phantom\nexec rm -rf ~\n",
		"dangling after":   "beat a\nchapter 1\nmech ls.phantom\nafter b\n",
	}
	for name, src := range bad {
		if _, err := script.ParseBeats(src, mech.Known); err == nil {
			t.Errorf("%s: parsed without error", name)
		}
	}
}

func TestRender(t *testing.T) {
	vars := func(n string) (string, bool) {
		if n == "cwd" {
			return "~/x", true
		}
		return "", false
	}
	if got, ok := script.Render("in {{cwd}}", vars); !ok || got != "in ~/x" {
		t.Errorf("Render = %q, %v", got, ok)
	}
	if _, ok := script.Render("{{cwd}} and {{cmd.top}}", vars); ok {
		t.Error("Render succeeded with an unknown variable")
	}
}

func TestDialogueRouting(t *testing.T) {
	d, err := script.ParseDialogue("@start\nhi\n? leave evict => bye\n? why want => why\n? * => start\n@bye\nok\n-> END\n@why\nbecause\n-> start\n")
	if err != nil {
		t.Fatal(err)
	}
	n := d.Nodes["start"]
	for in, want := range map[string]string{
		"please leave":            "bye",
		"I'm evicting you":        "bye",
		"what do you want":        "why",
		"banana":                  "start",
		"LEAVE. NOW.":             "bye",
		"go away? no, why stay?!": "why",
	} {
		if got := n.Route(in); got != want {
			t.Errorf("Route(%q) = %q, want %q", in, got, want)
		}
	}
}
