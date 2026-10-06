package main

import (
	"go/ast"
	"go/token"
	"testing"
)

// fileOpeners are the calls that open a file by path, by import path and
// function name. io/fs has no package-level Open; its ReadFile reads through
// an fs.FS, which os.DirFS makes of a directory.
var fileOpeners = map[string]map[string]bool{
	"os":    {"ReadFile": true, "Open": true, "OpenFile": true, "DirFS": true},
	"io/fs": {"ReadFile": true},
}

// diskFileOwners are the production files that call kb.Source.DiskFile, which
// reads a path's bytes on disk whatever the source holds for it, each for
// what it reads.
var diskFileOwners = map[string]string{
	"internal/write/store.go":   "a write target's live bytes, checked unchanged before the write lands over them",
	"internal/index/refresh.go": "a regenerated file's live bytes, compared so an unchanged file is not rewritten",
}

// fileOpenOwners are the production files that open a file by path, each for
// what it reads. A KB's files — kb-root's documents and index and the build
// records beside it — are read through kb.Source alone, which the loader
// constructs; no other file opens one.
var fileOpenOwners = map[string]string{
	"internal/kb/source.go":             "the KB source: every read of a KB file",
	"internal/kbload/kbload.go":         "the loader: the stamp and the files a migration converts",
	"internal/atomicfile/atomicfile.go": "the atomic writer's temporary file",
	"internal/docgraph/build.go":        "a volume root's LaTeX source",
	"internal/docgraph/tree.go":         "a volume's image assets",
	"internal/records/records.go":       "the document graph's records in the state store",
	"internal/config/config.go":         "the configuration files",
	"internal/config/write.go":          "the configuration files",
	"internal/config/inference.go":      "a key file",
	"internal/config/providers.go":      "the configuration files and key files",
	"internal/filelock/filelock.go":     "the lock files",
	"internal/asks/call/call.go":        "the answer cache in the state store",
	"internal/log/log.go":               "the log file",
	"internal/build/rows.go":            "the charter, and the tree regenerated into the state store's scratch",
	"internal/build/store.go":           "the state store's lock and progress record",
	"internal/build/storeformat.go":     "the state store's format stamp",
	"cmd/write_op.go":                   "a write op's values document",
	"cmd/mcp.go":                        "the detached build's captured output in the state store",
}

// onDiskOwners are the production files that read a KB with no stamp check:
// the loader, and the build's own stages over the tree they are writing.
var onDiskOwners = map[string]string{
	"internal/kbload/kbload.go":     "the loader, at the current version",
	"internal/docgraph/validate.go": "the tree the document graph has just written, before any stamp",
	"internal/claimgraph/tree.go":   "the KB the build is writing, whose format its start saved",
}

// TestKBReadPolicy scans every production Go file and fails on a file opened
// by path outside fileOpenOwners, a DiskFile read outside diskFileOwners, or
// a KB read with kb.OnDisk outside onDiskOwners: a KB file read beside the
// loader sees an older KB's old form, not the migrated one.
func TestKBReadPolicy(t *testing.T) {
	fset := token.NewFileSet()
	seen := map[string]bool{}
	seenDiskFile := map[string]bool{}
	for _, m := range moduleGoFiles(t, fset) {
		if !m.production() {
			continue
		}
		rel := m.rel
		imports := importedAs(m.f)
		ast.Inspect(m.f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel.Sel.Name == "DiskFile" {
				seenDiskFile[rel] = true
				if _, ok := diskFileOwners[rel]; !ok {
					t.Errorf("%s: DiskFile reads a KB file on disk past the source; read it through kb.Source, or name this file's purpose in diskFileOwners", fset.Position(call.Pos()))
				}
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			switch imported := imports[pkg.Name]; {
			case fileOpeners[imported][sel.Sel.Name]:
				seen[rel] = true
				if _, ok := fileOpenOwners[rel]; !ok {
					t.Errorf("%s: %s.%s opens a file by path; read a KB file through kb.Source, or name this file's purpose in fileOpenOwners", fset.Position(call.Pos()), imported, sel.Sel.Name)
				}
			case imported == "kbase/internal/kb" && sel.Sel.Name == "OnDisk":
				if _, ok := onDiskOwners[rel]; !ok {
					t.Errorf("%s: kb.OnDisk reads a KB with no stamp check; open it with kbload.Open", fset.Position(call.Pos()))
				}
			}
			return true
		})
	}
	for owner := range fileOpenOwners {
		if !seen[owner] {
			t.Errorf("%s is listed as opening files and opens none; drop it from fileOpenOwners", owner)
		}
	}
	for owner := range diskFileOwners {
		if !seenDiskFile[owner] {
			t.Errorf("%s is listed as calling DiskFile and calls none; drop it from diskFileOwners", owner)
		}
	}
}
