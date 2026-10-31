package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// apiKeyFileReporter is the one production file that names the apiKeyFile
// field in text: show-config's report.
const apiKeyFileReporter = "internal/config/report.go"

// TestNoProductionTextNamesAPIKeyFile: no string literal of a production Go
// file outside apiKeyFileReporter, and no line of the MCP instructions or a
// stamped template, contains apiKeyFile. Struct tags and comments are not text
// a user is shown. Texts composed at run time are checked by servedtext_test.go.
func TestNoProductionTextNamesAPIKeyFile(t *testing.T) {
	fset := token.NewFileSet()
	for _, m := range moduleGoFiles(t, fset) {
		if !m.production() || m.rel == apiKeyFileReporter {
			continue
		}
		for _, pos := range apiKeyFileLiterals(m.f) {
			t.Errorf("%s: a string literal names apiKeyFile", fset.Position(pos))
		}
	}

	root := moduleRootDir(t)
	files := []string{filepath.Join(root, "internal/mcp/instructions.txt")}
	templates, err := filepath.Glob(filepath.Join(root, "internal/kbdocs/templates/*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(templates) == 0 {
		t.Fatal("no stamped templates found under internal/kbdocs/templates")
	}
	for _, p := range append(files, templates...) {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			if strings.Contains(line, "apiKeyFile") {
				t.Errorf("%s:%d: names apiKeyFile", filepath.ToSlash(rel), i+1)
			}
		}
	}
}

func TestAPIKeyFileLiteralsScan(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      int
	}{
		{"an interpreted string", `package p; var s = "set apiKeyFile"`, 1},
		{"a raw string", "package p; var s = `set apiKeyFile`", 1},
		{"an argument", `package p; func f() { g("apiKeyFile") }`, 1},
		{"a struct tag", "package p; type T struct { F string `toml:\"apiKeyFile\"` }", 0},
		{"a comment", "package p // apiKeyFile\n/* apiKeyFile */ var s = \"x\"", 0},
		{"an identifier", `package p; var apiKeyFile = "x"`, 0},
		{"another string", `package p; var s = "apiKey"`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, err := parser.ParseFile(token.NewFileSet(), "p.go", tc.src, 0)
			if err != nil {
				t.Fatal(err)
			}
			if got := len(apiKeyFileLiterals(f)); got != tc.want {
				t.Errorf("found %d, want %d", got, tc.want)
			}
		})
	}
}

// apiKeyFileLiterals is the position of every string literal in f containing
// apiKeyFile, struct tags excluded.
func apiKeyFileLiterals(f *ast.File) []token.Pos {
	var found []token.Pos
	tags := map[*ast.BasicLit]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Field:
			if n.Tag != nil {
				tags[n.Tag] = true
			}
		case *ast.BasicLit:
			if n.Kind == token.STRING && !tags[n] && strings.Contains(n.Value, "apiKeyFile") {
				found = append(found, n.Pos())
			}
		}
		return true
	})
	return found
}
