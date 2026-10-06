package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// selfExec is the binary os.Executable names: kbase itself.
const selfExec = "kbase"

// execOwners are the only places that may start a process, each with the one
// binary it starts: a package, or one file of a package that otherwise may
// not — the MCP server's detached build, which execs kbase itself. Every
// other package reaches each binary through its owner.
var execOwners = map[string]string{
	"internal/latex/pandoc": "pandoc",
	"internal/ledger":       "git",
	"internal/sheet":        "dot",
	"cmd/mcp.go":            selfExec,
}

// execNamers are the os/exec calls that name a binary, with the position of
// the argument that names it.
var execNamers = map[string]int{"Command": 0, "CommandContext": 1, "LookPath": 0}

// processStarters are the calls that start a process without os/exec, by
// import path and function name.
var processStarters = map[string]map[string]bool{
	"os":      {"StartProcess": true},
	"syscall": {"Exec": true, "ForkExec": true, "StartProcess": true},
}

// TestExecPolicy scans every Go file in the module, tests included, and fails
// on any import of os/exec or any process-starting call outside execOwners,
// on a process an owner starts other than through os/exec, and on an os/exec
// call in an owner that does not name the owner's binary.
func TestExecPolicy(t *testing.T) {
	fset := token.NewFileSet()
	byDir := map[string][]moduleGoFile{}
	for _, m := range moduleGoFiles(t, fset) {
		byDir[path.Dir(m.rel)] = append(byDir[path.Dir(m.rel)], m)
	}
	named := map[string]bool{}
	for dir, files := range byDir {
		pkg := make([]*ast.File, len(files))
		for i, m := range files {
			pkg[i] = m.f
		}
		for _, m := range files {
			owner, binary := m.rel, execOwners[m.rel]
			if binary == "" {
				owner, binary = dir, execOwners[dir]
			}
			for _, v := range violations(m.f, binary != "") {
				t.Errorf("%s: %s — only %s may start a process, through os/exec; reach pandoc, git or dot through its owning package",
					m.rel, v, strings.Join(slices.Sorted(maps.Keys(execOwners)), " or "))
			}
			if binary == "" {
				continue
			}
			for _, s := range execSites(m.f, pkg) {
				named[owner] = true
				switch s.binary {
				case binary:
				case "":
					t.Errorf("%s: exec.%s's binary argument resolves to no binary; name %s as binaryResolver reads one", fset.Position(s.pos), s.call, binary)
				default:
					t.Errorf("%s: exec.%s names %s; %s execs %s alone", fset.Position(s.pos), s.call, s.binary, owner, binary)
				}
			}
		}
	}
	for owner, binary := range execOwners {
		if !named[owner] {
			t.Errorf("%s is listed as execing %s and makes no os/exec call; drop it from execOwners", owner, binary)
		}
	}
}

// violations lists every process-starting call in f and, unless f belongs
// to an exec owner, its import of os/exec.
func violations(f *ast.File, ownsExec bool) []string {
	var found []string
	imports := importedAs(f)
	if !ownsExec && slices.Contains(slices.Collect(maps.Values(imports)), "os/exec") {
		found = append(found, "imports os/exec")
	}
	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); ok && processStarters[imports[pkg.Name]][sel.Sel.Name] {
			found = append(found, "calls "+imports[pkg.Name]+"."+sel.Sel.Name)
		}
		return true
	})
	return found
}

// execSite is an os/exec call that names a binary, and the binary its
// argument resolves to — empty where it resolves to none.
type execSite struct {
	pos    token.Pos
	call   string
	binary string
}

// execSites lists every execNamers call in f, resolving its binary argument
// over pkg, the files of f's package, f among them.
func execSites(f *ast.File, pkg []*ast.File) []execSite {
	imports := importedAs(f)
	r := binaryResolver{pkg: pkg, visiting: map[*ast.Field]bool{}}
	var sites []execSite
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		x, ok := sel.X.(*ast.Ident)
		if !ok || imports[x.Name] != "os/exec" {
			return true
		}
		if i, ok := execNamers[sel.Sel.Name]; ok {
			s := execSite{pos: call.Pos(), call: sel.Sel.Name}
			if i < len(call.Args) {
				s.binary = r.resolve(f, call.Args[i])
			}
			sites = append(sites, s)
		}
		return true
	})
	return sites
}

