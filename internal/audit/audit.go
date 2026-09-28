// Package audit is the only package in tenant that touches the filesystem.
//
// Everything that concerns the player's own files goes through here and is
// written to the audit log before it happens. The log is what `tenant
// confess` prints, so it has to be complete and it has to be true.
package audit

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// Paths are the directories tenant is allowed to know about.
type Paths struct {
	Home  string // $HOME
	State string // $XDG_STATE_HOME/tenant
}

// Resolve reads the paths from the environment. Relative values are ignored,
// as the XDG spec requires.
func Resolve() (Paths, error) {
	home := os.Getenv("HOME")
	if home == "" || !filepath.IsAbs(home) {
		return Paths{}, errors.New("HOME is not set to an absolute path")
	}
	home = filepath.Clean(home)
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" || !filepath.IsAbs(base) {
		base = filepath.Join(home, ".local", "state")
	}
	return Paths{Home: home, State: filepath.Join(filepath.Clean(base), "tenant")}, nil
}

// Tilde shortens a path under home to ~/..., the way shells show it.
func (p Paths) Tilde(path string) string {
	if path == p.Home {
		return "~"
	}
	if strings.HasPrefix(path, p.Home+"/") {
		return "~" + path[len(p.Home):]
	}
	return path
}

// UnderHome reports whether path is home or inside it.
func (p Paths) UnderHome(path string) bool {
	path = filepath.Clean(path)
	return path == p.Home || strings.HasPrefix(path, p.Home+"/")
}

// FS is the gateway. The zero value is not usable; use New.
type FS struct {
	P   Paths
	Now func() time.Time
}

// New returns a gateway for the given paths.
func New(p Paths) *FS { return &FS{P: p, Now: time.Now} }

// Own file names. Nothing outside this list is ever written.
const (
	FileActive = "active"
	FileState  = "state.json"
	FileSeen   = "seen.json"
	FileLog    = "audit.log"
	FileLock   = "lock"
)

var ownFiles = map[string]bool{FileActive: true, FileState: true, FileSeen: true, FileLog: true, FileLock: true}

func (f *FS) own(name string) (string, error) {
	if !ownFiles[name] {
		return "", fmt.Errorf("audit: %q is not one of tenant's own files", name)
	}
	return filepath.Join(f.P.State, name), nil
}

// Active reports whether `tenant start` has been run and not evicted.
func (f *FS) Active() bool {
	_, err := os.Lstat(filepath.Join(f.P.State, FileActive))
	return err == nil
}

// Prepare creates the state directory. It is called by `tenant start`
// before the sandbox is applied.
func (f *FS) Prepare() error { return os.MkdirAll(f.P.State, 0o700) }

