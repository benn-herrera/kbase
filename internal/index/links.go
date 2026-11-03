package index

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"kbase/internal/kb"
)

// CheckDeadLink names the dead-link gate's fault: a link whose target, or
// whose fragment, names nothing.
const CheckDeadLink = "dead-link"

// LinkFindings is the dead-link gate over kb-root/ and the claim-id link
// check: every node id a citation's link title names, and every claim,
// analysis or experiment id anywhere else outside code, must be a node the
// records carry (kb.CheckUnknownID). Every link's fragment into a document
// under kb-root/ is checked; one into a register page is judged against the
// slugs of the entries derived for that page, never the page itself. A dead
// link whose target leaves the repository is not a finding. A KB state Derive
// refuses is its refusal.
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
		out = append(out, Finding{Check: CheckDeadLink, Path: filepath.ToSlash(d.File), Line: d.Line, Detail: d.String()})
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
			var ids []string
			for _, l := range kb.Links(line) {
				if id, cites := l.CitedID(); cites {
					ids = append(ids, id)
					line = line[:l.DestEnd] + strings.Repeat(" ", l.End-l.DestEnd) + line[l.End:]
				}
			}
			for _, id := range append(ids, kb.NodeIDs(line)...) {
				if _, ok := known[id]; !ok && !kb.IDPlaceholders[id] {
					out = append(out, Finding{Check: kb.CheckUnknownID, Path: filepath.ToSlash(rel), Line: n + 1, Detail: fmt.Sprintf("%s:%d %s", filepath.ToSlash(rel), n+1, id)})
				}
			}
		}
	}
	return out, nil
}

// SectionNodes is, in id order, every node whose first statement marker in
// text stands in the section of the heading anchored at anchor: the nodes a
// citation of that section can cite.
func SectionNodes(text, anchor string) []string {
	var out []string
	for id, line := range kb.MarkerLines(text) {
		if SectionSlug(text, line) == anchor {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}
