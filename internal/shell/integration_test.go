package shell_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// hostileDir is a directory name that would run a command if tenant ever
// let it reach prompt expansion or eval.
const hostileDir = "x$(touch pwned)`touch pwned2`%F{red}a"

type fixture struct {
	root, home, bin string
	uid             int
}

// setupFixture builds the binary and a fake home. As root, the shells run
// as nobody, because tenant stays quiet for root.
func setupFixture(t *testing.T) *fixture {
	t.Helper()
	root, err := os.MkdirTemp("", "tenant-it-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	os.Chmod(root, 0o755)
	f := &fixture{root: root, home: filepath.Join(root, "home"), bin: filepath.Join(root, "bin"), uid: -1}
	build := exec.Command("go", "build", "-o", filepath.Join(f.bin, "tenant"), "../../cmd/tenant")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	for _, d := range []string{"code/rarepulls/cmd", "code/rarepulls/internal", "code/rarepulls/web", "notes", hostileDir} {
		os.MkdirAll(filepath.Join(f.home, d), 0o755)
	}
	for _, file := range []string{"code/rarepulls/go.mod", "code/rarepulls/go.sum", "notes/todo.md", "notes/ideas.txt"} {
		os.WriteFile(filepath.Join(f.home, file), []byte("private contents\n"), 0o644)
	}
	if os.Geteuid() == 0 {
		f.uid = 65534
		filepath.Walk(root, func(p string, _ os.FileInfo, _ error) error {
			if strings.HasPrefix(p, f.home) {
				os.Lchown(p, f.uid, f.uid)
			}
			return nil
		})
	}
	return f
}

func (f *fixture) env(extra ...string) []string {
	return append([]string{
		"HOME=" + f.home, "USER=sam", "PATH=" + f.bin + ":/usr/local/bin:/usr/bin:/bin",
		"TERM=xterm-256color", "LANG=C.UTF-8", "TENANT_DEV=1", "TENANT_DEADLINE_MS=5000",
		"LS_COLORS=di=01;34:*.txt=00;32",
	}, extra...)
}

func (f *fixture) write(t *testing.T, rel, content string) string {
	t.Helper()
	p := filepath.Join(f.home, rel)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if f.uid >= 0 {
		os.Lchown(p, f.uid, f.uid)
	}
	return p
}

func need(t *testing.T, sh string) {
	if testing.Short() {
		t.Skip("integration test")
	}
	if _, err := exec.LookPath(sh); err != nil {
		t.Skip(sh, "not installed")
	}
}

func TestBash(t *testing.T) {
	need(t, "bash")
	f := setupFixture(t)
	rc := f.write(t, ".bashrc-test", `PS1='[\#] \w \$ '
HISTFILE=$HOME/.bash_history
HISTCONTROL=ignorespace
PROMPT_COMMAND='true'
eval "$(tenant init bash)"
`)
	s := startSession(t, f.uid, f.env(), "bash", "--noprofile", "--rcfile", rc, "-i")
	s.waitPrompt()
	play(t, f, s, "bash")
}

func TestZsh(t *testing.T) {
	need(t, "zsh")
	f := setupFixture(t)
	os.MkdirAll(filepath.Join(f.home, ".zdot"), 0o755)
	if f.uid >= 0 {
		os.Lchown(filepath.Join(f.home, ".zdot"), f.uid, f.uid)
	}
	f.write(t, ".zdot/.zshrc", `PROMPT='[%h] %~ %# '
HISTFILE=$HOME/.zsh_history
HISTSIZE=100
SAVEHIST=100
setopt inc_append_history hist_ignore_space
bindkey -e
eval "$(tenant init zsh)"
`)
	s := startSession(t, f.uid, f.env("ZDOTDIR="+filepath.Join(f.home, ".zdot")), "zsh", "-i")
	s.waitPrompt()
	play(t, f, s, "zsh")
}

// play runs the same scenario in either shell.
func play(t *testing.T, f *fixture, s *session, sh string) {
	check := func(what, out string, re string) {
		t.Helper()
		if !regexp.MustCompile(re).MatchString(Plain(out)) {
			t.Fatalf("%s: want %q in output:\n%s", what, re, Plain(out))
		}
	}
	checkNot := func(what, out string, re string) {
		t.Helper()
		if regexp.MustCompile(re).MatchString(Plain(out)) {
			t.Fatalf("%s: did not want %q in output:\n%s", what, re, Plain(out))
		}
	}

	// Loaded, but inert until started.
	check("hook loaded", s.run(`echo "hook=$TENANT_HOOK"`), `hook=`+sh)
	s.run("false")
	check("status before start", s.run(`echo "st=$?"`), `st=1`)
	check("start", s.run("tenant start --compressed"), `tenant has started \(compressed pace\)`)

	// $? survives the hook.
	s.run("cd ~/notes")
	s.run("cd ~/code/rarepulls")
	s.run("false")
	check("status while active", s.run(`echo "st=$?"`), `st=1`)

	// ls.phantom: an extra name after ls, gone the next time.
	s.run("tenant _force ls.phantom")
	out := s.run("ls")
	check("phantom", out, `go\.sum[\s\S]*\n(todo\.md|ideas\.txt|untitled\.txt)\n`)
	checkNot("phantom gone", s.run("ls"), `todo\.md|ideas\.txt|untitled\.txt`)

	// Mechanics that can fire on any prompt fire on the one straight after
	// `tenant _force`.

	// prompt.glyph: one letter off, for one prompt only.
	out = s.run("tenant _force prompt.glyph")
	check("glyph", out, `\] ~/code/r[a-z]+ [$%]\s*$`)
	checkNot("glyph changed a letter", out, `\] ~/code/rarepulls [$%]\s*$`)
	check("glyph restored", s.run("true"), `\] ~/code/rarepulls [$%]\s*$`)

	// history.ghost: Up shows a command nobody typed, once.
	s.run("true")
	s.run("tenant _force history.ghost")
	s.send("\x1b[A")
	s.expect(`first saw you|only ever read`)
	s.send("\x15") // kill the line
	s.send("\x1b[A")
	s.expect(`tenant _force history.ghost`)
	s.send("\x15")
	s.run("")

	// A ghost that isn't harmless is never armed: Up behaves normally.
	s.run("tenant _force history.ghost 'cd ~/x$(touch ghostpwned)'")
	s.send("\x1b[A")
	s.expect(`tenant _force history.ghost`)
	s.send("\x15")
	s.run("")
	if sh == "bash" {
		check("up restored", s.run(`bind -q previous-history`), `\\(e|M-)\[A.*\\(e|M-)OA|\\(e|M-)OA.*\\(e|M-)\[A`)
	}

	// notfound.remark: the usual message, then one more line.
	s.run("tenant _force notfound.remark")
	out = s.run("gti")
	check("notfound", out, `command not found`)
	check("remark", out, `not found\S*[\s\S]*\n[^\n]*(meant|isn't here|neither am i|still here)`)

	// clear.residue.
	s.run("tenant _force clear.residue")
	check("residue", s.run("clear"), `cd ~/|you know what|haven't typed|counting|still here`)

	// prompt.time.
	check("time", s.run("tenant _force prompt.time"), `23:41|waiting`)

	// title.whisper writes a window title.
	if out := s.run("tenant _force title.whisper"); !strings.Contains(out, "\x1b]2;") {
		t.Fatalf("title: no OSC 2 in %q", out)
	}

	// Hostile directory names never reach eval or prompt expansion.
	s.run("cd ~/" + shQuote(hostileDir))
	s.run("tenant _force prompt.glyph")
	s.run("true")
	s.run("ls")
	s.run("cd ~")
	for _, p := range []string{"pwned", "pwned2", "ghostpwned"} {
		if _, err := os.Stat(filepath.Join(f.home, p)); err == nil {
			t.Fatalf("a hostile name ran a command: %s exists", p)
		}
		if _, err := os.Stat(filepath.Join(f.home, hostileDir, p)); err == nil {
			t.Fatalf("a hostile name ran a command: %s exists", p)
		}
	}

	// A space-prefixed command is invisible to tenant.
	s.run(" secretcommand")

	// confess: the log, and nothing written.
	out = s.run("tenant confess")
	check("confess", out, `list\s+~/`)
	check("confess", out, `wrote nothing`)
	checkNot("confess", out, `private contents|secretcommand`)

	// evict: gone from every shell at the next prompt.
	check("evict", s.run("tenant evict"), `tenant has been evicted`)
	s.run("true")
	// bash leaves two do-nothing stand-ins behind, in case another tool
	// folded them into its own PROMPT_COMMAND entry; nothing else remains.
	checkNot("unloaded", s.run(`typeset -f _tenant_precmd _tenant_classify _tenant_glyph`), `_tenant_(precmd|classify|glyph) \(\)`)
	if sh == "bash" {
		checkNot("unloaded", s.run(`echo "pc=[${PROMPT_COMMAND[*]}]"`), `_tenant`)
	} else {
		checkNot("unloaded", s.run(`echo "pc=[${precmd_functions[*]}]"`), `_tenant`)
	}
	check("unloaded", s.run(`echo "hook=[$TENANT_HOOK]"`), `hook=\[\]`)
	if _, err := os.Stat(filepath.Join(f.home, ".local/state/tenant")); err == nil {
		t.Fatal("state directory still exists after evict")
	}
	s.run("false")
	check("status after evict", s.run(`echo "st=$?"`), `st=1`)

	s.send("exit\r")
	s.cmd.Wait()

	// The ghost never reached the history file.
	for _, hist := range []string{".bash_history", ".zsh_history"} {
		data, _ := os.ReadFile(filepath.Join(f.home, hist))
		for _, bad := range []string{"first saw you", "only ever read", "secretcommand"} {
			if strings.Contains(string(data), bad) {
				t.Fatalf("%s contains %q", hist, bad)
			}
		}
	}
}

func shQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// Strict shells: set -eu, a scalar PROMPT_COMMAND that another line appends
// to, and commands tenant can't classify. The shell must survive and print
// nothing extra.
func TestBashStrict(t *testing.T) {
	need(t, "bash")
	f := setupFixture(t)
	rc := f.write(t, ".bashrc-strict", `PS1='[\#] \w \$ '
HISTCONTROL=ignorespace
PROMPT_COMMAND='history -a'
eval "$(tenant init bash)"
PROMPT_COMMAND="${PROMPT_COMMAND[0]}"$'\n'"true"
set -eu
`)
	s := startSession(t, f.uid, f.env(), "bash", "--noprofile", "--rcfile", rc, "-i")
	s.waitPrompt()
	s.run("tenant start --compressed")
	s.run("cd ~/code/rarepulls")
	for _, c := range []string{"[ -d / ]", "FOO=1 true", "[[ -n x ]]", "ls"} {
		s.run(c)
	}
	out := s.run("tenant _force prompt.glyph")
	if !strings.Contains(Plain(out), "~/code/r") {
		t.Fatalf("glyph under set -eu: %q", Plain(out))
	}
	s.run("tenant evict")
	out = s.run("true") + s.run("true")
	check := Plain(out)
	for _, bad := range []string{"unbound variable", "command not found", "logout", "exit"} {
		if strings.Contains(check, bad) {
			t.Fatalf("strict bash printed %q:\n%s", bad, check)
		}
	}
	if !strings.Contains(Plain(s.run("echo alive")), "alive") {
		t.Fatal("the shell died")
	}
}

// zsh with the options most likely to break a hook script.
func TestZshOptions(t *testing.T) {
	need(t, "zsh")
	f := setupFixture(t)
	os.MkdirAll(filepath.Join(f.home, ".zdot"), 0o755)
	if f.uid >= 0 {
		os.Lchown(filepath.Join(f.home, ".zdot"), f.uid, f.uid)
	}
	f.write(t, ".zdot/.zshrc", `PROMPT='[%h] %/ %# '
u1() { : }
u2() { : }
precmd_functions=(u1 u2)
bindkey -e
setopt ksharrays nounset globsubst warncreateglobal errexit
eval "$(tenant init zsh)"
`)
	s := startSession(t, f.uid, f.env("ZDOTDIR="+filepath.Join(f.home, ".zdot")), "zsh", "-i")
	s.waitPrompt()
	if out := Plain(s.run(`print -r -- "pf=${precmd_functions[*]}"`)); !strings.Contains(out, "pf=_tenant_status u1 u2 _tenant_precmd") {
		t.Fatalf("precmd_functions: %s", out)
	}
	s.run("tenant start --compressed")
	s.run("cd ~/code/rarepulls")
	out := Plain(s.run("tenant _force prompt.glyph"))
	if !strings.Contains(out, f.home+"/code/r") || strings.Contains(out, f.home+"/code/rarepulls %") {
		t.Fatalf("glyph with %%/ under hostile options: %s", out)
	}
	if out := Plain(s.run("true")); !strings.Contains(out, f.home+"/code/rarepulls %") {
		t.Fatalf("prompt not restored: %s", out)
	}
	s.run("tenant _force prompt.time")
	s.run("tenant evict")
	all := Plain(s.run("true"))
	if out := Plain(s.run(`print -r -- "pf=${precmd_functions[*]}"`)); !strings.Contains(out, "pf=u1 u2") {
		t.Fatalf("unload lost the user's hooks: %s", out)
	}
	s.mu.Lock()
	all += Plain(s.buf.String())
	s.mu.Unlock()
	for _, bad := range []string{"parameter not set", "created globally", "command not found", "bad pattern"} {
		if strings.Contains(all, bad) {
			t.Fatalf("zsh printed %q:\n%s", bad, all)
		}
	}
}
