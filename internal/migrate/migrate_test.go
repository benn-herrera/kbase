package migrate

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

const (
	// synthetic090Dir is a 0.9.0 KB laid out repository-relative: the results
	// fixture's documents as they stood at 0.9.0, a leaf declaring nodes, the
	// index and the build records.
	synthetic090Dir = "testdata/0.9.0"
	golden100Dir    = "testdata/1.0.0"
	// gotDir receives a golden mismatch's output, laid out as the goldens are.
	gotDir = "../../test_data/transient/unit_tests/migrate/got"
)

// readTree is every file under dir by slash path relative to dir, prefixed.
func readTree(t *testing.T, dir, prefix string, into Files) {
	t.Helper()
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		into[path.Join(prefix, filepath.ToSlash(rel))] = b
		return err
	})
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
}

func fixture090(t *testing.T) Files {
	t.Helper()
	in := Files{}
	readTree(t, synthetic090Dir, "", in)
	return in
}

// TestConvert090To100Goldens: the results fixture's documents and the
// synthetic records and index, converted, equal the 1.0.0 goldens file for
// file and byte for byte.
func TestConvert090To100Goldens(t *testing.T) {
	got, _, err := Chain(version090, version100, fixture090(t))
	if err != nil {
		t.Fatalf("Chain: %v", err)
	}
	want := Files{}
	if _, err := os.Stat(golden100Dir); err == nil {
		readTree(t, golden100Dir, "", want)
	}
	paths := slices.Sorted(maps.Keys(got))
	if wantPaths := slices.Sorted(maps.Keys(want)); !slices.Equal(paths, wantPaths) {
		t.Errorf("converted paths %q, goldens %q", paths, wantPaths)
	}
	for _, p := range paths {
		if w, ok := want[p]; ok && !bytes.Equal(got[p], w) {
			t.Errorf("%s differs from its golden", p)
		}
	}
	if t.Failed() {
		for _, p := range paths {
			dst := filepath.Join(gotDir, filepath.FromSlash(p))
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(dst, got[p], 0o644); err != nil {
				t.Fatal(err)
			}
		}
		t.Logf("the converted files are under %s", gotDir)
	}
}

// TestConvert090To100Obsolete: the obsolete set is exactly the .json and
// .jsonl paths given, none of which the output still holds.
func TestConvert090To100Obsolete(t *testing.T) {
	in := fixture090(t)
	out, obsolete, err := Chain(version090, version100, in)
	if err != nil {
		t.Fatalf("Chain: %v", err)
	}
	var want []string
	for p := range in {
		if ext := path.Ext(p); ext == ".json" || ext == ".jsonl" {
			want = append(want, p)
		}
	}
	slices.Sort(want)
	if len(want) != 9 {
		t.Fatalf("the fixture holds %d .json/.jsonl paths, want the 3 records and 6 index files", len(want))
	}
	if !slices.Equal(obsolete, want) {
		t.Errorf("obsolete %q, want %q", obsolete, want)
	}
	for _, p := range obsolete {
		if _, ok := out[p]; ok {
			t.Errorf("the output still holds obsolete %s", p)
		}
	}
}

// pyLineBreaks is every character Python's str.splitlines breaks on; the last
// two are U+2028 and U+2029.
const pyLineBreaks = "\n\r\v\f\x1c\x1d\x1e\xc2\x85\xe2\x80\xa8\xe2\x80\xa9"

