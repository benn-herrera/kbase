package pandoc

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// execOwners are the only packages that may start a process: the one that
// execs pandoc and the one that execs git. Every other package reaches either
// binary through its owner.
var execOwners = map[string]bool{
	"internal/latex/pandoc": true,
	"internal/ledger":       true,
}

// processStarters are the calls that start a process without os/exec, by
// import path and function name.
var processStarters = map[string]map[string]bool{
	"os":      {"StartProcess": true},
	"syscall": {"Exec": true, "ForkExec": true, "StartProcess": true},
}

// skipDirs hold no module source.
var skipDirs = map[string]bool{"bin": true, "dist": true, "build": true, "test_data": true, "testdata": true, "vendor": true}

// TestExecPolicy scans every Go file in the module, tests included, and fails
// on any import of os/exec or any process-starting call outside execOwners.
func TestExecPolicy(t *testing.T) {
	root := moduleRoot(t)
	fset := token.NewFileSet()
	checked := 0
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && (strings.HasPrefix(d.Name(), ".") || skipDirs[d.Name()]) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		f, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		checked++
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if execOwners[path.Dir(rel)] {
			return nil
		}
		for _, v := range violations(f) {
			t.Errorf("%s: %s — only %s may start a process; reach pandoc or git through its owning package",
				rel, v, strings.Join(owners(), " or "))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the module: %v", err)
	}
	if checked == 0 {
		t.Fatalf("no Go files found under %s", root)
	}
}

// violations lists every os/exec import and process-starting call in f.
func violations(f *ast.File) []string {
	var found []string
	local := map[string]string{} // local name -> import path
	for _, spec := range f.Imports {
		imported, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		if imported == "os/exec" {
			found = append(found, "imports os/exec")
		}
		if _, ok := processStarters[imported]; ok {
			name := path.Base(imported)
			if spec.Name != nil {
				name = spec.Name.Name
			}
			local[name] = imported
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); ok && processStarters[local[pkg.Name]][sel.Sel.Name] {
			found = append(found, "calls "+local[pkg.Name]+"."+sel.Sel.Name)
		}
		return true
	})
	return found
}

func owners() []string { return slices.Sorted(maps.Keys(execOwners)) }

func TestViolations(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      int
	}{
		{"clean", `package p; import "os"; func f() { os.Getenv("x") }`, 0},
		{"os/exec import", `package p; import "os/exec"; func f() { exec.Command("pandoc") }`, 1},
		{"aliased os/exec", `package p; import x "os/exec"; func f() { x.Command("git") }`, 1},
		{"os.StartProcess", `package p; import "os"; func f() { os.StartProcess("git", nil, nil) }`, 1},
		{"aliased syscall.Exec", `package p; import s "syscall"; func f() { s.Exec("/bin/git", nil, nil) }`, 1},
		{"syscall umask", `package p; import "syscall"; func f() { syscall.Umask(0) }`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, err := parser.ParseFile(token.NewFileSet(), "p.go", tc.src, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			if got := violations(f); len(got) != tc.want {
				t.Errorf("violations = %q, want %d", got, tc.want)
			}
		})
	}
}

// moduleRoot is the directory holding go.mod above the package under test.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the package under test")
		}
		dir = parent
	}
}
