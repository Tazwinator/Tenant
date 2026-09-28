package main

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Tazwinator/Tenant/internal/audit"
	"github.com/Tazwinator/Tenant/internal/engine"
	"github.com/Tazwinator/Tenant/internal/finale"
	"github.com/Tazwinator/Tenant/internal/mech"
	"github.com/Tazwinator/Tenant/internal/sandbox"
	"github.com/Tazwinator/Tenant/internal/sched"
	"github.com/Tazwinator/Tenant/internal/script"
	"github.com/Tazwinator/Tenant/internal/state"
	"github.com/Tazwinator/Tenant/internal/term"
)

// cmdStart is consent. Nothing happens anywhere until it has run.
func cmdStart(args []string) int {
	pace := "normal"
	for _, a := range args {
		switch {
		case a == "--compressed":
			pace = "compressed"
		case strings.HasPrefix(a, "--pace="):
			pace = strings.TrimPrefix(a, "--pace=")
		default:
			fmt.Fprintf(os.Stderr, "tenant start: unknown option %q\n", a)
			return 2
		}
	}
	if _, ok := sched.Paces[pace]; !ok {
		fmt.Fprintf(os.Stderr, "tenant start: unknown pace %q (normal, compressed, tester)\n", pace)
		return 2
	}
	if isRoot() {
		fmt.Fprintln(os.Stderr, "tenant doesn't run as root. Start it as yourself.")
		return 1
	}
	p, err := audit.Resolve()
	if err != nil {
		fmt.Fprintln(os.Stderr, "tenant:", err)
		return 1
	}
	if err := audit.New(p).Prepare(); err != nil {
		fmt.Fprintln(os.Stderr, "tenant:", err)
		return 1
	}
	e, err := setup(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tenant:", err)
		return 1
	}
	if e.fs.Active() {
		fmt.Println("tenant is already here. `tenant evict` removes it.")
		return 1
	}
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		fmt.Fprintln(os.Stderr, "tenant:", err)
		return 1
	}
	st := state.New(binary.LittleEndian.Uint64(b[:]), pace, time.Now())
	if err := state.Save(e.fs, st); err != nil {
		fmt.Fprintln(os.Stderr, "tenant:", err)
		return 1
	}
	e.fs.Record("started", pace+" pace")
	if err := e.fs.Activate(); err != nil {
		fmt.Fprintln(os.Stderr, "tenant:", err)
		return 1
	}
	when := "Nothing will happen for a day or two. Then, rarely, small things."
	switch pace {
	case "compressed":
		when = "Compressed mode: the whole story plays out in about 30 minutes of steady use."
	case "tester":
		when = "Tester pace: quiet for about 12 hours, then a chapter every sitting."
	}
	sh := currentShell()
	fmt.Printf(`tenant has started (%s pace).

  %s

  It reads the names of directories under your home, never the contents of
  a file, and never your history file. It writes only to %s.
  It stays quiet as root, outside your home, and in shells with TENANT_OFF=1.

  tenant confess   everything it has looked at, any time
  tenant evict     stop it in every shell and delete everything
  tenant doctor    check your setup (no spoilers)
`, pace, when, e.paths.Tilde(e.paths.State))
	if os.Getenv("TENANT_HOOK") == "" {
		fmt.Printf("\n  The hook isn't loaded in this shell. Add this as the last line of %s:\n\n    %s\n", rcFile(sh), rcLine(sh))
	}
	return 0
}

// cmdEvict is the safeword. It is always out of character.
func cmdEvict(args []string) int {
	p, err := audit.Resolve()
	if err != nil {
		fmt.Fprintln(os.Stderr, "tenant:", err)
		return 1
	}
	// Removing its own directory needs one extra right on the parent.
	e, err := setup(false, sandbox.Rule{Path: filepath.Dir(p.State), Access: sandbox.RemoveDir})
	if err != nil {
		fmt.Fprintln(os.Stderr, "tenant:", err)
		return 1
	}
	existed := e.fs.StateExists()
	if err := e.fs.Evict(); err != nil {
		fmt.Fprintln(os.Stderr, "tenant: couldn't remove", e.paths.Tilde(e.paths.State)+":", err)
		return 1
	}
	sh := currentShell()
	if existed {
		fmt.Printf("tenant has been evicted.\n\n  Deleted %s: its state, its notes and its audit log.\n  Every open shell switches the hook off at its next prompt.\n",
			e.paths.Tilde(e.paths.State))
	} else {
		fmt.Println("tenant isn't here. There was nothing to delete.")
	}
	fmt.Printf("\n  To finish, remove this line from %s:\n\n    %s\n\n  and uninstall the package.\n", rcFile(sh), rcLine(sh))
	return 0
}

