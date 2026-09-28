// Package finale runs the typed conversation at the end of the story.
package finale

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/Tazwinator/Tenant/internal/script"
	"github.com/Tazwinator/Tenant/internal/term"
)

// Runner plays a dialogue on a terminal.
type Runner struct {
	D    *script.Dialogue
	Vars script.Vars
	In   *os.File
	Out  io.Writer
	CPS  int  // typewriter speed in characters per second; 0 prints instantly
	Slow bool // pause between lines

	mu   sync.Mutex
	mode *term.Mode
}

// Run plays the dialogue. It returns true if the player reached the end,
// and false if they left early with Ctrl-D. Ctrl-C always exits at once,
// with the terminal restored. Leaving early is always allowed: typing
// `tenant` again starts the conversation over.
func (r *Runner) Run() bool {
	fd := int(r.In.Fd())
	tty := term.IsTTY(fd)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)
	go func() {
		if _, ok := <-sig; ok {
			r.mu.Lock()
			r.mode.Restore()
			fmt.Fprintln(r.Out)
			os.Exit(130)
		}
	}()
	in := bufio.NewReader(r.In)

	name, answers := "start", 0
	for {
		node := r.D.Nodes[name]
		if node == nil {
			return false
		}
		for _, l := range node.Lines {
			if text, ok := script.Render(l, r.Vars); ok {
				r.say(term.Clean(strings.TrimSpace(text), 400), fd, tty)
			}
		}
		switch node.Next {
		case script.End:
			return true
		case "":
		default:
			name = node.Next
			continue
		}
		prompt := "> "
		if tty {
			prompt = term.Dim(prompt)
		}
		fmt.Fprint(r.Out, "\n"+prompt)
		input, err := in.ReadString('\n')
		if err != nil {
			fmt.Fprintln(r.Out)
			return false
		}
		fmt.Fprintln(r.Out)
		answers++
		next := node.Route(input)
		if (r.D.Limit > 0 && answers >= r.D.Limit) || next == "" {
			next = r.D.To
		}
		name = next
	}
}

// say types one line out. Any key finishes the line at once.
func (r *Runner) say(line string, fd int, tty bool) {
	if !tty || r.CPS <= 0 {
		fmt.Fprintln(r.Out, line)
		return
	}
	mode, err := term.Poll(fd)
	if err != nil {
		fmt.Fprintln(r.Out, line)
		return
	}
	r.mu.Lock()
	r.mode = mode
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		mode.Restore()
		r.mode = nil
		r.mu.Unlock()
	}()
	delay := time.Second / time.Duration(r.CPS)
	runes := []rune(line)
	buf := make([]byte, 16)
	for i, c := range runes {
		if n, _ := r.In.Read(buf); n > 0 {
			fmt.Fprint(r.Out, string(runes[i:]))
			break
		}
		fmt.Fprint(r.Out, string(c))
		time.Sleep(delay)
	}
	fmt.Fprintln(r.Out)
	term.Flush(fd)
	if r.Slow {
		time.Sleep(350 * time.Millisecond)
	}
}
