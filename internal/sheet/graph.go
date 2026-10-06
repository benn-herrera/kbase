package sheet

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"

	"kbase/internal/asks"
	"kbase/internal/buildrecords"
	"kbase/internal/kb"
)

// A claim's kind, as its leaf shows it: a labelled block, a displayed
// equation, or prose. Any other node draws as its node type, and an edge end
// no index row carries as kindGhost.
const (
	kindBlock    = "block"
	kindEquation = "equation"
	kindProse    = "prose"
	kindGhost    = "ghost"
)

// mathInfo is the info string of a displayed equation's fence.
const mathInfo = "math"

// rootTitleDefault is the root bucket's name where the entry point carries
// no H1.
const rootTitleDefault = kb.KBDir

// kb_tools' claim_sheet rules for a claim's kind: a title read where no leaf
// marks the claim, and the first line of the blockquote a marker sits in.
var (
	blockTitleRE   = regexp.MustCompile(`^(?:Theorem|Proposition|Lemma|Corollary|Definition|Conjecture|Remark|Assumption|Axiom|Claim|Example)\b`)
	printedNameRE  = kb.PyRE(`^\S+(?: \S+)? \d+(?:\.\d+)*$`)
	blockOpeningRE = regexp.MustCompile(`^(?:<span\b[^>]*>)?\*\*`)
)

type node struct {
	id, kind, title string
	// href is the register entry, relative to kb-root; "" for a ghost.
	href string
	// band is a claim's or a support's build band slug, "" for any other node.
	band string
	// group is the top-level directory of the node's canonical path, "" for
	// a node registered at kb-root.
	group string
}

// volume is one cluster of the full sheet and one box of the digest; key is
// "" for the kb-root bucket.
type volume struct{ key, title, href string }

// edge is one stroke, in index direction.
type edge struct {
	source, target string
	stroke         *stroke
}

// graph is everything a sheet draws from.
type graph struct {
	// kbTitle is the entry point's H1, naming the full sheet and the root bucket.
	kbTitle string
	// volumes holds every group a node sits in, by key, the root bucket last.
	volumes []volume
	// nodes is every index node and ghost, by id.
	nodes []node
	// edges is every drawn edge, one per source, target and relation, in
	// that order.
	edges []edge
}

// readIndex is every row of T's index file; a line that is not one is an
// error naming the first.
func readIndex[T kb.IndexRowType](src *kb.Source) ([]T, error) {
	path, rows, problems, err := kb.ReadIndex[T](src, nil)
	if err != nil {
		return nil, err
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("%s:%d: %w", path, problems[0].Line, problems[0].Err)
	}
	return rows, nil
}

type pair struct{ source, target string }

// letterPairs is every entry's pair whose chosen letter is letter.
func letterPairs(entries []buildrecords.CandidateEntry, letter string) map[pair]bool {
	out := map[pair]bool{}
	for _, e := range entries {
		if e.Letter != nil && *e.Letter == letter {
			out[pair{e.Source, e.Target}] = true
		}
	}
	return out
}

// load reads the graph of the KB src reads.
func load(src *kb.Source) (*graph, error) {
	claims, err := readIndex[kb.NodeRow](src)
	if err != nil {
		return nil, err
	}
	rows, err := readIndex[kb.EdgeRow](src)
	if err != nil {
		return nil, err
	}
	cites, err := readIndex[kb.CiteRow](src)
	if err != nil {
		return nil, err
	}
	unmarked, _, err := buildrecords.ReadUnmarked(src)
	if err != nil {
		return nil, err
	}
	kbTitle, err := h1(src, kb.EntryPointFile)
	if err != nil {
		return nil, err
	}
	g := &graph{kbTitle: cmp.Or(kbTitle, rootTitleDefault)}

	leaves := map[string][]string{}
	for _, c := range cites {
		leaves[c.ClaimID] = append(leaves[c.ClaimID], c.LeafPath)
	}
	for _, l := range leaves {
		slices.Sort(l)
	}
	kinds := kindReader{src: src, texts: map[string]string{}}
	byID := map[string]node{}
	for _, c := range claims {
		n := node{id: c.ID, kind: c.NodeType, title: c.Title, href: c.CanonicalPath}
		if c.CanonicalAnchor != "" {
			n.href += "#" + c.CanonicalAnchor
		}
		if dir, _, ok := strings.Cut(c.CanonicalPath, "/"); ok {
			n.group = dir
		}
		switch c.NodeType {
		case kb.NodeKindClaim:
			n.kind = kinds.claimKind(c.ID, c.Title, leaves[c.ID])
			n.band = c.BuildBand
			if _, ok := bandFills[n.band]; !ok {
				n.band = kb.UnknownBandSlug
			}
		case kb.NodeKindSupport:
			n.band = kb.UnknownBandSlug
			if b := kb.BandFor(c.Solidity); b != nil {
				n.band = b.Slug
			}
		}
		byID[c.ID] = n
	}
	g.edges = drawnEdges(rows, letterPairs(unmarked.Pairs, asks.LetterPoints))
	for _, e := range g.edges {
		for _, end := range []string{e.source, e.target} {
			if _, ok := byID[end]; !ok {
				byID[end] = node{id: end, kind: kindGhost}
			}
		}
	}
	held := map[string]bool{}
	for _, id := range slices.Sorted(maps.Keys(byID)) {
		n := byID[id]
		g.nodes = append(g.nodes, n)
		if n.kind != kindGhost {
			held[n.group] = true
		}
	}
	for _, key := range slices.Sorted(maps.Keys(held)) {
		if key == "" {
			continue
		}
		title, err := h1(src, key+"/"+kb.IndexFile)
		if err != nil {
			return nil, err
		}
		g.volumes = append(g.volumes, volume{key, cmp.Or(title, key), key + "/" + kb.IndexFile})
	}
	if held[""] {
		g.volumes = append(g.volumes, volume{"", g.kbTitle, kb.EntryPointFile})
	}
	return g, nil
}

