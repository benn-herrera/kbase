package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The module walk every policy scan reads: the exec, import, read and
// relation policies agree on which files are the module's Go source.

// rootSkipDirs hold no module source at the module root; deeper, a directory
// of the same name (internal/build) is source.
var rootSkipDirs = map[string]bool{"bin": true, "dist": true, "build": true, "test_data": true, "vendor": true}

// moduleGoFile is one Go file of the module, parsed with object resolution,
// by module-relative slash path.
type moduleGoFile struct {
	rel string
	f   *ast.File
}

// production is whether the file is built into the binary, not a test.
func (m moduleGoFile) production() bool { return !strings.HasSuffix(m.rel, "_test.go") }

// moduleGoFiles is every Go file of the module, tests included, positions
// recorded in fset. Dot directories and testdata are skipped at any depth,
// rootSkipDirs at the module root only.
func moduleGoFiles(t *testing.T, fset *token.FileSet) []moduleGoFile {
	t.Helper()
	root := moduleRootDir(t)
	var files []moduleGoFile
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && (strings.HasPrefix(d.Name(), ".") || d.Name() == "testdata" || rootSkipDirs[d.Name()] && filepath.Dir(p) == root) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		files = append(files, moduleGoFile{filepath.ToSlash(rel), f})
		return nil
	})
	if err != nil {
		t.Fatalf("walking the module: %v", err)
	}
	reached := false
	for _, m := range files {
		reached = reached || path.Dir(m.rel) == "internal/build"
	}
	if !reached {
		t.Fatal("the walk never reached internal/build: a root-only skip reached a source directory of the same name")
	}
	return files
}

// moduleRootDir is the directory holding go.mod above the package under test.
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

// importedAs maps each name f refers to an import by to the import's path.
func importedAs(f *ast.File) map[string]string {
	names := map[string]string{}
	for _, spec := range f.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := path.Base(p)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		names[name] = p
	}
	return names
}
