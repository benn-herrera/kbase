package index

import (
	"fmt"
	"path/filepath"

	"kbase/internal/kb"
)

// LinkFindings is the dead-link gate over kb-root/ and the claim-id link
// check: every node id a document names must be a node the records carry. The
// fragment of a link into a register page is judged against the slugs of the
// entries derived for that page, never the page itself. A dead link whose
// target leaves the repository is not a finding. A KB state Derive refuses is
// its refusal.
func LinkFindings(src *kb.Source) ([]Finding, error) {
	root := src.Root()
	x, err := loadDerivation(src)
	if err != nil {
		return nil, err
	}
	dead, err := kb.DeadLinksIn(src, src.Repo(), x.pageSlugs())
	if err != nil {
		return nil, err
	}
	var out []Finding
	for _, d := range dead {
		if d.Kind == kb.BrokenInter {
			continue
		}
		out = append(out, Finding{Check: "dead link", Path: filepath.ToSlash(d.File), Line: d.Line, Detail: d.String()})
	}
	known := GraphKinds(x.g)
	files, err := kb.MarkdownFiles(src)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		rel, err := filepath.Rel(root, file)
		if err != nil {
			return nil, err
		}
		text, err := src.ReadText(file)
		if err != nil {
			return nil, fmt.Errorf("%s could not be read to check its node ids: %w", filepath.ToSlash(rel), err)
		}
		for n, line := range kb.SplitLines(kb.StripCodeSplitLines(text)) {
			for _, id := range kb.NodeIDs(line) {
				if _, ok := known[id]; !ok && !kb.IDPlaceholders[id] {
					out = append(out, Finding{Check: "unknown id", Path: filepath.ToSlash(rel), Line: n + 1, Detail: fmt.Sprintf("%s:%d %s", filepath.ToSlash(rel), n+1, id)})
				}
			}
		}
	}
	return out, nil
}
