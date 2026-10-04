package kb

import (
	"fmt"
	"path/filepath"
	"strings"
)

// MalformedError is authored metadata no reader can take: kb_tools raises on
// it and its refresh and verify stop.
type MalformedError struct{ Msg string }

func (e MalformedError) Error() string { return e.Msg }

func malformed(format string, args ...any) error {
	return MalformedError{Msg: fmt.Sprintf(format, args...)}
}

var (
	tier2InlineRE    = PyRE(`(?s)<!--\s*claim-quality:\s*(.*?)\s*-->`)
	strengthensPair  = PyRE(`^\s*(?:-\s*)?(` + IDBody("clm") + `)\s*:\s*(-?\d+(?:\.\d+)?)\s*$`)
	headingLineRE    = PyRE(`^(#{1,6})\s+(.*)$`)
	expIDFullRE      = PyRE(`^` + IDBody("exp") + `$`)
	supIDFullRE      = PyRE(`^` + IDBody("sup") + `$`)
	invariantHeading = PyRE(`^### (INVARIANT-[A-Z]+[0-9]+):\s*(.+)$`)
	axiomBulletRE    = PyRE(`^- Axiom (\d+): \*\*(.+?)\*\*`)
	anyHeadingRE     = PyRE(`^(#{1,6})\s`)
)

// LeafRecord is a kind: leaf document's claim-graph metadata.
type LeafRecord struct {
	Path, Kind     string
	Claims         []string
	Tier2Marked    map[string]bool
	NoClaimReason  string
	ExperimentRefs []string
}

// IndexRecord is a kind: index or entry-point document's declared aggregates.
type IndexRecord struct {
	Path, Kind                 string
	DeclaredSubtreeClaims      []string
	DeclaredSubtreeExperiments []string
}

// ExperimentNode is a physical experiment: terminal, strengthening claims.
type ExperimentNode struct {
	ID, Title, CanonicalPath, CanonicalAnchor, Status string
	Strengthens                                       []StrengthensPair
}

// StrengthensPair is one claim an experiment strengthens, and by how much.
type StrengthensPair struct {
	ClaimID  string
	Strength float64
}

// SupportNode is an analytical support: its hosting leaf's fan-out joined
// with its register entry's quality and dependencies.
type SupportNode struct {
	ID, Title, CanonicalPath, CanonicalAnchor string
	Quality                                   *float64
	DependsOn                                 []Edge
	Supports                                  []SupportPair
	Rationale                                 string
	Solidity                                  *float64
	SolidityTrace                             string
}

// FrameworkNode is an invariant or axiom from the framework source.
type FrameworkNode struct {
	NodeType, ID, Title, CanonicalPath, CanonicalAnchor string
}

