package index

import (
	"os"
	"path/filepath"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/kbload"
	"kbase/internal/log"
)

// TestRefreshFinishesAStoppedSave: a save that wrote the migrated files, and
// stopped either before or after draining the obsolete paths and before the
// entry point's stamp, leaves 2.0.0 files under a 1.0.0 stamp; the KB opens,
// its conversion changes nothing, and the next refresh completes the save.
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
				p := src.Path(rel)
				data, err := src.ReadFile(p)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, data, 0o644); err != nil {
					t.Fatal(err)
				}
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
			if _, err := kb.Discover(reopened, log.Discard()); err != nil {
				t.Fatalf("reading a KB whose save stopped before the stamp: %v", err)
			}
			if _, err := Refresh(reopened, log.Discard()); err != nil {
				t.Fatalf("refreshing a KB whose save stopped before the stamp: %v", err)
			}
			if stamp, _ := kb.FormatStamp(read(t, filepath.Join(root, kb.EntryPointFile))); stamp != kb.FormatVersion {
				t.Errorf("the refresh left the stamp at %q, want %s", stamp, kb.FormatVersion)
			}
		})
	}
}
