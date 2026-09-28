// Package sandbox restricts tenant with Landlock before it does anything
// that depends on the player's files.
//
// It uses raw syscalls rather than a library, so the whole sandbox can be
// read in one sitting. On kernels without Landlock it does nothing, and
// Status says so plainly.
package sandbox

// Rule grants access beneath one directory.
type Rule struct {
	Path   string
	Access Access
}

// Access is a set of filesystem rights, a subset of what Landlock handles.
type Access uint64

// Filesystem rights, as defined by the Landlock ABI.
const (
	Execute    Access = 1 << 0
	WriteFile  Access = 1 << 1
	ReadFile   Access = 1 << 2
	ReadDir    Access = 1 << 3
	RemoveDir  Access = 1 << 4
	RemoveFile Access = 1 << 5
	MakeChar   Access = 1 << 6
	MakeDir    Access = 1 << 7
	MakeReg    Access = 1 << 8
	MakeSock   Access = 1 << 9
	MakeFifo   Access = 1 << 10
	MakeBlock  Access = 1 << 11
	MakeSym    Access = 1 << 12
	Refer      Access = 1 << 13 // ABI 2
	Truncate   Access = 1 << 14 // ABI 3
	IoctlDev   Access = 1 << 15 // ABI 5
)

// OwnDir is everything tenant needs in its own state directory.
const OwnDir = ReadFile | WriteFile | ReadDir | RemoveDir | RemoveFile | MakeDir | MakeReg | Refer | Truncate

// Status describes what the sandbox managed to enforce.
type Status struct {
	ABI      int
	Files    bool
	Network  bool
	Scopes   bool
	Reason   string // why it isn't fully enforced
	Enforced bool
}

func (s Status) String() string {
	if !s.Enforced {
		return "unavailable (" + s.Reason + "), enforced by code only"
	}
	what := "files"
	if s.Network {
		what += ", network"
	}
	if s.Scopes {
		what += ", signals and abstract sockets"
	}
	return "landlock ABI " + itoa(s.ABI) + ": " + what + " restricted"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
