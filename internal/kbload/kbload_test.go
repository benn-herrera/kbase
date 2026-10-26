package kbload

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/formats"
	"kbase/internal/kb"
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
// minor from disk, older through the migration, newer, missing or unreadable
// refused — and an entry point is required.
func TestOpen(t *testing.T) {
	for _, tc := range []struct {
		name, entryPoint string
		migrated         bool
		version          string
		refused          string
		remedy           string
	}{
		{name: "current", entryPoint: "---\nkb-format: \"3.0.0\"\n---\n# KB\n", version: kb.FormatVersion},
		{name: "a newer patch", entryPoint: "---\nkb-format: \"3.0.4\"\n---\n# KB\n", version: kb.FormatVersion},
		{name: "the previous major", entryPoint: "---\nkb-format: \"2.0.0\"\n---\n# KB\n", migrated: true, version: "2.0.0"},
		{name: "the previous major, a later patch", entryPoint: "---\nkb-format: \"2.0.1\"\n---\n# KB\n", migrated: true, version: "2.0.1"},
		{name: "the first major", entryPoint: "---\nkb-format: \"1.0.0\"\n---\n# KB\n", migrated: true, version: "1.0.0"},
		{name: "the first major, a later patch", entryPoint: "---\nkb-format: \"1.0.4\"\n---\n# KB\n", migrated: true, version: "1.0.4"},
		{name: "unstamped, no frontmatter", entryPoint: "# KB\n", refused: CheckFormat},
		{name: "YAML frontmatter, unstamped", entryPoint: "---\nkind: entry-point\n---\n# KB\n", refused: CheckFormat},
		{name: "a newer major", entryPoint: "---\nkb-format: \"4.0.0\"\n---\n", refused: CheckFormat, remedy: UpdateRemedy},
		{name: "a newer minor", entryPoint: "---\nkb-format: \"3.1.0\"\n---\n", refused: CheckFormat, remedy: UpdateRemedy},
		{name: "not a version", entryPoint: "---\nkb-format: \"one\"\n---\n", refused: CheckFormat},
		{name: "not a string", entryPoint: "---\nkb-format: [1]\n---\n", refused: CheckFormat},
		{name: "no entry point", refused: kb.CheckKBRoot},
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

// TestOpenStampFile: where kb.FormatFile stands it is the KB's stamp and
// decides how the KB is read — at the current major and minor from disk,
// older through the migration, newer refused — its unreadable content is a
// kb-format refusal naming the file, and where a frontmatter stamp stands
// beside it the file's stamp is the one.
func TestOpenStampFile(t *testing.T) {
	for _, tc := range []struct {
		name              string
		stamp, entryPoint string // stamp: the stamp file's content; "" writes no file
		migrated          bool
		version, refused  string
		remedy, detail    string
	}{
		{name: "current", stamp: "kb-format: \"3.0.0\"\n", entryPoint: "# KB\n", version: kb.FormatVersion},
		{name: "a newer patch", stamp: "kb-format: \"3.0.4\"\n", entryPoint: "# KB\n", version: kb.FormatVersion},
		{name: "the previous major", stamp: "kb-format: \"2.0.0\"\n", entryPoint: "# KB\n", migrated: true, version: "2.0.0"},
		{name: "a newer minor", stamp: "kb-format: \"3.1.0\"\n", entryPoint: "# KB\n", refused: CheckFormat, remedy: UpdateRemedy},
		{name: "a newer major", stamp: "kb-format: \"4.0.0\"\n", entryPoint: "# KB\n", refused: CheckFormat, remedy: UpdateRemedy},
		{name: "not a version", stamp: "kb-format: one\n", entryPoint: "# KB\n", refused: CheckFormat},
		{name: "another key", stamp: "kind: entry-point\n", entryPoint: "# KB\n", refused: CheckFormat},
		{name: "no key", stamp: "{}\n", entryPoint: "# KB\n", refused: CheckFormat},
		{name: "a repeated key", stamp: "kb-format: \"3.0.0\"\nkb-format: \"3.0.1\"\n", entryPoint: "# KB\n", refused: CheckFormat},
		{name: "not a string", stamp: "kb-format: [1]\n", entryPoint: "# KB\n", refused: CheckFormat},
		{name: "an empty file", stamp: "\n", entryPoint: "# KB\n", refused: CheckFormat},
		{name: "the file over the frontmatter", stamp: "kb-format: \"2.0.0\"\n",
			entryPoint: "---\nkb-format: \"3.0.0\"\n---\n# KB\n", migrated: true, version: "2.0.0"},
		{name: "a stamp file and no entry point", stamp: "kb-format: \"3.0.0\"\n", version: kb.FormatVersion},
		{name: "stamped in neither place", entryPoint: "# KB\n", refused: CheckFormat,
			detail: "neither " + kb.FormatFile + " nor " + kb.EntryPointFile + "'s frontmatter"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := writeEntryPoint(t, tc.entryPoint)
			if tc.stamp != "" {
				writeFiles(t, root, map[string]string{kb.FormatFile: tc.stamp})
			}
			src, err := Open(root)
			var r result.Refusal
			switch {
			case tc.refused != "":
				if !errors.As(err, &r) || len(r) != 1 || r[0].Check != tc.refused || r[0].Remedy != tc.remedy ||
					!strings.Contains(r[0].Detail, tc.detail) {
					t.Errorf("Open = %v, want refused %s with remedy %q naming %q", err, tc.refused, tc.remedy, tc.detail)
					return
				}
				want := kb.EntryPointFile
				if tc.stamp != "" {
					want = kb.FormatFile
				}
				if !strings.HasSuffix(r[0].Path, filepath.FromSlash(want)) {
					t.Errorf("the refusal names %s, want it at %s", r[0].Path, want)
				}
			case err != nil:
				t.Fatalf("Open: %v", err)
			case src.Migrated() != tc.migrated || src.Version() != tc.version:
				t.Errorf("Open: migrated %t at %s, want %t at %s", src.Migrated(), src.Version(), tc.migrated, tc.version)
			}
		})
	}
}

// TestHasFormatStamp: the seeding build asks the loader whether a stamp
// stands, and the loader answers for the same places Open reads. A broken
// frontmatter still stands as stamped: covering it is not the caller's call.
func TestHasFormatStamp(t *testing.T) {
	for _, tc := range []struct {
		name             string
		stamp, entryText string
		want             bool
	}{
		{name: "a stamp file", stamp: "kb-format: \"3.0.0\"\n", want: true},
		{name: "a frontmatter stamp", entryText: "---\nkb-format: \"2.0.0\"\n---\n# KB\n", want: true},
		{name: "a plain entry point", entryText: "# KB\n", want: false},
		{name: "frontmatter that will not parse", entryText: "---\nkb-format: [1\n---\n# KB\n", want: true},
		{name: "no entry point", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := writeEntryPoint(t, tc.entryText)
			if tc.stamp != "" {
				writeFiles(t, root, map[string]string{kb.FormatFile: tc.stamp})
			}
			if got := HasFormatStamp(root); got != tc.want {
				t.Errorf("HasFormatStamp = %t, want %t", got, tc.want)
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

// TestOpenRefusesWhatThe200LoadRefuses: a 2.0.0 KB the 2.0.x load refuses is
// refused with that load's own items — one metadata item per fault, its field,
// file, line, allowed values, remedy and detail as that load's golden states
// them — and not as one format refusal.
func TestOpenRefusesWhatThe200LoadRefuses(t *testing.T) {
	root := writeEntryPoint(t, "---\nkb-format: \"2.0.0\"\n---\n# KB\n")
	register, err := os.ReadFile("../formats/testdata/bad-token-2.0.0/kb-root/claim-rigor.md")
	if err != nil {
		t.Fatal(err)
	}
	writeFiles(t, root, map[string]string{"claim-rigor.md": string(register)})
	golden, err := os.ReadFile("../formats/testdata/bad-token-2.0.0.faults.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var faults []struct{ Field, Path, Line, Allowed, Remedy, Detail string }
	if err := yaml.Unmarshal(golden, &faults); err != nil {
		t.Fatal(err)
	}
	_, err = Open(root)
	var r result.Refusal
	if !errors.As(err, &r) || len(r) != len(faults) || len(faults) == 0 {
		t.Fatalf("Open = %v, want the %d items of %s", err, len(faults), "bad-token-2.0.0.faults.yaml")
	}
	for i, f := range faults {
		got := r[i]
		if got.Check != kb.CheckMetadata || got.Key != f.Field || got.Path != f.Path || strconv.Itoa(got.Line) != f.Line ||
			!slices.Equal(got.Allowed, []string{f.Allowed}) || got.Remedy != f.Remedy || got.Detail != f.Detail {
			t.Errorf("item %d = %+v, want the fault %+v", i, got, f)
		}
	}
}

// TestChainReachesTheCurrentVersion: the migration chain leads from 1.0.0 to
// the version kbase reads, so a gap in the converter registry is a test
// failure here, not a refusal in use.
func TestChainReachesTheCurrentVersion(t *testing.T) {
	if _, _, _, err := formats.Chain("1.0.0", kb.FormatVersion, formats.Files{}); err != nil {
		t.Errorf("Chain(1.0.0, %s): %v", kb.FormatVersion, err)
	}
}
