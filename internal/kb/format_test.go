package kb

import "testing"

// TestFormatStamp: the stamp is read from YAML frontmatter alone; an entry
// point carrying none, and a stamp that is not one non-empty string, is
// refused.
func TestFormatStamp(t *testing.T) {
	cases := []struct {
		name, entryPoint, want string
		refused                bool
	}{
		{name: "no frontmatter", entryPoint: "# KB\n", refused: true},
		{name: "stamped", entryPoint: "---\nkind: entry-point\nkb-format: \"1.0.0\"\n---\n\n# KB\n", want: "1.0.0"},
		{name: "stamped unquoted", entryPoint: "---\nkind: entry-point\nkb-format: 1.0.3\n---\n", want: "1.0.3"},
		{name: "fences ending in CRLF", entryPoint: "---\r\nkb-format: \"1.0.0\"\r\n---\r\n# KB\r\n", want: "1.0.0"},
		{name: "no stamp", entryPoint: "---\nkind: entry-point\n---\n# KB\n", refused: true},
		{name: "a list for a stamp", entryPoint: "---\nkb-format: [1, 0]\n---\n", refused: true},
		{name: "an empty stamp", entryPoint: "---\nkb-format:\n---\n", refused: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := FormatStamp(c.entryPoint)
			if (err != nil) != c.refused || got != c.want {
				t.Errorf("FormatStamp = %q, %v; want %q, refused %t", got, err, c.want, c.refused)
			}
		})
	}
}

// TestFormatVersion: kbase reads and writes 3.0.0, the major whose nodes and
// edges are records.
func TestFormatVersion(t *testing.T) {
	if FormatVersion != "3.0.0" {
		t.Errorf("FormatVersion = %q, want 3.0.0", FormatVersion)
	}
}
