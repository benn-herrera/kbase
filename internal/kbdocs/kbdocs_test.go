package kbdocs

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/result"
)

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

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestStampWritesOnlyWhatIsAbsent(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{ConventionsFile: "the project's own\n"})
	written, reports, err := Stamp(kb.OnDisk(root), "proj", "A pin naming {braces} as prose.")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"AGENTS.md: written from its template", "CONVENTIONS.md: present, left as authored", "CLAUDE.md: written as the redirect to AGENTS.md"}
	if !slices.Equal(reports, want) {
		t.Errorf("reports = %q, want %q", reports, want)
	}
	if want := []string{kb.AgentsFile, kb.AgentsRedirectFile}; !slices.Equal(written, want) {
		t.Errorf("written = %q, want %q: the authored CONVENTIONS.md is not one", written, want)
	}
	if got := readFile(t, filepath.Join(root, ConventionsFile)); got != "the project's own\n" {
		t.Errorf("an authored CONVENTIONS.md was rewritten: %q", got)
	}
	if got := readFile(t, filepath.Join(root, kb.AgentsRedirectFile)); got != "@AGENTS.md\n" {
		t.Errorf("CLAUDE.md = %q", got)
	}
	agents := readFile(t, filepath.Join(root, kb.AgentsFile))
	for _, want := range []string{"# proj knowledge base", "A pin naming {braces} as prose."} {
		if !strings.Contains(agents, want) {
			t.Errorf("AGENTS.md lacks %q", want)
		}
	}
	rewritten, again, err := Stamp(kb.OnDisk(root), "proj", "another pin")
	if err != nil {
		t.Fatal(err)
	}
	if len(rewritten) != 0 {
		t.Errorf("second stamp wrote %q, want nothing", rewritten)
	}
	if want := []string{"AGENTS.md: present, left as authored", "CONVENTIONS.md: present, left as authored", "CLAUDE.md: present, already the redirect"}; !slices.Equal(again, want) {
		t.Errorf("second stamp = %q, want %q", again, want)
	}
}

func TestStampRefusesAnUnmigratedClaudeFile(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{kb.AgentsRedirectFile: "# old agents file\n"})
	_, _, err := Stamp(kb.OnDisk(root), "proj", AuthoredKBPin)
	var r result.Refusal
	if !errors.As(err, &r) {
		t.Fatalf("Stamp over an unmigrated CLAUDE.md = %v, want a Refusal", err)
	}
	if _, err := os.Stat(filepath.Join(root, kb.AgentsFile)); !errors.Is(err, os.ErrNotExist) {
		t.Error("the refusal wrote AGENTS.md")
	}
}

// TestStampedDocumentsCarryNoSlotAndNoFrameworkHeading: every slot a
// template names is filled, and no stamped document declares a framework
// node, which a reader would take AGENTS.md for where invariants.md is
// absent.
func TestStampedDocumentsCarryNoSlotAndNoFrameworkHeading(t *testing.T) {
	root := t.TempDir()
	if _, _, err := Stamp(kb.OnDisk(root), "proj", AuthoredKBPin); err != nil {
		t.Fatal(err)
	}
	readme, err := Overview("proj", "A passage.", false)
	if err != nil {
		t.Fatal(err)
	}
	docs := map[string]string{OverviewFile: readme}
	for _, name := range kb.StampedFiles {
		docs[name] = readFile(t, filepath.Join(root, name))
	}
	for name, text := range docs {
		if m := slotRE.FindString(text); m != "" {
			t.Errorf("%s carries the slot %s", name, m)
		}
		if strings.Contains(text, "### INVARIANT-") {
			t.Errorf("%s carries a framework-node heading", name)
		}
	}
	if !strings.Contains(docs[ConventionsFile], "`claim`, `analysis`") {
		t.Error("CONVENTIONS.md does not name the node kinds")
	}
}

// The session rule reaches a session as plain text on the MCP wire, so it
// holds no Markdown.
func TestSessionRuleHoldsNoMarkdown(t *testing.T) {
	rule := SessionRule()
	if i := strings.IndexAny(rule, "`*_[#"); i >= 0 {
		t.Errorf("the session rule holds the Markdown character %q", rule[i])
	}
}

func TestFillRefusesAnUnfilledSlot(t *testing.T) {
	if _, err := fill(OverviewFile, map[string]string{slotProjectName: "p"}); err == nil || !strings.Contains(err.Error(), "overview-passage") {
		t.Errorf("fill without the passage = %v, want a refusal naming it", err)
	}
}