// ReadOwn reads one of tenant's own files.
func (f *FS) ReadOwn(name string) ([]byte, error) {
	path, err := f.own(name)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

// WriteOwn replaces one of tenant's own files atomically.
func (f *FS) WriteOwn(name string, data []byte) error {
	path, err := f.own(name)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(f.P.State, "."+name+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Lock takes the state lock, so that several shells can't interleave. It
// gives up after about 40ms rather than stall a prompt.
func (f *FS) Lock() (unlock func(), err error) {
	path, _ := f.own(FileLock)
	fh, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	for i := 0; ; i++ {
		err = syscall.Flock(int(fh.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if i == 20 {
			fh.Close()
			return nil, errors.New("audit: state is locked by another shell")
		}
		time.Sleep(2 * time.Millisecond)
	}
	return func() {
		syscall.Flock(int(fh.Fd()), syscall.LOCK_UN)
		fh.Close()
	}, nil
}

// Entry is one line of the audit log.
type Entry struct {
	At     time.Time
	Kind   string // started, list, wrote
	Detail string
}

// Record appends an entry to the audit log.
func (f *FS) Record(kind, detail string) error {
	path, _ := f.own(FileLog)
	fh, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer fh.Close()
	clean := strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace
	_, err = fmt.Fprintf(fh, "%s\t%s\t%s\n", f.Now().Format(time.RFC3339), clean(kind), clean(detail))
	return err
}

// Entries reads the whole audit log.
func (f *FS) Entries() ([]Entry, error) {
	path, _ := f.own(FileLog)
	fh, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	var out []Entry
	sc := bufio.NewScanner(fh)
	for sc.Scan() {
		parts := strings.SplitN(sc.Text(), "\t", 3)
		if len(parts) != 3 {
			continue
		}
		at, err := time.Parse(time.RFC3339, parts[0])
		if err != nil {
			continue
		}
		out = append(out, Entry{At: at, Kind: parts[1], Detail: parts[2]})
	}
	return out, sc.Err()
}

// Name is one directory entry: its name, and whether it is a directory.
// Nothing else about the entry is looked at.
type Name struct {
	Name string
	Dir  bool
}

// MaxNames is the most entries tenant will read from one directory.
const MaxNames = 256

// ListDir reads the names in a directory under $HOME. The listing is logged
// before it happens and the names are kept in seen.json for confess.
func (f *FS) ListDir(dir string) ([]Name, error) {
	dir = filepath.Clean(dir)
	if !filepath.IsAbs(dir) || !f.P.UnderHome(dir) {
		return nil, fmt.Errorf("audit: %s is outside $HOME", dir)
	}
	if err := f.Record("list", f.P.Tilde(dir)+" (names only)"); err != nil {
		return nil, err
	}
	fh, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	ents, err := fh.ReadDir(MaxNames)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	names := make([]Name, 0, len(ents))
	for _, e := range ents {
		names = append(names, Name{Name: e.Name(), Dir: e.IsDir()})
	}
	sort.Slice(names, func(i, j int) bool { return names[i].Name < names[j].Name })
	if err := f.remember(f.P.Tilde(dir), names); err != nil {
		return nil, err
	}
	return names, nil
}

// Seen maps a directory (as ~/...) to every name tenant has read in it.
// Directories are shown with a trailing slash.
type Seen map[string][]string

// Seen returns everything tenant has ever read.
func (f *FS) Seen() (Seen, error) {
	data, err := f.ReadOwn(FileSeen)
	if errors.Is(err, os.ErrNotExist) {
		return Seen{}, nil
	}
	if err != nil {
		return nil, err
	}
	seen := Seen{}
	if err := json.Unmarshal(data, &seen); err != nil {
		return nil, err
	}
	return seen, nil
}

func (f *FS) remember(dir string, names []Name) error {
	seen, err := f.Seen()
	if err != nil {
		seen = Seen{}
	}
	set := map[string]bool{}
	for _, n := range seen[dir] {
		set[n] = true
	}
	for _, n := range names {
		if n.Dir {
			set[n.Name+"/"] = true
		} else {
			set[n.Name] = true
		}
	}
	all := make([]string, 0, len(set))
	for n := range set {
		all = append(all, n)
	}
	sort.Strings(all)
	seen[dir] = all
	data, err := json.MarshalIndent(seen, "", "  ")
	if err != nil {
		return err
	}
	return f.WriteOwn(FileSeen, data)
}

// Exists reports whether a path exists, without opening it. Only used for
// the git rebase markers, and disclosed as a standing behaviour in confess.
func (f *FS) Exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// TTY returns the name of the terminal on stdin, such as "pts/3".
func (f *FS) TTY() string {
	target, err := os.Readlink("/proc/self/fd/0")
	if err != nil || !strings.HasPrefix(target, "/dev/") {
		return ""
	}
	return strings.TrimPrefix(target, "/dev/")
}

// Activate writes the sentinel that switches the hooks on.
func (f *FS) Activate() error {
	return f.WriteOwn(FileActive, []byte("tenant is active. `tenant evict` removes this directory.\n"))
}

// Evict removes the sentinel first, so every shell goes quiet at its next
// prompt, then everything in the state directory, then the directory. It
// never opens the parent directory, so the sandbox only has to allow
// removing one directory there.
func (f *FS) Evict() error {
	err := os.Remove(filepath.Join(f.P.State, FileActive))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	ents, err := os.ReadDir(f.P.State)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range ents {
		if err := os.RemoveAll(filepath.Join(f.P.State, e.Name())); err != nil {
			return err
		}
	}
	return os.Remove(f.P.State)
}

// StateExists reports whether the state directory is present at all.
func (f *FS) StateExists() bool {
	_, err := os.Lstat(f.P.State)
	return err == nil
}
