package index

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/kbload"
	"kbase/internal/log"
)

// TestAStoppedMigrationSaveResumes: a 2.0.0 KB's save writes the edge file,
// then the node file, then the leaves, then the stamp file at the version the
// tree is still at, then the stripped entry point, and last rewrites the stamp
// file at the current version. Stopped before the node file, the KB keeps its
// stamp and its leaves, and its next load converts it afresh; stopped before
// the leaves, its next load takes both record files as they stand and removes
// each leaf's frontmatter unread. Either way the reopened KB loads as a
// conversion from 2.0.0.
func TestAStoppedMigrationSaveLoadsAsAConversion(t *testing.T) {
	leaf := "---\nkind: leaf\nclaims: [clm-aaaaaa]\n---\n[↑ Vol](index.md)\n\n# A\n\nThe result holds. <!-- claim-rigor: clm-aaaaaa -->\n"
	for _, tc := range []struct {
		name string
		// block makes the save's next write fail and returns what lifts it.
		block   func(t *testing.T, root string) func()
		resumed bool
	}{
		{"before the node file", func(t *testing.T, root string) func() {
			p := filepath.Join(root, filepath.FromSlash(kb.GraphNodesFile))
			if err := os.Mkdir(p, 0o755); err != nil {
				t.Fatal(err)
			}
			return func() {
				if err := os.Remove(p); err != nil {
					t.Fatal(err)
				}
			}
		}, false},
		{"before the leaves", func(t *testing.T, root string) func() {
			vol := filepath.Join(root, "vol")
			if err := os.Chmod(vol, 0o555); err != nil {
				t.Fatal(err)
			}
			lift := func() {
				if err := os.Chmod(vol, 0o755); err != nil {
					t.Error(err)
				}
			}
			t.Cleanup(lift)
			return lift
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := writeKB(t, map[string]string{
				"entry-point.md": "---\nkind: entry-point\nkb-format: \"2.0.0\"\n---\n\n# KB\n\n- [vol](vol/index.md)\n",
				"vol/index.md":   "---\nkind: index\n---\n[↑ KB](../entry-point.md)\n\n# Vol\n\n- [a](a.md)\n",
				"vol/a.md":       leaf,
				"claim-rigor.md": "# Claim Rigor\n\n---\n\n## Claim A\n<!-- id: clm-aaaaaa -->\n\n> **Leaf references:** *(none)*\n\n" +
					"### Rigor\n- rigor: 0.5\n\n### Strength\n- strength: 0.5\n\n---\n\n",
				".index/claims.yaml": "--- {}\n",
			})
			lift := tc.block(t, root)
			src, err := kbload.Open(root)
			if err != nil {
				t.Fatalf("opening the 2.0.0 KB: %v", err)
			}
			if _, _, _, err := UpdateFormat(src, nil, nil, log.Discard()); err == nil {
				t.Fatal("the blocked save reported no error")
			}
			edges, err := src.ReadFile(src.KBPath(kb.GraphEdgesFile))
			if err != nil {
				t.Fatal(err)
			}
			if got := read(t, src.KBPath(kb.GraphEdgesFile)); got != string(edges) {
				t.Errorf("the edge file on disk =\n%s\nwant the converted records\n%s", got, edges)
			}
			if got := read(t, filepath.Join(root, "vol", "a.md")); got != leaf {
				t.Errorf("the leaf was written before the save stopped:\n%s", got)
			}
			if stamp, _ := kb.FormatStamp(read(t, filepath.Join(root, kb.EntryPointFile))); stamp != "2.0.0" {
				t.Errorf("the stopped save left the stamp at %q, want 2.0.0", stamp)
			}
			lift()

			reopened, err := kbload.Open(root)
			if err != nil {
				t.Fatalf("reopening the KB the save stopped on: %v", err)
			}
			text, err := reopened.ReadText(filepath.Join(root, "vol", "a.md"))
			if err != nil {
				t.Fatal(err)
			}
			if marked := strings.Contains(text, kb.StatementMarker("clm-aaaaaa")); !reopened.Migrated() || strings.HasPrefix(text, "---") || !marked {
				t.Errorf("the reopened KB: migrated %t, its leaf\n%s\nwant a conversion with the leaf's frontmatter gone and its marker kept", reopened.Migrated(), text)
			}
			if tc.resumed {
				nodes, err := reopened.ReadFile(reopened.KBPath(kb.GraphNodesFile))
				if err != nil || string(nodes) != read(t, reopened.KBPath(kb.GraphNodesFile)) {
					t.Errorf("the reopened KB's node records = %q (%v), want the node file as it stands", nodes, err)
				}
			}
		})
	}
}

