package index

import (
	"os"
	"path/filepath"
	"slices"
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
// each leaf's frontmatter unread. Either way the next refresh completes the
// save.
func TestAStoppedMigrationSaveResumes(t *testing.T) {
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
			if _, err := Refresh(src, log.Discard()); err == nil {
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
			if _, err := Refresh(reopened, log.Discard()); err != nil {
				t.Fatalf("refreshing the KB the save stopped on: %v", err)
			}
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(kb.FormatFile)))
			if err != nil {
				t.Fatalf("reading the stamp file after the resumed save: %v", err)
			}
			if stamp, err := kb.DecodeFormatStamp(data); err != nil || stamp != kb.FormatVersion {
				t.Errorf("the refresh left the stamp at %q (%v), want %s", stamp, err, kb.FormatVersion)
			}
			if _, err := os.Stat(filepath.Join(root, kb.IndexDir, "claims.yaml")); !os.IsNotExist(err) {
				t.Errorf("the obsolete 2.0.0 index file stands after the save: %v", err)
			}
		})
	}
}

// TestRefreshFinishesAStoppedSave: a save that wrote the migrated files and
// the stamp file at the version the tree is still at, and stopped either
// before or after draining the obsolete paths, leaves files of the current
// form under a 1.0.0 stamp in the stamp file and the entry point's frontmatter;
// the KB opens, and the next refresh completes the save.
func TestRefreshFinishesAStoppedSave(t *testing.T) {
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
				for _, rel := range src.Obsolete() {
					if err := os.Remove(src.Path(rel)); err != nil {
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
			if _, err := Refresh(reopened, log.Discard()); err != nil {
				t.Fatalf("refreshing a KB whose save stopped before the stamp: %v", err)
			}
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(kb.FormatFile)))
			if err != nil {
				t.Fatalf("reading the stamp file after the save: %v", err)
			}
			if stamp, err := kb.DecodeFormatStamp(data); err != nil || stamp != kb.FormatVersion {
				t.Errorf("the refresh left the stamp at %q (%v), want %s", stamp, err, kb.FormatVersion)
			}
		})
	}
}

// TestRefreshCompletesASaveStoppedBeforeTheStamp: the save's last write is the
// stamp at kb.FormatVersion, so a save that stops after its entry point leaves
// every migrated file written, the entry point stripped of its frontmatter,
// and the stamp file at the version the tree converts from. The loader reads
// that tree at 2.0.0 and resumes the conversion; the refresh completes the
// save, the stamp file is never among the paths reported removed, and a second
// refresh writes nothing.
func TestRefreshCompletesASaveStoppedBeforeTheStamp(t *testing.T) {
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
	r, err := RefreshWithWarnings(reopened, log.Discard())
	if err != nil {
		t.Fatalf("refreshing the KB the save stopped on: %v", err)
	}
	if slices.Contains(r.Removed, kb.FormatFile) {
		t.Errorf("the refresh removed the stamp file: %q", r.Removed)
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(kb.FormatFile)))
	if err != nil {
		t.Fatalf("reading the stamp file after the resumed save: %v", err)
	}
	if stamp, err := kb.DecodeFormatStamp(data); err != nil || stamp != kb.FormatVersion {
		t.Errorf("the refresh left the stamp at %q (%v), want %s", stamp, err, kb.FormatVersion)
	}
	current, err := kbload.Open(root)
	if err != nil {
		t.Fatalf("reopening the KB after the resumed save: %v", err)
	}
	if again, err := Refresh(current, log.Discard()); err != nil || len(again) != 0 {
		t.Errorf("a second refresh wrote %q (%v), want nothing", again, err)
	}
}
