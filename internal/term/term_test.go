package term

import "testing"

func TestClean(t *testing.T) {
	for in, want := range map[string]string{
		"still_here.txt":      "still_here.txt",
		"\x1b]2;pwned\a":      "]2;pwned",
		"\x1b[31mred\x1b[0m":  "[31mred[0m",
		"line\nbreak":         "linebreak",
		"\u202eevil\u2066":    "evil",
		"c1\u009bcontrol":     "c1control",
		"bad\xffutf8":         "badutf8",
		"ünïcødé ok":          "ünïcødé ok",
		"zero\u200bwidth":     "zerowidth",
		"tab\there":           "tabhere",
		"del\x7fete":          "delete",
		"$(touch pwned)`id`%": "$(touch pwned)`id`%",
	} {
		if got := Clean(in, 100); got != want {
			t.Errorf("Clean(%q) = %q, want %q", in, got, want)
		}
	}
	if got := Clean("abcdef", 3); got != "abc" {
		t.Errorf("Clean truncation = %q", got)
	}
}

func TestLSColor(t *testing.T) {
	lc := "rs=0:di=01;34:fi=00:*.txt=00;32:*.tar.gz=01;31:*.gz=01;35:evil=\x1b[31m"
	for _, c := range []struct {
		name string
		dir  bool
		want string
	}{
		{"notes.txt", false, "00;32"},
		{"x.tar.gz", false, "01;31"},
		{"x.gz", false, "01;35"},
		{"cmd", true, "01;34"},
		{"README", false, "00"},
	} {
		if got := LSColor(lc, c.name, c.dir); got != c.want {
			t.Errorf("LSColor(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}