// cmdConfess prints the audit log. The story may add a passage after it,
// never inside it.
func cmdConfess(args []string) int {
	summary := len(args) > 0 && args[0] == "--summary"
	e, err := setup(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tenant:", err)
		return 1
	}
	st, err := state.Load(e.fs)
	if err != nil {
		fmt.Println("tenant hasn't looked at anything. It hasn't been started.")
		return 0
	}
	entries, _ := e.fs.Entries()
	seen, _ := e.fs.Seen()
	now := time.Now()
	stamp := func(t time.Time) string { return t.Local().Format("2006-01-02 15:04") }

	fmt.Println("tenant confess: everything it has looked at.")
	fmt.Println("Each entry is written before the access it describes. The story never edits this log.")
	fmt.Println()
	fmt.Println("standing behaviour")
	fmt.Println("  every prompt  noted the current directory and the first word of the command you ran")
	fmt.Println("  every prompt  checked whether a git rebase, merge or bisect was in progress (marker names only)")
	fmt.Println("  every prompt  looked up which terminal it was running on")
	fmt.Println("  every run     read the system time zone (/etc/localtime)")
	fmt.Printf("  always        kept its own notes in %s\n", e.paths.Tilde(e.paths.State))
	fmt.Println()
	fmt.Println("log")
	wrote := 0
	for _, en := range entries {
		if en.Kind == "wrote" {
			wrote++
		}
		fmt.Printf("%s  %-8s  %s\n", stamp(en.At), en.Kind, term.Clean(en.Detail, 300))
	}
	names := 0
	for _, ns := range seen {
		names += len(ns)
	}
	if !summary {
		dirs := make([]string, 0, len(seen))
		for d := range seen {
			dirs = append(dirs, d)
		}
		sort.Strings(dirs)
		fmt.Printf("\nnames read (%d, in %d directories)\n", names, len(dirs))
		for _, d := range dirs {
			fmt.Printf("  %s\n", term.Clean(d, 300))
			var clean []string
			for _, n := range seen[d] {
				clean = append(clean, term.Clean(n, 100))
			}
			fmt.Printf("    %s\n", strings.Join(clean, "  "))
		}
		fmt.Printf("\ncommands observed (names only)\n")
		type kv struct {
			cmd string
			n   int
		}
		var cmds []kv
		for c, n := range st.Cmds {
			cmds = append(cmds, kv{c, n})
		}
		sort.Slice(cmds, func(i, j int) bool {
			if cmds[i].n != cmds[j].n {
				return cmds[i].n > cmds[j].n
			}
			return cmds[i].cmd < cmds[j].cmd
		})
		var parts []string
		for _, c := range cmds {
			parts = append(parts, fmt.Sprintf("%s ×%d", term.Clean(c.cmd, 40), c.n))
		}
		fmt.Printf("  %s\n", strings.Join(parts, "  "))
	}
	fmt.Println()
	fmt.Printf("%s  observed %d commands (names only)\n", stamp(now), st.CommandsSeen())
	if wrote == 0 {
		fmt.Printf("%s  wrote nothing\n", stamp(now))
	} else {
		fmt.Printf("%s  wrote %d times outside its own directory (listed above)\n", stamp(now), wrote)
	}

	if st.FinaleDone {
		s, err := loadStory()
		if err != nil {
			return 0
		}
		g := &engine.Engine{Story: s, Pace: sched.Get(st.Pace)}
		env := buildEnv(e.paths, "prompt", currentShell(), 0, "", "-", false, 80, 0)
		base := g.Vars(st, env)
		vars := func(name string) (string, bool) {
			if name == "seen.count" {
				return fmt.Sprint(names), true
			}
			return base(name)
		}
		fmt.Println()
		fmt.Println("────────────────────────────────────────")
		fmt.Println()
		for _, line := range s.Epilogue {
			if text, ok := script.Render(line, vars); ok {
				fmt.Println(term.Clean(text, 400))
			}
		}
	}
	return 0
}