// binaryResolver follows a binary argument to the binary it names, in the
// forms the owners write: a string literal; a constant; a variable every
// assignment of which names the same binary; a function's parameter every
// call in the package passes the same binary; exec.LookPath's path for the
// binary it looks up; os.Executable's path, kbase itself. Any other form
// names none.
type binaryResolver struct {
	pkg      []*ast.File
	visiting map[*ast.Field]bool
}

func (r binaryResolver) resolve(f *ast.File, e ast.Expr) string {
	switch x := e.(type) {
	case *ast.BasicLit:
		if s, err := strconv.Unquote(x.Value); err == nil && x.Kind == token.STRING {
			return s
		}
	case *ast.CallExpr:
		sel, ok := x.Fun.(*ast.SelectorExpr)
		if !ok {
			return ""
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok {
			return ""
		}
		switch imported := importedAs(f)[pkg.Name]; {
		case imported == "os" && sel.Sel.Name == "Executable" && len(x.Args) == 0:
			return selfExec
		case imported == "os/exec" && sel.Sel.Name == "LookPath" && len(x.Args) == 1:
			return r.resolve(f, x.Args[0])
		}
	case *ast.Ident:
		return r.resolveIdent(f, x)
	}
	return ""
}

func (r binaryResolver) resolveIdent(f *ast.File, id *ast.Ident) string {
	obj := id.Obj
	if obj == nil {
		return ""
	}
	var values []string
	switch d := obj.Decl.(type) {
	case *ast.ValueSpec:
		if obj.Kind != ast.Con {
			return ""
		}
		names := make([]ast.Expr, len(d.Names))
		for i, n := range d.Names {
			names[i] = n
		}
		return r.resolveAssigned(f, names, d.Values, obj)
	case *ast.Field:
		values = append(values, r.resolveParam(f, d, obj))
	case *ast.AssignStmt: // the scan below reads it with every reassignment
	default:
		return ""
	}
	ast.Inspect(f, func(n ast.Node) bool {
		a, ok := n.(*ast.AssignStmt)
		if !ok || !slices.ContainsFunc(a.Lhs, func(l ast.Expr) bool { i, ok := l.(*ast.Ident); return ok && i.Obj == obj }) {
			return true
		}
		if a.Tok != token.DEFINE && a.Tok != token.ASSIGN {
			values = append(values, "")
		} else {
			values = append(values, r.resolveAssigned(f, a.Lhs, a.Rhs, obj))
		}
		return true
	})
	return agreed(values)
}

// resolveAssigned is the binary obj takes from rhs, where obj is one of lhs:
// rhs's own element, or the first result of the one call rhs holds.
func (r binaryResolver) resolveAssigned(f *ast.File, lhs, rhs []ast.Expr, obj *ast.Object) string {
	for i, l := range lhs {
		if id, ok := l.(*ast.Ident); !ok || id.Obj != obj {
			continue
		}
		switch {
		case len(rhs) == len(lhs):
			return r.resolve(f, rhs[i])
		case len(rhs) == 1 && i == 0:
			return r.resolve(f, rhs[0])
		}
		return ""
	}
	return ""
}

// resolveParam is the binary every call of field's function in the package
// passes for obj; the function must be one f declares, with no receiver, and
// named nowhere but in a call.
func (r binaryResolver) resolveParam(f *ast.File, field *ast.Field, obj *ast.Object) string {
	if r.visiting[field] {
		return ""
	}
	r.visiting[field] = true
	defer delete(r.visiting, field)
	var fn *ast.FuncDecl
	idx := 0
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Recv != nil {
			continue
		}
		at := 0
		for _, p := range fd.Type.Params.List {
			for j, n := range p.Names {
				if p == field && n.Obj == obj {
					fn, idx = fd, at+j
				}
			}
			at += max(1, len(p.Names))
		}
	}
	if fn == nil {
		return ""
	}
	if _, variadic := field.Type.(*ast.Ellipsis); variadic {
		return ""
	}
	var values []string
	for _, g := range r.pkg {
		called := map[*ast.Ident]bool{}
		ast.Inspect(g, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if name, ok := call.Fun.(*ast.Ident); ok && name.Name == fn.Name.Name {
				called[name] = true
				if idx < len(call.Args) && !call.Ellipsis.IsValid() {
					values = append(values, r.resolve(g, call.Args[idx]))
				} else {
					values = append(values, "")
				}
			}
			return true
		})
		ast.Inspect(g, func(n ast.Node) bool {
			if name, ok := n.(*ast.Ident); ok && name.Name == fn.Name.Name && name != fn.Name && !called[name] {
				values = append(values, "")
			}
			return true
		})
	}
	return agreed(values)
}

