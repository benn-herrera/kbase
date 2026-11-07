package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path"
	"slices"
	"testing"
)

// osStdoutOwner is the one production file that names os.Stdout: main, which
// hands it to the result emitter as the command tree's output.
const osStdoutOwner = "cmd/main.go"

// ownStdoutPrograms are the module's directories that build a program other
// than kbase, whose stdout is that program's own.
var ownStdoutPrograms = map[string]string{
	"tools/probestub": "the integration tests' probe stub, which prints the address it listens on",
}

// stdoutNames are the package-level names that reach stdout without a writer
// being handed in.
var stdoutNames = map[string]map[string]bool{
	"os":  {"Stdout": true},
	"fmt": {"Print": true, "Printf": true, "Println": true},
}

// stdoutWrites lists every reference in f to stdout other than through a
// writer it was handed: os.Stdout, fmt's Print family, and the builtins print
// and println, as info resolves f's identifiers (packageInfo).
func stdoutWrites(f *ast.File, info *types.Info) []pkgRef {
	refs := pkgRefs(f, stdoutNames)
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if id, ok := call.Fun.(*ast.Ident); ok {
			if b, ok := info.Uses[id].(*types.Builtin); ok && (b.Name() == "print" || b.Name() == "println") {
				refs = append(refs, pkgRef{id.Pos(), id.Name})
			}
		}
		return true
	})
	return refs
}

// TestStdoutPolicy scans every production Go file of kbase and fails on a
// write to stdout anywhere but osStdoutOwner's naming of os.Stdout: the result
// emitter owns stdout, and a stray line there breaks the one result a caller
// parses.
func TestStdoutPolicy(t *testing.T) {
	fset := token.NewFileSet()
	ownerNames, programs := false, map[string]bool{}
	var files []moduleGoFile
	packages := map[string][]*ast.File{}
	for _, m := range moduleGoFiles(t, fset) {
		if !m.production() {
			continue
		}
		if dir := path.Dir(m.rel); ownStdoutPrograms[dir] != "" {
			programs[dir] = true
			continue
		}
		files = append(files, m)
		packages[path.Dir(m.rel)] = append(packages[path.Dir(m.rel)], m.f)
	}
	infos := map[string]*types.Info{}
	for dir, pkg := range packages {
		infos[dir] = packageInfo(fset, pkg)
	}
	for _, m := range files {
		for _, r := range stdoutWrites(m.f, infos[path.Dir(m.rel)]) {
			if m.rel == osStdoutOwner && r.name == "os.Stdout" {
				ownerNames = true
				continue
			}
			t.Errorf("%s: %s writes to stdout past the result emitter; write to the stdout writer the verb was handed", fset.Position(r.pos), r.name)
		}
	}
	if !ownerNames {
		t.Errorf("%s names no os.Stdout; name the file that does as osStdoutOwner", osStdoutOwner)
	}
	for dir := range ownStdoutPrograms {
		if !programs[dir] {
			t.Errorf("%s is listed as a program of its own and holds no production Go file; drop it from ownStdoutPrograms", dir)
		}
	}
}

func TestStdoutWrites(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      []string
	}{
		{"a handed writer", `package p; import ("fmt"; "io"); func f(w io.Writer) { fmt.Fprintln(w, "x") }`, nil},
		{"stderr", `package p; import ("fmt"; "os"); func f() { fmt.Fprintln(os.Stderr, "x") }`, nil},
		{"a string", `package p; import "fmt"; func f() string { return fmt.Sprint(1) }`, nil},
		{"fmt.Println", `package p; import "fmt"; func f() { fmt.Println("x") }`, []string{"fmt.Println"}},
		{"aliased fmt.Printf", `package p; import f "fmt"; func g() { f.Printf("x") }`, []string{"fmt.Printf"}},
		{"fmt.Print as a value", `package p; import "fmt"; var out = fmt.Print`, []string{"fmt.Print"}},
		{"os.Stdout handed on", `package p; import ("fmt"; "os"); func f() { fmt.Fprintln(os.Stdout, "x") }`, []string{"os.Stdout"}},
		{"builtin println", `package p; func f() { println("x") }`, []string{"println"}},
		{"builtin print", `package p; func f() { print("x") }`, []string{"print"}},
		{"a function named print", `package p; func print(s string) {}; func f() { print("x") }`, nil},
		{"a method named Println", `package p; func f(l interface{ Println(...any) }) { l.Println("x") }`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "p.go", tc.src, 0)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, r := range stdoutWrites(f, packageInfo(fset, []*ast.File{f})) {
				got = append(got, r.name)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("stdout writes = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestStdoutWritesResolvesPrintAcrossThePackage: a print the package declares
// in another file shadows the builtin in this one.
func TestStdoutWritesResolvesPrintAcrossThePackage(t *testing.T) {
	fset := token.NewFileSet()
	var files []*ast.File
	for i, src := range []string{`package p; func f() { print("x") }`, `package p; func print(s string) {}`} {
		f, err := parser.ParseFile(fset, fmt.Sprintf("p%d.go", i), src, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	if got := stdoutWrites(files[0], packageInfo(fset, files)); len(got) != 0 {
		t.Errorf("stdout writes = %v, want none: the package's own print is called", got)
	}
}
