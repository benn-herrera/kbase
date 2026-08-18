package treeplan

import (
	"fmt"
	"sort"
	"strings"

	"kbase/internal/dissect"
	"kbase/internal/ingest"
	"kbase/internal/log"
	"kbase/internal/survey"
)

// Verifier composes and checks tree plans over ONE corpus.
//
// It holds the survey artifact (the taxonomy stage's only view of the corpus,
// ARCHITECTURE §4 row 3) and the source under custody, because §2.4's split
// expansion runs dissect.Split for real: the number of parts an oversized span
// becomes is knowable at design time, and knowing it is what lets the tree plan
// name the leaves before stage 4 has cut anything.
//
// One is built per job and used for every call of the descent
// (CheckAnswer) and once at the end of it (Compose).
type Verifier struct {
	art    survey.Artifact
	corpus ingest.Corpus
	p      Params
	lg     log.Logger

	// files indexes art.Files by path. It is a lookup table built once, never
	// marshalled and never iterated — the artifact's own ordering is the one
	// that matters and it lives in the slice.
	files map[string]int
}

// NewVerifier returns a verifier over one corpus.
//
// It refuses a survey and a custody set that describe different bytes. The
// tree plan names byte ranges into ingested files and dissect.Split reads those
// bytes; a survey taken over a different state of the corpus would produce
// offsets that index the wrong material, and every check below would pass over
// it.
//
// lg is here because composition has one event a run must be able to see
// afterwards and no artifact field can carry on its own: a content-floor merge
// that crossed a parent index boundary (see floor). A caller with nothing to
// hand it passes log.Discard().
func NewVerifier(art survey.Artifact, corpus ingest.Corpus, p Params, lg log.Logger) (*Verifier, error) {
	if err := p.Budgets.Validate(); err != nil {
		return nil, err
	}
	if art.Schema != survey.SchemaVersion {
		return nil, fmt.Errorf("treeplan: survey declares schema %q, this build reads %q",
			art.Schema, survey.SchemaVersion)
	}
	if art.Corpus.ContentHash != corpus.ContentHash {
		return nil, fmt.Errorf("treeplan: the survey describes corpus %s and custody holds %s",
			art.Corpus.ContentHash, corpus.ContentHash)
	}
	if lg == nil {
		return nil, fmt.Errorf("treeplan: a verifier needs a logger; pass log.Discard() to silence it")
	}
	v := &Verifier{art: art, corpus: corpus, p: p, lg: lg, files: make(map[string]int, len(art.Files))}
	for i, f := range art.Files {
		v.files[f.Path] = i
	}
	return v, nil
}

// file returns the survey's inventory of one corpus document.
func (v *Verifier) file(path string) (survey.File, bool) {
	i, ok := v.files[path]
	if !ok {
		return survey.File{}, false
	}
	return v.art.Files[i], true
}

// bytes returns one document's source under custody.
func (v *Verifier) bytes(path string) ([]byte, bool) {
	u, ok := v.corpus.Doc(path)
	if !ok {
		return nil, false
	}
	return u.Bytes, true
}

// split runs the mechanical splitter over one span at the leaf budget.
//
// It is the single place this package asks how many leaves a span becomes.
// Compose asks it during expansion and Check asks it again over the finished
// artifact, and the two agreeing is the artifact's whole claim about a group:
// the tree plan owns the span, the budget and the count, and the count is
// whatever Split says it is.
func (v *Verifier) split(span Span) ([]survey.Span, error) {
	f, ok := v.file(span.File)
	if !ok {
		return nil, DefectError{Subject: span.File, Reason: "no survey inventory for this file"}
	}
	src, ok := v.bytes(span.File)
	if !ok {
		return nil, DefectError{Subject: span.File, Reason: "no source under custody for this file"}
	}
	return dissect.Split(src, survey.Span{Start: span.Start, End: span.End}, f.Cuts,
		dissect.Params{Est: v.p.Est, BudgetTokens: v.p.Budgets.LeafTokens})
}

// CheckAnnexes validates declared prefixes against the survey.
//
// Compose checks them too, but that is at the END of the taxonomy fold, by
// which point every container call has been spent on a descent whose
// composition is about to be rejected. §2.8 requires a prefix naming nothing
// to refuse at JOB SETUP, and this is what the composing verb calls there.
func (v *Verifier) CheckAnnexes(annexes []Annex) error { return checkAnnexes(annexes, v.art) }

