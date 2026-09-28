package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/Tazwinator/Tenant/internal/audit"
	"github.com/Tazwinator/Tenant/internal/engine"
	"github.com/Tazwinator/Tenant/internal/mech"
	"github.com/Tazwinator/Tenant/internal/sched"
	"github.com/Tazwinator/Tenant/internal/shell"
	"github.com/Tazwinator/Tenant/internal/state"
	"github.com/Tazwinator/Tenant/internal/term"
)

var (
	cmdName = regexp.MustCompile(`^[A-Za-z0-9._+:@-]{1,40}$`)
	pidRE   = regexp.MustCompile(`^[0-9]{1,10}$`)
)

// deadline is how long a hook call may take before it gives up and does
// nothing. The prompt matters more than the story.
func deadline() time.Duration {
	if ms, err := strconv.Atoi(os.Getenv("TENANT_DEADLINE_MS")); err == nil && ms > 0 {
		return time.Duration(ms) * time.Millisecond
	}
	return 25 * time.Millisecond
}

// cmdInit prints the hook script.
func cmdInit(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: tenant init bash|zsh")
		return 2
	}
	bin, err := os.Executable()
	if err != nil {
		bin = "tenant"
	}
	s, err := shell.Script(args[0], bin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tenant:", err)
		return 2
	}
	fmt.Print(s)
	return 0
}

// cmdHook is called by the hook on every prompt, and by the not-found
// handler. It must be fast, and it must never fail loudly.
func cmdHook(args []string) int {
	if len(args) == 0 || os.Getenv("TENANT_OFF") != "" || isRoot() {
		return 0
	}
	kind := args[0]
	if kind != "prompt" && kind != "notfound" {
		return 0
	}
	fl := flag.NewFlagSet("_hook", flag.ContinueOnError)
	fl.SetOutput(io.Discard)
	sh := fl.String("shell", "bash", "")
	pid := fl.String("pid", "0", "")
	status := fl.Int("status", 0, "")
	cmd := fl.String("cmd", "", "")
	shape := fl.String("shape", "-", "")
	first := fl.Int("first", 0, "")
	cols := fl.Int("cols", 80, "")
	caps := fl.Int("caps", 0, "")
	if fl.Parse(args[1:]) != nil {
		return 0
	}
	if *sh != "bash" && *sh != "zsh" {
		return 0
	}
	if !cmdName.MatchString(*cmd) {
		*cmd = ""
	}
	if !pidRE.MatchString(*pid) {
		*pid = "0"
	}

	timer := time.AfterFunc(deadline(), func() { os.Exit(0) })
	e, err := setup(true)
	if err != nil || !e.fs.Active() {
		return 0
	}
	s, err := loadStory()
	if err != nil {
		return 0
	}
	unlock, err := e.fs.Lock()
	if err != nil {
		return 0
	}
	defer unlock()
	if !e.fs.Active() { // evicted while we waited for the lock
		return 0
	}
	st, err := state.Load(e.fs)
	if err != nil {
		return 0
	}
	g := &engine.Engine{Story: s, Pace: sched.Get(st.Pace), List: e.fs.ListDir, Exists: e.fs.Exists}
	env := buildEnv(e.paths, kind, *sh, *status, *cmd, *shape, *first == 1, *cols, *caps)
	env.TTY = e.fs.TTY()
	env.Pid = *pid
	eff := g.Tick(st, env)
	// Past this point nothing may be cut short: what is saved gets shown.
	if !timer.Stop() || state.Save(e.fs, st) != nil {
		return 0
	}
	if eff.Say != "" {
		io.WriteString(os.Stderr, eff.Say)
	}
	io.WriteString(os.Stdout, shell.Encode(eff.Acts))
	return 0
}

func buildEnv(p audit.Paths, kind, sh string, status int, cmd, shape string, first bool, cols, caps int) *mech.Env {
	cwd, err := os.Getwd()
	under := err == nil && p.UnderHome(cwd)
	return &mech.Env{
		Now: time.Now(), Shell: sh, Kind: kind, Status: status, Cmd: cmd,
		Shape: mech.ParseShape(shape), First: first, Caps: mech.Caps(caps), Cols: cols,
		Home: p.Home, Cwd: cwd, CwdShown: p.Tilde(cwd), UnderHome: under,
		User: username(), Host: hostname(), LSColors: os.Getenv("LS_COLORS"),
		Styled: term.IsTTY(2),
	}
}

// cmdHandoff prints text the last prompt left for this shell (the ghost
// command, or the time), once.
func cmdHandoff(what string, args []string) int {
	if isRoot() {
		return 0
	}
	pid := "0"
	for i, a := range args {
		if a == "--pid" && i+1 < len(args) && pidRE.MatchString(args[i+1]) {
			pid = args[i+1]
		}
	}
	timer := time.AfterFunc(deadline(), func() { os.Exit(0) })
	e, err := setup(false)
	if err != nil || !e.fs.Active() {
		return 0
	}
	unlock, err := e.fs.Lock()
	if err != nil {
		return 0
	}
	defer unlock()
	st, err := state.Load(e.fs)
	if err != nil {
		return 0
	}
	h := st.Handoffs[pid]
	if h == nil {
		return 0
	}
	var out string
	switch what {
	case "ghost":
		out, h.Ghost = h.Ghost, ""
	case "time":
		out, h.Time = h.Time, ""
	}
	if out == "" || !timer.Stop() || state.Save(e.fs, st) != nil {
		return 0
	}
	fmt.Println(term.Clean(out, 120))
	return 0
}
