package modeltest

import (
	"strings"
	"testing"
)

// TestMockStreamSplitPreservesContent: whatever the chunk count, the
// concatenated pieces reconstruct the response exactly — including the
// degenerate cases (empty body, more chunks than runes, multi-byte runes,
// n ≤ 0).
func TestMockStreamSplitPreservesContent(t *testing.T) {
	for _, content := range []string{"", "a", "abcdefghij", "héllo wörld ✓"} {
		for _, n := range []int{-1, 0, 1, 3, 8, 64} {
			pieces := splitForChunks(content, n)
			if got := strings.Join(pieces, ""); got != content {
				t.Errorf("splitForChunks(%q, %d) concat = %q", content, n, got)
			}
			if len(pieces) == 0 {
				t.Errorf("splitForChunks(%q, %d) produced no pieces", content, n)
			}
		}
	}
}
