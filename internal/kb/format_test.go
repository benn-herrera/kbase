package kb

import "testing"

// TestFormatStamp: the stamp is read from YAML frontmatter alone; an entry
// point without YAML frontmatter is in a superseded form, and a stamp that
// is not one non-empty string is refused.
func TestFormatStamp(t *testing.T) {
	cases := []struct {
		name, entryPoint, want string
		yamlForm, refused      bool
	}{
		{name: "no frontmatter", entryPoint: "# KB\n"},
		{name: "a comment block", entryPoint: "<!-- kb-frontmatter\nkind: entry-point\nkb-format: 1.0.0\n-->\n\n# KB\n"},
		{name: "stamped", entryPoint: "---\nkind: entry-point\nkb-format: \"1.0.0\"\n---\n\n# KB\n", want: "1.0.0", yamlForm: true},
		{name: "stamped unquoted", entryPoint: "---\nkind: entry-point\nkb-format: 1.0.3\n---\n", want: "1.0.3", yamlForm: true},
		{name: "fences ending in CRLF", entryPoint: "---\r\nkb-format: \"1.0.0\"\r\n---\r\n# KB\r\n", want: "1.0.0", yamlForm: true},
		{name: "no stamp", entryPoint: "---\nkind: entry-point\n---\n# KB\n", want: UnstampedFormatVersion, yamlForm: true},
		{name: "a list for a stamp", entryPoint: "---\nkb-format: [1, 0]\n---\n", yamlForm: true, refused: true},
		{name: "an empty stamp", entryPoint: "---\nkb-format:\n---\n", yamlForm: true, refused: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, yamlForm, err := FormatStamp(c.entryPoint)
			if (err != nil) != c.refused || got != c.want || yamlForm != c.yamlForm {
				t.Errorf("FormatStamp = %q, %t, %v; want %q, %t, refused %t", got, yamlForm, err, c.want, c.yamlForm, c.refused)
			}
		})
	}
}

// TestFormatVersion: kbase reads and writes 1.0.0.
func TestFormatVersion(t *testing.T) {
	if FormatVersion != "1.0.0" {
		t.Errorf("FormatVersion = %q, want 1.0.0", FormatVersion)
	}
}

// TestStampFormat: the stamp is the frontmatter's last key, replacing one it
// carried, and a document with no frontmatter gains one holding it alone.
func TestStampFormat(t *testing.T) {
	for _, c := range []struct{ name, in, want string }{
		{"no frontmatter", "# KB\n", "---\nkb-format: \"1.0.0\"\n---\n# KB\n"},
		{"appended last", "---\nkind: entry-point\n---\n\n# KB\n", "---\nkind: entry-point\nkb-format: \"1.0.0\"\n---\n\n# KB\n"},
		{"an older stamp moved last", "---\nkb-format: \"0.9.0\"\nkind: entry-point\nsubtree-claims: []\n---\n", "---\nkind: entry-point\nsubtree-claims: []\nkb-format: \"1.0.0\"\n---\n"},
		{"already stamped", "---\nkind: entry-point\nkb-format: \"1.0.0\"\n---\n", "---\nkind: entry-point\nkb-format: \"1.0.0\"\n---\n"},
		{"empty frontmatter", "---\n---\n# KB\n", "---\nkb-format: \"1.0.0\"\n---\n# KB\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := StampFormat(c.in, FormatVersion)
			if err != nil || got != c.want {
				t.Errorf("StampFormat = %q, %v; want %q", got, err, c.want)
			}
		})
	}
}
