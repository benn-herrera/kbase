package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"
)

// revealOwners are the production files that may take a key's text out of a
// secret.Key: the two clients setting their auth header, the scrub of a key a
// server echoes, the status error's cut around it, and the store's seal.
var revealOwners = []string{
	"internal/auth/auth.go",
	"internal/model/anthropic_client.go",
	"internal/model/openai_client.go",
	"internal/model/transport.go",
}

// TestKeyRevealedOnlyByItsOwners: no production file outside revealOwners
// names Reveal, and each of them does. A Key renders redacted everywhere else,
// so a key's text reaches only the wire and the store's file.
func TestKeyRevealedOnlyByItsOwners(t *testing.T) {
	fset := token.NewFileSet()
	seen := map[string]bool{}
	for _, m := range moduleGoFiles(t, fset) {
		if !m.production() {
			continue
		}
		for _, pos := range revealSelectors(m.f) {
			seen[m.rel] = true
			if !slices.Contains(revealOwners, m.rel) {
				t.Errorf("%s: Reveal outside %s", fset.Position(pos), strings.Join(revealOwners, ", "))
			}
		}
	}
	for _, owner := range revealOwners {
		if !seen[owner] {
			t.Errorf("%s is listed as revealing a key and does not; drop it from revealOwners", owner)
		}
	}
}

func TestRevealSelectorsScan(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      int
	}{
		{"a call", `package p; func f(k K) string { return k.Reveal() }`, 1},
		{"a field's call", `package p; func f(e E) string { return e.APIKey.Reveal() }`, 1},
		{"a method value", `package p; func f(k K) func() string { return k.Reveal }`, 1},
		{"a method expression", `package p; import "kbase/internal/secret"; var g = secret.Key.Reveal`, 1},
		{"another method", `package p; func f(k K) bool { return k.IsZero() }`, 0},
		{"a function named Reveal", `package p; func Reveal() {}; func f() { Reveal() }`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, err := parser.ParseFile(token.NewFileSet(), "p.go", tc.src, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			if got := len(revealSelectors(f)); got != tc.want {
				t.Errorf("found %d, want %d", got, tc.want)
			}
		})
	}
}

// revealSelectors is the position of every selector naming Reveal in f — a
// call, a method value or a method expression. It matches the name, not the
// type: no other type in the module has a Reveal, and a new one fails here
// rather than passing unseen.
func revealSelectors(f *ast.File) []token.Pos {
	var found []token.Pos
	ast.Inspect(f, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "Reveal" {
			found = append(found, sel.Pos())
		}
		return true
	})
	return found
}
