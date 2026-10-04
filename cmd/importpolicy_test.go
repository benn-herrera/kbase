package main

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The package map's dependency rules (ARCHITECTURE §11), checked over the
// module's import graph: every production Go file is parsed for its imports,
// and a rule names who may not reach whom, directly or through any chain.

const modulePath = "kbase"

// dependencyRule binds the packages under from — every package, where from
// is empty — less those under except: none may reach a package under
// forbidden, and where allowed is set, none may reach a module package
// outside it. direct limits a rule to a package's own imports. A package
// matches a prefix where it is the prefix or beneath it.
type dependencyRule struct {
	from, except       []string
	forbidden, allowed []string
	direct             bool
	why                string
}

var dependencyRules = []dependencyRule{{
	from:      []string{"internal/docgraph"},
	forbidden: []string{"internal/claimgraph"},
	why:       "docgraph knows nothing of claims; the record schema both read lives in internal/records",
}, {
	from:      []string{"internal/claimgraph"},
	forbidden: []string{"internal/latex"},
	why:       "the claim graph reads the tree and the records, never the reader",
}, {
	from:      []string{"internal/claimgraph"},
	forbidden: []string{"internal/model", "internal/asks/call"},
	why:       "nothing in claimgraph knows a network exists: it asks through the letter seam build injects",
}, {
	except:    []string{"internal/build", "internal/asks/call"},
	forbidden: []string{"internal/asks/call"},
	direct:    true,
	why:       "only build imports the caller",
}, {
	from:      []string{"internal/write", "internal/index", "internal/query", "internal/sheet", "internal/kbdocs"},
	forbidden: []string{"internal/docgraph", "internal/claimgraph", "internal/build"},
	why:       "the maintenance packages stand below the build",
}, {
	from:    []string{"internal/kb"},
	allowed: []string{"internal/log"},
	why:     "kb imports no other kbase package but log",
}, {
	from:    []string{"internal/ledger"},
	allowed: []string{"internal/log"},
	why:     "ledger imports log and nothing else of kbase",
}, {
	except:    []string{"internal/build", "internal/ledger"},
	forbidden: []string{"internal/ledger"},
	direct:    true,
	why:       "only build imports ledger",
}, {
	except:    []string{"internal/asks/call", "internal/build", "internal/model", "cmd"},
	forbidden: []string{"internal/model"},
	direct:    true,
	why:       "only the caller, build, and cmd's provider verbs import model",
}}

func underAny(pkg string, prefixes []string) bool {
	return slices.ContainsFunc(prefixes, func(p string) bool { return under(pkg, p) })
}

func (r dependencyRule) binds(pkg string) bool {
	return (len(r.from) == 0 || underAny(pkg, r.from)) && !underAny(pkg, r.except)
}

func (r dependencyRule) refuses(dep string) bool {
	return underAny(dep, r.forbidden) || (len(r.allowed) > 0 && !underAny(dep, r.allowed))
}

// under is whether pkg is prefix or beneath it.
func under(pkg, prefix string) bool { return pkg == prefix || strings.HasPrefix(pkg, prefix+"/") }

// importGraph is every module package's in-module imports, from its
// production files, by module-relative directory.
func importGraph(t *testing.T, root string) map[string][]string {
	t.Helper()
	graph := map[string][]string{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && (strings.HasPrefix(d.Name(), ".") || slices.Contains([]string{"bin", "dist", "build", "test_data", "testdata", "vendor"}, d.Name()) && filepath.Dir(p) == root) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, p, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, filepath.Dir(p))
		if err != nil {
			return err
		}
		pkg := filepath.ToSlash(rel)
		if _, ok := graph[pkg]; !ok {
			graph[pkg] = nil
		}
		for _, spec := range f.Imports {
			imported, err := strconv.Unquote(spec.Path.Value)
			if err != nil || !strings.HasPrefix(imported, modulePath+"/") {
				continue
			}
			dep := strings.TrimPrefix(imported, modulePath+"/")
			if !slices.Contains(graph[pkg], dep) {
				graph[pkg] = append(graph[pkg], dep)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the module: %v", err)
	}
	return graph
}

// reach is every package pkg imports, directly or through others.
func reach(graph map[string][]string, pkg string) map[string]bool {
	seen := map[string]bool{}
	pending := slices.Clone(graph[pkg])
	for len(pending) > 0 {
		next := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if !seen[next] {
			seen[next] = true
			pending = append(pending, graph[next]...)
		}
	}
	return seen
}

func TestDependencyRules(t *testing.T) {
	root := moduleRootDir(t)
	graph := importGraph(t, root)
	if len(graph[path.Join("internal", "kb")]) == 0 && len(graph["cmd"]) == 0 {
		t.Fatal("no module packages read; the walk found nothing to check")
	}
	pkgs := make([]string, 0, len(graph))
	for p := range graph {
		pkgs = append(pkgs, p)
	}
	slices.Sort(pkgs)
	for _, rule := range dependencyRules {
		for _, pkg := range pkgs {
			if !rule.binds(pkg) {
				continue
			}
			reached := map[string]bool{}
			if rule.direct {
				for _, d := range graph[pkg] {
					reached[d] = true
				}
			} else {
				reached = reach(graph, pkg)
			}
			for dep := range reached {
				if rule.refuses(dep) {
					t.Errorf("%s reaches %s — %s", pkg, dep, rule.why)
				}
			}
		}
	}
}

func TestReach(t *testing.T) {
	graph := map[string][]string{"a": {"b"}, "b": {"c"}, "c": {"a"}, "d": nil}
	if got := reach(graph, "a"); !got["b"] || !got["c"] || !got["a"] || got["d"] {
		t.Errorf("reach(a) = %v, want the cycle a, b, c and not d", got)
	}
}

func moduleRootDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
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
