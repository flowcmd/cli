package textutil

import "testing"

func TestLastLine(t *testing.T) {
	cases := map[string]string{
		"":             "",
		"one":          "one",
		"one\ntwo":     "two",
		"one\ntwo\n":   "two",
		"a\nb\nc\n\n":  "c",
	}
	for in, want := range cases {
		if got := LastLine(in); got != want {
			t.Errorf("LastLine(%q)=%q want %q", in, got, want)
		}
	}
}

func TestIndent(t *testing.T) {
	if got := Indent("a\nb", "> "); got != "> a\n> b" {
		t.Errorf("got %q", got)
	}
	if got := Indent("a\n", "  "); got != "  a" {
		t.Errorf("trailing newline not trimmed: %q", got)
	}
}
