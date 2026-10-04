package index

import (
	"fmt"
	"path/filepath"
	"strings"

	"kbase/internal/kb"
)

// LinkFindings is the dead-link gate over kb-root/ and the claim-id link
// check: every node id a document names must be a node .index/claims.jsonl
// records. A dead link whose target leaves the repository is not a finding,
// as kb_tools reports such a link without gating on it.
func LinkFindings(kbRoot string) ([]Finding, error) {
	root, err := filepath.Abs(kbRoot)
	if err != nil {
		return nil, err
	}
	dead, err := kb.DeadLinks(root)
	if err != nil {
		return nil, err
	}
	repo := filepath.Dir(root)
	var out []Finding
	for _, d := range dead {
		if d.Kind == kb.BrokenInter && !kb.Within(repo, d.Resolved) {
			continue
		}
		out = append(out, Finding{Check: "dead link", Path: filepath.ToSlash(d.File), Line: d.Line, Detail: d.String()})
	}
	known, ok, err := knownNodeIDs(filepath.Join(root, kb.IndexDir, "claims.jsonl"))
	if err != nil || !ok {
		return out, err
	}
	files, err := kb.MarkdownFiles(root)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		text, err := kb.ReadText(file)
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

// knownNodeIDs is every string id claims.jsonl records; ok is false where
// the file is absent, which skips the check.
func knownNodeIDs(path string) (map[string]bool, bool, error) {
	if !kb.IsFile(path) {
		return nil, false, nil
	}
	text, err := kb.ReadText(path)
	if err != nil {
		return nil, false, err
	}
	ids := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		if kb.Strip(line) == "" {
			continue
		}
		v, ok := parseJSON(kb.Strip(line))
		if !ok {
			continue
		}
		if rec, ok := v.(map[string]any); ok {
			if id, ok := rec["id"].(string); ok {
				ids[id] = true
			}
		}
	}
	return ids, true, nil
}
