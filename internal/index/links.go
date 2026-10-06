package index

import (
	"fmt"
	"path/filepath"

	"kbase/internal/kb"
)

// LinkFindings is the dead-link gate over kb-root/ and the claim-id link
// check: every node id a document names must be a node .index/claims.yaml
// records. A dead link whose target leaves the repository is not a finding,
// as kb_tools reports such a link without gating on it.
func LinkFindings(src *kb.Source) ([]Finding, error) {
	root := src.Root()
	dead, err := kb.DeadLinksIn(src, src.Repo())
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
	known, ok, err := knownNodeIDs(src)
	if err != nil || !ok {
		return out, err
	}
	files, err := kb.MarkdownFiles(src)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		text, err := src.ReadText(file)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(root, file)
		if err != nil {
			return nil, err
		}
		for n, line := range kb.SplitLines(kb.StripCodeSplitLines(text)) {
			for _, id := range kb.NodeIDs(line) {
				if !kb.IDPlaceholders[id] && !known[id] {
					out = append(out, Finding{Check: "unknown id", Path: filepath.ToSlash(rel), Line: n + 1, Detail: fmt.Sprintf("%s:%d %s", filepath.ToSlash(rel), n+1, id)})
				}
			}
		}
	}
	return out, nil
}

// knownNodeIDs is every string id the claims index records; ok is false
// where the file is absent, which skips the check.
func knownNodeIDs(src *kb.Source) (map[string]bool, bool, error) {
	recs, _, ok, err := recordLines(src, "claims")
	if err != nil || !ok {
		return nil, false, err
	}
	ids := map[string]bool{}
	for _, v := range recs {
		if rec, ok := v.(map[string]any); ok {
			if id, ok := rec["id"].(string); ok {
				ids[id] = true
			}
		}
	}
	return ids, true, nil
}