// TestASaveStoppedAtTheOldStampLoadsAsAConversion: a save that wrote the migrated files
// and the stamp file at the version the tree is still at, and stopped either
// before or after draining the obsolete paths, leaves files of the current
// form under a 1.0.0 stamp in the stamp file and the entry point's frontmatter;
// the KB opens as a conversion and derives.
func TestASaveStoppedAtTheOldStampLoadsAsAConversion(t *testing.T) {
	for _, c := range []struct {
		name  string
		drain bool
	}{{"obsolete paths kept", false}, {"obsolete paths drained", true}} {
		t.Run(c.name, func(t *testing.T) {
			root := writeKB(t, map[string]string{
				"entry-point.md": "---\nkind: entry-point\nkb-format: \"1.0.0\"\n---\n\n# KB\n\n- [vol](vol/index.md)\n",
				"vol/index.md":   "---\nkind: index\n---\n[↑ KB](../entry-point.md)\n\n# Vol\n\n- [a](a.md)\n",
				"vol/a.md":       "---\nkind: leaf\nclaims: [clm-aaaaaa, clm-bbbbbb]\n---\n[↑ Vol](index.md)\n\n# A\n",
				"vol/claim-quality.md": "# Register\n\n## Claim A\n<!-- id: clm-aaaaaa -->\n\n### Quality\n- confidence: 0.8\n" +
					"- depends-on:\n  - clm-bbbbbb — Claim B (solidity 0.9)\n\n### Solidity\n- solidity: 0.8\n\n---\n\n" +
					"## Claim B\n<!-- id: clm-bbbbbb -->\n\n### Quality\n- confidence: 0.9\n\n### Solidity\n- solidity: 0.9\n",
			})
			src, err := kbload.Open(root)
			if err != nil {
				t.Fatalf("opening the 1.0.0 KB: %v", err)
			}
			for _, rel := range src.MigratedFiles() {
				if rel == kb.KBDir+"/"+kb.EntryPointFile {
					continue // the save writes the entry point after the stamp file at the old version
				}
				p := src.Path(rel)
				data, err := src.ReadFile(p)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(kb.FormatFile)), kb.EncodeFormatStamp(src.Version()), 0o644); err != nil {
				t.Fatal(err)
			}
			if c.drain {
				// As the save drains them: a path only an intermediate step wrote
				// was never on disk.
				for _, rel := range src.Obsolete() {
					if err := os.Remove(src.Path(rel)); err != nil && !os.IsNotExist(err) {
						t.Fatal(err)
					}
				}
			}

			reopened, err := kbload.Open(root)
			if err != nil {
				t.Fatalf("reopening a KB whose save stopped before the stamp: %v", err)
			}
			if !reopened.Migrated() {
				t.Fatalf("the reopened KB reads as current; its entry point kept the previous stamp")
			}
			if _, err := Derive(reopened); err != nil {
				t.Fatalf("reading a KB whose save stopped before the stamp: %v", err)
			}
		})
	}
}

