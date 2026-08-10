package text

import (
	"fmt"
	"strings"
	"testing"
)

// TestCapWords covers the rule directly: collapse whitespace, then cut at a
// word boundary and mark the cut.
func TestCapWords(t *testing.T) {
	const cap = 40
	long := make([]string, cap*2)
	for i := range long {
		long[i] = fmt.Sprintf("w%d", i)
	}

	for _, tc := range []struct {
		name, in string
		n        int
		want     string
	}{
		{"whitespace runs collapse", " a\tb\n c ", cap, "a b c"},
		{"empty stays empty", "   \n\t ", cap, ""},
		{"under the cap is untouched", "just a few words", cap, "just a few words"},
		{"at the cap is not marked", strings.Join(long[:cap], " "), cap, strings.Join(long[:cap], " ")},
		{"over the cap is cut and marked", strings.Join(long, " "), cap, strings.Join(long[:cap], " ") + CapEllipsis},
		{"the cap is the caller's", "one two three four", 2, "one two" + CapEllipsis},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := CapWords(tc.in, tc.n); got != tc.want {
				t.Errorf("CapWords(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
			}
		})
	}
}
