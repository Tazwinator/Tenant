package engine

import (
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/Tazwinator/Tenant/internal/audit"
	"github.com/Tazwinator/Tenant/internal/mech"
	"github.com/Tazwinator/Tenant/internal/sched"
	"github.com/Tazwinator/Tenant/internal/script"
	"github.com/Tazwinator/Tenant/internal/state"
	"github.com/Tazwinator/Tenant/internal/term"
)

// goodName is a name that looks natural in a terminal: no spaces, no
// quoting, not hidden.
var goodName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,39}$`)

// VarNames lists every template variable, for the story tests and docs.
var VarNames = []string{
	"user", "host", "cwd", "cwd.off", "names.here", "names.elsewhere",
	"last_night.dir", "last_night.time", "first_seen.dir",
	"cmd.top", "cmd.rare", "days", "typo", "typo.meant",
	"cmds.total", "dirs.count", "seen.count",
}

// vars resolves template variables lazily: names are only read when a beat
// that is about to fire needs them.
func (g *Engine) vars(st *state.State, env *mech.Env) script.Vars {
	p := audit.Paths{Home: env.Home}
	var here []audit.Name
	listedHere := false
	listHere := func() []audit.Name {
		if !listedHere && env.UnderHome && g.List != nil {
			here, _ = g.List(env.Cwd)
			listedHere = true
		}
		return here
	}
	return func(name string) (string, bool) {
		switch name {
		case "user":
			return nonEmpty(term.Clean(env.User, 32))
		case "host":
			return nonEmpty(term.Clean(env.Host, 64))
		case "cwd":
			return nonEmpty(env.CwdShown)
		case "cwd.off":
			if !env.UnderHome {
				return "", false
			}
			return mech.ShiftVowel(env.CwdShown, sched.Roll(st, "cwd.off"))
		case "names.here":
			return pick(st, "here", goodNames(listHere(), nil))
		case "names.elsewhere":
			return g.elsewhere(st, env, listHere())
		case "last_night.dir", "last_night.time":
			dir, at := lastNightDir(st, env.Now)
			if dir == "" {
				return "", false
			}
			if name == "last_night.dir" {
				return p.Tilde(dir), true
			}
			return at.Local().Format("15:04"), true
		case "first_seen.dir":
			if st.FirstDir == "" {
				return "", false
			}
			return p.Tilde(st.FirstDir), true
		case "cmd.top":
			top, _ := topAndRare(st)
			return nonEmpty(top)
		case "cmd.rare":
			_, rare := topAndRare(st)
			return nonEmpty(rare)
		case "days":
			d := int(env.Now.Sub(st.Started)/(24*time.Hour)) + 1
			return strconv.Itoa(d), true
		case "typo":
			if env.Kind != "notfound" {
				return "", false
			}
			return nonEmpty(env.Cmd)
		case "typo.meant":
			if env.Kind != "notfound" {
				return "", false
			}
			return nonEmpty(closest(st, env.Cmd))
		case "cmds.total":
			return strconv.Itoa(st.CommandsSeen()), true
		case "dirs.count":
			return strconv.Itoa(len(st.Dirs)), true
		}
		return "", false
	}
}

func nonEmpty(s string) (string, bool) { return s, s != "" }

func goodNames(names []audit.Name, exclude map[string]bool) []string {
	var out []string
	for _, n := range names {
		if !n.Dir && goodName.MatchString(n.Name) && !exclude[n.Name] {
			out = append(out, n.Name)
		}
	}
	return out
}

func pick(st *state.State, purpose string, names []string) (string, bool) {
	if len(names) == 0 {
		return "", false
	}
	return names[sched.Roll(st, "pick."+purpose)%uint64(len(names))], true
}

// elsewhere finds a real name from another directory you visited recently,
// one that isn't in the current directory.
func (g *Engine) elsewhere(st *state.State, env *mech.Env, here []audit.Name) (string, bool) {
	if g.List == nil {
		return "", false
	}
	exclude := map[string]bool{}
	for _, n := range here {
		exclude[n.Name] = true
	}
	type kv struct {
		dir  string
		last time.Time
	}
	var dirs []kv
	for dir, d := range st.Dirs {
		if dir != env.Cwd && dir != env.Home && filepath.IsAbs(dir) {
			dirs = append(dirs, kv{dir, d.Last})
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].last.After(dirs[j].last) })
	for i, d := range dirs {
		if i == 3 {
			break
		}
		names, err := g.List(d.dir)
		if err != nil {
			continue
		}
		if name, ok := pick(st, "elsewhere", goodNames(names, exclude)); ok {
			return name, true
		}
	}
	return "", false
}

func lastNightDir(st *state.State, now time.Time) (string, time.Time) {
	var dir string
	var best time.Time
	for k, d := range st.Dirs {
		if d.Late.Before(st.SittingStart) && now.Sub(d.Late) < 48*time.Hour && d.Late.After(best) {
			dir, best = k, d.Late
		}
	}
	return dir, best
}

// topAndRare returns your most used command and one you rarely use. Both
// need enough history to mean something.
func topAndRare(st *state.State) (top, rare string) {
	type kv struct {
		cmd string
		n   int
	}
	var all []kv
	for c, n := range st.Cmds {
		if c != "tenant" {
			all = append(all, kv{c, n})
		}
	}
	if len(all) < 5 || st.CommandsSeen() < 20 {
		return "", ""
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].n != all[j].n {
			return all[i].n > all[j].n
		}
		return all[i].cmd < all[j].cmd
	})
	return all[0].cmd, all[len(all)-1].cmd
}

// closest finds the command you probably meant: a known one within two
// edits that you've used at least three times.
func closest(st *state.State, typo string) string {
	best, bestD := "", 3
	for c, n := range st.Cmds {
		if n < 3 || c == typo {
			continue
		}
		if d := distance(typo, c); d < bestD || (d == bestD && c < best) {
			best, bestD = c, d
		}
	}
	return best
}

func distance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

// Vars exposes the template variables for the finale and the epilogue.
func (g *Engine) Vars(st *state.State, env *mech.Env) script.Vars { return g.vars(st, env) }
