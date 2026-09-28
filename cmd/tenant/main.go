// Command tenant is a slow-burn horror story that lives in your shell.
//
// See the README for what it does, and docs/SECURITY.md for what it can't.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Tazwinator/Tenant/internal/audit"
	"github.com/Tazwinator/Tenant/internal/mech"
	"github.com/Tazwinator/Tenant/internal/sandbox"
	"github.com/Tazwinator/Tenant/internal/script"
	"github.com/Tazwinator/Tenant/story"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "0.1.0-dev"

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	if len(args) == 0 {
		return cmdBare()
	}
	rest := args[1:]
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Print(helpText)
		return 0
	case "version", "--version":
		fmt.Println("tenant", version)
		return 0
	case "init":
		return cmdInit(rest)
	case "start":
		return cmdStart(rest)
	case "evict":
		return cmdEvict(rest)
	case "confess":
		return cmdConfess(rest)
	case "doctor":
		return cmdDoctor(rest)
	case "_hook":
		return cmdHook(rest)
	case "_ghost":
		return cmdHandoff("ghost")
	case "_text":
		return cmdHandoff("time")
	case "_force", "_sim", "_schedule", "_invite":
		if os.Getenv("TENANT_DEV") != "1" {
			fmt.Fprintln(os.Stderr, "tenant: developer command; set TENANT_DEV=1 (contains spoilers)")
			return 2
		}
		return cmdDev(args[0], rest)
	}
	fmt.Fprintf(os.Stderr, "tenant: unknown command %q\n\n%s", args[0], helpText)
	return 2
}

const helpText = `tenant: something has been living in your shell. It can only read names.

  tenant init bash|zsh          print the hook; add eval "$(tenant init bash)" to ~/.bashrc
  tenant start [--compressed]   begin (nothing happens until you run this)
  tenant evict                  the safeword: stop everything and delete all state
  tenant confess [--summary]    everything tenant has ever looked at
  tenant doctor                 check your setup (no spoilers)
  tenant help                   this

It never reads the contents of your files or your history file, never
writes outside ~/.local/state/tenant, and never touches the network.
`

// env is what every command needs.
type env struct {
	fs    *audit.FS
	paths audit.Paths
	box   sandbox.Status
}

// setup resolves paths, loads what has to be loaded before the sandbox (the
// time zone), and restricts the process. extra rules are added to the
// default: read names under $HOME, and use the state directory.
func setup(names bool, extra ...sandbox.Rule) (*env, error) {
	p, err := audit.Resolve()
	if err != nil {
		return nil, err
	}
	_ = time.Local.String() // load the zone now, before the sandbox shuts /etc
	rules := []sandbox.Rule{{Path: p.State, Access: sandbox.OwnDir}}
	if names {
		rules = append(rules, sandbox.Rule{Path: p.Home, Access: sandbox.ReadDir})
	}
	rules = append(rules, extra...)
	e := &env{fs: audit.New(p), paths: p}
	e.box = sandbox.Restrict(rules)
	return e, nil
}

func loadStory() (*script.Story, error) { return script.Load(story.Act1(), mech.Known) }

func isRoot() bool { return os.Geteuid() == 0 }

func hostname() string {
	var u syscall.Utsname
	if syscall.Uname(&u) != nil {
		return ""
	}
	var b strings.Builder
	for _, c := range u.Nodename {
		if c == 0 {
			break
		}
		b.WriteByte(byte(c))
	}
	h, _, _ := strings.Cut(b.String(), ".")
	return h
}

func username() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return os.Getenv("LOGNAME")
}

// rcLine is the line the player adds to their rc file.
func rcLine(sh string) string { return fmt.Sprintf(`eval "$(tenant init %s)"`, sh) }

func rcFile(sh string) string {
	if sh == "zsh" {
		return "~/.zshrc"
	}
	return "~/.bashrc"
}

// currentShell guesses the player's shell for hints.
func currentShell() string {
	if h := os.Getenv("TENANT_HOOK"); h == "bash" || h == "zsh" {
		return h
	}
	if filepath.Base(os.Getenv("SHELL")) == "zsh" {
		return "zsh"
	}
	return "bash"
}
