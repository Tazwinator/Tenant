package finale

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/Tazwinator/Tenant/internal/mech"
	"github.com/Tazwinator/Tenant/internal/script"
	"github.com/Tazwinator/Tenant/story"
)

func run(t *testing.T, input string) (bool, string) {
	t.Helper()
	s, err := script.Load(story.Act1(), mech.Known)
	if err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	w.WriteString(input)
	w.Close()
	var out bytes.Buffer
	vars := func(n string) (string, bool) {
		switch n {
		case "cmd.top":
			return "git", true
		case "first_seen.dir":
			return "~/code/rarepulls", true
		}
		return "", false
	}
	done := (&Runner{D: s.Finale, Vars: vars, In: r, Out: &out}).Run()
	return done, out.String()
}

func TestFinaleEnds(t *testing.T) {
	done, out := run(t, "who are you\nwhat do you want\nplease leave\n")
	if !done {
		t.Fatalf("didn't reach the end:\n%s", out)
	}
	for _, want := range []string{"you type git", "i lived here before you", "~/code/rarepulls", "evict", "tenant confess"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	// Lines with unknown variables are skipped, never printed half-filled.
	if strings.Contains(out, "{{") || strings.Contains(out, "something called  in") {
		t.Errorf("unrendered variable in:\n%s", out)
	}
}

func TestFinaleLimit(t *testing.T) {
	done, out := run(t, strings.Repeat("banana\n", 10))
	if !done || !strings.Contains(out, "it's late") {
		t.Fatalf("the turn limit didn't end the conversation:\n%s", out)
	}
}

func TestFinaleLeaveEarly(t *testing.T) {
	if done, _ := run(t, "hello\n"); done {
		t.Fatal("end of input should leave the finale unfinished")
	}
}
