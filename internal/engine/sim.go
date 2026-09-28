package engine

import (
	"fmt"
	"io"
	"math/rand/v2"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Tazwinator/Tenant/internal/audit"
	"github.com/Tazwinator/Tenant/internal/mech"
	"github.com/Tazwinator/Tenant/internal/sched"
	"github.com/Tazwinator/Tenant/internal/script"
	"github.com/Tazwinator/Tenant/internal/state"
)

// A simulation drives the engine with synthetic usage and a fake clock, to
// tune pacing without living through it. It never touches the filesystem.

// Profile describes a kind of user.
type Profile struct {
	Name     string
	Sittings func(day time.Time) [][2]time.Duration // start and end offsets from midnight
	Rate     time.Duration                          // average time between commands
}

// Profiles are the built-in usage profiles.
var Profiles = map[string]Profile{
	"daily": {Name: "daily", Rate: 3 * time.Minute, Sittings: func(day time.Time) [][2]time.Duration {
		return [][2]time.Duration{{8*time.Hour + 30*time.Minute, 10 * time.Hour}, {12*time.Hour + 30*time.Minute, 13*time.Hour + 15*time.Minute},
			{18 * time.Hour, 19*time.Hour + 30*time.Minute}, {21*time.Hour + 30*time.Minute, 23*time.Hour + 45*time.Minute}}
	}},
	"evenings": {Name: "evenings", Rate: 3 * time.Minute, Sittings: func(day time.Time) [][2]time.Duration {
		return [][2]time.Duration{{20 * time.Hour, 22*time.Hour + 30*time.Minute}}
	}},
	"weekend": {Name: "weekend", Rate: 3 * time.Minute, Sittings: func(day time.Time) [][2]time.Duration {
		if wd := day.Weekday(); wd != time.Saturday && wd != time.Sunday {
			return nil
		}
		return [][2]time.Duration{{10 * time.Hour, 12 * time.Hour}, {15 * time.Hour, 17 * time.Hour}, {21 * time.Hour, 23 * time.Hour}}
	}},
	"steady": {Name: "steady", Rate: 25 * time.Second, Sittings: func(day time.Time) [][2]time.Duration {
		return [][2]time.Duration{{9 * time.Hour, 22 * time.Hour}}
	}},
}

// Event is one thing that happened in a simulation.
type Event struct {
	At      time.Time
	Beat    string
	Chapter int
	Sitting int
	Say     string
}

// Result is a whole simulated run.
type Result struct {
	Start   time.Time
	Events  []Event
	Invited time.Time // zero if the invitation never came
	Chapter int       // chapter reached
}

var simTree = map[string][]string{
	"/home/sim/code/rarepulls": {"cmd/", "internal/", "web/", "go.mod", "go.sum", "README.md"},
	"/home/sim/code/tenant":    {"cmd/", "docs/", "story/", "go.mod", "LICENSE"},
	"/home/sim/notes":          {"todo.md", "ideas.txt", "recipes.md"},
	"/home/sim/Downloads":      {"invoice-2026-09.pdf", "archlinux-2026.10.01-x86_64.iso"},
}

var simCmds = []struct {
	cmd, shape string
	weight     int
}{
	{"ls", "ls:pc", 15}, {"ll", "ls:lc", 4}, {"cd", "cd", 10}, {"git", "-", 25}, {"nvim", "-", 14},
	{"go", "-", 10}, {"make", "-", 4}, {"clear", "clear", 4}, {"cat", "-", 5}, {"rg", "-", 5}, {"htop", "-", 1},
	{"gti", "notfound", 1},
}

// Simulate runs a profile for up to days days (or until the invitation).
func Simulate(story *script.Story, p sched.Pace, prof Profile, seed uint64, days int) Result {
	loc := time.Local
	start := time.Date(2026, 10, 1, 9, 0, 0, 0, loc) // a Thursday
	st := state.New(seed, p.Name, start)
	g := &Engine{Story: story, Pace: p, List: simList, Exists: func(string) bool { return false }}
	rng := rand.New(rand.NewPCG(seed, 0x7e4a47))
	total := 0
	for _, c := range simCmds {
		total += c.weight
	}
	dirs := make([]string, 0, len(simTree))
	for d := range simTree {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	res := Result{Start: start}
	cwd := dirs[0]
	for d := 0; d < days && res.Invited.IsZero(); d++ {
		day := time.Date(2026, 10, 1+d, 0, 0, 0, 0, loc)
		for _, span := range prof.Sittings(day) {
			t := day.Add(span[0])
			first := true
			for t.Before(day.Add(span[1])) && res.Invited.IsZero() {
				if t.Before(start) {
					t = t.Add(prof.Rate)
					continue
				}
				pick := rng.IntN(total)
				var cmd, shape string
				for _, c := range simCmds {
					if pick < c.weight {
						cmd, shape = c.cmd, c.shape
						break
					}
					pick -= c.weight
				}
				if cmd == "cd" {
					cwd = dirs[rng.IntN(len(dirs))]
				}
				env := &mech.Env{
					Now: t, Shell: "bash", Pid: "1", Kind: "prompt", Cmd: cmd, Shape: mech.ParseShape(shape),
					First: first || rng.IntN(15) == 0, Caps: mech.Caps(63), Cols: 100,
					Home: "/home/sim", Cwd: cwd, CwdShown: "~" + strings.TrimPrefix(cwd, "/home/sim"),
					UnderHome: true, User: "sim", Host: "zireael", TTY: "pts/1", Styled: true,
				}
				if shape == "notfound" {
					env.Kind, env.Shape, env.First = "notfound", mech.Shape{Kind: "other"}, false
				}
				before := len(st.Fired)
				eff := g.Tick(st, env)
				if len(st.Fired) > before {
					f := st.Fired[len(st.Fired)-1]
					say := ansi.ReplaceAllString(eff.Say+eff.Ghost+eff.Time, "")
					res.Events = append(res.Events, Event{At: t, Beat: f.ID, Chapter: f.Chapter, Sitting: st.Sitting, Say: strings.TrimSpace(say)})
				}
				if st.Invited && res.Invited.IsZero() {
					res.Invited = t
				}
				first = false
				t = t.Add(time.Duration(float64(prof.Rate) * (0.3 + 1.4*rng.Float64())))
			}
		}
	}
	res.Chapter = st.Chapter
	return res
}

// ansi matches the escape sequences the mechanics print, so a timeline
// stays readable.
var ansi = regexp.MustCompile(`\x1b(\[[0-9;]*[a-zA-Z]|\][^\a]*\a)`)

func simList(dir string) ([]audit.Name, error) {
	names, ok := simTree[filepath.Clean(dir)]
	if !ok {
		return nil, fmt.Errorf("no such directory")
	}
	out := make([]audit.Name, 0, len(names))
	for _, n := range names {
		out = append(out, audit.Name{Name: strings.TrimSuffix(n, "/"), Dir: strings.HasSuffix(n, "/")})
	}
	return out, nil
}

// Print writes a simulation's timeline. It is full of spoilers.
func (r Result) Print(w io.Writer) {
	for _, e := range r.Events {
		fmt.Fprintf(w, "%s  day %d  sitting %-3d ch%d  %-15s %s\n", e.At.Format("Mon 15:04"),
			int(e.At.Sub(r.Start).Hours()/24)+1, e.Sitting, e.Chapter, e.Beat, e.Say)
	}
	if r.Invited.IsZero() {
		fmt.Fprintf(w, "no invitation; reached chapter %d\n", r.Chapter)
		return
	}
	fmt.Fprintf(w, "invited after %s\n", r.Invited.Sub(r.Start).Round(time.Minute))
}
