//go:build !linux

package term

// IsTTY is always false off Linux.
func IsTTY(fd int) bool { return false }

// Width is unknown off Linux.
func Width(fd int) int { return 0 }

// Mode is a saved terminal state.
type Mode struct{}

// Poll is unsupported off Linux.
func Poll(fd int) (*Mode, error) { return nil, errUnsupported }

// Restore does nothing.
func (m *Mode) Restore() {}

// Flush does nothing.
func Flush(fd int) {}

type unsupported struct{}

func (unsupported) Error() string { return "unsupported" }

var errUnsupported error = unsupported{}
