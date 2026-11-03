package from300to400

import (
	"strings"
	"testing"

	"kbase/internal/formats/common"
)

// TestALinkIntoARemovedRegisterPageNeverStaysThere: a link into a 3.0.0
// register page whose anchor names no one place is pointed at the summary
// page replacing the page, and where no node renders on the page that would
// replace it, so no page does, the link goes and its text stands, a bare id in
// that text converted as any bare id is; each is reported but a link naming
// the page alone where a summary page replaces it.
func TestALinkIntoARemovedRegisterPageNeverStaysThere(t *testing.T) {
	cases := []struct {
		name, link, want string
		reported         bool
	}{
		{"an anchor naming no entry", "[b](claim-rigor.md#no-entry)", "[b](kb-summary.md)", true},
		{"the replaced page alone", "[c](claim-rigor.md)", "[c](kb-summary.md)", false},
		{"an anchor into a page nothing replaces", "[d](../claim-rigor.md#main-result)", "d", true},
		{"a page nothing replaces alone", "[e](../claim-rigor.md)", "e", true},
		{"a bare id in the text of a link that goes", "[see clm-aaaaaa](../claim-rigor.md)",
			`see [Main Result](leaf.md#main-result "clm-aaaaaa")`, true},
	}
	var links []string
	for _, c := range cases {
		links = append(links, c.link)
	}
	files := common.Files{
		nodesPath: []byte(`- id: "clm-aaaaaa"
  title: "Main Result"
  rigor: "*pending*"
  manually-scored: false
  rationale: ""
  no-edge: ""
  strengthen-by: []
`),
		edgesPath:                    []byte("[]\n"),
		"kb-root/entry-point.md":     []byte("# KB\n\n- [Vol](vol/index.md)\n"),
		"kb-root/claim-rigor.md":     []byte("# Claim register\n"),
		"kb-root/vol/index.md":       []byte("# Vol\n"),
		"kb-root/vol/claim-rigor.md": []byte("# Claim register\n\n## Main Result\n<!-- id: clm-aaaaaa -->\n"),
		"kb-root/vol/leaf.md":        []byte("# Leaf\n\n## Main Result <!-- claim-rigor: clm-aaaaaa -->\n"),
		"kb-root/vol/notes.md":       []byte(strings.Join(links, "\n")),
	}
	got, _, warnings, err := Convert300To400(files)
	if err != nil {
		t.Fatalf("Convert300To400: %v", err)
	}
	reported := map[int]bool{}
	for _, w := range warnings {
		if w.List == common.ListUnresolvedCitations && w.Path == "kb-root/vol/notes.md" {
			reported[w.Line] = true
		}
	}
	lines := strings.Split(string(got["kb-root/vol/notes.md"]), "\n")
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if lines[i] != c.want {
				t.Errorf("%s converts to %s, want %s", c.link, lines[i], c.want)
			}
			if reported[i+1] != c.reported {
				t.Errorf("%s reported %t, want %t", c.link, reported[i+1], c.reported)
			}
		})
	}
}
