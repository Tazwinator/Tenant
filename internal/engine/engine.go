// Package engine runs one tick of the story: it records what it observed,
// moves the story clock on, and decides whether a beat fires.
package engine

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Tazwinator/Tenant/internal/audit"
	"github.com/Tazwinator/Tenant/internal/mech"
	"github.com/Tazwinator/Tenant/internal/sched"
	"github.com/Tazwinator/Tenant/internal/script"
	"github.com/Tazwinator/Tenant/internal/state"
)

// Engine holds what a tick needs besides the state.
type Engine struct {
	Story  *script.Story
	Pace   sched.Pace
	List   func(dir string) ([]audit.Name, error) // reads names; logged by audit
	Exists func(path string) bool                 // existence only, for git markers
}

// MaxCmds caps how many distinct command names are remembered.
const MaxCmds = 300

// Tick handles one prompt (or one not-found command) and returns what to show.
func (g *Engine) Tick(st *state.State, env *mech.Env) (out mech.Effect) {
	if st.FinaleDone {
		return out
	}
	now := env.Now
	if env.Kind == "prompt" {
		st.Caps[env.Shell] = state.Caps{Bits: int(env.Caps), At: now}
		// This shell's last hand-offs are spent: its hook has disarmed. Other
		// shells' hand-offs are theirs and stay.
		if h := st.Handoffs[env.Pid]; h != nil {
			if h.Title {
				out.Say += mech.TitlePop
			}
			delete(st.Handoffs, env.Pid)
		}
	}
	defer func() {
		if out.Ghost != "" || out.Time != "" || out.Title {
			st.Handoffs[env.Pid] = &state.Handoff{Ghost: out.Ghost, Time: out.Time, Title: out.Title, At: now}
		}
	}()
	g.observe(st, env)
	sched.Advance(st, now, g.Pace, g.chapterDone(st))
	env.LateStamp = lastNight(st, now)
	env.Roll = func(purpose string) uint64 { return sched.Roll(st, purpose) }

	opportunity := env.Kind == "notfound" || env.Cmd != "" || env.First
	if !opportunity {
		return out
	}
	if env.Kind == "prompt" {
		st.Prompts++
		st.SittingPrompts++
	}
	if st.Force != "" {
		if eff, ok := g.force(st, env); ok {
			out.Merge(eff)
		}
		return out
	}
	if !env.UnderHome || g.inGitOperation(env) || !sched.Allowed(st, now, g.Pace) {
		return out
	}

	cands, weight := g.candidates(st, env)
	if len(cands) == 0 {
		return out
	}
	// A stalled chapter's next required beat fires at its next chance;
	// everything else has to win the roll.
	stalledReq := cands[0].beat.Required && sched.Stalled(st, g.Pace)
	if !stalledReq && sched.Uniform(sched.Roll(st, "fire")) >= sched.Chance(st, g.Pace, weight) {
		return out
	}
	vars := g.vars(st, env)
	for _, c := range cands {
		eff, ok := fire(c.mech, env, c.texts, vars)
		if !ok {
			continue
		}
		st.Fired = append(st.Fired, state.Fired{ID: c.beat.ID, At: now, Chapter: st.Chapter})
		st.LastEvent = now
		st.LastEventPrompt = st.Prompts
		st.SittingEvents++
		st.Days[sched.Day(now)]++
		if c.beat.Invite {
			st.Invited = true
		}
		out.Merge(eff)
		break
	}
	return out
}

func (g *Engine) observe(st *state.State, env *mech.Env) {
	if env.Kind != "prompt" || env.Cmd == "" {
		return
	}
	if _, ok := st.Cmds[env.Cmd]; ok || len(st.Cmds) < MaxCmds {
		st.Cmds[env.Cmd]++
	}
	if !env.UnderHome {
		return
	}
	d := st.Dirs[env.Cwd]
	if d == nil {
		d = &state.Dir{}
		st.Dirs[env.Cwd] = d
	}
	d.N++
	d.Last = env.Now
	if late(env.Now) {
		d.Late = env.Now
	}
	if st.FirstDir == "" && env.Cwd != env.Home {
		st.FirstDir = env.Cwd
	}
}

func late(t time.Time) bool {
	h := t.Local().Hour()
	return h >= 21 || h < 4
}

// lastNight is the latest late-night visit before this sitting, within two
// days. Zero if there isn't one.
func lastNight(st *state.State, now time.Time) time.Time {
	var best time.Time
	for _, d := range st.Dirs {
		if d.Late.Before(st.SittingStart) && now.Sub(d.Late) < 48*time.Hour && d.Late.After(best) {
			best = d.Late
		}
	}
	return best
}

func (g *Engine) chapterDone(st *state.State) sched.Done {
	return func(ch int) bool {
		for _, b := range g.Story.InChapter(ch) {
			if b.Required && !st.HasFired(b.ID) {
				return false
			}
		}
		return true
	}
}

