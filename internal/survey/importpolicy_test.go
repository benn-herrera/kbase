package survey

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// This is the front door of the format seam (ARCHITECTURE.md §4). The survey
// artifact is the source-format independence boundary, and a boundary that
// depends on everyone remembering it is a boundary that lasts until the first
// hurried afternoon. So the rule is a test: every Go file in the module is
// parsed for its imports, and a parser library found outside its adapter
// fails the build with the file, the import, and the rule that was broken.
//
// Adding a format means adding a row, not remembering a convention.

// modulePath is this module's import prefix. It is asserted against go.mod
// below, because a module rename that nobody noticed here would silently
// disable every in-module rule.
const modulePath = "kbase"

// importRule restricts who may import a package.
type importRule struct {
	// path is the restricted import path: the rule covers it and everything
	// beneath it, so one row governs a library's whole subtree.
	path string
	// allow lists the module-relative package directories whose files may
	// import it. A directory does not cover its subdirectories.
	allow []string
	// allowTest lists directories whose _test.go files may import it while
	// their production files may not.
	allowTest []string
	// why is printed on a violation: the reason matters more than the rule,
	// because a future format adapter will need to know which side of the
	// seam it is on.
	why string
}

var importPolicy = []importRule{{
	path:  "github.com/yuin/goldmark",
	allow: []string{"internal/survey/markdown"},
	why: "parsing Markdown is format knowledge; it belongs to the adapter, " +
		"which hands back a neutral survey.Artifact and no parser nodes",
}, {
	path:  "gopkg.in/yaml.v3",
	allow: []string{"internal/survey/markdown"},
	why: "YAML front matter is part of the Markdown convention, not of the artifact; " +
		"a later format's metadata block will be read by its own adapter",
}, {
	path:      modulePath + "/internal/survey/markdown",
	allow:     []string{"cmd"},
	allowTest: []string{"internal/survey"},
	why: "only the composition root picks a source format; internal/survey's tests may drive " +
		"an adapter to check artifact properties, but its production code may not (that is an import cycle)",
}}

// skipDirs are directories with no module source in them: build outputs, test
// corpora, and anything the Go toolchain itself ignores.
var skipDirs = map[string]bool{
	"bin":       true,
	"dist":      true,
	"build":     true,
	"test_data": true,
	"testdata":  true,
	"vendor":    true,
}

// TestImportPolicy enforces the seam over every Go file in the module, test
// files and build-tagged files included. Tests are bound by the same policy:
// a test that reaches around the seam is how the next production caller
// learns it may.
func TestImportPolicy(t *testing.T) {
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
		// Imports-only, so build tags are irrelevant: a file excluded from
		// this build is still module source, and still bound by the policy.
		f, perr := parser.ParseFile(fset, p, nil, parser.ImportsOnly)
		if perr != nil {
			return perr
		}
		checked++

		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		dir := packageDir(rel)
		isTest := strings.HasSuffix(d.Name(), "_test.go")

		for _, spec := range f.Imports {
			imported, uerr := strconv.Unquote(spec.Path.Value)
			if uerr != nil {
				return uerr
			}
			for _, r := range importPolicy {
				if !r.covers(imported) || r.permits(dir, isTest) {
					continue
				}
				t.Errorf("import policy violated\n  file:   %s\n  import: %s\n  rule:   only %s may import %s\n  why:    %s",
					rel, imported, strings.Join(r.permitted(), " or "), r.path, r.why)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the module: %v", err)
	}
	// A policy test that silently stopped finding source would pass forever.
	if checked == 0 {
		t.Fatalf("no Go files found under %s", root)
	}
}

// covers reports whether the rule governs an import path: the path itself, or
// anything beneath it.
func (r importRule) covers(imported string) bool {
	return imported == r.path || strings.HasPrefix(imported, r.path+"/")
}

// permits reports whether a file in dir may carry the import.
func (r importRule) permits(dir string, isTest bool) bool {
	for _, a := range r.allow {
		if dir == a {
			return true
		}
	}
	if !isTest {
		return false
	}
	for _, a := range r.allowTest {
		if dir == a {
			return true
		}
	}
	return false
}

// permitted renders the rule's allow lists for a failure message.
func (r importRule) permitted() []string {
	out := append([]string(nil), r.allow...)
	for _, a := range r.allowTest {
		out = append(out, a+" (tests only)")
	}
	return out
}

// packageDir is the module-relative package directory a file belongs to; "." for a
// file at the module root.
func packageDir(rel string) string {
	i := strings.LastIndex(rel, "/")
	if i < 0 {
		return "."
	}
	return rel[:i]
}

// moduleRoot finds the directory holding go.mod, walking up from the package
// under test, and checks that the module still answers to modulePath.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	for {
		gomod := filepath.Join(dir, "go.mod")
		if b, err := os.ReadFile(gomod); err == nil {
			assertModulePath(t, gomod, string(b))
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the package under test; the import policy has nothing to walk")
		}
		dir = parent
	}
}

// assertModulePath fails if go.mod no longer declares modulePath, because
// every in-module rule above is written in terms of it.
func assertModulePath(t *testing.T, gomod, contents string) {
	t.Helper()
	for line := range strings.SplitSeq(contents, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			if got := strings.TrimSpace(rest); got != modulePath {
				t.Fatalf("%s declares module %q, the import policy is written for %q — update modulePath",
					gomod, got, modulePath)
			}
			return
		}
	}
	t.Fatalf("%s declares no module path", gomod)
}
