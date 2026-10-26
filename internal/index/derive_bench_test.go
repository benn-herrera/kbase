package index

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/kb"
)

// sheetFixtureKB is the claim-graph sheet fixture's kb-root, which carries
// every node kind and every relation.
var sheetFixtureKB = filepath.Join("..", "sheet", "testdata", "adjagent-sheet", "kb-root")

// Fixture rows per copy: what Derive yields for the sheet fixture as committed.
const sheetFixtureNodes, sheetFixtureEdges = 14, 17

// sheetFixtureReplica is the sheet fixture's files copied n times into one
// in-memory source under an empty kb-root. Copy 0 is the fixture's bytes as
// committed; copy k > 0 suffixes every top-level directory with k and renames
// every id it carries within that id's grammar, its node and edge records
// appended to the one pair of record files.
func sheetFixtureReplica(b *testing.B, n int) *kb.Source {
	b.Helper()
	committed := map[string]string{}
	err := filepath.WalkDir(sheetFixtureKB, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(sheetFixtureKB, p)
		if err != nil {
			return err
		}
		committed[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		b.Fatal(err)
	}
	root := filepath.Join(b.TempDir(), kb.KBDir)
	if err := os.Mkdir(root, 0o755); err != nil {
		b.Fatal(err)
	}
	files := map[string][]byte{}
	put := func(rel, text string) { files[kb.KBDir+"/"+rel] = append(files[kb.KBDir+"/"+rel], text...) }
	for rel, text := range committed {
		put(rel, text)
	}
	for k := 1; k < n; k++ {
		rename := strings.NewReplacer(
			"-aaa", fmt.Sprintf("-a%02d", k),
			"-bbb", fmt.Sprintf("-b%02d", k),
			"wrk-smith2020", fmt.Sprintf("wrk-smith2020c%02d", k),
			"axm-1", fmt.Sprintf("axm-1c%02d", k),
			"axm-S1", fmt.Sprintf("axm-S1c%02d", k),
		)
		for rel, text := range committed {
			dir, rest, nested := strings.Cut(rel, "/")
			switch {
			case rel == kb.GraphNodesFile || rel == kb.GraphEdgesFile:
				put(rel, rename.Replace(text))
			case nested:
				put(fmt.Sprintf("%s-%02d/%s", dir, k, rest), rename.Replace(text))
			}
		}
	}
	return kb.MigratedSource(root, kb.FormatVersion, files, nil)
}

// BenchmarkLoadDerive times Derive — the record files' strict load, the
// documents' marker scan, the strength walk and the rows — over the sheet
// fixture as committed and replicated ×40 in memory.
func BenchmarkLoadDerive(b *testing.B) {
	for _, n := range []int{1, 40} {
		b.Run(fmt.Sprintf("x%d", n), func(b *testing.B) {
			src := sheetFixtureReplica(b, n)
			rows, err := Derive(src)
			if err != nil {
				b.Fatal(err)
			}
			if len(rows.Nodes) != n*sheetFixtureNodes || len(rows.Edges) != n*sheetFixtureEdges {
				b.Fatalf("the replica derives %d nodes and %d edges, want %d and %d",
					len(rows.Nodes), len(rows.Edges), n*sheetFixtureNodes, n*sheetFixtureEdges)
			}
			b.ReportAllocs()
			for b.Loop() {
				if _, err := Derive(src); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
