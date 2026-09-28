//go:build !linux

package sandbox

// ABI is always 0 off Linux.
func ABI() int { return 0 }

// Restrict does nothing off Linux.
func Restrict(rules []Rule) Status {
	return Status{Reason: "Landlock is Linux-only"}
}
