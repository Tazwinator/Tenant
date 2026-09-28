package shell_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// session is an interactive shell on a pseudo-terminal.
type session struct {
	t      *testing.T
	master *os.File
	cmd    *exec.Cmd
	mu     sync.Mutex
	buf    bytes.Buffer
	mark   int
}

func ioctl(fd uintptr, req uintptr, arg unsafe.Pointer) error {
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg)); e != 0 {
		return e
	}
	return nil
}

func openPTY(uid int) (*os.File, *os.File, error) {
	m, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	var n uint32
	if err := ioctl(m.Fd(), syscall.TIOCGPTN, unsafe.Pointer(&n)); err != nil {
		return nil, nil, err
	}
	var unlock int32
	if err := ioctl(m.Fd(), syscall.TIOCSPTLCK, unsafe.Pointer(&unlock)); err != nil {
		return nil, nil, err
	}
	ws := struct{ Row, Col, X, Y uint16 }{40, 120, 0, 0}
	ioctl(m.Fd(), syscall.TIOCSWINSZ, unsafe.Pointer(&ws))
	name := fmt.Sprintf("/dev/pts/%d", n)
	if uid >= 0 {
		os.Chown(name, uid, uid)
	}
	s, err := os.OpenFile(name, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	return m, s, nil
}

func startSession(t *testing.T, uid int, env []string, name string, args ...string) *session {
	t.Helper()
	m, s, err := openPTY(uid)
	if err != nil {
		t.Skip("no pty:", err)
	}
	cmd := exec.Command(name, args...)
	cmd.Env = env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = s, s, s
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if uid >= 0 {
		cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(uid)}
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	s.Close()
	se := &session{t: t, master: m, cmd: cmd}
	go func() {
		b := make([]byte, 4096)
		for {
			n, err := m.Read(b)
			se.mu.Lock()
			se.buf.Write(b[:n])
			se.mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
		m.Close()
	})
	return se
}

// since returns the output after the mark.
func (s *session) since() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()[s.mark:]
}

// expect waits for a pattern in the output after the mark, then moves the
// mark past it.
func (s *session) expect(re string) string {
	s.t.Helper()
	rx := regexp.MustCompile(re)
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		out := s.buf.String()[s.mark:]
		if loc := rx.FindStringIndex(out); loc != nil {
			s.mark += loc[1]
			s.mu.Unlock()
			return out[:loc[1]]
		}
		s.mu.Unlock()
		time.Sleep(20 * time.Millisecond)
	}
	s.t.Fatalf("timed out waiting for %q; output since mark:\n%q", re, s.since())
	return ""
}

func (s *session) send(text string) {
	s.master.Write([]byte(text))
}

// promptLine is the test prompt, "[N] dir $ " or "[N] dir % ", perhaps with
// a right-aligned time after it.
var promptLine = regexp.MustCompile(`^\[\d+\] .*[$%]( .*)?$`)

var ansi = regexp.MustCompile(`\x1b(\[[0-9;?]*[a-zA-Z]|\][^\a]*\a|[=>])`)

// Plain strips escape sequences and renders carriage returns the way a
// terminal would: later text overwrites the start of the line.
func Plain(s string) string {
	var lines []string
	var line []rune
	col := 0
	for _, r := range ansi.ReplaceAllString(s, "") {
		switch r {
		case '\n':
			lines = append(lines, strings.TrimRight(string(line), " "))
			line, col = nil, 0
		case '\r':
			col = 0
		default:
			if col < len(line) {
				line[col] = r
			} else {
				line = append(line, r)
			}
			col++
		}
	}
	return strings.Join(append(lines, string(line)), "\n")
}

// waitPrompt waits until the shell has gone quiet showing a prompt, and
// returns everything printed since the mark.
func (s *session) waitPrompt() string {
	s.t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	last, lastChange := -1, time.Now()
	for time.Now().Before(deadline) {
		s.mu.Lock()
		out := s.buf.String()[s.mark:]
		s.mu.Unlock()
		if len(out) != last {
			last, lastChange = len(out), time.Now()
		}
		plain := Plain(out)
		lastLine := plain[strings.LastIndex(plain, "\n")+1:]
		if time.Since(lastChange) > 250*time.Millisecond && promptLine.MatchString(lastLine) {
			s.mu.Lock()
			s.mark += len(out)
			s.mu.Unlock()
			return out
		}
		time.Sleep(20 * time.Millisecond)
	}
	s.t.Fatalf("timed out waiting for a prompt; output since mark:\n%q", s.since())
	return ""
}

// run types a command and waits for the next prompt. It returns everything
// printed in between, including the new prompt.
func (s *session) run(line string) string {
	s.t.Helper()
	s.send(line + "\r")
	return s.waitPrompt()
}