// cmdDoctor checks the setup without spoiling anything.
func cmdDoctor(args []string) int {
	e, err := setup(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tenant:", err)
		return 1
	}
	row := func(k, v string) { fmt.Printf("  %-12s %s\n", k, v) }
	fmt.Println("tenant doctor")
	fmt.Println()
	row("version", version)
	st, err := state.Load(e.fs)
	switch {
	case isRoot():
		row("status", "running as root: tenant stays quiet")
	case err != nil || !e.fs.Active():
		row("status", "not started (run `tenant start`)")
	case st.FinaleDone:
		row("status", "the story is finished. `tenant evict` removes everything.")
	default:
		row("status", fmt.Sprintf("active since %s, %s pace", st.Started.Local().Format("Mon 2 Jan 15:04"), st.Pace))
	}
	if h := os.Getenv("TENANT_HOOK"); h != "" {
		row("hook", h+", loaded in this shell")
	} else {
		sh := currentShell()
		row("hook", fmt.Sprintf("not loaded in this shell; add %s to %s", rcLine(sh), rcFile(sh)))
	}
	if st != nil {
		shells := make([]string, 0, len(st.Caps))
		for sh := range st.Caps {
			shells = append(shells, sh)
		}
		sort.Strings(shells)
		for _, sh := range shells {
			c := st.Caps[sh]
			var yes, no []string
			for _, n := range mech.CapNames {
				if mech.Caps(c.Bits)&n.Cap != 0 {
					yes = append(yes, n.Name)
				} else {
					no = append(no, n.Name)
				}
			}
			row(sh, fmt.Sprintf("last prompt %s ago", time.Since(c.At).Round(time.Second)))
			row("", "can show: "+orNone(yes))
			row("", "can't show: "+orNone(no))
		}
	}
	row("sandbox", e.box.String())
	if st != nil {
		row("latency", latency(e.paths, st))
	}
	row("history", "never read")
	row("network", "no network code in this binary")
	return 0
}

func orNone(s []string) string {
	if len(s) == 0 {
		return "(none)"
	}
	return strings.Join(s, ", ")
}

// latency times the work tenant does on a prompt (a tick on a copy of the
// state, then encoding it), without listing anything or saving. Process
// start comes on top.
func latency(p audit.Paths, st *state.State) string {
	s, err := loadStory()
	if err != nil {
		return "unknown"
	}
	var times []time.Duration
	for i := 0; i < 100; i++ {
		cp := *st
		cp.Cmds, cp.Dirs, cp.Days, cp.Caps = copyMap(st.Cmds), copyDirs(st.Dirs), copyMap(st.Days), map[string]state.Caps{}
		cp.Fired = append([]state.Fired(nil), st.Fired...)
		cp.Force = ""
		g := &engine.Engine{Story: s, Pace: sched.Get(st.Pace)}
		env := &mech.Env{Now: time.Now(), Kind: "prompt", Shell: "bash", Cmd: "git", Shape: mech.ParseShape("-"),
			Caps: mech.Caps(63), Home: p.Home, Cwd: p.Home + "/x", CwdShown: "~/x", UnderHome: true}
		t0 := time.Now()
		g.Tick(&cp, env)
		json.Marshal(&cp)
		times = append(times, time.Since(t0))
	}
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	ms := func(d time.Duration) string { return fmt.Sprintf("%.2f ms", float64(d)/float64(time.Millisecond)) }
	return fmt.Sprintf("tenant's own work p50 %s, p95 %s per prompt, plus process start", ms(times[50]), ms(times[95]))
}

