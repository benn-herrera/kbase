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
		File         string  `yaml:"file"`
		Origin       *string `yaml:"origin"`
		OriginCommit *string `yaml:"origin-commit"`
		Authorship   string  `yaml:"authorship"`
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
// and nothing else, each with an authorship of imported, kbase-authored or
// kbase-origin. An imported or kbase-authored entry names its origin and
// origin-commit; a kbase-origin entry names neither — kbase authored it de novo.
func TestProvenanceCoversTheShelf(t *testing.T) {
	var listed []string
	for _, f := range readProvenance(t).Files {
		if f.File == "" {
			t.Errorf("an entry lacks its file: %+v", f)
		}
		switch f.Authorship {
		case "imported", "kbase-authored":
			if f.Origin == nil || *f.Origin == "" {
				t.Errorf("%s: authorship %q without an origin", f.File, f.Authorship)
			}
			if f.OriginCommit == nil || len(*f.OriginCommit) != 40 {
				t.Errorf("%s: authorship %q without a full origin-commit", f.File, f.Authorship)
			}
		case "kbase-origin":
			if f.Origin != nil || f.OriginCommit != nil {
				t.Errorf("%s: kbase-origin with an origin or origin-commit", f.File)
			}
		default:
			t.Errorf("%s: authorship %q, want imported, kbase-authored or kbase-origin", f.File, f.Authorship)
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

// TestOriginExistsAtTheRecordedCommitAndImportsMatchIt reads each entry's
// origin file from the adjagent clone at the commit it records: it must
// exist, and an imported file's embedded bytes must equal it. It skips where
// the clone is absent.
func TestOriginExistsAtTheRecordedCommitAndImportsMatchIt(t *testing.T) {
	clone := filepath.Join("..", "..", ".claude", "adjagent")
	if _, err := os.Stat(filepath.Join(clone, ".git")); err != nil {
		t.Skipf("no adjagent clone at %s: %v", clone, err)
	}
	repo, err := ledger.Open(clone, nil, log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range readProvenance(t).Files {
		if f.Authorship == "kbase-origin" {
			continue
		}
		origin, err := repo.Files(*f.OriginCommit, *f.Origin)
		if err != nil {
			t.Errorf("%s at %s: %v", *f.Origin, *f.OriginCommit, err)
			continue
		}
		if len(origin) != 1 {
			t.Errorf("%s at %s names %d files, want one", *f.Origin, *f.OriginCommit, len(origin))
			continue
		}
		if f.Authorship != "imported" {
			continue
		}
		var want []byte
		for _, b := range origin {
			want = b
		}
		got, err := shelf.ReadFile(shelfDir + "/" + f.File)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs from %s at %s", f.File, *f.Origin, *f.OriginCommit)
		}
	}
}
