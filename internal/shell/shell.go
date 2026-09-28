// Package shell holds the hook scripts and the protocol they speak.
//
// The protocol has two channels. Text for the player goes straight to the
// terminal on stderr. Actions for the hook go on stdout, one per line: a
// verb from a fixed list, then integers only. The hook dispatches them with
// `case` and never evaluates them.
package shell

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/Tazwinator/Tenant/internal/mech"
)

//go:embed bash.sh
var bashScript string

//go:embed zsh.zsh
var zshScript string

// Shells lists the supported shells.
var Shells = []string{"bash", "zsh"}

// Script returns the hook for a shell, pointing at the binary.
func Script(sh, bin string) (string, error) {
	var s string
	switch sh {
	case "bash":
		s = bashScript
	case "zsh":
		s = zshScript
	default:
		return "", fmt.Errorf("unsupported shell %q (supported: %s)", sh, strings.Join(Shells, ", "))
	}
	return strings.ReplaceAll(s, "__TENANT_BIN__", Quote(bin)), nil
}

// Quote single-quotes a string for bash and zsh.
func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Verbs is every action the hook understands.
var Verbs = map[string]int{ // verb → number of integer arguments
	"glyph": 3,
	"time":  0,
	"ghost": 0,
}

// Encode renders actions as protocol lines. Unknown verbs and wrong arity
// are dropped, so nothing unexpected can ever reach the hook.
func Encode(acts []mech.Action) string {
	var b strings.Builder
	for _, a := range acts {
		if n, ok := Verbs[a.Verb]; !ok || n != len(a.Args) {
			continue
		}
		b.WriteString(a.String())
		b.WriteByte('\n')
	}
	return b.String()
}
