package main

import (
	"go/ast"
	"go/token"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"kbase/internal/kb"
)

// relationHomonyms are the production files outside internal/kb whose string
// literals spell a relation's word as a name of another vocabulary, by the
// words each spells.
var relationHomonyms = map[string][]string{
	// write's per-op value keys.
	"internal/write/values.go": {kb.RelationReferences},
	// a write op's value key, and the claim graph's report fact names.
	"internal/claimgraph/write.go":  {kb.RelationReferences},
	"internal/claimgraph/stages.go": {kb.RelationReferences, kb.RelationDemoted},
	// a pandoc Div class.
	"internal/docgraph/ast.go": {kb.RelationReferences},
}

// relationSeries matches two or more relation names in a prose series, joined
// by a comma, "and" or "or" across any whitespace, with no letter or hyphen
// touching the series' ends (so restore-demoted names no relation).
var relationSeries = func() *regexp.Regexp {
	var names []string
	for _, r := range kb.Relations {
		names = append(names, regexp.QuoteMeta(r))
	}
	name := `(?:` + strings.Join(names, "|") + `)`
	sep := `(?:\s*,\s*(?:(?:and|or)\s+)?|\s+(?:and|or)\s+)`
	return regexp.MustCompile(`(?:^|[^\pL-])` + name + `(?:` + sep + name + `)+(?:$|[^\pL-])`)
}()

// TestNoRelationSpelledOutsideKB: a relation is named through kb's constants,
// and a list of relations is composed from them, never kept as prose. The
// formats tree is outside the rule: it imports nothing of kbase outside
// itself, and format400 is where the live format defines the relations.
func TestNoRelationSpelledOutsideKB(t *testing.T) {
	fset := token.NewFileSet()
	seen := map[string][]string{}
	for _, m := range moduleGoFiles(t, fset) {
		if !m.production() || under(path.Dir(m.rel), "internal/kb") || under(path.Dir(m.rel), "internal/formats") {
			continue
		}
		rel := m.rel
		ast.Inspect(m.f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			s, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			if series := relationSeries.FindString(s); series != "" {
				t.Errorf("%s: %q lists relations in prose; compose the list from kb's relation sets", fset.Position(lit.Pos()), strings.TrimSpace(series))
			}
			if !slices.Contains(kb.Relations, s) {
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
