package script

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Dialogue is the finale: a small graph of nodes. Each node says some lines
// and then either jumps to another node or waits for the player's input and
// routes on keywords.
//
//	limit 6 => end
//
//	@start
//	oh.
//	you typed it.
//	-> ask
//
//	@ask
//	? who what name => who
//	? *             => shrug
type Dialogue struct {
	Nodes map[string]*Node
	Limit int    // after this many answers, go to LimitTo
	To    string // node to go to when the limit is reached
}

// Node is one step of the dialogue.
type Node struct {
	Name   string
	Lines  []string
	Next   string // "" if the node waits for input; "END" to finish
	Routes []Route
}

// Route sends input containing any keyword to a node. A keyword of "*"
// matches anything.
type Route struct {
	Keywords []string
	To       string
}

// End is the name of the implicit final node.
const End = "END"

// ParseDialogue reads the finale format.
func ParseDialogue(src string) (*Dialogue, error) {
	d := &Dialogue{Nodes: map[string]*Node{}}
	var cur *Node
	sc := bufio.NewScanner(strings.NewReader(src))
	for n := 1; sc.Scan(); n++ {
		raw := sc.Text()
		line := strings.TrimSpace(raw)
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case strings.HasPrefix(line, "limit "):
			f := strings.Fields(line)
			if len(f) != 4 || f[2] != "=>" {
				return nil, fmt.Errorf("line %d: want `limit N => node`", n)
			}
			l, err := strconv.Atoi(f[1])
			if err != nil {
				return nil, fmt.Errorf("line %d: bad limit", n)
			}
			d.Limit, d.To = l, f[3]
		case strings.HasPrefix(line, "@"):
			cur = &Node{Name: line[1:]}
			if d.Nodes[cur.Name] != nil {
				return nil, fmt.Errorf("line %d: duplicate node %s", n, cur.Name)
			}
			d.Nodes[cur.Name] = cur
		case cur == nil:
			return nil, fmt.Errorf("line %d: text outside a node", n)
		case strings.HasPrefix(line, "->"):
			cur.Next = strings.TrimSpace(line[2:])
		case strings.HasPrefix(line, "?"):
			kw, to, ok := strings.Cut(line[1:], "=>")
			if !ok {
				return nil, fmt.Errorf("line %d: want `? words => node`", n)
			}
			cur.Routes = append(cur.Routes, Route{Keywords: strings.Fields(strings.ToLower(kw)), To: strings.TrimSpace(to)})
		default:
			cur.Lines = append(cur.Lines, raw)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if d.Nodes["start"] == nil {
		return nil, fmt.Errorf("dialogue has no @start node")
	}
	for _, node := range d.Nodes {
		targets := []string{node.Next}
		for _, r := range node.Routes {
			targets = append(targets, r.To)
		}
		if node.Next == "" && len(node.Routes) == 0 {
			return nil, fmt.Errorf("node %s neither jumps nor asks", node.Name)
		}
		for _, t := range targets {
			if t != "" && t != End && d.Nodes[t] == nil {
				return nil, fmt.Errorf("node %s goes to unknown node %s", node.Name, t)
			}
		}
	}
	if d.To != "" && d.Nodes[d.To] == nil {
		return nil, fmt.Errorf("limit goes to unknown node %s", d.To)
	}
	return d, nil
}

// Route picks the next node for what the player typed.
func (n *Node) Route(input string) string {
	words := strings.FieldsFunc(strings.ToLower(input), func(r rune) bool { return !unicode.IsLetter(r) })
	fallback := ""
	for _, r := range n.Routes {
		for _, kw := range r.Keywords {
			if kw == "*" {
				if fallback == "" {
					fallback = r.To
				}
				continue
			}
			for _, w := range words {
				if w == kw || (len(kw) >= 3 && strings.HasPrefix(w, kw)) {
					return r.To
				}
			}
		}
	}
	return fallback
}
