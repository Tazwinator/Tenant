package sandbox

import (
	"syscall"
	"unsafe"
)

// Syscall numbers are shared by every architecture that has Landlock.
const (
	sysCreateRuleset = 444
	sysAddRule       = 445
	sysRestrictSelf  = 446

	createRulesetVersion = 1 << 0
	rulePathBeneath      = 1
	prSetNoNewPrivs      = 38
	oPath                = 0x200000 // O_PATH; the syscall package does not export it
)

type rulesetAttr struct {
	handledFS  uint64
	handledNet uint64
	scoped     uint64
}

// pathBeneathAttr is packed in the kernel (12 bytes). The Go struct has the
// same layout for those 12 bytes; the trailing padding is never read.
type pathBeneathAttr struct {
	allowed  uint64
	parentFd int32
}

// ABI returns the Landlock ABI version, or 0 if Landlock is unavailable.
func ABI() int {
	v, _, errno := syscall.Syscall(sysCreateRuleset, 0, 0, createRulesetVersion)
	if errno != 0 {
		return 0
	}
	return int(v)
}

func handledFS(abi int) Access {
	h := Access(1<<13 - 1) // ABI 1: execute .. make_sym
	if abi >= 2 {
		h |= Refer
	}
	if abi >= 3 {
		h |= Truncate
	}
	if abi >= 5 {
		h |= IoctlDev
	}
	return h
}

// Restrict applies the rules to the whole process. Everything not granted by
// a rule is denied: other files, TCP (ABI 4+), signals to other processes and
// abstract unix sockets (ABI 6+). Missing rule paths are skipped.
func Restrict(rules []Rule) Status {
	abi := ABI()
	if abi < 1 {
		return Status{Reason: "kernel has no Landlock"}
	}
	st := Status{ABI: abi, Files: true, Network: abi >= 4, Scopes: abi >= 6}
	attr := rulesetAttr{handledFS: uint64(handledFS(abi))}
	size := unsafe.Sizeof(attr.handledFS)
	if abi >= 4 {
		attr.handledNet = 1<<0 | 1<<1 // bind and connect TCP
		size += unsafe.Sizeof(attr.handledNet)
	}
	if abi >= 6 {
		attr.scoped = 1<<0 | 1<<1 // abstract unix sockets and signals
		size += unsafe.Sizeof(attr.scoped)
	}
	fd, _, errno := syscall.Syscall(sysCreateRuleset, uintptr(unsafe.Pointer(&attr)), size, 0)
	if errno != 0 {
		return Status{ABI: abi, Reason: "create_ruleset: " + errno.Error()}
	}
	defer syscall.Close(int(fd))

	for _, r := range rules {
		pfd, err := syscall.Open(r.Path, oPath|syscall.O_CLOEXEC, 0)
		if err != nil {
			continue
		}
		pb := pathBeneathAttr{allowed: uint64(r.Access & handledFS(abi)), parentFd: int32(pfd)}
		_, _, errno := syscall.Syscall6(sysAddRule, fd, rulePathBeneath, uintptr(unsafe.Pointer(&pb)), 0, 0, 0)
		syscall.Close(pfd)
		if errno != 0 {
			return Status{ABI: abi, Reason: "add_rule " + r.Path + ": " + errno.Error()}
		}
	}

	// Both calls must reach every thread the Go runtime has started.
	if _, _, errno := syscall.AllThreadsSyscall(syscall.SYS_PRCTL, prSetNoNewPrivs, 1, 0); errno != 0 {
		return Status{ABI: abi, Reason: "no_new_privs: " + errno.Error()}
	}
	if _, _, errno := syscall.AllThreadsSyscall(sysRestrictSelf, fd, 0, 0); errno != 0 {
		return Status{ABI: abi, Reason: "restrict_self: " + errno.Error()}
	}
	st.Enforced = true
	return st
}
