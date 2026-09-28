package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testFS(t *testing.T) *FS {
	t.Helper()
	home := t.TempDir()
	f := New(Paths{Home: home, State: filepath.Join(home, ".local", "state", "tenant")})
	if err := f.Prepare(); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestOwnFilesOnly(t *testing.T) {
	f := testFS(t)
	for _, name := range []string{"../../.bashrc", "notes.txt", "", "state.json/x"} {
		if err := f.WriteOwn(name, []byte("x")); err == nil {
			t.Errorf("WriteOwn(%q) was allowed", name)
		}
		if _, err := f.ReadOwn(name); err == nil {
			t.Errorf("ReadOwn(%q) was allowed", name)
		}
	}
}

func TestListDirLogsAndRemembers(t *testing.T) {
	f := testFS(t)
	dir := filepath.Join(f.P.Home, "code")
	os.MkdirAll(filepath.Join(dir, "cmd"), 0o755)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module secret"), 0o644)
	names, err := f.ListDir(dir)
	if err != nil || len(names) != 2 {
		t.Fatalf("ListDir = %v, %v", names, err)
	}
	entries, _ := f.Entries()
	if len(entries) != 1 || entries[0].Kind != "list" || !strings.HasPrefix(entries[0].Detail, "~/code") {
		t.Fatalf("log = %+v", entries)
	}
	seen, _ := f.Seen()
	if got := strings.Join(seen["~/code"], " "); got != "cmd/ go.mod" {
		t.Fatalf("seen = %q", got)
	}
	if _, err := f.ListDir("/etc"); err == nil {
		t.Fatal("listed a directory outside $HOME")
	}
	if _, err := f.ListDir(filepath.Join(f.P.Home, "..")); err == nil {
		t.Fatal("listed a directory outside $HOME via ..")
	}
}

func TestEvict(t *testing.T) {
	f := testFS(t)
	f.Activate()
	f.Record("started", "normal pace")
	if !f.Active() {
		t.Fatal("not active after Activate")
	}
	if err := f.Evict(); err != nil {
		t.Fatal(err)
	}
	if f.StateExists() {
		t.Fatal("state directory survived evict")
	}
	if err := f.Evict(); err != nil {
		t.Fatal("evicting twice should be fine:", err)
	}
}

func TestListDirSymlinkOutOfHome(t *testing.T) {
	f := testFS(t)
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "secret-name"), nil, 0o644)
	link := filepath.Join(f.P.Home, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skip(err)
	}
	if _, err := f.ListDir(link); err == nil {
		t.Fatal("followed a symlink out of $HOME")
	}
	inside := filepath.Join(f.P.Home, "inner")
	os.Mkdir(inside, 0o755)
	os.Symlink(inside, filepath.Join(f.P.Home, "alias"))
	if _, err := f.ListDir(filepath.Join(f.P.Home, "alias")); err != nil {
		t.Fatalf("a symlink that stays inside $HOME was refused: %v", err)
	}
}
