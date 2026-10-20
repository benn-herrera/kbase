package kbload

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

// writeFiles writes each kb-root-relative slash path with its text, under
// root.
func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, text := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
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
		{name: "current", entryPoint: "---\nkb-format: \"2.0.0\"\n---\n# KB\n", version: kb.FormatVersion},
		{name: "a newer patch", entryPoint: "---\nkb-format: \"2.0.4\"\n---\n# KB\n", version: kb.FormatVersion},
		{name: "the previous major", entryPoint: "---\nkb-format: \"1.0.0\"\n---\n# KB\n", migrated: true, version: "1.0.0"},
		{name: "the previous major, a later patch", entryPoint: "---\nkb-format: \"1.0.4\"\n---\n# KB\n", migrated: true, version: "1.0.4"},
		{name: "unstamped, no frontmatter", entryPoint: "# KB\n", migrated: true, version: kb.UnstampedFormatVersion},
		{name: "a comment block", entryPoint: "<!-- kb-frontmatter\nkind: entry-point\n-->\n# KB\n", migrated: true, version: kb.UnstampedFormatVersion},
		{name: "a comment block stamped", entryPoint: "<!-- kb-frontmatter\nkb-format: 0.9.3\n-->\n# KB\n", migrated: true, version: "0.9.3"},
		{name: "YAML frontmatter, unstamped", entryPoint: "---\nkind: entry-point\n---\n# KB\n", migrated: true, version: kb.UnstampedFormatVersion},
		{name: "a newer major", entryPoint: "---\nkb-format: \"3.0.0\"\n---\n", refused: CheckFormat, remedy: UpdateRemedy},
		{name: "a newer minor", entryPoint: "---\nkb-format: \"2.1.0\"\n---\n", refused: CheckFormat, remedy: UpdateRemedy},
		{name: "a newer major in a comment block", entryPoint: "<!-- kb-frontmatter\nkb-format: 3.0.0\n-->\n", refused: CheckFormat, remedy: UpdateRemedy},
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

// TestOpenRefusesASymlinkedDocument: a KB document that is a symbolic link is
// refused, naming it, rather than left out of the files a conversion reads —
// the 1.0.0 register the conversion renames included.
func TestOpenRefusesASymlinkedDocument(t *testing.T) {
	for _, name := range []string{"claim-rigor.md", "claim-quality.md"} {
		t.Run(name, func(t *testing.T) {
			root := writeEntryPoint(t, "---\nkb-format: \"1.0.0\"\n---\n# KB\n")
			target := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(target, []byte("# Register\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(root, name)
			if err := os.Symlink(target, link); err != nil {
				t.Fatal(err)
			}
			_, err := Open(root)
			var r result.Refusal
			if !errors.As(err, &r) || len(r) != 1 || r[0].Check != kb.CheckSymlink || r[0].Path != link {
				t.Fatalf("Open over a symlinked %s = %v, want one %s refusal naming %s", name, err, kb.CheckSymlink, link)
			}
		})
	}
}

// TestOpenConvertsTheReaderFileSet: a conversion holds no document in a
// directory the reader never descends into, so nothing there is renamed,
// written or declared obsolete.
func TestOpenConvertsTheReaderFileSet(t *testing.T) {
	root := writeEntryPoint(t, "---\nkb-format: \"1.0.0\"\n---\n# KB\n")
	register := "# Register\n\n## Claim A\n<!-- id: clm-aaaaaa -->\n\n### Quality\n- confidence: 0.70\n"
	writeFiles(t, root, map[string]string{
		"tools/claim-quality.md": register,
		"session/notes.md":       "# Notes\n",
	})
	src, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	for _, p := range slices.Concat(src.MigratedFiles(), src.Obsolete()) {
		for _, dir := range []string{"tools", "session"} {
			if strings.HasPrefix(p, kb.KBDir+"/"+dir+"/") {
				t.Errorf("%s is in the conversion's file set, and the reader never reads it", p)
			}
		}
	}
}

// TestOpenRefusesAnInvariantDocumentWithProse: an invariants.md holding
// content beside its declarations is refused as a conversion, naming the file
// and the line, and its bytes stand.
func TestOpenRefusesAnInvariantDocumentWithProse(t *testing.T) {
	root := writeEntryPoint(t, "---\nkb-format: \"1.0.0\"\n---\n# KB\n")
	text := "# Invariants\n\nRead this before changing anything.\n\n### INVARIANT-S3: A labelling convention\n\nBody.\n"
	writeFiles(t, root, map[string]string{kb.InvariantsFile: text})
	_, err := Open(root)
	var r result.Refusal
	if !errors.As(err, &r) || len(r) != 1 || r[0].Check != CheckConversion || r[0].Line != 3 ||
		r[0].Path != filepath.Join(root, kb.InvariantsFile) || !strings.Contains(r[0].Detail, "Read this before changing anything.") {
		t.Fatalf("Open over an invariants.md holding prose = %v, want one %s refusal naming line 3 and its content", err, CheckConversion)
	}
	if got, err := os.ReadFile(filepath.Join(root, kb.InvariantsFile)); err != nil || string(got) != text {
		t.Errorf("%s = %q (%v), want it as written", kb.InvariantsFile, got, err)
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
