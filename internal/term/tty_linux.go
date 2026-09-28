package term

import (
	"syscall"
	"unsafe"
)

func ioctl(fd int, req uintptr, arg unsafe.Pointer) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), req, uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}

// IsTTY reports whether fd is a terminal.
func IsTTY(fd int) bool {
	var t syscall.Termios
	return ioctl(fd, syscall.TCGETS, unsafe.Pointer(&t)) == nil
}

// Width returns the terminal width of fd, or 0.
func Width(fd int) int {
	var ws struct{ Row, Col, X, Y uint16 }
	if ioctl(fd, syscall.TIOCGWINSZ, unsafe.Pointer(&ws)) != nil {
		return 0
	}
	return int(ws.Col)
}

// Mode is a saved terminal state.
type Mode struct {
	fd int
	t  syscall.Termios
}

// Poll switches fd to a mode where reads return straight away with whatever
// has been typed, without echo, so the typewriter can notice a keypress.
// Restore puts it back.
func Poll(fd int) (*Mode, error) {
	var t syscall.Termios
	if err := ioctl(fd, syscall.TCGETS, unsafe.Pointer(&t)); err != nil {
		return nil, err
	}
	saved := &Mode{fd: fd, t: t}
	t.Lflag &^= syscall.ICANON | syscall.ECHO
	t.Cc[syscall.VMIN] = 0
	t.Cc[syscall.VTIME] = 0
	if err := ioctl(fd, syscall.TCSETS, unsafe.Pointer(&t)); err != nil {
		return nil, err
	}
	return saved, nil
}

// Restore puts the terminal back the way it was.
func (m *Mode) Restore() {
	if m != nil {
		ioctl(m.fd, syscall.TCSETS, unsafe.Pointer(&m.t))
	}
}

// Flush discards typed-ahead input on fd.
func Flush(fd int) {
	const tcflsh, tciflush = 0x540B, 0 // not exported by the syscall package
	syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), tcflsh, tciflush)
}
