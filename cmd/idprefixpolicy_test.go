package main

import (
	"go/ast"
	"go/token"
	"maps"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"kbase/internal/kb"
)

// idPrefixSpellingRE matches a node id's prefix as a string spells it: the
// prefix and its dash, at the start of a word.
var idPrefixSpellingRE = regexp.MustCompile(`\b(?:` + strings.Join([]string{
	kb.ClaimPrefix, kb.AnalysisPrefix, kb.ExperimentPrefix, kb.WorkPrefix, kb.AxiomPrefix,
}, "|") + `)-`)

// idPrefixProse are the production files outside internal/formats whose string
// literals spell an id prefix in prose a reader is shown, by the number of
// such literals each holds: each command's help, which its MCP tool's
// description carries, naming the ids the command takes or mints.
var idPrefixProse = map[string]int{
	"cmd/add-rests-on.go":            1,
	"cmd/insert-analysis-entry.go":   1,
	"cmd/insert-axiom-entry.go":      1,
	"cmd/insert-claim-entry.go":      1,
	"cmd/insert-experiment-entry.go": 1,
	"cmd/insert-work-entry.go":       1,
}

// TestNoIDPrefixSpelledOutsideFormats: outside internal/formats, a node id is
// composed and matched through kb's prefix constants and kb.IDBody, so no
// production string literal spells a prefix. The formats tree is outside the
// rule: format300 defines the prefixes, and a superseded format keeps its own.
func TestNoIDPrefixSpelledOutsideFormats(t *testing.T) {
	fset := token.NewFileSet()
	seen := map[string]int{}
	for _, m := range moduleGoFiles(t, fset) {
		if !m.production() || under(path.Dir(m.rel), "internal/formats") {
			continue
		}
		rel := m.rel
		ast.Inspect(m.f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			s, err := strconv.Unquote(lit.Value)
			if err != nil || !idPrefixSpellingRE.MatchString(s) {
				return true
			}
			seen[rel]++
			if _, prose := idPrefixProse[rel]; !prose {
				t.Errorf("%s: %q spells an id prefix; compose and match ids through kb's prefix constants and kb.IDBody", fset.Position(lit.Pos()), s)
			}
			return true
		})
	}
	for _, file := range slices.Sorted(maps.Keys(idPrefixProse)) {
		if seen[file] != idPrefixProse[file] {
			t.Errorf("idPrefixProse lists %d literals for %s, which holds %d; update the entry", idPrefixProse[file], file, seen[file])
		}
	}
}