// ParseLeaf is a leaf's record, or nil where the document has no frontmatter
// or is not kind: leaf.
func ParseLeaf(text, rel string) (*LeafRecord, error) {
	fm := ParseFrontmatter(text)
	if len(fm) == 0 || fm.Kind() != DocumentLeaf {
		return nil, nil
	}
	claims, err := fm.ListOrEmpty("claims")
	if err != nil {
		return nil, malformed("%s: claims: %v", rel, err)
	}
	leaf := &LeafRecord{Path: rel, Kind: DocumentLeaf, Claims: claims, Tier2Marked: map[string]bool{}}
	if v, ok := fm.Get("no-claim"); ok && !v.IsList && !v.IsBool {
		leaf.NoClaimReason = v.Str
	}
	refs, err := fm.ListOrEmpty("experiments")
	if err != nil {
		return nil, malformed("%s: experiments: %v", rel, err)
	}
	for _, r := range refs {
		if strings.HasPrefix(r, "exp-") {
			leaf.ExperimentRefs = append(leaf.ExperimentRefs, r)
		}
	}
	for _, m := range tier2InlineRE.FindAllStringSubmatch(StripFrontmatter(text), -1) {
		for _, cid := range ClaimIDs(m[1]) {
			if containsString(claims, cid) {
				leaf.Tier2Marked[cid] = true
			}
		}
	}
	return leaf, nil
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// firstHeading is the text of a document's first heading at any level.
func firstHeading(text string) string {
	for _, line := range SplitLines(text) {
		if m := headingLineRE.FindStringSubmatch(line); m != nil {
			return Strip(m[2])
		}
	}
	return ""
}

// frontmatterKey splits a frontmatter line the way the node-body scans do.
func frontmatterKey(stripped string) (key, value string, ok bool) {
	k, v, ok := strings.Cut(stripped, ":")
	if !ok {
		return "", "", false
	}
	return Strip(strings.TrimLeft(Strip(k), "- ")), Strip(v), true
}

// ParseExperimentLeaf is every experiment a kind: leaf container hosts, one
// per exp-id: key, each owning the status: and strengthens: below it.
func ParseExperimentLeaf(text, rel string) ([]ExperimentNode, error) {
	m := FindFrontmatter(text, 0)
	if m == nil {
		return nil, nil
	}
	kind, hasRefs, inStrengthens := "", false, false
	var ids []string
	var statuses []*string
	var pairs [][]StrengthensPair
	for _, line := range SplitLines(text[m[2]:m[3]]) {
		s := Strip(line)
		if s == "" {
			continue
		}
		if p := strengthensPair.FindStringSubmatch(line); inStrengthens && p != nil && len(pairs) > 0 {
			pairs[len(pairs)-1] = append(pairs[len(pairs)-1], StrengthensPair{p[1], parseFloat(p[2])})
			continue
		}
		key, value, ok := frontmatterKey(s)
		if !ok {
			continue
		}
		if key == "strengthens" {
			inStrengthens = true
			continue
		}
		inStrengthens = false
		switch key {
		case "kind":
			kind = value
		case "exp-id":
			ids = append(ids, value)
			statuses = append(statuses, nil)
			pairs = append(pairs, nil)
		case "status":
			if len(statuses) > 0 {
				statuses[len(statuses)-1] = &value
			}
		case "experiments":
			if value != "" {
				hasRefs = true
			}
		}
	}
	if kind != DocumentLeaf || len(ids) == 0 {
		return nil, nil
	}
	if hasRefs {
		return nil, malformed("%s: experiment-hosting leaf carries experiments: — an owning experiment leaf must not also reference other experiments", rel)
	}
	heading := firstHeading(text)
	var nodes []ExperimentNode
	for i, id := range ids {
		if !expIDFullRE.MatchString(id) {
			return nil, malformed("%s: experiment-hosting leaf has malformed exp-id %q", rel, id)
		}
		if statuses[i] == nil || (*statuses[i] != "run" && *statuses[i] != "pending") {
			return nil, malformed("%s: experiment %s has invalid status (expected 'run' or 'pending')", rel, id)
		}
		for _, p := range pairs[i] {
			if !(0 <= p.Strength && p.Strength <= 1) {
				return nil, malformed("%s: experiment %s strength for %s is %v — must be in [0, 1]", rel, id, p.ClaimID, p.Strength)
			}
		}
		nodes = append(nodes, ExperimentNode{ID: id, Title: heading, CanonicalPath: rel, CanonicalAnchor: Slugify(heading),
			Status: *statuses[i], Strengthens: pairs[i]})
	}
	return nodes, nil
}

// ParseSupportLeaf is every support a kind: leaf container hosts, one per
// sup-id: key with the supports: pairs below it; quality and dependencies
// are its register entry's and are joined by Discover.
func ParseSupportLeaf(text, rel string) ([]SupportNode, error) {
	m := FindFrontmatter(text, 0)
	if m == nil {
		return nil, nil
	}
	kind, inSupports := "", false
	var ids []string
	var pairs [][]SupportPair
	for _, line := range SplitLines(text[m[2]:m[3]]) {
		s := Strip(line)
		if s == "" {
			continue
		}
		if p, ok := ParseSupportPair(line); inSupports && ok && len(pairs) > 0 {
			pairs[len(pairs)-1] = append(pairs[len(pairs)-1], p)
			continue
		}
		key, value, ok := frontmatterKey(s)
		if !ok {
			continue
		}
		if key == "supports" {
			inSupports = true
			continue
		}
		inSupports = false
		switch key {
		case "kind":
			kind = value
		case "sup-id":
			ids = append(ids, value)
			pairs = append(pairs, nil)
		}
	}
	if kind != DocumentLeaf || len(ids) == 0 {
		return nil, nil
	}
	heading := firstHeading(text)
	var nodes []SupportNode
	for i, id := range ids {
		if !supIDFullRE.MatchString(id) {
			return nil, malformed("%s: support-hosting leaf has malformed sup-id %q", rel, id)
		}
		for _, p := range pairs[i] {
			if !p.Fraction.Pending && !(0 <= p.Fraction.Value && p.Fraction.Value <= 1) {
				return nil, malformed("%s: support %s on-point fraction for %s is %v — must be in [0, 1] or *pending*", rel, id, p.ClaimID, p.Fraction.Value)
			}
		}
		nodes = append(nodes, SupportNode{ID: id, Title: heading, CanonicalPath: rel, CanonicalAnchor: Slugify(heading), Supports: pairs[i]})
	}
	return nodes, nil
}

// parseIndex is a kind: index or entry-point document's record, or nil.
func parseIndex(text, rel string) (*IndexRecord, error) {
	fm := ParseFrontmatter(text)
	if len(fm) == 0 {
		return nil, nil
	}
	kind := fm.Kind()
	if kind != DocumentIndex && kind != DocumentEntryPoint {
		return nil, nil
	}
	claims, err := fm.ListOrEmpty("subtree-claims")
	if err != nil {
		return nil, malformed("%s: subtree-claims: %v", rel, err)
	}
	exps, err := fm.ListOrEmpty("subtree-experiments")
	if err != nil {
		return nil, malformed("%s: subtree-experiments: %v", rel, err)
	}
	rec := &IndexRecord{Path: rel, Kind: kind, DeclaredSubtreeClaims: claims}
	for _, e := range exps {
		if strings.HasPrefix(e, "exp-") {
			rec.DeclaredSubtreeExperiments = append(rec.DeclaredSubtreeExperiments, e)
		}
	}
	return rec, nil
}

// FrameworkSource is the file framework nodes are read from — invariants.md,
// else the legacy AGENTS.md — or "" where neither is a file.
func FrameworkSource(kbRoot string) string {
	for _, name := range []string{InvariantsFile, AgentsFile} {
		if p := filepath.Join(kbRoot, name); IsFile(p) {
			return p
		}
	}
	return ""
}

// ParseFrameworkNodes is every invariant heading and then every axiom bullet
// of the framework source; axioms anchor at INVARIANT-S2's heading.
func ParseFrameworkNodes(kbRoot string) ([]FrameworkNode, error) {
	source := FrameworkSource(kbRoot)
	if source == "" {
		return nil, nil
	}
	text, err := ReadText(source)
	if err != nil {
		return nil, err
	}
	name := filepath.Base(source)
	lines := SplitLines(text)
	var nodes []FrameworkNode
	s2 := ""
	for _, line := range lines {
		if m := invariantHeading.FindStringSubmatch(line); m != nil {
			anchor := Slugify(Strip(line[4:]))
			nodes = append(nodes, FrameworkNode{"invariant", m[1], Strip(m[2]), name, anchor})
			if m[1] == "INVARIANT-S2" {
				s2 = anchor
			}
		}
	}
	for _, line := range lines {
		if m := axiomBulletRE.FindStringSubmatch(line); m != nil {
			nodes = append(nodes, FrameworkNode{"axiom", "axiom-" + m[1], Strip(m[2]), name, s2})
		}
	}
	return nodes, nil
}

// AnchorSection is the code-stripped body of the section whose heading
// slugifies to anchor, up to the next heading as deep or shallower; ok is
// false where no heading anchors there.
func AnchorSection(text, anchor string) (string, bool) {
	lines := strings.Split(StripCodeSplitLines(text), "\n")
	for i, line := range lines {
		h := anyHeadingRE.FindStringSubmatch(line)
		if h == nil || Slugify(Strip(line[len(h[1]):])) != anchor {
			continue
		}
		var body []string
		for _, next := range lines[i+1:] {
			if n := anyHeadingRE.FindStringSubmatch(next); n != nil && len(n[1]) <= len(h[1]) {
				break
			}
			body = append(body, next)
		}
		return strings.Join(body, "\n"), true
	}
	return "", false
}

// UnmigratedAgentsCheck names UnmigratedAgentsFile's refusal.
const UnmigratedAgentsCheck = "agents-redirect"

// UnmigratedAgentsFile is the refusal for a KB whose CLAUDE.md is not the
// one-line redirect to AGENTS.md, or "" where it is or is absent.
func UnmigratedAgentsFile(kbRoot string) (string, error) {
	redirect := filepath.Join(kbRoot, AgentsRedirectFile)
	if !IsFile(redirect) {
		return "", nil
	}
	text, err := ReadText(redirect)
	if err != nil {
		return "", err
	}
	if Strip(text) == AgentsRedirect {
		return "", nil
	}
	return fmt.Sprintf("%s is not the one-line redirect '%s' — this KB predates the %s split, or the file was edited. "+
		"Nothing here converts it: move its content into %s (append if that file exists), make %s the single line '%s', then rerun.",
		redirect, AgentsRedirect, AgentsFile, filepath.Join(kbRoot, AgentsFile), redirect, AgentsRedirect), nil
}
