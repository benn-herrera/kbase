package kb

import (
	"fmt"
	"path/filepath"
	"strconv"
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
	pairClaimIDRE    = PyRE(`^` + IDBody("clm") + `$`)
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

// MarkerLines is, for every claim id a tier-2 marker in text names, the
// 0-based line the first such marker opens on.
func MarkerLines(text string) map[string]int {
	out := map[string]int{}
	for _, m := range tier2InlineRE.FindAllStringSubmatchIndex(text, -1) {
		line := strings.Count(text[:m[0]], "\n")
		for _, cid := range ClaimIDs(text[m[2]:m[3]]) {
			if _, seen := out[cid]; !seen {
				out[cid] = line
			}
		}
	}
	return out
}

// documentFrontmatter is the document's frontmatter, its malformation named
// with the document.
func documentFrontmatter(text, rel string) (Frontmatter, error) {
	fm, err := ParseFrontmatter(text)
	if err != nil {
		return nil, malformed("%s: %v", rel, err)
	}
	return fm, nil
}

// ParseLeaf is a leaf's record, or nil where the document has no frontmatter
// or is not kind: leaf.
func ParseLeaf(text, rel string) (*LeafRecord, error) {
	fm, err := documentFrontmatter(text, rel)
	if err != nil {
		return nil, err
	}
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

// declarations is a leaf's node declaration list under key — nil where the
// document is no kind: leaf or declares none — refusing an entry that lacks
// its id key.
func declarations(fm Frontmatter, key, idKey, rel string) ([]Frontmatter, error) {
	if fm.Kind() != DocumentLeaf {
		return nil, nil
	}
	v := fm[key]
	if !v.IsList && v.Truthy() || len(v.List) > 0 {
		return nil, malformed("%s: %s is not a list of mappings", rel, key)
	}
	for _, node := range v.Entries {
		if _, ok := node[idKey]; !ok {
			return nil, malformed("%s: an entry of %s carries no %s", rel, key, idKey)
		}
	}
	return v.Entries, nil
}

// pairs is a node's score list under key: each one-key mapping whose key is a
// claim id and whose score reads, in list order. A score that reads as
// neither a number nor, where pending is allowed, the pending literal is
// skipped, as the line scan of the earlier format skipped it.
func pairs(node Frontmatter, key, rel, id string, pending bool) ([]SupportPair, error) {
	var out []SupportPair
	for _, entry := range node[key].Entries {
		claimID, score, ok := scorePair(entry)
		if !ok {
			return nil, malformed("%s: %s: a %s entry is not one claim id and its score", rel, id, key)
		}
		if !pairClaimIDRE.MatchString(claimID) || score.IsList || score.IsBool {
			continue
		}
		p := SupportPair{ClaimID: claimID, Fraction: Fraction{Set: true}}
		if pending && score.Str == PendingLiteral {
			p.Fraction.Pending = true
		} else if v, err := strconv.ParseFloat(score.Str, 64); err == nil {
			p.Fraction.Value = v
		} else {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

// ParseExperimentLeaf is every experiment a kind: leaf container declares
// under experiment-nodes, each with its status and strengthens pairs.
func ParseExperimentLeaf(text, rel string) ([]ExperimentNode, error) {
	fm, err := documentFrontmatter(text, rel)
	if err != nil {
		return nil, err
	}
	decls, err := declarations(fm, ExperimentNodesKey, ExpIDKey, rel)
	if err != nil || len(decls) == 0 {
		return nil, err
	}
	if v, ok := fm["experiments"]; ok && (v.IsList || v.IsBool || v.Str != "") {
		return nil, malformed("%s: experiment-hosting leaf carries experiments: — an owning experiment leaf must not also reference other experiments", rel)
	}
	heading := firstHeading(StripFrontmatter(text))
	var nodes []ExperimentNode
	for _, d := range decls {
		id := d.Str(ExpIDKey)
		if !expIDFullRE.MatchString(id) {
			return nil, malformed("%s: experiment-hosting leaf has malformed exp-id %q", rel, id)
		}
		status, ok := d[StatusKey]
		if !ok || status.IsList || status.IsBool || (status.Str != "run" && status.Str != "pending") {
			return nil, malformed("%s: experiment %s has invalid status (expected 'run' or 'pending')", rel, id)
		}
		scored, err := pairs(d, StrengthensKey, rel, id, false)
		if err != nil {
			return nil, err
		}
		var strengthens []StrengthensPair
		for _, p := range scored {
			if !(0 <= p.Fraction.Value && p.Fraction.Value <= 1) {
				return nil, malformed("%s: experiment %s strength for %s is %v — must be in [0, 1]", rel, id, p.ClaimID, p.Fraction.Value)
			}
			strengthens = append(strengthens, StrengthensPair{p.ClaimID, p.Fraction.Value})
		}
		nodes = append(nodes, ExperimentNode{ID: id, Title: heading, CanonicalPath: rel, CanonicalAnchor: Slugify(heading),
			Status: status.Str, Strengthens: strengthens})
	}
	return nodes, nil
}

// ParseSupportLeaf is every support a kind: leaf container declares under
// support-nodes, each with its supports pairs; quality and dependencies are
// its register entry's and are joined by Discover.
func ParseSupportLeaf(text, rel string) ([]SupportNode, error) {
	fm, err := documentFrontmatter(text, rel)
	if err != nil {
		return nil, err
	}
	decls, err := declarations(fm, SupportNodesKey, SupIDKey, rel)
	if err != nil || len(decls) == 0 {
		return nil, err
	}
	heading := firstHeading(StripFrontmatter(text))
	var nodes []SupportNode
	for _, d := range decls {
		id := d.Str(SupIDKey)
		if !supIDFullRE.MatchString(id) {
			return nil, malformed("%s: support-hosting leaf has malformed sup-id %q", rel, id)
		}
		supports, err := pairs(d, SupportsKey, rel, id, true)
		if err != nil {
			return nil, err
		}
		for _, p := range supports {
			if !p.Fraction.Pending && !(0 <= p.Fraction.Value && p.Fraction.Value <= 1) {
				return nil, malformed("%s: support %s on-point fraction for %s is %v — must be in [0, 1] or *pending*", rel, id, p.ClaimID, p.Fraction.Value)
			}
		}
		nodes = append(nodes, SupportNode{ID: id, Title: heading, CanonicalPath: rel, CanonicalAnchor: Slugify(heading), Supports: supports})
	}
	return nodes, nil
}

// parseIndex is a kind: index or entry-point document's record, or nil.
func parseIndex(text, rel string) (*IndexRecord, error) {
	fm, err := documentFrontmatter(text, rel)
	if err != nil {
		return nil, err
	}
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
func FrameworkSource(src *Source) string {
	for _, name := range []string{InvariantsFile, AgentsFile} {
		if p := src.KBPath(name); src.IsFile(p) {
			return p
		}
	}
	return ""
}

// ParseFrameworkNodes is every invariant heading and then every axiom bullet
// of the framework source; axioms anchor at INVARIANT-S2's heading.
func ParseFrameworkNodes(src *Source) ([]FrameworkNode, error) {
	source := FrameworkSource(src)
	if source == "" {
		return nil, nil
	}
	text, err := src.ReadText(source)
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
func UnmigratedAgentsFile(src *Source) (string, error) {
	kbRoot := src.Root()
	redirect := filepath.Join(kbRoot, AgentsRedirectFile)
	if !src.IsFile(redirect) {
		return "", nil
	}
	text, err := src.ReadText(redirect)
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