// TestIndexRecordIsOnePhysicalLine: a record holding multi-line strings and
// a very long one is one "--- {…}" line, breaking on nothing a line reader
// breaks on, and reads back to the same strings.
func TestIndexRecordIsOnePhysicalLine(t *testing.T) {
	long := strings.Repeat(`a long title: with "quotes", [brackets], {braces} # and hashes `, 400)
	records := []map[string]string{
		{"title": "first line\nsecond line\r\nthird\rfourth"},
		{"title": "ls\xe2\x80\xa8ls ps\xe2\x80\xa9ps nel\xc2\x85nel fs\x1cfs vt\vvt ff\fff"},
		{"title": long},
		{"title": "- a leading dash", "context": "  indented\n\tand tabbed  "},
		{"title": "c1 \xc2\x80 del \x7f bom \xef\xbb\xbf fffe \xef\xbf\xbe ffff \xef\xbf\xbf <html> & more"},
	}
	var jsonl bytes.Buffer
	for _, r := range records {
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		jsonl.Write(b)
		jsonl.WriteByte('\n')
	}
	p := indexDir + "/claims.jsonl"
	out, _, err := Chain(version090, version100, Files{p: jsonl.Bytes()})
	if err != nil {
		t.Fatalf("Chain: %v", err)
	}
	stream := string(out[indexDir+"/claims.yaml"])
	lines := strings.SplitAfter(stream, "\n")
	if last := lines[len(lines)-1]; last != "" {
		t.Fatalf("stream does not end in a line break: %q", last)
	}
	lines = lines[:len(lines)-1]
	if len(lines) != len(records) {
		t.Fatalf("%d records emitted as %d lines:\n%s", len(records), len(lines), stream)
	}
	for i, line := range lines {
		body := strings.TrimSuffix(line, "\n")
		if !strings.HasPrefix(body, "--- {") || !strings.HasSuffix(body, "}") {
			t.Errorf("line %d is not a flow document: %q", i+1, body)
		}
		if strings.ContainsAny(body, pyLineBreaks) {
			t.Errorf("line %d holds a line break character: %q", i+1, body)
		}
		var got map[string]string
		if err := json.Unmarshal([]byte(strings.TrimPrefix(body, "--- ")), &got); err != nil {
			t.Errorf("line %d past its marker is not JSON: %v", i+1, err)
		} else if !maps.Equal(got, records[i]) {
			t.Errorf("line %d reads as JSON %q, want %q", i+1, got, records[i])
		}
	}
	dec := yaml.NewDecoder(strings.NewReader(stream))
	for i, want := range records {
		var got map[string]string
		if err := dec.Decode(&got); err != nil {
			t.Fatalf("record %d does not read back: %v", i+1, err)
		}
		if !maps.Equal(got, want) {
			t.Errorf("record %d reads back as %q, want %q", i+1, got, want)
		}
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		t.Errorf("stream holds more than %d documents: %v", len(records), err)
	}
}

