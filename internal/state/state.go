// Package state holds everything tenant remembers between prompts.
package state

import (
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/Tazwinator/Tenant/internal/audit"
)

// Version is bumped whenever the layout changes incompatibly.
const Version = 1

// MaxDirs caps how many directories are remembered.
const MaxDirs = 400

// Dir is what tenant knows about a directory: when you were there. It never
// holds anything about the directory's contents.
type Dir struct {
	N    int       `json:"n"`
	Last time.Time `json:"last"`
	Late time.Time `json:"late,omitempty"` // last visit between 21:00 and 04:00
}

// Fired records one beat that has happened.
type Fired struct {
	ID      string    `json:"id"`
	At      time.Time `json:"at"`
	Chapter int       `json:"chapter"`
}

// Caps is the latest capability report from one shell, for doctor.
type Caps struct {
	Bits int       `json:"bits"`
	At   time.Time `json:"at"`
}

// State is the whole of tenant's memory.
type State struct {
	Version int       `json:"version"`
	Seed    uint64    `json:"seed"`
	Pace    string    `json:"pace"`
	Started time.Time `json:"started"`

	// Story clock.
	Prompts         int            `json:"prompts"` // opportunities so far
	LastSeen        time.Time      `json:"last_seen"`
	Sitting         int            `json:"sitting"`
	SittingStart    time.Time      `json:"sitting_start"`
	SittingPrompts  int            `json:"sitting_prompts"`
	SittingEvents   int            `json:"sitting_events"`
	Chapter         int            `json:"chapter"`
	ChapterStart    time.Time      `json:"chapter_start"`
	ChapterSitting  int            `json:"chapter_sitting"`
	Fired           []Fired        `json:"fired"`
	LastEvent       time.Time      `json:"last_event"`
	LastEventPrompt int            `json:"last_event_prompt"`
	Days            map[string]int `json:"days"`
	Invited         bool           `json:"invited"`
	FinaleDone      bool           `json:"finale_done"`

	// Observations: names only.
	Cmds     map[string]int  `json:"cmds"`
	Dirs     map[string]*Dir `json:"dirs"`
	FirstDir string          `json:"first_dir"`

	// Hand-offs to the shell for the next call.
	Ghost       string `json:"ghost,omitempty"`
	TimeText    string `json:"time_text,omitempty"`
	Force       string `json:"force,omitempty"`
	ForceText   string `json:"force_text,omitempty"`
	TitlePushed bool   `json:"title_pushed,omitempty"`

	Caps map[string]Caps `json:"caps"`
}

// New returns a fresh state for `tenant start`.
func New(seed uint64, pace string, now time.Time) *State {
	return &State{
		Version: Version, Seed: seed, Pace: pace, Started: now,
		Days: map[string]int{}, Cmds: map[string]int{}, Dirs: map[string]*Dir{}, Caps: map[string]Caps{},
	}
}

// ErrNotStarted means there is no state: tenant hasn't been started.
var ErrNotStarted = errors.New("tenant has not been started")

// Load reads state.json.
func Load(fs *audit.FS) (*State, error) {
	data, err := fs.ReadOwn(audit.FileState)
	if err != nil {
		return nil, ErrNotStarted
	}
	st := &State{}
	if err := json.Unmarshal(data, st); err != nil {
		return nil, err
	}
	if st.Version != Version {
		return nil, errors.New("state was written by a different version of tenant; run `tenant evict` and start again")
	}
	st.fill()
	return st, nil
}

// Save writes state.json atomically.
func Save(fs *audit.FS, st *State) error {
	st.prune()
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return fs.WriteOwn(audit.FileState, data)
}

func (st *State) fill() {
	if st.Days == nil {
		st.Days = map[string]int{}
	}
	if st.Cmds == nil {
		st.Cmds = map[string]int{}
	}
	if st.Dirs == nil {
		st.Dirs = map[string]*Dir{}
	}
	if st.Caps == nil {
		st.Caps = map[string]Caps{}
	}
}

// prune keeps the state small: the most recent directories, and a week of
// daily event counts.
func (st *State) prune() {
	if len(st.Dirs) > MaxDirs {
		type kv struct {
			k string
			t time.Time
		}
		all := make([]kv, 0, len(st.Dirs))
		for k, d := range st.Dirs {
			all = append(all, kv{k, d.Last})
		}
		sort.Slice(all, func(i, j int) bool { return all[i].t.After(all[j].t) })
		for _, e := range all[MaxDirs:] {
			if e.k != st.FirstDir {
				delete(st.Dirs, e.k)
			}
		}
	}
	if len(st.Days) > 8 {
		keys := make([]string, 0, len(st.Days))
		for k := range st.Days {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys[:len(keys)-8] {
			delete(st.Days, k)
		}
	}
}

// HasFired reports whether a beat has already happened.
func (st *State) HasFired(id string) bool {
	for _, f := range st.Fired {
		if f.ID == id {
			return true
		}
	}
	return false
}

// Finished reports whether the story is over.
func (st *State) Finished() bool { return st.FinaleDone }

// CommandsSeen is the total number of commands observed.
func (st *State) CommandsSeen() int {
	n := 0
	for _, c := range st.Cmds {
		n += c
	}
	return n
}
