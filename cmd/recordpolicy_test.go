package main

import (
	"go/ast"
	"go/token"
	"path"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"kbase/internal/kb"
)

// recordCodecHomes are the directories that may decode or encode a record
// file: kb, whose loader and encoders are the record files' one reader and
// writer, and the formats tree, which holds the live format's definition and
// the converters to it.
var recordCodecHomes = []string{"internal/kb", "internal/formats"}

// recordFileNames are the names of the two record files' paths, by the import
// path that exports them.
var recordFileNames = map[string][]string{
	"kbase/internal/kb":                {"GraphNodesFile", "GraphEdgesFile"},
	"kbase/internal/formats/format300": {"NodesFile", "EdgesFile"},
}

// recordOnlyKeys are the record keys whose YAML tag makes a struct a record
// struct: the bearing a missing field zeroes, and the flag only records carry.
// Result documents carry a bearing too, but are composed as records, never
// tagged structs.
var recordOnlyKeys = []string{kb.BearingKey, kb.ManuallyScoredKey}

const yamlImport = "go.yaml.in/yaml/v3"

// TestRecordFilesOnlyThroughTheLoader scans every production Go file outside
// recordCodecHomes and fails on a struct field tagged with a record-only YAML
// key, and on a function that both names a record file's path and calls the
// YAML package: a second record struct or an untyped decode reads a missing
// bearing as the judgement 0 where the strict loader refuses the record. A
// path that reaches a YAML call through another function is beyond a syntactic
// scan.
func TestRecordFilesOnlyThroughTheLoader(t *testing.T) {
	fset := token.NewFileSet()
	for _, m := range moduleGoFiles(t, fset) {
		if !m.production() || underAny(path.Dir(m.rel), recordCodecHomes) {
			continue
		}
		yamlNames, recordNames := map[string]bool{}, map[string][]string{}
		for _, spec := range m.f.Imports {
			p, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				continue
			}
			name := path.Base(p)
			if p == yamlImport {
				name = "yaml" // the package's name, not its path's last element
			}
			if spec.Name != nil {
				name = spec.Name.Name
			}
			switch names, ok := recordFileNames[p]; {
			case p == yamlImport:
				yamlNames[name] = true
			case ok:
				recordNames[name] = names
			}
		}
		ast.Inspect(m.f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.Field:
				if key, ok := recordTagKey(n); ok {
					t.Errorf("%s: a field tagged yaml %q is a record struct; read the record files with kb.LoadGraph or kb.LoadAuthoredGraph and write them with kb.EncodeNodes and kb.EncodeEdges",
						fset.Position(n.Pos()), key)
				}
			case *ast.FuncDecl:
				if n.Body == nil {
					return false
				}
				var named, yamlCall token.Pos
				ast.Inspect(n.Body, func(b ast.Node) bool {
					sel, ok := b.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					pkg, ok := sel.X.(*ast.Ident)
					if !ok {
						return true
					}
					if named == token.NoPos && slices.Contains(recordNames[pkg.Name], sel.Sel.Name) {
						named = sel.Pos()
					}
					if yamlCall == token.NoPos && yamlNames[pkg.Name] {
						yamlCall = sel.Pos()
					}
					return true
				})
				if named != token.NoPos && yamlCall != token.NoPos {
					t.Errorf("%s: %s names a record file and calls the YAML package at %s; read the record files with kb.LoadGraph or kb.LoadAuthoredGraph and write them with kb.EncodeNodes and kb.EncodeEdges",
						fset.Position(named), n.Name.Name, fset.Position(yamlCall))
				}
			}
			return true
		})
	}
}

// recordTagKey is the record-only key f's yaml tag names, if it names one.
func recordTagKey(f *ast.Field) (string, bool) {
	if f.Tag == nil {
		return "", false
	}
	tag, err := strconv.Unquote(f.Tag.Value)
	if err != nil {
		return "", false
	}
	key, _, _ := strings.Cut(reflect.StructTag(tag).Get("yaml"), ",")
	return key, slices.Contains(recordOnlyKeys, key)
}
