// Package term handles everything tenant prints to a terminal: making names
// safe to print, colours, window titles and the finale's typewriter.
package term

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Clean makes a string safe to print. It drops control characters (C0, C1,
// DEL, so no escape sequences), bidi and zero-width format characters, and
// invalid UTF-8, and caps the length in runes. A file called "\e]2;pwned\a"
// comes out as the harmless "]2;pwned".
func Clean(s string, max int) string {
	var b strings.Builder
	n := 0
	for len(s) > 0 && n < max {
		r, size := utf8.DecodeRuneInString(s)
		s = s[size:]
		if r == utf8.RuneError && size <= 1 {
			continue
		}
		if r < 0x20 || (r >= 0x7f && r < 0xa0) || unicode.Is(unicode.Cf, r) {
			continue
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}

// Dim wraps text in the faint style.
func Dim(s string) string { return "\x1b[2m" + s + "\x1b[0m" }

// Title returns the OSC 2 sequence that sets the window title.
func Title(s string) string { return "\x1b]2;" + Clean(s, 80) + "\a" }

// LSColor returns the SGR code GNU ls would use for a name, from LS_COLORS.
// Only the name's extension and the plain file or directory codes are used.
func LSColor(lsColors, name string, dir bool) string {
	var fi, di, best string
	bestLen := 0
	for _, part := range strings.Split(lsColors, ":") {
		k, v, ok := strings.Cut(part, "=")
		if !ok || !validSGR(v) {
			continue
		}
		switch {
		case k == "fi":
			fi = v
		case k == "di":
			di = v
		case strings.HasPrefix(k, "*") && len(k) > bestLen && strings.HasSuffix(strings.ToLower(name), strings.ToLower(k[1:])):
			best, bestLen = v, len(k)
		}
	}
	if dir {
		if di == "" {
			return "01;34"
		}
		return di
	}
	if best != "" {
		return best
	}
	return fi
}

// Paint wraps text in an SGR code, if there is one.
func Paint(code, s string) string {
	if code == "" || code == "0" || code == "00" {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func validSGR(v string) bool {
	if v == "" || len(v) > 32 {
		return false
	}
	for _, r := range v {
		if (r < '0' || r > '9') && r != ';' {
			return false
		}
	}
	return true
}