// agreed is the one binary every value names, or empty where they differ,
// one names none, or there are none.
func agreed(values []string) string {
	if len(values) == 0 || slices.Contains(values, "") {
		return ""
	}
	for _, v := range values[1:] {
		if v != values[0] {
			return ""
		}
	}
	return values[0]
}

func TestViolations(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		ownsExec  bool
		want      int
	}{
		{"clean", `package p; import "os"; func f() { os.Getenv("x") }`, false, 0},
		{"os/exec import", `package p; import "os/exec"; func f() { exec.Command("pandoc") }`, false, 1},
		{"aliased os/exec", `package p; import x "os/exec"; func f() { x.Command("git") }`, false, 1},
		{"os.StartProcess", `package p; import "os"; func f() { os.StartProcess("git", nil, nil) }`, false, 1},
		{"aliased syscall.Exec", `package p; import s "syscall"; func f() { s.Exec("/bin/git", nil, nil) }`, false, 1},
		{"syscall umask", `package p; import "syscall"; func f() { syscall.Umask(0) }`, false, 0},
		{"os/exec import in an owner", `package p; import "os/exec"; func f() { exec.Command("git") }`, true, 0},
		{"syscall.Exec in an owner", `package p; import "syscall"; func f() { syscall.Exec("/bin/sh", nil, nil) }`, true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, err := parser.ParseFile(token.NewFileSet(), "p.go", tc.src, 0)
			if err != nil {
				t.Fatal(err)
			}
			if got := violations(f, tc.ownsExec); len(got) != tc.want {
				t.Errorf("violations = %q, want %d", got, tc.want)
			}
		})
	}
}

func TestExecSites(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      []string
	}{
		{"literal", `package p; import "os/exec"; func f() { exec.Command("git", "status") }`, []string{"git"}},
		{"wrong binary", `package p; import "os/exec"; const bin = "sh"; func f() { exec.Command(bin) }`, []string{"sh"}},
		{"aliased os/exec", `package p; import x "os/exec"; func f() { x.LookPath("dot") }`, []string{"dot"}},
		{"constant", `package p; import "os/exec"; const bin = "pandoc"; func f() { exec.CommandContext(ctx, bin) }`, []string{"pandoc"}},
		{"LookPath through a parameter",
			`package p; import "os/exec"; func f() { dot, _ := exec.LookPath("dot"); run(dot, "x") }; func run(dot, text string) { exec.Command(dot) }`,
			[]string{"dot", "dot"}},
		{"os.Executable", `package p; import ("os"; "os/exec"); func f() { exe, _ := os.Executable(); exec.Command(exe) }`, []string{selfExec}},
		{"a parameter one call passes another binary",
			`package p; import "os/exec"; func f() { run("dot"); run("sh") }; func run(bin string) { exec.Command(bin) }`,
			[]string{""}},
		{"a parameter passed as a function value",
			`package p; import "os/exec"; func f() { run("dot"); g(run) }; func run(bin string) { exec.Command(bin) }`,
			[]string{""}},
		{"a variable reassigned", `package p; import ("os"; "os/exec"); func f() { bin := "git"; bin = os.Getenv("B"); exec.Command(bin) }`, []string{""}},
		{"an environment variable", `package p; import ("os"; "os/exec"); func f() { exec.Command(os.Getenv("B")) }`, []string{""}},
		{"a package variable", `package p; import "os/exec"; var bin = "git"; func f() { exec.Command(bin) }`, []string{""}},
		{"no binary argument", `package p; import "os/exec"; func f() { exec.CommandContext(ctx) }`, []string{""}},
		{"another package's Command", `package p; import "x"; func f() { x.Command("sh") }`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, err := parser.ParseFile(token.NewFileSet(), "p.go", tc.src, 0)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, s := range execSites(f, []*ast.File{f}) {
				got = append(got, s.binary)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("binaries = %q, want %q", got, tc.want)
			}
		})
	}
}