type candidate struct {
	beat  *script.Beat
	mech  mech.Mechanic
	texts []string
}

// candidates lists what could fire right now. While a chapter has required
// beats left, only the next one is a candidate: the story comes first.
// After that, optional beats fill the wait for the next chapter, in a
// seeded order. weight is the largest trigger weight among them.
func (g *Engine) candidates(st *state.State, env *mech.Env) ([]candidate, float64) {
	var req, opt []candidate
	weight := 0.0
	nextRequired := true
	storyDone := g.chapterDone(st)(st.Chapter)
	stalled := sched.Stalled(st, g.Pace)
	for _, b := range g.Story.InChapter(st.Chapter) {
		if st.HasFired(b.ID) {
			continue
		}
		if (b.Required && !nextRequired) || (!b.Required && !storyDone) {
			continue
		}
		if b.After != "" && !st.HasFired(b.After) {
			continue
		}
		c, ok := g.usable(b, env, false)
		if !ok && b.Fallback != "" {
			// Fall back when this shell can never show the mechanic, or
			// when a required beat's trigger just isn't happening.
			if m, _ := mech.Get(b.Mech); (b.Required && stalled) || !m.Probe(env) {
				c, ok = g.usable(b, env, true)
			}
		}
		if b.Required {
			nextRequired = false
		}
		if !ok {
			continue
		}
		if w := c.mech.Weight(); w > weight {
			weight = w
		}
		if b.Required {
			req = append(req, c)
		} else {
			opt = append(opt, c)
		}
	}
	r := sched.Roll(st, "order")
	sort.SliceStable(opt, func(i, j int) bool {
		return sched.Roll(st, opt[i].beat.ID)^r < sched.Roll(st, opt[j].beat.ID)^r
	})
	return append(req, opt...), weight
}

func (g *Engine) usable(b *script.Beat, env *mech.Env, fallback bool) (candidate, bool) {
	id, texts := b.Mech, b.Text
	if fallback {
		id, texts = b.Fallback, b.FText
	}
	m, ok := mech.Get(id)
	if !ok || !m.Triggered(env) || !m.Probe(env) {
		return candidate{}, false
	}
	return candidate{beat: b, mech: m, texts: texts}, true
}

func fire(m mech.Mechanic, env *mech.Env, texts []string, vars script.Vars) (mech.Effect, bool) {
	if !m.NeedsText() {
		return m.Fire(env, "")
	}
	for _, t := range texts {
		text, ok := script.Render(t, vars)
		if !ok {
			continue
		}
		if eff, ok := m.Fire(env, text); ok {
			return eff, true
		}
	}
	return mech.Effect{}, false
}

// force fires a mechanic on demand, ignoring every guard. Developer only:
// this is how the teaser is recorded.
func (g *Engine) force(st *state.State, env *mech.Env) (mech.Effect, bool) {
	m, ok := mech.Get(st.Force)
	if !ok {
		st.Force = ""
		return mech.Effect{}, false
	}
	if !m.Triggered(env) || !m.Probe(env) {
		return mech.Effect{}, false
	}
	var texts []string
	if st.ForceText != "" {
		texts = []string{st.ForceText} // directed: exactly this, or nothing
	} else {
		for _, b := range g.Story.Beats {
			if b.Mech == st.Force {
				texts = append(texts, b.Text...)
			}
		}
		texts = append(texts, devText[st.Force])
	}
	eff, ok := fire(m, env, texts, g.vars(st, env))
	// The trigger happened, so this was its chance: fired or refused (a
	// ghost that isn't harmless, say), the force is used up.
	st.Force, st.ForceText = "", ""
	return eff, ok
}

var devText = map[string]string{
	"ls.phantom":      "still_here.txt",
	"history.ghost":   "ls -la  # still here",
	"motd.lastlogin":  "{{cwd}}",
	"clear.residue":   "still here.",
	"notfound.remark": "still here.",
	"title.whisper":   "still here",
	"prompt.time":     "23:41",
}

// inGitOperation reports whether cwd is inside a repository that is part
// way through a rebase, merge or bisect. It only checks that marker names
// exist; nothing is opened.
func (g *Engine) inGitOperation(env *mech.Env) bool {
	if g.Exists == nil {
		return false
	}
	for dir := env.Cwd; strings.HasPrefix(dir, env.Home); dir = filepath.Dir(dir) {
		git := filepath.Join(dir, ".git")
		if g.Exists(git) {
			for _, m := range []string{"rebase-merge", "rebase-apply", "MERGE_HEAD", "BISECT_LOG"} {
				if g.Exists(filepath.Join(git, m)) {
					return true
				}
			}
			return false
		}
		if dir == env.Home {
			break
		}
	}
	return false
}
