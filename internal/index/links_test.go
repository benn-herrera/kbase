package index

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/kbload"
)

// graphFiles is g's two record files by kb-root-relative path, as writeKB
// takes them.
func graphFiles(g kb.Graph) map[string]string {
	return map[string]string{kb.GraphNodesFile: string(kb.EncodeNodes(g)), kb.GraphEdgesFile: string(kb.EncodeEdges(g))}
}

func TestLinkFindingsResolveAgainstTheRepository(t *testing.T) {
	files := graphFiles(kb.Graph{})
	files["leaf.md"] = "[a](missing.md)\n[b](../assets/x.pdf)\n[c](../assets/gone.pdf)\n"
	root := writeKB(t, files)
	repo := filepath.Dir(root)
	for rel, text := range map[string]string{"docs/x.md": "[a](missing.md)\n", "assets/x.pdf": "%PDF\n"} {
		p := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	found, err := LinkFindings(kb.OnDisk(root))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range found {
		got = append(got, f.Detail)
	}
	want := []string{`leaf.md:1 broken intra "missing.md"`, `leaf.md:3 broken intra "../assets/gone.pdf"`}
	if !slices.Equal(got, want) {
		t.Errorf("LinkFindings =\n%q\nwant\n%q", got, want)
	}
}

// TestLinkFindingsReportAMovedNodesOldFragment: a claim whose marker moved from
// volume a to volume b is rendered on b's page, so a prose link to its old
// entry on a's page — which still stands, unrefreshed and holding the entry —
// is a dead link in the linking document, while the link to b's page holds.
func TestLinkFindingsReportAMovedNodesOldFragment(t *testing.T) {
	files := graphFiles(kb.Graph{Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "Theorem 1"}}})
	for rel, text := range map[string]string{
		kb.EntryPointFile: "# KB\n\n- [a](a/index.md)\n- [b](b/index.md)\n",
		"a/index.md":      "[↑ KB](../entry-point.md)\n\n# A\n\n- [prose](prose.md)\n",
		"a/prose.md": "[↑ A](index.md)\n\n# Prose\n\nSee [the old entry](kb-summary.md#theorem-1) and " +
			"[the new one](../b/kb-summary.md#theorem-1).\n",
		"a/kb-summary.md": "# Claim register\n\n## Theorem 1\n<!-- id: clm-aaaaaa -->\n",
		"b/index.md":      "[↑ KB](../entry-point.md)\n\n# B\n\n- [leaf](leaf.md)\n",
		"b/leaf.md":       "[↑ B](index.md)\n\n# Leaf\n\nA theorem. <!-- kbase-id: clm-aaaaaa -->\n",
	} {
		files[rel] = text
	}
	root := writeKB(t, files)
	src := kb.OnDisk(root)
	pages, err := renderRegisterPages(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b", kb.RegisterFile), pages["b/"+kb.RegisterFile], 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := LinkFindings(kb.OnDisk(root))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range found {
		got = append(got, f.Path+" "+f.Detail)
	}
	if want := []string{`a/prose.md a/prose.md:5 broken fragment "kb-summary.md#theorem-1"`}; !slices.Equal(got, want) {
		t.Errorf("LinkFindings =\n%q\nwant\n%q", got, want)
	}
}

// TestLinkFindingsReadCitations: a citation is a link the gate reads, its
// title beside its destination; a citation whose title names no node is an
// unknown-id, of a work's kind as of a claim's, and one naming a node is
// none; a bare id of any kind naming no node is an unknown-id too, a work's
// and an axiom's as a claim's.
func TestLinkFindingsReadCitations(t *testing.T) {
	files := graphFiles(kb.Graph{Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "One"}}})
	files["leaf.md"] = "# One\n\nOne holds." + kb.StatementMarker("clm-aaaaaa") + "\n\n" +
		`[One](leaf.md#one "clm-aaaaaa") [Gone](gone.md#x "clm-aaaaaa") [Two](leaf.md#one "clm-bbbbbb") [A Work](leaf.md "wrk-zzzzzz")` + "\n" +
		"Bare clm-yyyyyy, wrk-yyyyyy and axm-yyyyyy.\n"
	found, err := LinkFindings(kb.OnDisk(writeKB(t, files)))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range found {
		got = append(got, f.Check+" "+f.Detail)
	}
	want := []string{`dead-link leaf.md:5 broken intra "gone.md#x"`, "unknown-id leaf.md:5 clm-bbbbbb", "unknown-id leaf.md:5 wrk-zzzzzz",
		"unknown-id leaf.md:6 clm-yyyyyy", "unknown-id leaf.md:6 wrk-yyyyyy", "unknown-id leaf.md:6 axm-yyyyyy"}
	if !slices.Equal(got, want) {
		t.Errorf("LinkFindings =\n%q\nwant\n%q", got, want)
	}
}

