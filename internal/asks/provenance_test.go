package asks

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/ledger"
	"kbase/internal/log"
)

type provenance struct {
	Files []struct {
		File       string `yaml:"file"`
		Upstream   string `yaml:"upstream"`
		Commit     string `yaml:"commit"`
		Authorship string `yaml:"authorship"`
	} `yaml:"files"`
}

func readProvenance(t *testing.T) provenance {
	t.Helper()
	b, err := os.ReadFile("provenance.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var p provenance
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&p); err != nil {
		t.Fatalf("provenance.yaml: %v", err)
	}
	return p
}

// TestProvenanceCoversTheShelf: the manifest names every embedded file once,
// and nothing else, each with an authorship of imported or kbase-authored.
func TestProvenanceCoversTheShelf(t *testing.T) {
	var listed []string
	for _, f := range readProvenance(t).Files {
		if f.File == "" || f.Upstream == "" || len(f.Commit) != 40 {
			t.Errorf("an entry lacks its file, upstream path or full commit: %+v", f)
		}
		if f.Authorship != "imported" && f.Authorship != "kbase-authored" {
			t.Errorf("%s: authorship %q, want imported or kbase-authored", f.File, f.Authorship)
		}
		listed = append(listed, f.File)
	}
	slices.Sort(listed)
	shelved, err := shelfFiles()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(listed, shelved) {
		t.Errorf("provenance.yaml lists %q; the shelf holds %q", listed, shelved)
	}
}

// TestUpstreamExistsAtTheRecordedCommitAndImportsMatchIt reads each entry's
// upstream file from the adjagent clone at the commit it records: it must
// exist, and an imported file's embedded bytes must equal it. It skips where
// the clone is absent.
func TestUpstreamExistsAtTheRecordedCommitAndImportsMatchIt(t *testing.T) {
	clone := filepath.Join("..", "..", ".claude", "adjagent")
	if _, err := os.Stat(filepath.Join(clone, ".git")); err != nil {
		t.Skipf("no adjagent clone at %s: %v", clone, err)
	}
	repo, err := ledger.Open(clone, nil, log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range readProvenance(t).Files {
		upstream, err := repo.Files(f.Commit, f.Upstream)
		if err != nil {
			t.Errorf("%s at %s: %v", f.Upstream, f.Commit, err)
			continue
		}
		if len(upstream) != 1 {
			t.Errorf("%s at %s names %d files, want one", f.Upstream, f.Commit, len(upstream))
			continue
		}
		if f.Authorship != "imported" {
			continue
		}
		var want []byte
		for _, b := range upstream {
			want = b
		}
		got, err := shelf.ReadFile(shelfDir + "/" + f.File)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs from %s at %s", f.File, f.Upstream, f.Commit)
		}
	}
}