// TestStampWithNoScopePinLeavesNoBlankRun: a build passes no scope pin, and an
// empty slot must leave one blank line where it stood, not a run of them.
func TestStampWithNoScopePinLeavesNoBlankRun(t *testing.T) {
	root := t.TempDir()
	if _, _, err := Stamp(kb.OnDisk(root), "proj", ""); err != nil {
		t.Fatal(err)
	}
	agents := readFile(t, filepath.Join(root, kb.AgentsFile))
	if agents == "" {
		t.Fatal("AGENTS.md is empty")
	}
	if strings.Contains(agents, "\n\n\n") {
		t.Errorf("AGENTS.md with an empty scope pin carries a run of blank lines:\n%s", agents)
	}
}

func TestNotProse(t *testing.T) {
	for _, tc := range []struct {
		line   string
		reject bool
	}{
		{"An ordinary sentence about Part 4 and three mechanisms.", false},
		{"## A heading", true},
		{"#hashtag is prose", false},
		{"- a list item", true},
		{"12. a numbered item", true},
		{"| a | table |", true},
		{"```", true},
		{"see [the leaf](vol/leaf.md)", true},
		{"named in intro.md here", true},
		{"a markdown file is .md alone", false},
	} {
		if got := len(NotProse("A first line.\n"+tc.line)) == 1; got != tc.reject {
			t.Errorf("NotProse(%q) rejected = %t, want %t", tc.line, got, tc.reject)
		}
	}
}

func TestComposeExcerpts(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"entry-point.md": "---\nkind: entry-point\nkb-format: \"1.0.0\"\n---\n# Knowledge Base\n\n- [Volume One](vol/index.md)\n- [Volume One](vol/index.md)\n",
		"vol/index.md":   "---\nkind: index\n---\n[↑ Knowledge Base](../entry-point.md)\n\n# Volume One\n\n- [Overview](overview.md)\n- [Methods](methods.md)\n",
		"vol/overview.md": "[↑ Volume One](index.md)\n\n# Overview\n\nFirst paragraph with a [link](methods.md).\n\n" +
			strings.Repeat("é", ExcerptDocumentChars) + "\n",
	})
	got, err := ComposeExcerpts(kb.OnDisk(root))
	if err != nil {
		t.Fatal(err)
	}
	want := "==> Knowledge Base <==\n# Knowledge Base\n\n- Volume One\n- Volume One\n\n" +
		"==> Volume One <==\n# Volume One\n\n- Overview\n- Methods\n\n" +
		"==> Volume One › Overview <==\n# Overview\n\nFirst paragraph with a link."
	if got.Text != want {
		t.Errorf("excerpts =\n%s\nwant\n%s", got.Text, want)
	}
	if len(got.Cuts) != 1 || got.Cuts[0].Title != "Volume One › Overview" || got.Cuts[0].Excluded != ExcerptDocumentChars+2 {
		t.Errorf("cuts = %+v, want the overview leaf's long paragraph, counted in characters", got.Cuts)
	}
}

var (
	excerptsKBRoot = flag.String("kbdocs.kbroot", "", "a kb-root to compose excerpts over")
	excerptsWant   = flag.String("kbdocs.want", "", "the expected excerpts text over that kb-root")
)

// TestExcerptsEqualReference compares the excerpts over a tree with the expected
// composition over it; test-integration-build-arxiv supplies both.
func TestExcerptsEqualReference(t *testing.T) {
	if *excerptsKBRoot == "" {
		t.Skip("no kb-root named; test-integration-build-arxiv supplies one")
	}
	got, err := ComposeExcerpts(kb.OnDisk(*excerptsKBRoot))
	if err != nil {
		t.Fatal(err)
	}
	if want := readFile(t, *excerptsWant); got.Text != want {
		t.Errorf("excerpts differ from the expected text (%d against %d characters); first difference at character %d",
			chars(got.Text), chars(want), firstDifference([]rune(got.Text), []rune(want)))
	}
}

func firstDifference(a, b []rune) int {
	for i := range min(len(a), len(b)) {
		if a[i] != b[i] {
			return i
		}
	}
	return min(len(a), len(b))
}

// TestOverviewDigestChunk: the digest sentence fills the digest-chunk slot
// where the digest is drawn, with the link a verify would follow; where it is
// not, the README names no digest at all — a link a verify of a one-volume KB
// would find dead.
func TestOverviewDigestChunk(t *testing.T) {
	with, err := Overview("proj", "A passage.", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(with, "[`claim-graph-digest.svg`](claim-graph-digest.svg)") {
		t.Error("the digest-drawn README does not link claim-graph-digest.svg")
	}
	if strings.Contains(with, "claim-graph-digest") && !strings.Contains(with, "two or more volumes") {
		t.Error("the digest-drawn README's digest sentence lost its drawn-where clause")
	}
	without, err := Overview("proj", "A passage.", false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(without, "claim-graph-digest") {
		t.Error("the one-volume README carries a digest mention a verify would trip over")
	}
	if m := slotRE.FindString(without); m != "" {
		t.Errorf("the one-volume README carries the slot %s", m)
	}
}
