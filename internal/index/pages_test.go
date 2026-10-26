package index

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/kbload"
)

// TestRegisterPagesOverTheSheetFixture: the sheet fixture renders the root
// page, holding its work, and one page per volume hosting a node — alpha's
// and beta's, gamma hosting none. Each entry's heading slug is its node's
// canonical anchor on that page, every node whose canonical path is a page has
// its entry there, and every link a page renders resolves.
func TestRegisterPagesOverTheSheetFixture(t *testing.T) {
	src, err := kbload.Open(sheetFixtureKB)
	if err != nil {
		t.Fatal(err)
	}
	pages, err := renderRegisterPages(src)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"alpha/" + kb.RegisterFile, "beta/" + kb.RegisterFile, kb.RegisterFile}
	if got := slices.Sorted(maps.Keys(pages)); !slices.Equal(got, want) {
		t.Fatalf("pages %q, want %q", got, want)
	}
	rows, err := Derive(src)
	if err != nil {
		t.Fatal(err)
	}
	onPage := map[string]map[string]string{} // page → node id → the slug its heading spells
	for rel, data := range pages {
		onPage[rel] = map[string]string{}
		lines := kb.SplitLines(string(data))
		for i, line := range lines[:len(lines)-1] {
			if title, ok := strings.CutPrefix(line, "## "); ok {
				for _, n := range rows.Nodes {
					if lines[i+1] == kb.RenderIDMarker(n.ID) {
						onPage[rel][n.ID] = kb.Slugify(title)
					}
				}
			}
		}
	}
	for _, n := range rows.Nodes {
		if _, isPage := pages[n.CanonicalPath]; !isPage {
			continue
		}
		if slug, ok := onPage[n.CanonicalPath][n.ID]; !ok || slug != n.CanonicalAnchor {
			t.Errorf("%s's entry on %s has slug %q (present %v), want its canonical anchor %q", n.ID, n.CanonicalPath, slug, ok, n.CanonicalAnchor)
		}
	}
	x, err := loadDerivation(src)
	if err != nil {
		t.Fatal(err)
	}
	for rel, data := range pages {
		src.Put(src.KBPath(rel), data)
	}
	dead, err := kb.DeadLinksIn(src, src.Repo(), x.pageSlugs())
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range dead {
		if _, isPage := pages[d.File]; isPage {
			t.Errorf("a rendered page's link does not resolve: %s", d)
		}
	}
}

// TestRegisterPagesOrderEntries: a page's hosted nodes come in reading order
// of their marker — the host's place in the spine walk, not its path, then the
// marker's line — and the root page's works by id.
func TestRegisterPagesOrderEntries(t *testing.T) {
	g := kb.Graph{
		Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "A"}, {ID: "clm-bbbbbb", Title: "B"}, {ID: "clm-cccccc", Title: "C"}},
		Works:  []kb.WorkNode{{ID: "wrk-zed2020", Title: "Zed"}, {ID: "wrk-abe2020", Title: "Abe"}},
	}
	x, err := loadDerivation(graphKB(t, g, map[string]string{
		kb.EntryPointFile: "# KB\n\n- [v](v/index.md)\n",
		"v/index.md":      "[↑ KB](../entry-point.md)\n\n# V\n\n- [z](z.md)\n- [a](a.md)\n",
		"v/z.md":          "[↑ V](index.md)\n\n# Z\n\nB. <!-- claim-rigor: clm-bbbbbb -->\n\nA. <!-- claim-rigor: clm-aaaaaa -->\n",
		"v/a.md":          "[↑ V](index.md)\n\n# A\n\nC. <!-- claim-rigor: clm-cccccc -->\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	entries := x.pageEntries()
	if got, want := entries["v/"+kb.RegisterFile], []string{"clm-bbbbbb", "clm-aaaaaa", "clm-cccccc"}; !slices.Equal(got, want) {
		t.Errorf("v's page orders %q, want %q", got, want)
	}
	if got, want := entries[kb.RegisterFile], []string{"wrk-abe2020", "wrk-zed2020"}; !slices.Equal(got, want) {
		t.Errorf("the root page orders %q, want %q", got, want)
	}
}

// TestRegisterFreshnessFaultAndFix: a page called for and missing, a page
// whose bytes differ from what refresh renders, and a page standing in a volume
// no node is rendered on are each a refresh-fixable register freshness
// finding, and a register file in a directory no volume is is none; written as
// rendered, and the uncalled page gone, none is.
func TestRegisterFreshnessFaultAndFix(t *testing.T) {
	files := graphFiles(kb.Graph{
		Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "Theorem 1", Rigor: f(0.9)}},
		Works:  []kb.WorkNode{{ID: "wrk-key2020", Title: "Key 2020", Rigor: f(0.8)}},
		Founds: []kb.BearingEdge{{Source: "wrk-key2020", Target: "clm-aaaaaa", Bearing: kb.Bearing{Value: 1}}},
	})
	for rel, text := range map[string]string{
		kb.EntryPointFile:      "# KB\n\n- [a](a/index.md)\n- [g](g/index.md)\n",
		"a/index.md":           "[↑ KB](../entry-point.md)\n\n# A\n\n- [leaf](leaf.md)\n",
		"a/leaf.md":            "[↑ A](index.md)\n\n# Leaf\n\nA theorem. <!-- claim-rigor: clm-aaaaaa -->\n",
		"g/index.md":           "[↑ KB](../entry-point.md)\n\n# G\n",
		"g/claim-rigor.md":     "# Claim register\n",
		"a/claim-rigor.md":     "# Claim register\n\nstale\n",
		"loose/claim-rigor.md": "# In a directory no volume is, so no page\n",
	} {
		files[rel] = text
	}
	root := writeKB(t, files)
	findings := func() []string {
		t.Helper()
		found, err := RegisterFreshness(kb.OnDisk(root))
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, f := range found {
			if f.Check != CheckRegisterFreshness || !f.RefreshFixable {
				t.Errorf("%+v is not a refresh-fixable %s finding", f, CheckRegisterFreshness)
			}
			out = append(out, f.Detail)
		}
		return out
	}
	want := []string{
		"a/claim-rigor.md is not as refresh renders it",
		"claim-rigor.md is missing",
		"g/claim-rigor.md stands, and no node is rendered on it",
	}
	if got := findings(); !slices.Equal(got, want) {
		t.Errorf("RegisterFreshness =\n%q\nwant\n%q", got, want)
	}
	pages, err := renderRegisterPages(kb.OnDisk(root))
	if err != nil {
		t.Fatal(err)
	}
	for rel, data := range pages {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(filepath.Join(root, "g", kb.RegisterFile)); err != nil {
		t.Fatal(err)
	}
	if got := findings(); len(got) != 0 {
		t.Errorf("RegisterFreshness after the fix = %q, want none", got)
	}
}
