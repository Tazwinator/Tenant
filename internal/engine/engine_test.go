package engine

import (
	"strings"
	"testing"
	"time"

	"github.com/Tazwinator/Tenant/internal/mech"
	"github.com/Tazwinator/Tenant/internal/sched"
	"github.com/Tazwinator/Tenant/internal/script"
	"github.com/Tazwinator/Tenant/internal/state"
	"github.com/Tazwinator/Tenant/story"
)

func act1(t *testing.T) *script.Story {
	t.Helper()
	s, err := script.Load(story.Act1(), mech.Known)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// The tuning targets from docs/SCHEDULER.md, checked across several seeds.
func TestPacingTargets(t *testing.T) {
	s := act1(t)
	cases := []struct {
		profile, pace string
		min, max      time.Duration
		days          int
	}{
		{"daily", "normal", 5 * 24 * time.Hour, 8 * 24 * time.Hour, 14},
		{"evenings", "normal", 6 * 24 * time.Hour, 14 * 24 * time.Hour, 21},
		{"weekend", "normal", 7 * 24 * time.Hour, 21 * 24 * time.Hour, 28},
		{"steady", "compressed", 25 * time.Minute, 45 * time.Minute, 1},
	}
	for _, c := range cases {
		for seed := uint64(1); seed <= 12; seed++ {
			r := Simulate(s, sched.Get(c.pace), Profiles[c.profile], seed, c.days)
			if r.Invited.IsZero() {
				t.Errorf("%s/%s seed %d: never invited (reached chapter %d)", c.profile, c.pace, seed, r.Chapter)
				continue
			}
			if d := r.Invited.Sub(r.Start); d < c.min || d > c.max {
				t.Errorf("%s/%s seed %d: invited after %s, want %s to %s", c.profile, c.pace, seed, d, c.min, c.max)
			}
			checkGuards(t, c.profile+"/"+c.pace, sched.Get(c.pace), r)
		}
	}
}

func checkGuards(t *testing.T, name string, p sched.Pace, r Result) {
	t.Helper()
	perDay := map[string]int{}
	for i, e := range r.Events {
		if e.At.Sub(r.Start) < p.Dormancy {
			t.Errorf("%s: %s fired during dormancy at %s", name, e.Beat, e.At)
		}
		perDay[sched.Day(e.At)]++
		if i > 0 && e.At.Sub(r.Events[i-1].At) < p.MinGap {
			t.Errorf("%s: %s and %s only %s apart", name, r.Events[i-1].Beat, e.Beat, e.At.Sub(r.Events[i-1].At))
		}
	}
	for day, n := range perDay {
		if n > p.DailyCap {
			t.Errorf("%s: %d events on %s, cap is %d", name, n, day, p.DailyCap)
		}
	}
}

// Every required beat of every chapter fires, in order.
func TestStoryOrder(t *testing.T) {
	s := act1(t)
	r := Simulate(s, sched.Get("normal"), Profiles["daily"], 7, 14)
	fired := map[string]int{}
	for i, e := range r.Events {
		fired[e.Beat] = i + 1
	}
	last := 0
	for _, b := range s.Beats {
		if !b.Required {
			continue
		}
		at := fired[b.ID]
		if at == 0 {
			t.Errorf("required beat %s never fired", b.ID)
			continue
		}
		if at < last {
			t.Errorf("required beat %s fired out of order", b.ID)
		}
		last = at
	}
}

// The same seed and the same usage give the same haunting.
func TestDeterministic(t *testing.T) {
	s := act1(t)
	a := Simulate(s, sched.Get("normal"), Profiles["daily"], 42, 14)
	b := Simulate(s, sched.Get("normal"), Profiles["daily"], 42, 14)
	if len(a.Events) != len(b.Events) || !a.Invited.Equal(b.Invited) {
		t.Fatal("two runs with the same seed differ")
	}
	for i := range a.Events {
		if a.Events[i] != b.Events[i] {
			t.Fatalf("event %d differs: %+v vs %+v", i, a.Events[i], b.Events[i])
		}
	}
}

func baseEnv(now time.Time) *mech.Env {
	return &mech.Env{
		Now: now, Shell: "bash", Kind: "prompt", Cmd: "ls", Shape: mech.ParseShape("ls:p"),
		Caps: mech.Caps(63), Home: "/home/sim", Cwd: "/home/sim/code/rarepulls",
		CwdShown: "~/code/rarepulls", UnderHome: true, User: "sim", Host: "zireael",
	}
}

func TestQuietOutsideHomeAndInRebase(t *testing.T) {
	s := act1(t)
	now := time.Date(2026, 10, 10, 20, 0, 0, 0, time.Local)
	st := state.New(1, "compressed", now.Add(-time.Hour))
	st.Chapter, st.Sitting, st.SittingPrompts, st.SittingStart, st.LastSeen = 1, 5, 10, now.Add(-time.Minute), now.Add(-time.Second)
	st.Force = "ls.phantom"

	g := &Engine{Story: s, Pace: sched.Get("compressed"), List: simList, Exists: func(p string) bool {
		return p == "/home/sim/code/rarepulls/.git" || p == "/home/sim/code/rarepulls/.git/rebase-merge"
	}}
	// Forced beats ignore the guards, so clear the force and fill the
	// hazard: only the rebase check can stop this one.
	st.Force = ""
	st.Prompts, st.LastEventPrompt = 1000, 0
	for i := 0; i < 50; i++ {
		env := baseEnv(now.Add(time.Duration(i) * time.Second))
		if eff := g.Tick(st, env); eff.Say != "" || len(eff.Acts) > 0 {
			t.Fatalf("fired during a rebase: %+v", eff)
		}
	}
	g.Exists = func(string) bool { return false }
	for i := 0; i < 50; i++ {
		env := baseEnv(now.Add(time.Duration(60+i) * time.Second))
		env.Cwd, env.CwdShown, env.UnderHome = "/etc", "/etc", false
		if eff := g.Tick(st, env); eff.Say != "" || len(eff.Acts) > 0 {
			t.Fatalf("fired outside $HOME: %+v", eff)
		}
	}
}

func TestForce(t *testing.T) {
	s := act1(t)
	now := time.Now()
	st := state.New(1, "normal", now)
	st.Force = "ls.phantom"
	g := &Engine{Story: s, Pace: sched.Get("normal"), List: simList}
	eff := g.Tick(st, baseEnv(now))
	if !strings.Contains(eff.Say, ".") || st.Force != "" {
		t.Fatalf("forced phantom didn't fire: %q (force %q)", eff.Say, st.Force)
	}
	if len(st.Fired) != 0 {
		t.Fatal("a forced beat must not count as story progress")
	}
}

func TestFinishedIsSilent(t *testing.T) {
	s := act1(t)
	st := state.New(1, "compressed", time.Now().Add(-time.Hour))
	st.FinaleDone = true
	st.Force = "ls.phantom"
	g := &Engine{Story: s, Pace: sched.Get("compressed"), List: simList}
	if eff := g.Tick(st, baseEnv(time.Now())); eff.Say != "" || len(eff.Acts) > 0 {
		t.Fatalf("finished story still fired: %+v", eff)
	}
}

func TestClosest(t *testing.T) {
	st := state.New(1, "normal", time.Now())
	st.Cmds = map[string]int{"git": 40, "grep": 5, "go": 9, "gdb": 1}
	if got := closest(st, "gti"); got != "git" {
		t.Errorf("closest(gti) = %q, want git", got)
	}
	if got := closest(st, "kubectl"); got != "" {
		t.Errorf("closest(kubectl) = %q, want nothing", got)
	}
}

// Hand-offs belong to the shell that got them: another terminal's prompt
// must not eat this one's ghost or its title pop.
func TestHandoffsPerShell(t *testing.T) {
	s := act1(t)
	now := time.Now()
	st := state.New(1, "normal", now)
	g := &Engine{Story: s, Pace: sched.Get("normal"), List: simList}

	a := baseEnv(now)
	a.Pid, a.Styled, a.Cmd, a.Shape = "100", true, "true", mech.ParseShape("-")
	st.Force, st.ForceText = "history.ghost", "ls  # hello"
	if eff := g.Tick(st, a); len(eff.Acts) != 1 || eff.Acts[0].Verb != "ghost" {
		t.Fatalf("ghost not armed: %+v", eff)
	}
	st.Force, st.ForceText = "title.whisper", "hello"
	b := baseEnv(now.Add(time.Second))
	b.Pid, b.Styled, b.Cmd, b.Shape = "200", true, "true", mech.ParseShape("-")
	if eff := g.Tick(st, b); !eff.Title {
		t.Fatalf("title not pushed in shell b: %+v", eff)
	}
	if h := st.Handoffs["100"]; h == nil || h.Ghost != "ls  # hello" {
		t.Fatalf("shell b's prompt ate shell a's ghost: %+v", st.Handoffs)
	}
	a.Now = now.Add(2 * time.Second)
	if eff := g.Tick(st, a); strings.Contains(eff.Say, mech.TitlePop) {
		t.Fatal("shell a popped shell b's title")
	}
	if st.Handoffs["100"] != nil {
		t.Fatal("shell a's spent ghost wasn't cleared")
	}
	b.Now = now.Add(3 * time.Second)
	if eff := g.Tick(st, b); !strings.Contains(eff.Say, mech.TitlePop) {
		t.Fatal("shell b didn't pop its own title")
	}
}
