// Package sched decides when things happen. It is pure: given the state, the
// time and a pace, it answers the same way every time, so a fake clock and a
// recorded usage trace replay any haunting exactly.
package sched

import (
	"hash/fnv"
	"time"

	"github.com/Tazwinator/Tenant/internal/state"
)

// Pace is a set of pacing rules.
type Pace struct {
	Name             string
	Dormancy         time.Duration // nothing at all before this long after start
	DormancySittings int           // ... or before this many sittings have passed
	SittingIdle      time.Duration // a gap this long starts a new sitting
	RollHour         int           // the first prompt after this local hour starts a new sitting; -1 for none
	SittingEvery     time.Duration // compressed mode: a new sitting every so often; 0 for none
	ChapterGap       time.Duration // minimum time between chapters
	MinGap           time.Duration // minimum time between events
	MinGapPrompts    int           // minimum prompts between events
	Settle           int           // quiet prompts at the start of each sitting
	DailyCap         int           // most events in one calendar day
	Budget           [6]int        // most events per sitting, by chapter
	PBase, K, PMax   float64       // hazard: PBase*(1+K*quiet prompts), capped at PMax
	StallSittings    int           // sittings before a chapter's fallbacks kick in
}

// Paces are the presets.
var Paces = map[string]Pace{
	"normal": {
		Name: "normal", Dormancy: 36 * time.Hour, DormancySittings: 2,
		SittingIdle: 3 * time.Hour, RollHour: 5, ChapterGap: 16 * time.Hour,
		MinGap: 20 * time.Minute, MinGapPrompts: 10, Settle: 3, DailyCap: 3,
		Budget: [6]int{0, 2, 2, 2, 2, 2}, PBase: 0.03, K: 0.02, PMax: 0.5, StallSittings: 2,
	},
	"tester": {
		Name: "tester", Dormancy: 12 * time.Hour, DormancySittings: 1,
		SittingIdle: 2 * time.Hour, RollHour: 5, ChapterGap: 0,
		MinGap: 10 * time.Minute, MinGapPrompts: 5, Settle: 2, DailyCap: 6,
		Budget: [6]int{0, 2, 2, 2, 2, 2}, PBase: 0.05, K: 0.03, PMax: 0.45, StallSittings: 2,
	},
	"compressed": {
		Name: "compressed", Dormancy: 2 * time.Minute, DormancySittings: 0,
		SittingIdle: 3 * time.Minute, RollHour: -1, SittingEvery: 3 * time.Minute, ChapterGap: 2 * time.Minute,
		MinGap: 40 * time.Second, MinGapPrompts: 3, Settle: 1, DailyCap: 1000,
		Budget: [6]int{0, 3, 3, 3, 3, 3}, PBase: 0.15, K: 0.12, PMax: 0.75, StallSittings: 2,
	},
}

// Get returns a pace by name, or normal.
func Get(name string) Pace {
	if p, ok := Paces[name]; ok {
		return p
	}
	return Paces["normal"]
}

// Last is the final chapter.
const Last = 5

// Done reports whether every required beat of a chapter has fired.
type Done func(chapter int) bool

// Advance moves the story clock on for a prompt at now: it starts a new
// sitting when one is due, and unlocks the next chapter at the start of a
// sitting. It reports whether a new sitting began.
func Advance(st *state.State, now time.Time, p Pace, done Done) bool {
	fresh := st.Sitting == 0 ||
		now.Sub(st.LastSeen) >= p.SittingIdle ||
		(p.RollHour >= 0 && crossed(st.LastSeen, now, p.RollHour)) ||
		(p.SittingEvery > 0 && now.Sub(st.SittingStart) >= p.SittingEvery)
	if fresh {
		st.Sitting++
		st.SittingStart = now
		st.SittingPrompts = 0
		st.SittingEvents = 0
		unlock(st, now, p, done)
	}
	if now.After(st.LastSeen) {
		st.LastSeen = now
	}
	return fresh
}

func unlock(st *state.State, now time.Time, p Pace, done Done) {
	switch {
	case st.Chapter == 0:
		if now.Sub(st.Started) < p.Dormancy || st.Sitting <= p.DormancySittings {
			return
		}
	case st.Chapter < Last:
		if !done(st.Chapter) || now.Sub(st.ChapterStart) < p.ChapterGap {
			return
		}
	default:
		return
	}
	st.Chapter++
	st.ChapterStart = now
	st.ChapterSitting = st.Sitting
}

// crossed reports whether the local hour boundary fell between a and b.
func crossed(a, b time.Time, hour int) bool {
	if a.IsZero() || !b.After(a) {
		return false
	}
	b = b.Local()
	boundary := time.Date(b.Year(), b.Month(), b.Day(), hour, 0, 0, 0, b.Location())
	if boundary.After(b) {
		boundary = boundary.AddDate(0, 0, -1)
	}
	return a.Before(boundary)
}

// Day is the calendar day used for the daily cap.
func Day(t time.Time) string { return t.Local().Format("2006-01-02") }

// Allowed reports whether the guards let anything fire now.
func Allowed(st *state.State, now time.Time, p Pace) bool {
	switch {
	case st.Chapter < 1:
		return false
	case st.SittingPrompts <= p.Settle:
		return false
	case !st.LastEvent.IsZero() && (now.Sub(st.LastEvent) < p.MinGap || st.Prompts-st.LastEventPrompt < p.MinGapPrompts):
		return false
	case st.SittingEvents >= p.Budget[min(st.Chapter, Last)]:
		return false
	case st.Days[Day(now)] >= p.DailyCap:
		return false
	}
	return true
}

// Chance is the probability of an event on this opportunity. It rises the
// longer it has been quiet, and weight boosts rare triggers.
func Chance(st *state.State, p Pace, weight float64) float64 {
	quiet := float64(st.Prompts - st.LastEventPrompt)
	c := p.PBase * (1 + p.K*quiet) * weight
	if c > p.PMax {
		c = p.PMax
	}
	return c
}

// Stalled reports whether the current chapter has run long enough that its
// required beats may use their fallbacks.
func Stalled(st *state.State, p Pace) bool {
	return st.Sitting-st.ChapterSitting >= p.StallSittings
}

// Roll returns a deterministic pseudo-random number for this moment.
func Roll(st *state.State, purpose string) uint64 {
	h := fnv.New64a()
	var b [8]byte
	for _, v := range []uint64{st.Seed, uint64(st.Sitting), uint64(st.SittingPrompts), uint64(st.Prompts)} {
		for i := range b {
			b[i] = byte(v >> (8 * i))
		}
		h.Write(b[:])
	}
	h.Write([]byte(purpose))
	return mix(h.Sum64())
}

// Uniform maps a roll to [0, 1).
func Uniform(r uint64) float64 { return float64(r>>11) / (1 << 53) }

// mix is the splitmix64 finaliser.
func mix(z uint64) uint64 {
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}