// TestASaveStoppedBeforeTheStampLoadsAtTheOldVersion: the save's last write
// is the stamp at kb.FormatVersion, so a save that stops after its entry point
// leaves every migrated file written, the entry point stripped of its
// frontmatter, and the stamp file at the version the tree converts from. The
// loader reads that tree at 2.0.0 and resumes the conversion.
func TestASaveStoppedBeforeTheStampLoadsAtTheOldVersion(t *testing.T) {
	root := writeKB(t, map[string]string{
		"entry-point.md": "---\nkind: entry-point\nkb-format: \"2.0.0\"\n---\n\n# KB\n\n- [vol](vol/index.md)\n",
		"vol/index.md":   "---\nkind: index\n---\n[↑ KB](../entry-point.md)\n\n# Vol\n\n- [a](a.md)\n",
		"vol/a.md": "---\nkind: leaf\nclaims: [clm-aaaaaa]\n---\n[↑ Vol](index.md)\n\n# A\n\n" +
			"The result holds. <!-- claim-rigor: clm-aaaaaa -->\n",
		"claim-rigor.md": "# Claim Rigor\n\n---\n\n## Claim A\n<!-- id: clm-aaaaaa -->\n\n> **Leaf references:** *(none)*\n\n" +
			"### Rigor\n- rigor: 0.5\n\n### Strength\n- strength: 0.5\n\n---\n\n",
		".index/claims.yaml": "--- {}\n",
	})
	src, err := kbload.Open(root)
	if err != nil {
		t.Fatalf("opening the 2.0.0 KB: %v", err)
	}
	for _, rel := range src.MigratedFiles() {
		p := src.Path(rel)
		data, err := src.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, rel := range src.Obsolete() {
		if err := os.Remove(src.Path(rel)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(kb.FormatFile)), kb.EncodeFormatStamp(src.Version()), 0o644); err != nil {
		t.Fatal(err)
	}
	if entry := read(t, filepath.Join(root, kb.EntryPointFile)); strings.HasPrefix(entry, "---") {
		t.Errorf("the entry point still carries its frontmatter:\n%s", entry)
	}

	reopened, err := kbload.Open(root)
	if err != nil {
		t.Fatalf("opening the KB the save stopped on: %v", err)
	}
	if !reopened.Migrated() || reopened.Version() != "2.0.0" {
		t.Fatalf("the reopened KB: migrated %t at %q, want a 2.0.0 KB converted in memory", reopened.Migrated(), reopened.Version())
	}
}

// citationsFixtureKB is a 3.0.0 KB whose prose cites into marked headings,
// links into re-slugged headings, cites a work through the root register and
// names nodes by bare ids.
var citationsFixtureKB = filepath.Join("..", "formats", "testdata", "citations-3.0.0", "kb-root")

// TestAConvertedKBsLinksResolve: the citations fixture saved by update-format in the
// 4.0.0 form has its links resolve. The dead-link gate finds no dead link but
// one the conversion reported unresolved, a link into a removed register page
// included; every link into a heading of a
// KB document — a citation into a marked heading, an ordinary link into a
// re-slugged one, into a heading whose bare id became a citation, or into one
// carrying code — names one of that document's anchors, but the one the
// conversion reported naming no heading; the conversion reports the link whose
// anchor names one heading under 3.0.0's slugging and another under 4.0.0's;
// and a work's citation names its entry on the root summary page, which the
// gate checks. The save never reports the stamp file removed.
func TestAConvertedKBsLinksResolve(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repo", kb.KBDir)
	if err := os.CopyFS(root, os.DirFS(citationsFixtureKB)); err != nil {
		t.Fatal(err)
	}
	converted, err := kbload.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	migration, _ := converted.Migration()
	unresolved := map[string]bool{}
	for _, w := range migration.Warnings {
		if w.List == kbload.ListUnresolvedCitations {
			unresolved[w.Path+":"+strconv.Itoa(w.Line)] = true
		}
	}
	_, removed, _, err := UpdateFormat(converted, nil, nil, log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(removed, kb.FormatFile) {
		t.Errorf("the save removed the stamp file: %q", removed)
	}
	src := kb.OnDisk(root)
	found, err := LinkFindings(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range found {
		if f.Check == CheckDeadLink && !unresolved[f.Path+":"+strconv.Itoa(f.Line)] {
			t.Errorf("a dead link the conversion did not report unresolved: %s", f.Detail)
		}
	}
	if !unresolved["vol/notes.md:14"] {
		t.Errorf("vol/notes.md:14, an anchor naming two headings, is not reported; reported %v", unresolved)
	}
	notes, err := src.ReadText(src.KBPath("vol/notes.md"))
	if err != nil {
		t.Fatal(err)
	}
	var resolved []string
	for n, line := range kb.SplitLines(kb.StripCodeSplitLines(notes)) {
		for _, l := range kb.Links(line) {
			target, fragment, ok := strings.Cut(l.Destination, "#")
			if !ok || filepath.Base(target) == kb.RegisterFile {
				continue
			}
			text, err := src.ReadText(src.KBPath(filepath.ToSlash(filepath.Join("vol", target))))
			if err != nil {
				t.Fatal(err)
			}
			switch {
			case slices.Contains(slices.Collect(maps.Values(kb.HeadingAnchors(text))), fragment):
				resolved = append(resolved, l.Destination)
			case !unresolved["vol/notes.md:"+strconv.Itoa(n+1)]:
				t.Errorf("vol/notes.md:%d: %s names no heading of %s, and the conversion did not report it", n+1, l.Destination, target)
			}
		}
	}
	for _, want := range []string{"results.md#main-result", "results.md#scope--limits", "results.md#a--pair", "../axioms.md#axiom-s3-a-labelling-convention",
		"headings.md#on-main-result", "headings.md#the-kbase-tool--usage"} {
		if !slices.Contains(resolved, want) {
			t.Errorf("no link resolves at %s; resolved %q", want, resolved)
		}
	}
}
