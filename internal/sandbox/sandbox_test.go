package sandbox

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// Landlock can't be undone, so the checks run in a child process: the test
// binary re-runs itself with TENANT_SANDBOX_CHILD set.
func TestMain(m *testing.M) {
	if dir := os.Getenv("TENANT_SANDBOX_CHILD"); dir != "" {
		os.Exit(child(dir))
	}
	os.Exit(m.Run())
}

func child(dir string) int {
	home, state := filepath.Join(dir, "home"), filepath.Join(dir, "home", ".local", "state", "tenant")
	st := Restrict([]Rule{{Path: home, Access: ReadDir}, {Path: state, Access: OwnDir}})
	if !st.Enforced {
		os.Stdout.WriteString("unenforced: " + st.Reason)
		return 3
	}
	var fails []string
	expect := func(what string, err error, allowed bool) {
		if allowed && err != nil {
			fails = append(fails, what+" should work: "+err.Error())
		}
		if !allowed && err == nil {
			fails = append(fails, what+" should be denied")
		}
	}
	_, err := os.ReadDir(home)
	expect("listing $HOME", err, true)
	_, err = os.ReadFile(filepath.Join(home, "secret.txt"))
	expect("reading a file in $HOME", err, false)
	err = os.WriteFile(filepath.Join(home, "new.txt"), []byte("x"), 0o600)
	expect("writing in $HOME", err, false)
	err = os.WriteFile(filepath.Join(state, "state.json"), []byte("{}"), 0o600)
	expect("writing own state", err, true)
	_, err = os.ReadFile(filepath.Join(state, "state.json"))
	expect("reading own state", err, true)
	_, err = os.ReadFile("/etc/hostname")
	expect("reading /etc", err, false)
	// /bin/false: if exec ever succeeded, the child would exit 1 and fail the test.
	err = syscall.Exec("/bin/false", []string{"false"}, nil)
	expect("exec", err, false)
	if st.Network {
		// Raw syscalls: importing net would link cgo into the test binary,
		// and Landlock needs a cgo-free process.
		fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
		if err == nil {
			err = syscall.Connect(fd, &syscall.SockaddrInet4{Port: 9, Addr: [4]byte{127, 0, 0, 1}})
			syscall.Close(fd)
		}
		if err == nil || !errors.Is(err, syscall.EACCES) {
			fails = append(fails, "tcp connect should be denied by landlock, got: "+errString(err))
		}
	}
	if len(fails) > 0 {
		os.Stdout.WriteString(strings.Join(fails, "\n"))
		return 1
	}
	return 0
}

func errString(err error) string {
	if err == nil {
		return "no error"
	}
	return err.Error()
}

func TestRestrict(t *testing.T) {
	if ABI() < 1 {
		t.Skip("kernel has no Landlock")
	}
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "home", ".local", "state", "tenant"), 0o700)
	os.WriteFile(filepath.Join(dir, "home", "secret.txt"), []byte("private"), 0o600)
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), "TENANT_SANDBOX_CHILD="+dir)
	out, err := cmd.CombinedOutput()
	if code := cmd.ProcessState.ExitCode(); code == 3 {
		t.Skip(string(out))
	}
	if err != nil {
		t.Fatalf("sandbox child failed: %v\n%s", err, out)
	}
}
