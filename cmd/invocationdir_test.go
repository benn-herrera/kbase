package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// getwdOwner is the one function in cmd that may call os.Getwd: it falls back
// to the working directory only where no MCP tool call names a directory.
const getwdOwner = "invocationDir"

// TestVerbsStartFromTheInvocationDir scans every production file in cmd and
// fails on a call to os.Getwd anywhere but getwdOwner: a RunE's helpers are
// where verb logic lives, and an os.Getwd there ignores the KB an MCP tool
// call names.
func TestVerbsStartFromTheInvocationDir(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	owned := false
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, file, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		owned = owned || slices.ContainsFunc(f.Decls, isGetwdOwner)
		for _, pos := range getwdOutsideOwner(f) {
			t.Errorf("%s: os.Getwd outside %s; start from %s(cmd)", fset.Position(pos), getwdOwner, getwdOwner)
		}
	}
	if !owned {
		t.Fatalf("no function %s in cmd; the one allowed os.Getwd site names nothing", getwdOwner)
	}
}

func TestGetwdOutsideOwnerScan(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      int
	}{
		{"literal field", `package p; import "os"; var c = C{RunE: func() { os.Getwd() }}`, 1},
		{"aliased os", `package p; import o "os"; var c = C{RunE: func() { o.Getwd() }}`, 1},
		{"assigned", `package p; import "os"; func f() { c.RunE = func() { os.Getwd() } }`, 1},
		{"a helper RunE calls", `package p; import "os"; var c = C{RunE: func() { run() }}; func run() { os.Getwd() }`, 1},
		{"package-level var", `package p; import "os"; var d, _ = os.Getwd()`, 1},
		{"inside invocationDir", `package p; import "os"; func invocationDir() { os.Getwd() }`, 0},
		{"a method named invocationDir", `package p; import "os"; func (x T) invocationDir() { os.Getwd() }`, 1},
		{"another package's Getwd", `package p; import "x"; var c = C{RunE: func() { x.Getwd() }}`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, err := parser.ParseFile(token.NewFileSet(), "p.go", tc.src, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			if got := len(getwdOutsideOwner(f)); got != tc.want {
				t.Errorf("found %d, want %d", got, tc.want)
			}
		})
	}
}

// getwdOutsideOwner is the position of every os.Getwd call in f, under
// whatever name f imports os, outside the function getwdOwner.
func getwdOutsideOwner(f *ast.File) []token.Pos {
	imports := importedAs(f)
	var found []token.Pos
	for _, d := range f.Decls {
		if isGetwdOwner(d) {
			continue
		}
		ast.Inspect(d, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Getwd" {
					if pkg, ok := sel.X.(*ast.Ident); ok && imports[pkg.Name] == "os" {
						found = append(found, call.Pos())
					}
				}
			}
			return true
		})
	}
	return found
}

// isGetwdOwner is whether d declares the function getwdOwner, not a method.
func isGetwdOwner(d ast.Decl) bool {
	fn, ok := d.(*ast.FuncDecl)
	return ok && fn.Recv == nil && fn.Name.Name == getwdOwner
}
