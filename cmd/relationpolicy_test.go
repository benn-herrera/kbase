package main

import (
	"go/ast"
	"go/token"
	"path"
	"slices"
	"strconv"
	"testing"

	"kbase/internal/kb"
)

// relationHomonyms are the production files outside internal/kb whose string
// literals spell a relation's word as a name of another vocabulary, by the
// words each spells.
var relationHomonyms = map[string][]string{
	// write's per-op value keys, and the register fields its renderers and
	// readback name.
	"internal/write/values.go": {kb.RelationReferences, kb.RelationSupports, kb.RelationStrengthens},
	"internal/write/ops.go":    {kb.RelationReferences, kb.RelationSupports, kb.RelationStrengthens},
	"internal/write/store.go":  {kb.RelationReferences, kb.RelationSupports},
	// a write op's value key, and the claim graph's report fact names.
	"internal/claimgraph/write.go":  {kb.RelationReferences},
	"internal/claimgraph/stages.go": {kb.RelationReferences, kb.RelationDemoted},
	// a pandoc Div class.
	"internal/docgraph/ast.go": {kb.RelationReferences},
}

// TestNoRelationSpelledOutsideKB: a relation is named through kb's constants.
// internal/migrate is outside the rule: it imports nothing of kbase (I8).
func TestNoRelationSpelledOutsideKB(t *testing.T) {
	fset := token.NewFileSet()
	seen := map[string][]string{}
	for _, m := range moduleGoFiles(t, fset) {
		if !m.production() || under(path.Dir(m.rel), "internal/kb") || under(path.Dir(m.rel), "internal/migrate") {
			continue
		}
		rel := m.rel
		ast.Inspect(m.f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			s, err := strconv.Unquote(lit.Value)
			if err != nil || !slices.Contains(kb.Relations, s) {
				return true
			}
			if !slices.Contains(seen[rel], s) {
				seen[rel] = append(seen[rel], s)
			}
			if !slices.Contains(relationHomonyms[rel], s) {
				t.Errorf("%s: %q spells a relation; name it through kb's Relation constants", fset.Position(lit.Pos()), s)
			}
			return true
		})
	}
	for file, words := range relationHomonyms {
		for _, w := range words {
			if !slices.Contains(seen[file], w) {
				t.Errorf("relationHomonyms lists %q for %s, which no longer spells it; drop the entry", w, file)
			}
		}
	}
}