// TestDocumentYAMLFrontmatter: each comment-block form kb_tools writes
// becomes the same keys in the same order at the top of the document, the
// block and its line break gone and every other byte kept.
func TestDocumentYAMLFrontmatter(t *testing.T) {
	cases := []struct {
		name, in, want string
		stamp          bool
	}{{
		name: "no block",
		in:   "# Title\n\ntext\n",
		want: "# Title\n\ntext\n",
	}, {
		name: "scalars and an inline list",
		in:   "<!-- kb-frontmatter\nkind: leaf\npath-stable: \"Part I: the setup\"\nclaims: [clm-aaaaaa, clm-bbbbbb]\nexperiments: []\nflag: true\nempty:\n-->\n\n# T\n",
		want: "---\nkind: leaf\npath-stable: \"Part I: the setup\"\nclaims: [clm-aaaaaa, clm-bbbbbb]\nexperiments: []\nflag: true\nempty: \"\"\n---\n\n# T\n",
	}, {
		name: "a wrapped list and a bullet list",
		in:   "<!-- kb-frontmatter\nkind: index\nsubtree-claims: [clm-aaaaaa,\n  clm-bbbbbb]\nsubtree-experiments:\n  - exp-cccccc\n  - exp-dddddd\n-->\n",
		want: "---\nkind: index\nsubtree-claims: [clm-aaaaaa, clm-bbbbbb]\nsubtree-experiments:\n  - exp-cccccc\n  - exp-dddddd\n---\n",
	}, {
		name: "an experiment and its scored pairs, a strength the 0.9.0 reader could not read ending the list",
		in:   "<!-- kb-frontmatter\nkind: leaf\nexp-id: exp-cccccc\nstatus: run\nstrengthens:\n  - clm-aaaaaa: 0.8\n  - clm-bbbbbb: *pending*\n  - clm-dddddd: 0.5\n-->\n",
		want: "---\nkind: leaf\nexperiment-nodes:\n  - exp-id: exp-cccccc\n    status: run\n    strengthens:\n      - clm-aaaaaa: 0.8\n---\n",
	}, {
		name: "two experiments and two supports, a key between",
		in:   "<!-- kb-frontmatter\nkind: leaf\nexp-id: exp-cccccc\nstatus: run\nsup-id: sup-eeeeee\nsupports:\n  - clm-aaaaaa: 0.5\nno-claim: \"hosts nodes\"\nexp-id: exp-dddddd\nstrengthens:\n  - clm-bbbbbb: 1.0\nstatus: pending\nsup-id: sup-ffffff\n-->\n",
		want: "---\nkind: leaf\nexperiment-nodes:\n  - exp-id: exp-cccccc\n    status: run\n  - exp-id: exp-dddddd\n    strengthens:\n      - clm-bbbbbb: 1.0\n    status: pending\nsupport-nodes:\n  - sup-id: sup-eeeeee\n    supports:\n      - clm-aaaaaa: 0.5\n  - sup-id: sup-ffffff\nno-claim: \"hosts nodes\"\n---\n",
	}, {
		name: "an experiments reference list beside a declaration",
		in:   "<!-- kb-frontmatter\nkind: leaf\nexperiments: [exp-dddddd]\nexp-id: exp-cccccc\nstatus: run\n-->\n",
		want: "---\nkind: leaf\nexperiments: [exp-dddddd]\nexperiment-nodes:\n  - exp-id: exp-cccccc\n    status: run\n---\n",
	}, {
		name: "a block below the up-link",
		in:   "[↑ KB](entry-point.md)\n\n<!-- kb-frontmatter\nkind: leaf\nno-claim: \"prose\"\n-->\n\n# A\n",
		want: "---\nkind: leaf\nno-claim: prose\n---\n[↑ KB](entry-point.md)\n\n\n# A\n",
	}, {
		name: "declarations and members written with a leading dash",
		in:   "<!-- kb-frontmatter\nkind: leaf\n- exp-id: exp-cccccc\n- status: run\n- sup-id: sup-eeeeee\n-->\n",
		want: "---\nkind: leaf\nexperiment-nodes:\n  - exp-id: exp-cccccc\n    status: run\nsupport-nodes:\n  - sup-id: sup-eeeeee\n---\n",
	}, {
		name: "pairs without a bullet",
		in:   "<!-- kb-frontmatter\nkind: leaf\nexp-id: exp-cccccc\nstatus: run\nstrengthens:\nclm-aaaaaa: 0.8\nclm-bbbbbb: 0.5\n-->\n",
		want: "---\nkind: leaf\nexperiment-nodes:\n  - exp-id: exp-cccccc\n    status: run\n    strengthens:\n      - clm-aaaaaa: 0.8\n      - clm-bbbbbb: 0.5\n---\n",
	}, {
		name: "a member after another key attaches to its kind's latest node",
		in:   "<!-- kb-frontmatter\nexp-id: exp-cccccc\nkind: leaf\nstatus: run\nexp-id: exp-dddddd\nclaims: [clm-aaaaaa]\nstatus: pending\n-->\n",
		want: "---\nexperiment-nodes:\n  - exp-id: exp-cccccc\n    status: run\n  - exp-id: exp-dddddd\n    status: pending\nkind: leaf\nclaims: [clm-aaaaaa]\n---\n",
	}, {
		name: "a declaration inside a pair list opens a node",
		in:   "<!-- kb-frontmatter\nkind: leaf\nexp-id: exp-cccccc\nstatus: run\nstrengthens:\n  - clm-aaaaaa: 0.8\n  - exp-id: exp-dddddd\nstatus: pending\n-->\n",
		want: "---\nkind: leaf\nexperiment-nodes:\n  - exp-id: exp-cccccc\n    status: run\n    strengthens:\n      - clm-aaaaaa: 0.8\n  - exp-id: exp-dddddd\n    status: pending\n---\n",
	}, {
		name: "a score in exponent form stays a number",
		in:   "<!-- kb-frontmatter\nkind: leaf\nsup-id: sup-eeeeee\nsupports:\n  - clm-aaaaaa: 1e-05\n-->\n",
		want: "---\nkind: leaf\nsupport-nodes:\n  - sup-id: sup-eeeeee\n    supports:\n      - clm-aaaaaa: 1e-05\n---\n",
	}, {
		name: "a member with no node of its kind stays a key",
		in:   "<!-- kb-frontmatter\nkind: leaf\nstatus: run\n-->\n",
		want: "---\nkind: leaf\nstatus: run\n---\n",
	}, {
		name: "a document already in YAML frontmatter",
		in:   "---\nkind: leaf\n---\n[↑ KB](entry-point.md)\n",
		want: "---\nkind: leaf\n---\n[↑ KB](entry-point.md)\n",
	}, {
		name: "a document opening with a prose block between rules converts, the block left as prose",
		in:   "---\nSome notes: with: colons\n---\n<!-- kb-frontmatter\nkind: leaf\n-->\n# T\n",
		want: "---\nkind: leaf\n---\n---\nSome notes: with: colons\n---\n# T\n",
	}, {
		name:  "an entry point already in YAML frontmatter stamped in place",
		in:    "---\nkb-format: \"0.9.0\"\nkind: entry-point\n---\n\n# KB\n",
		want:  "---\nkind: entry-point\nkb-format: \"1.0.0\"\n---\n\n# KB\n",
		stamp: true,
	}, {
		name:  "the entry point stamped last",
		in:    "<!-- kb-frontmatter\nkb-format: 0.9.0\nkind: entry-point\n-->\n\n# KB\n",
		want:  "---\nkind: entry-point\nkb-format: \"1.0.0\"\n---\n\n# KB\n",
		stamp: true,
	}, {
		name:  "an entry point with no block stamped",
		in:    "# KB\n",
		want:  "---\nkb-format: \"1.0.0\"\n---\n# KB\n",
		stamp: true,
	}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := documentYAMLFrontmatter([]byte(c.in), c.stamp)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != c.want {
				t.Errorf("got\n%s\nwant\n%s", got, c.want)
			}
		})
	}
}