func copyMap(m map[string]int) map[string]int {
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func copyDirs(m map[string]*state.Dir) map[string]*state.Dir {
	out := make(map[string]*state.Dir, len(m))
	for k, v := range m {
		d := *v
		out[k] = &d
	}
	return out
}

// cmdBare is `tenant` on its own: help, or the finale once it has been
// invited.
func cmdBare() int {
	e, err := setup(true)
	if err != nil || isRoot() {
		fmt.Print(helpText)
		return 0
	}
	st, err := state.Load(e.fs)
	if err != nil || !e.fs.Active() || !st.Invited || st.FinaleDone {
		fmt.Print(helpText)
		if err == nil && st.FinaleDone {
			fmt.Println("\nThe story is finished. `tenant confess` has the ending; `tenant evict` removes everything.")
		}
		return 0
	}
	s, err := loadStory()
	if err != nil {
		fmt.Print(helpText)
		return 0
	}
	g := &engine.Engine{Story: s, Pace: sched.Get(st.Pace), List: e.fs.ListDir}
	env := buildEnv(e.paths, "prompt", currentShell(), 0, "", "-", false, 80, 0)
	r := &finale.Runner{D: s.Finale, Vars: g.Vars(st, env), In: os.Stdin, Out: os.Stdout, CPS: 40, Slow: true}
	fmt.Println()
	if !r.Run() {
		return 0
	}
	unlock, err := e.fs.Lock()
	if err != nil {
		return 1
	}
	defer unlock()
	if st, err = state.Load(e.fs); err == nil {
		st.FinaleDone = true
		state.Save(e.fs, st)
	}
	return 0
}

// cmdDev holds the developer tools. They are full of spoilers.
func cmdDev(name string, args []string) int {
	switch name {
	case "_sim":
		prof, pace, seed := "daily", "normal", uint64(1)
		for _, a := range args {
			switch {
			case strings.HasPrefix(a, "--profile="):
				prof = strings.TrimPrefix(a, "--profile=")
			case strings.HasPrefix(a, "--pace="):
				pace = strings.TrimPrefix(a, "--pace=")
			case strings.HasPrefix(a, "--seed="):
				fmt.Sscan(strings.TrimPrefix(a, "--seed="), &seed)
			}
		}
		p, ok := engine.Profiles[prof]
		if !ok {
			fmt.Fprintln(os.Stderr, "profiles: daily, evenings, weekend, steady")
			return 2
		}
		s, err := loadStory()
		if err != nil {
			fmt.Fprintln(os.Stderr, "tenant:", err)
			return 1
		}
		sandbox.Restrict(nil)
		engine.Simulate(s, sched.Get(pace), p, seed, 21).Print(os.Stdout)
		return 0
	}
	e, err := setup(false)
	if err != nil || !e.fs.Active() {
		fmt.Fprintln(os.Stderr, "tenant: not started")
		return 1
	}
	unlock, err := e.fs.Lock()
	if err != nil {
		fmt.Fprintln(os.Stderr, "tenant:", err)
		return 1
	}
	defer unlock()
	st, err := state.Load(e.fs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tenant:", err)
		return 1
	}
	switch name {
	case "_force":
		if len(args) < 1 || len(args) > 2 || !mech.Known(args[0]) {
			fmt.Fprintln(os.Stderr, "usage: tenant _force <mechanic> [text]; one of:", strings.Join(mech.IDs(), ", "))
			return 2
		}
		st.Force, st.ForceText = args[0], ""
		if len(args) == 2 {
			st.ForceText = args[1]
		}
		if err := state.Save(e.fs, st); err != nil {
			fmt.Fprintln(os.Stderr, "tenant:", err)
			return 1
		}
		fmt.Println("armed", args[0], "for its next trigger")
	case "_invite":
		st.Invited = true
		if err := state.Save(e.fs, st); err != nil {
			fmt.Fprintln(os.Stderr, "tenant:", err)
			return 1
		}
		fmt.Println("invited: `tenant` now opens the finale")
	case "_schedule":
		fmt.Printf("pace %s, chapter %d, sitting %d (prompt %d of this sitting), %d events this sitting\n",
			st.Pace, st.Chapter, st.Sitting, st.SittingPrompts, st.SittingEvents)
		fmt.Printf("invited %v, finale done %v, stalled %v\n", st.Invited, st.FinaleDone, sched.Stalled(st, sched.Get(st.Pace)))
		for _, f := range st.Fired {
			fmt.Printf("  fired %-16s ch%d  %s\n", f.ID, f.Chapter, f.At.Local().Format("Mon 15:04"))
		}
	}
	return 0
}