// h1 is the first "# " heading outside the metadata block of the
// kb-root-relative document rel, as written; "" where it has none or does
// not exist.
func h1(src *kb.Source, rel string) (string, error) {
	text, err := src.ReadText(src.KBPath(rel))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	for _, line := range kb.Lines(kb.StripFrontmatter(text)) {
		if h, ok := strings.CutPrefix(line, "# "); ok {
			return h, nil
		}
	}
	return "", nil
}

// drawnEdges is the depends-on rows the sheet draws, one per source, target,
// relation and origin whatever their context, ordered by those. A relation
// with no stroke — references — is not drawn; a depends row is inferred where
// the unmarked ask said it points, a demoted row where its origin says so.
func drawnEdges(rows []kb.EdgeRow, askedYes map[pair]bool) []edge {
	seen := map[[4]string]bool{}
	var out []edge
	for _, r := range rows {
		origin := ""
		if r.Origin != nil {
			origin = *r.Origin
		}
		inferred := askedYes[pair{r.Source, r.Target}]
		if r.Relation == kb.RelationDemoted {
			inferred = origin == kb.OriginInferred
		}
		s := strokeFor(r.Relation, inferred)
		key := [4]string{r.Source, r.Target, r.Relation, origin}
		if s == nil || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, edge{source: r.Source, target: r.Target, stroke: s})
	}
	slices.SortFunc(out, compareEdges)
	return out
}

func compareEdges(a, b edge) int {
	return cmp.Or(strings.Compare(a.source, b.source), strings.Compare(a.target, b.target), strings.Compare(a.stroke.relation, b.stroke.relation),
		strings.Compare(a.stroke.provenance, b.stroke.provenance))
}

// reduce is edges, ordered by compareEdges, less each depends edge whose
// target its source still reaches through the depends edges kept so far and
// those not yet taken; so no reachability is lost even where depends holds a
// cycle. No other relation is a route or dropped.
func reduce(edges []edge) []edge {
	following := map[string]map[string]bool{}
	for _, e := range edges {
		if isDepends(e) {
			if following[e.source] == nil {
				following[e.source] = map[string]bool{}
			}
			following[e.source][e.target] = true
		}
	}
	reaches := func(start, goal string) bool {
		seen := map[string]bool{start: true}
		pending := []string{start}
		for len(pending) > 0 {
			at := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			for n := range following[at] {
				if n == goal {
					return true
				}
				if !seen[n] {
					seen[n] = true
					pending = append(pending, n)
				}
			}
		}
		return false
	}
	var kept []edge
	for _, e := range edges {
		if isDepends(e) {
			delete(following[e.source], e.target)
			if reaches(e.source, e.target) {
				continue
			}
			following[e.source][e.target] = true
		}
		kept = append(kept, e)
	}
	return kept
}

func isDepends(e edge) bool { return e.stroke.relation == kb.RelationDepends }

// kindReader reads each claim's kind from its leaves, each leaf read once.
type kindReader struct {
	src   *kb.Source
	texts map[string]string
}

// text is leaf's text, "" where it does not read.
func (r *kindReader) text(leaf string) string {
	if t, ok := r.texts[leaf]; ok {
		return t
	}
	t, _ := r.src.ReadText(r.src.KBPath(leaf))
	r.texts[leaf] = t
	return t
}

// claimKind is the kind the first of leaves, in path order, that carries the
// claim's marker shows at it. Where none does, the title decides: an equation
// title; a block title, one opening with a block name or a printed name
// ("Lemma 2.1", "Test example 2"); else prose.
func (r *kindReader) claimKind(id, title string, leaves []string) string {
	for _, leaf := range leaves {
		text := r.text(leaf)
		if at, ok := kb.MarkerLines(text)[id]; ok {
			return kindAt(kb.Lines(text), at)
		}
	}
	if _, ok := kb.EquationLabel(title); ok {
		return kindEquation
	}
	if blockTitleRE.MatchString(title) || printedNameRE.MatchString(title) {
		return kindBlock
	}
	return kindProse
}

// kindAt is the kind of the claim whose marker is on line at: an equation
// inside a displayed equation's fence, a block inside a blockquote whose
// first non-empty line opens bold, prose otherwise.
func kindAt(lines []string, at int) string {
	for _, f := range kb.Fences(lines) {
		if at >= f.Start && at < f.End && f.Info == mathInfo {
			return kindEquation
		}
	}
	if !kb.BlockquotePrefix.MatchString(lines[at]) {
		return kindProse
	}
	unquoted := func(i int) string { return kb.Strip(lines[i][len(kb.BlockquotePrefix.FindString(lines[i])):]) }
	first := at
	for first > 0 && kb.BlockquotePrefix.MatchString(lines[first-1]) {
		first--
	}
	for first < at && unquoted(first) == "" {
		first++
	}
	if blockOpeningRE.MatchString(unquoted(first)) {
		return kindBlock
	}
	return kindProse
}
