package kbload

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/migrate"
	"kbase/internal/result"
)

func writeEntryPoint(t *testing.T, text string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), kb.KBDir)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if text != "" {
		if err := os.WriteFile(filepath.Join(root, kb.EntryPointFile), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// TestOpen: the stamp decides how a KB is read — at the current major and
// minor from disk, older through the migration, newer or unreadable
// refused — and an entry point is required.
func TestOpen(t *testing.T) {
	for _, tc := range []struct {
		name, entryPoint string
		migrated         bool
		version          string
		refused          string
		remedy           string
	}{
		{name: "current", entryPoint: "---\nkb-format: \"1.0.0\"\n---\n# KB\n", version: kb.FormatVersion},
		{name: "a newer patch", entryPoint: "---\nkb-format: \"1.0.4\"\n---\n# KB\n", version: kb.FormatVersion},
		{name: "unstamped, no frontmatter", entryPoint: "# KB\n", migrated: true, version: kb.UnstampedFormatVersion},
		{name: "a comment block", entryPoint: "<!-- kb-frontmatter\nkind: entry-point\n-->\n# KB\n", migrated: true, version: kb.UnstampedFormatVersion},
		{name: "a comment block stamped", entryPoint: "<!-- kb-frontmatter\nkb-format: 0.9.3\n-->\n# KB\n", migrated: true, version: "0.9.3"},
		{name: "YAML frontmatter, unstamped", entryPoint: "---\nkind: entry-point\n---\n# KB\n", migrated: true, version: kb.UnstampedFormatVersion},
		{name: "a newer major", entryPoint: "---\nkb-format: \"2.0.0\"\n---\n", refused: CheckFormat, remedy: UpdateRemedy},
		{name: "a newer minor", entryPoint: "---\nkb-format: \"1.1.0\"\n---\n", refused: CheckFormat, remedy: UpdateRemedy},
		{name: "a newer major in a comment block", entryPoint: "<!-- kb-frontmatter\nkb-format: 2.0.0\n-->\n", refused: CheckFormat, remedy: UpdateRemedy},
		{name: "not a version", entryPoint: "---\nkb-format: \"one\"\n---\n", refused: CheckFormat},
		{name: "not a string", entryPoint: "---\nkb-format: [1]\n---\n", refused: CheckFormat},
		{name: "no entry point", refused: CheckKBRoot},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src, err := Open(writeEntryPoint(t, tc.entryPoint))
			var r result.Refusal
			switch {
			case tc.refused != "":
				if !errors.As(err, &r) || len(r) != 1 || r[0].Check != tc.refused || r[0].Remedy != tc.remedy {
					t.Errorf("Open = %v, want refused %s with remedy %q", err, tc.refused, tc.remedy)
				}
			case err != nil:
				t.Fatalf("Open: %v", err)
			case src.Migrated() != tc.migrated || src.Version() != tc.version:
				t.Errorf("Open: migrated %t at %s, want %t at %s", src.Migrated(), src.Version(), tc.migrated, tc.version)
			}
		})
	}
}

// TestChainReachesTheCurrentVersion: the migration chain leads from the
// unstamped version to the version kbase reads, so a gap in the converter
// registry is a test failure here, not a refusal in use.
func TestChainReachesTheCurrentVersion(t *testing.T) {
	if _, _, err := migrate.Chain(kb.UnstampedFormatVersion, kb.FormatVersion, migrate.Files{}); err != nil {
		t.Errorf("Chain(%s, %s): %v", kb.UnstampedFormatVersion, kb.FormatVersion, err)
	}
}
