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
const sheetFixtureNodes, sheetFixtureEdges = 14, 18

// sheetFixtureReplica is the sheet fixture's documents copied n times into one
// in-memory source under an empty kb-root. Copy 0 is the fixture's bytes as
// committed; copy k > 0 suffixes every top-level directory with k and renames
// every id it declares within that id's grammar, its axiom and work
// declarations appended to the one root axioms.md and claim-rigor.md. The
// fixture's .index/ is left out: it is what Derive yields, not what it reads.
func sheetFixtureReplica(b *testing.B, n int) *kb.Source {
	b.Helper()
	committed := map[string]string{}
	err := filepath.WalkDir(sheetFixtureKB, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == kb.IndexDir {
				return fs.SkipDir
			}
			return nil
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
			"Axiom 1", fmt.Sprintf("Axiom 1c%02d", k),
			"Axiom S1", fmt.Sprintf("Axiom S1c%02d", k),
		)
		for rel, text := range committed {
			dir, rest, nested := strings.Cut(rel, "/")
			switch {
			case nested:
				put(fmt.Sprintf("%s-%02d/%s", dir, k, rest), rename.Replace(text))
			case rel == kb.AxiomsFile || rel == kb.RegisterFile:
				_, body, _ := strings.Cut(text, "\n")
				put(rel, rename.Replace(body))
			}
		}
	}
	return kb.MigratedSource(root, kb.FormatVersion, files, nil)
}

// BenchmarkLoadDerive times Derive — the documents' parse, the strength walk
// and the five row sets — over the sheet fixture as committed and replicated
// ×40 in memory.
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
