package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// registrationFile is the one production file of the module that adds a
// command to the tree or declares a command's binding, by module-relative
// path.
const registrationFile = "cmd/surface.go"

// bindingWrappers are the declarations registrationFile wraps each command in.
var bindingWrappers = map[string]bool{"readOnlyTool": true, "readWriteTool": true, "cliOnly": true}

// mcpBindingCalls are internal/mcp's command-level binding declarations, which
// the wrappers call; a flag's ExcludeFlag is not among them.
var mcpBindingCalls = map[string]bool{"Bind": true, "Exclude": true, "ReadOnly": true}

// TestCommandsRegisterOnlyInTheSurface scans every production file of the
// module and fails on an AddCommand, a binding wrapper or an internal/mcp
// binding call anywhere but registrationFile, so a command's registration,
// binding and read-only hint have one place to be read.
func TestCommandsRegisterOnlyInTheSurface(t *testing.T) {
	fset := token.NewFileSet()
	registered := false
	for _, m := range moduleGoFiles(t, fset) {
		if !m.production() {
			continue
		}
		calls := registrationCalls(m.f)
		if m.rel == registrationFile {
			registered = len(calls) > 0
			continue
		}
		for _, pos := range calls {
			t.Errorf("%s: a command added or bound outside %s; add it there, wrapped readOnlyTool, readWriteTool or cliOnly", fset.Position(pos), registrationFile)
		}
	}
	if !registered {
		t.Fatalf("%s adds no command; the one registration file registers nothing", registrationFile)
	}
}

func TestRegistrationCallsScan(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      int
	}{
		{"AddCommand on any receiver", `package p; func f() { c.AddCommand(d) }`, 1},
		{"a binding wrapper", `package p; func f() { g(readWriteTool(c)) }`, 1},
		{"cliOnly in a package-level var", `package p; var c = cliOnly(d, "r")`, 1},
		{"mcp.Bind", `package p; import "kbase/internal/mcp"; func f() { mcp.Bind(c) }`, 1},
		{"aliased mcp", `package p; import m "kbase/internal/mcp"; func f() { m.ReadOnly(m.Exclude(c, "r")) }`, 2},
		{"a flag's exclusion", `package p; import "kbase/internal/mcp"; func f() { mcp.ExcludeFlag(fl) }`, 0},
		{"another package's Bind", `package p; import "x/mcp"; func f() { mcp.Bind(c) }`, 0},
		{"a method named as a wrapper", `package p; func f() { x.cliOnly(c, "r") }`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, err := parser.ParseFile(token.NewFileSet(), "p.go", tc.src, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			if got := len(registrationCalls(f)); got != tc.want {
				t.Errorf("found %d, want %d", got, tc.want)
			}
		})
	}
}

// registrationCalls is the position of every call in f that adds a command to
// a tree or declares a command's binding: AddCommand on any receiver, a binding
// wrapper, or an internal/mcp binding call under whatever name f imports it.
func registrationCalls(f *ast.File) []token.Pos {
	imports := importedAs(f)
	var found []token.Pos
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			if bindingWrappers[fun.Name] {
				found = append(found, call.Pos())
			}
		case *ast.SelectorExpr:
			pkg, isPkg := fun.X.(*ast.Ident)
			switch {
			case fun.Sel.Name == "AddCommand":
				found = append(found, call.Pos())
			case isPkg && imports[pkg.Name] == "kbase/internal/mcp" && mcpBindingCalls[fun.Sel.Name]:
				found = append(found, call.Pos())
			}
		}
		return true
	})
	return found
}