// Compose turns a descent's plan into a verified artifact.
//
// The sequence is §2.7's fixed order with §2.4's expansion in it, and the
// composed post-conditions of §3.3 at the end:
//
//	shape → source custody → chain collapse → dissolution → content floor →
//	split expansion → index interposition → cap re-check → naming → Check
//
// Naming is last because it is the only step that cannot be undone: a path is
// what every later stage calls a node, so it is assigned once, over a tree no
// operator will touch again.
//
// Compose runs Check over its own output rather than trusting the construction
// that produced it. That is not belt-and-braces: Check is the sole statement of
// what a valid tree plan is, and running it here makes "produced by Compose"
// and "passes Check" the same predicate — which is what stage 9 later relies
// on when it re-checks an artifact it did not build.
func (v *Verifier) Compose(p TreeProposal, annexes []Annex) (TreePlan, error) {
	if err := checkAnnexes(annexes, v.art); err != nil {
		return TreePlan{}, err
	}
	root, err := toBuild(p)
	if err != nil {
		return TreePlan{}, err
	}
	if err := v.checkSources(root, annexes); err != nil {
		return TreePlan{}, err
	}

	collapse(root, v.p.Budgets.DepthCap)
	v.dissolve(root)
	crossed, err := v.floor(root)
	if err != nil {
		return TreePlan{}, err
	}
	groups, err := v.expand(root)
	if err != nil {
		return TreePlan{}, err
	}
	if err := v.interpose(root); err != nil {
		return TreePlan{}, err
	}
	if err := v.recheck(root); err != nil {
		return TreePlan{}, err
	}

	s := TreePlan{
		Schema:            SchemaVersion,
		CorpusHash:        v.art.Corpus.ContentHash,
		Budgets:           v.p.Budgets,
		Nodes:             v.name(root),
		Groups:            groups,
		CrossParentMerges: crossed,
	}
	if len(annexes) > 0 {
		s.Annexes = make([]Annex, 0, len(annexes))
		for _, a := range annexes {
			if a.Convention == "" {
				a.Convention = annexConvention(v.art, a.Prefix)
			}
			s.Annexes = append(s.Annexes, a)
		}
	}
	if err := v.Check(s); err != nil {
		return TreePlan{}, err
	}
	return s, nil
}

// name assigns every path in the tree and emits the node list, depth-first
// with parents before children.
//
// Slugs are unique within a directory by construction (see namer), which is
// why Check's duplicate-path test is a DefectError: if it ever fires, the
// guarantee this function makes has been broken, and no answer a model gave
// could have done it.
func (v *Verifier) name(root *buildNode) []Node {
	nodes := make([]Node, 0, 32)
	nodes = append(nodes, Node{
		Path:  entryPointPath,
		Kind:  KindEntryPoint,
		Title: root.title,
		Scope: root.scope,
	})
	v.nameChildren(root, "", entryPointPath, &nodes)
	return nodes
}

// nameChildren names one directory's worth of children and recurses.
func (v *Verifier) nameChildren(n *buildNode, dir, parent string, out *[]Node) {
	nm := newNamer()
	// A split family shares one base name, claimed once when its first part
	// is reached. The map is a local, never on the marshal path.
	bases := map[string]string{}

	for _, c := range n.children {
		if c.kind == KindLeaf {
			base, ok := bases[c.group]
			if !ok {
				base = nm.claim(Slug(c.base), c.origin, c.parts)
				bases[c.group] = base
			}
			name := base
			if c.parts > 1 {
				name = partName(base, c.part)
			}
			*out = append(*out, Node{
				Path:       joinDir(dir, name+mdExt),
				Kind:       KindLeaf,
				Parent:     parent,
				Title:      c.title,
				Scope:      c.scope,
				SplitGroup: c.group,
				Part:       c.part,
			})
			continue
		}
		slug := nm.claim(Slug(c.title), c.origin, 1)
		childDir := joinDir(dir, slug) + "/"
		childPath := childDir + indexFileName
		*out = append(*out, Node{
			Path:   childPath,
			Kind:   KindIndex,
			Parent: parent,
			Title:  c.title,
			Scope:  c.scope,
		})
		v.nameChildren(c, childDir, childPath, out)
	}
}

// dirOf is the directory a node's children live in: the node's path with its
// own file name removed. It is the one place the path grammar is read back,
// and Check uses it to prove every child really sits under its parent.
func dirOf(nodePath string) string {
	i := strings.LastIndex(nodePath, "/")
	if i < 0 {
		return ""
	}
	return nodePath[:i+1]
}

// sectionsOf flattens one file's surveyed sections: the preamble and the whole
// heading tree at every depth. Coverage is stated over these, because they are
// what the survey claims the document is made of.
func sectionsOf(f survey.File) []survey.Span {
	out := make([]survey.Span, 0, 8)
	if f.Preamble != nil {
		out = append(out, survey.Span{Start: f.Preamble.Start, End: f.Preamble.End})
	}
	var rec func(secs []survey.Section)
	rec = func(secs []survey.Section) {
		for _, s := range secs {
			out = append(out, survey.Span{Start: s.Start, End: s.End})
			rec(s.Children)
		}
	}
	rec(f.Sections)
	return out
}

// spansByFile groups the artifact's group spans per file, in start order, for
// the disjointness and coverage checks.
func spansByFile(groups []SplitGroup) map[string][]survey.Span {
	byFile := map[string][]survey.Span{}
	for _, g := range groups {
		byFile[g.Source.File] = append(byFile[g.Source.File],
			survey.Span{Start: g.Source.Start, End: g.Source.End})
	}
	for _, rs := range byFile {
		sort.Slice(rs, func(i, j int) bool { return rs[i].Start < rs[j].Start })
	}
	return byFile
}