// TestLinkFindingsCheckEveryFragment: a link's fragment into a document under
// kb-root/, a bare #fragment into its own included, is live where it names a
// heading's anchor or an element's id there, and an empty fragment names the
// top; one naming neither, or naming a heading or an id only inside code, is a
// dead-link fault. A fragment into a file kbase reads no anchors in is not
// checked.
func TestLinkFindingsCheckEveryFragment(t *testing.T) {
	files := graphFiles(kb.Graph{})
	files["leaf.md"] = "# Leaf\n\n## A Section\n\n<div id=\"ref-key\">Entry.</div>\n\n" +
		"[a](other.md#other-heading) [b](other.md#no-such-heading) [c](#a-section) [d](#nowhere) [e](#ref-key) " +
		"[f](other.md#) [g](x.pdf#page=2) [h](other.md#in-code) [i](other.md#coded)\n"
	files["other.md"] = "# Other Heading\n\n```\n# In code\n<span id=\"coded\"></span>\n```\n"
	files["x.pdf"] = "%PDF\n"
	found, err := LinkFindings(kb.OnDisk(writeKB(t, files)))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range found {
		got = append(got, f.Check+" "+f.Detail)
	}
	want := []string{`dead-link leaf.md:7 broken fragment "other.md#no-such-heading"`, `dead-link leaf.md:7 broken fragment "#nowhere"`,
		`dead-link leaf.md:7 broken fragment "other.md#in-code"`, `dead-link leaf.md:7 broken fragment "other.md#coded"`}
	if !slices.Equal(got, want) {
		t.Errorf("LinkFindings =\n%q\nwant\n%q", got, want)
	}
}

// TestLinkFindingsOverAPendingConversion: over the citations fixture loaded
// and not yet saved, a link resolves through the converted files, so a link
// into the root summary page the save renders is not dead; every dead link is
// one the conversion reported unresolved.
func TestLinkFindingsOverAPendingConversion(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repo", kb.KBDir)
	if err := os.CopyFS(root, os.DirFS(citationsFixtureKB)); err != nil {
		t.Fatal(err)
	}
	src, err := kbload.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	migration, _ := src.Migration()
	unresolved := map[string]bool{}
	for _, w := range migration.Warnings {
		if w.List == kbload.ListUnresolvedCitations {
			unresolved[fmt.Sprintf("%s:%d", w.Path, w.Line)] = true
		}
	}
	found, err := LinkFindings(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range found {
		if f.Check != CheckDeadLink {
			continue
		}
		if strings.Contains(f.Detail, kb.RegisterFile) {
			t.Errorf("a link into the summary page the save renders is dead: %s", f.Detail)
		}
		if !unresolved[fmt.Sprintf("%s:%d", f.Path, f.Line)] {
			t.Errorf("a dead link the conversion did not report unresolved: %s", f.Detail)
		}
	}
}

// TestSectionNodes: the nodes a section holds are those whose first marker
// stands under its heading, before the next heading; a section of none holds
// none.
func TestSectionNodes(t *testing.T) {
	text := "# Pair" + kb.StatementMarker("clm-aaaaaa") + kb.StatementMarker("ana-aaaaaa") + "\n\n## Bound\n\nThe bound." +
		kb.StatementMarker("clm-bbbbbb") + "\n\n## Empty\n\nNothing marked.\n"
	for anchor, want := range map[string][]string{
		"pair":  {"ana-aaaaaa", "clm-aaaaaa"},
		"bound": {"clm-bbbbbb"},
		"empty": nil,
	} {
		if got := SectionNodes(text, anchor); !slices.Equal(got, want) {
			t.Errorf("SectionNodes(%q) = %q, want %q", anchor, got, want)
		}
	}
}