// TestDocumentYAMLFrontmatterRefusesARepeatedKey: a key repeated outside a
// node declaration group has no YAML mapping form.
func TestDocumentYAMLFrontmatterRefusesARepeatedKey(t *testing.T) {
	in := "<!-- kb-frontmatter\nkind: leaf\nclaims: [clm-aaaaaa]\nclaims: [clm-bbbbbb]\n-->\n"
	if _, err := documentYAMLFrontmatter([]byte(in), false); err == nil || !strings.Contains(err.Error(), `"claims" repeats`) {
		t.Errorf("err = %v, want the repeated claims key named", err)
	}
}

// TestSupersededStamp: a comment block's kb-format is read, quotes stripped;
// a block without one, and a document without a block, declare none.
func TestSupersededStamp(t *testing.T) {
	for _, c := range []struct {
		in, want string
		ok       bool
	}{
		{"<!-- kb-frontmatter\nkind: entry-point\nkb-format: \"0.9.2\"\n-->\n# KB\n", "0.9.2", true},
		{"<!-- kb-frontmatter\nkb-format: 2.0.0\n-->\n", "2.0.0", true},
		{"<!-- kb-frontmatter\nkind: entry-point\n-->\n", "", false},
		{"# KB\n", "", false},
	} {
		if got, ok := SupersededStamp([]byte(c.in)); got != c.want || ok != c.ok {
			t.Errorf("SupersededStamp(%q) = %q, %t; want %q, %t", c.in, got, ok, c.want, c.ok)
		}
	}
}

// TestIndexLineMustBeAnObject: a JSONL line holding a value other than an
// object is refused, naming the line.
func TestIndexLineMustBeAnObject(t *testing.T) {
	_, _, err := Chain(version090, version100, Files{indexDir + "/claims.jsonl": []byte("{\"id\": \"a\"}\n[1, 2]\n")})
	if err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Errorf("err = %v, want line 2 named", err)
	}
}

// TestChainRefusesAGap: with converters to 1.1.0 and none beyond, a chain to
// 1.2.0 names the version it cannot leave.
func TestChainRefusesAGap(t *testing.T) {
	identity := func(f Files) (Files, []string, error) { return f, nil, nil }
	steps := append(slices.Clone(converters), converter{From: "1.0.0", To: "1.1.0", Convert: identity})
	_, _, err := chain(steps, version090, "1.2.0", Files{})
	if err == nil || err.Error() != "migrate: no converter leads from 1.1.0 toward 1.2.0" {
		t.Errorf("err = %v, want the gap at 1.1.0 named", err)
	}
}
