// Package script reads the story: beats, the finale dialogue and the
// epilogue. Story files are data. They can only name mechanics the engine
// already has, and they can only use the template variables the engine
// offers, so a story can never make tenant do anything new.
package script

import (
	"bufio"
	"fmt"
	"io/fs"
	"regexp"
	"strconv"
	"strings"
)

// Beat is one moment in the story.
type Beat struct {
	ID       string
	Chapter  int
	Required bool
	Invite   bool // firing this beat invites the player to type `tenant`
	Mech     string
	Text     []string // alternatives, tried in order until one renders
	After    string   // an optional beat that waits for another beat
	Fallback string   // mechanic to use if this one can't work here, or the chapter stalls
	FText    []string
	Line     int
}

// Story is a whole act.
type Story struct {
	Beats    []*Beat
	Finale   *Dialogue
	Epilogue []string
}

// Chapters is the number of chapters after dormancy.
const Chapters = 5

// Load reads an act from a directory holding beats.txt, finale.txt and
// epilogue.txt. known reports whether a mechanic exists.
func Load(fsys fs.FS, known func(string) bool) (*Story, error) {
	s := &Story{}
	data, err := fs.ReadFile(fsys, "beats.txt")
	if err != nil {
		return nil, err
	}
	if s.Beats, err = ParseBeats(string(data), known); err != nil {
		return nil, err
	}
	if data, err = fs.ReadFile(fsys, "finale.txt"); err != nil {
		return nil, err
	}
	if s.Finale, err = ParseDialogue(string(data)); err != nil {
		return nil, err
	}
	if data, err = fs.ReadFile(fsys, "epilogue.txt"); err != nil {
		return nil, err
	}
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if !strings.HasPrefix(line, "#") {
			s.Epilogue = append(s.Epilogue, line)
		}
	}
	return s, nil
}

// InChapter returns the beats of one chapter, in file order.
func (s *Story) InChapter(ch int) []*Beat {
	var out []*Beat
	for _, b := range s.Beats {
		if b.Chapter == ch {
			out = append(out, b)
		}
	}
	return out
}

// ParseBeats reads the beats format:
//
//	beat      phantom-first
//	chapter   1
//	required  yes
//	mech      ls.phantom
//	text      {{names.elsewhere}}
//
// Blocks are separated by blank lines. Lines starting with # are comments.
func ParseBeats(src string, known func(string) bool) ([]*Beat, error) {
	var beats []*Beat
	var cur *Beat
	seen := map[string]bool{}
	finish := func() error {
		if cur == nil {
			return nil
		}
		b := cur
		cur = nil
		switch {
		case b.Chapter < 1 || b.Chapter > Chapters:
			return fmt.Errorf("line %d: beat %s: chapter must be 1 to %d", b.Line, b.ID, Chapters)
		case !known(b.Mech):
			return fmt.Errorf("line %d: beat %s: unknown mechanic %q", b.Line, b.ID, b.Mech)
		case b.Fallback != "" && !known(b.Fallback):
			return fmt.Errorf("line %d: beat %s: unknown fallback %q", b.Line, b.ID, b.Fallback)
		case seen[b.ID]:
			return fmt.Errorf("line %d: duplicate beat %s", b.Line, b.ID)
		}
		seen[b.ID] = true
		beats = append(beats, b)
		return nil
	}

	sc := bufio.NewScanner(strings.NewReader(src))
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimRight(sc.Text(), " \t")
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if strings.TrimSpace(line) == "" {
			if err := finish(); err != nil {
				return nil, err
			}
			continue
		}
		key, val, _ := strings.Cut(line, " ")
		val = strings.TrimSpace(val)
		if key == "beat" {
			if err := finish(); err != nil {
				return nil, err
			}
			cur = &Beat{ID: val, Line: n}
			continue
		}
		if cur == nil {
			return nil, fmt.Errorf("line %d: %q outside a beat", n, key)
		}
		switch key {
		case "chapter":
			c, err := strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("line %d: bad chapter %q", n, val)
			}
			cur.Chapter = c
		case "required":
			cur.Required = val == "yes"
		case "invite":
			cur.Invite = val == "yes"
		case "mech":
			cur.Mech = val
		case "text":
			cur.Text = append(cur.Text, val)
		case "after":
			cur.After = val
		case "fallback":
			cur.Fallback = val
		case "ftext":
			cur.FText = append(cur.FText, val)
		default:
			return nil, fmt.Errorf("line %d: unknown key %q", n, key)
		}
	}
	if err := finish(); err != nil {
		return nil, err
	}
	for _, b := range beats {
		if b.After != "" && !seen[b.After] {
			return nil, fmt.Errorf("line %d: beat %s waits for unknown beat %s", b.Line, b.ID, b.After)
		}
	}
	return beats, sc.Err()
}

var varRE = regexp.MustCompile(`\{\{([a-z_.]+)\}\}`)

// Vars resolves template variables. ok is false when the engine doesn't
// know the answer (yet), and then the text can't be used.
type Vars func(name string) (value string, ok bool)

// Render fills in {{variables}}. It fails if any variable is unavailable.
func Render(tpl string, vars Vars) (string, bool) {
	ok := true
	out := varRE.ReplaceAllStringFunc(tpl, func(m string) string {
		v, found := vars(m[2 : len(m)-2])
		if !found {
			ok = false
		}
		return v
	})
	return out, ok
}

// Variables lists the variables a template uses.
func Variables(tpl string) []string {
	var out []string
	for _, m := range varRE.FindAllStringSubmatch(tpl, -1) {
		out = append(out, m[1])
	}
	return out
}
