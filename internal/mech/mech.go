// Package mech holds the mechanics: every kind of wrong thing tenant knows
// how to do. A mechanic never touches the filesystem and never writes shell
// code. It returns text for the terminal and protocol actions (a verb and
// integers) for the hook to carry out.
package mech

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

// Caps are what the hook found out about its shell. They arrive as a bitmask.
type Caps uint

const (
	CapCwdPrompt Caps = 1 << iota // the prompt shows the cwd through a token tenant can swap
	CapUp                         // the Up arrow can be borrowed for one press
	CapUTF8                       // the locale is UTF-8
	CapTitle                      // the terminal takes window titles
	CapRight                      // a right-aligned time can be shown
	CapNotFound                   // the not-found handler is wrapped
)

// CapNames names each capability, for doctor.
var CapNames = []struct {
	Cap  Caps
	Name string
}{
	{CapCwdPrompt, "directory in prompt"},
	{CapUp, "Up arrow"},
	{CapUTF8, "UTF-8"},
	{CapTitle, "window title"},
	{CapRight, "right-aligned prompt"},
	{CapNotFound, "not-found handler"},
}

// Shape is what kind of command just ran, as classified by the hook.
type Shape struct {
	Kind  string // "" (none), "ls", "clear", "cd" or "other"
	Long  bool   // ls -l
	Color bool   // ls colours its output
	All   bool   // ls -a
	Bad   bool   // ls with paths, pipes or icons: not safe to imitate
}

// ParseShape reads the hook's shape token: "ls:p", "ls:lca", "ls:x",
// "clear", "cd" or "-".
func ParseShape(s string) Shape {
	switch {
	case s == "" || s == "-":
		return Shape{Kind: "other"}
	case s == "clear" || s == "cd":
		return Shape{Kind: s}
	case strings.HasPrefix(s, "ls:"):
		f := s[3:]
		if f == "" || strings.Contains(f, "x") {
			return Shape{Kind: "ls", Bad: true}
		}
		return Shape{Kind: "ls", Long: strings.Contains(f, "l"), Color: strings.Contains(f, "c"), All: strings.Contains(f, "a")}
	}
	return Shape{Kind: "other"}
}

// Env is everything a mechanic may know about the moment.
type Env struct {
	Now       time.Time
	Shell     string // bash or zsh
	Kind      string // "prompt" or "notfound"
	Status    int
	Cmd       string // first word of the last command, or the missing command
	Shape     Shape
	First     bool // first prompt of this shell
	Caps      Caps
	Cols      int
	Home      string
	Cwd       string
	CwdShown  string // the cwd as the prompt shows it, with ~
	UnderHome bool
	User      string
	Host      string
	TTY       string
	LSColors  string
	Styled    bool      // stderr is a terminal, so colours are allowed
	LateStamp time.Time // the last late-night visit, if any
	Roll      func(purpose string) uint64
}

// Action is one protocol line for the hook: a verb and integers only.
type Action struct {
	Verb string
	Args []int
}

func (a Action) String() string {
	parts := []string{a.Verb}
	for _, n := range a.Args {
		parts = append(parts, strconv.Itoa(n))
	}
	return strings.Join(parts, " ")
}

// Effect is what firing a mechanic produces.
type Effect struct {
	Say   string   // written to the terminal (stderr) as is
	Acts  []Action // protocol lines for the hook (stdout)
	Ghost string   // held for `tenant _ghost`
	Time  string   // held for `tenant _text time`
	Title bool     // a window title was pushed, and should be popped next prompt
}

// Merge appends another effect.
func (e *Effect) Merge(o Effect) {
	e.Say += o.Say
	e.Acts = append(e.Acts, o.Acts...)
	if o.Ghost != "" {
		e.Ghost = o.Ghost
	}
	if o.Time != "" {
		e.Time = o.Time
	}
	e.Title = e.Title || o.Title
}

// Mechanic is one kind of wrong thing.
type Mechanic interface {
	ID() string
	// Probe reports whether this shell can show it.
	Probe(e *Env) bool
	// Triggered reports whether the right thing just happened.
	Triggered(e *Env) bool
	// Weight makes rare triggers (a typo, a clear) more likely to fire when
	// they do happen. Mechanics that can fire on any prompt weigh 1.
	Weight() float64
	// NeedsText reports whether the beat must supply text.
	NeedsText() bool
	// Fire produces the effect. ok is false if it can't work right now.
	Fire(e *Env, text string) (eff Effect, ok bool)
}

var registry = map[string]Mechanic{}

func register(m Mechanic) { registry[m.ID()] = m }

// Get returns a mechanic by ID.
func Get(id string) (Mechanic, bool) {
	m, ok := registry[id]
	return m, ok
}

// Known reports whether a mechanic exists.
func Known(id string) bool {
	_, ok := registry[id]
	return ok
}

// IDs lists every mechanic.
func IDs() []string {
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
