package format40

import "testing"

// TestAppendMarker: a marker attaches with no space after the line's last
// character, ahead of its trailing whitespace, a second marker abutting the
// first; a line carrying the marker stands.
func TestAppendMarker(t *testing.T) {
	claim, analysis := StatementMarker("clm-aaaaaa"), StatementMarker("ana-aaaaaa")
	for _, tc := range []struct{ line, marker, want string }{
		{"# Cure Is Hard", claim, "# Cure Is Hard" + claim},
		{"Text.  \t", claim, "Text." + claim + "  \t"},
		{"# Cure Is Hard" + claim, analysis, "# Cure Is Hard" + claim + analysis},
		{"Text." + claim + " ", claim, "Text." + claim + " "},
		{">", analysis, ">" + analysis},
	} {
		if got := AppendMarker(tc.line, tc.marker); got != tc.want {
			t.Errorf("AppendMarker(%q, %q) = %q, want %q", tc.line, tc.marker, got, tc.want)
		}
	}
}
