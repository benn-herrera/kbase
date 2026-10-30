package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/buildrecords"
	"kbase/internal/kb"
	"kbase/internal/kbdocs"
	"kbase/internal/mcp"
)

// The tracked KB at metadata format 1.0.0: the results fixture's documents, a
// leaf declaring nodes, the index and the build records.
const kb100Dir = "../internal/formats/testdata/1.0.0"

// stageRepo copies the repository layout under dir into a fresh repository
// and returns it.
func stageRepo(t *testing.T, dir string) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeValues(t, filepath.Join(repo, ".git"), "HEAD", "ref: refs/heads/main\n")
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		dst := filepath.Join(repo, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

// tree is every file under repo but .git, by slash path.
func tree(t *testing.T, repo string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(repo, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		data, err := os.ReadFile(p)
		rel, _ := filepath.Rel(repo, p)
		out[filepath.ToSlash(rel)] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// queryCases is every query over the fixture's ids.
var queryCases = [][]string{
	{"find", ""}, {"find", "result"}, {"deps", "clm-0dtsyu"}, {"deps", "clm-0dtsyu", "-i"}, {"gated-on", "clm-2h6i01"},
	{"cited-by", "clm-0dtsyu"}, {"referenced-by", "clm-0dtsyu"}, {"strength", "--max", "0.9"}, {"subtree", ""},
	{"subtree", "b"}, {"strength", "--sort", "dependents", "--min-dependents", "1", "--max", "0.65"}, {"strength", "--pending"},
	{"show", "clm-0dtsyu"}, {"show", "exp-k3m9q2"}, {"stats"}, {"edges"},
}

func mapsEqualStrings(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || w != v {
			return false
		}
	}
	return true
}

// TestA100KBRefreshesToTheCurrentFormat: a KB stamped 1.0.0 opens, verify
// reports its index stale with the refresh that migrates it, a refresh writes
// it at the current format and verify then finds nothing a refresh would
// clear.
func TestA100KBRefreshesToTheCurrentFormat(t *testing.T) {
	repo := stageRepo(t, kb100Dir)
	out, code := kbase(t, repo, "verify")
	d := checkDocument(t, "verify", out)
	if code != 1 || d.Outcome != "refused" {
		t.Fatalf("verify over 1.0.0: exit %d\n%s", code, out)
	}
	if m, _ := d.Values["migration"].(map[string]any); m["from"] != "1.0.0" || m["to"] != kb.FormatVersion {
		t.Errorf("verify over 1.0.0 reports migration %v", m)
	}
	if !slices.ContainsFunc(d.items("refusals"), func(it map[string]any) bool { return it["remedy"] == "kbase refresh" }) {
		t.Errorf("verify over 1.0.0 finds no stale index:\n%s", out)
	}
	if out, code := kbase(t, repo, "refresh"); code != 0 {
		t.Fatalf("refresh over 1.0.0: exit %d\n%s", code, out)
	}
	if stamp, err := kb.DecodeFormatStamp([]byte(tree(t, repo)[kb.KBDir+"/"+kb.FormatFile])); err != nil || stamp != kb.FormatVersion {
		t.Errorf("the refresh left the stamp at %q (%v), want %s", stamp, err, kb.FormatVersion)
	}
	out, _ = kbase(t, repo, "verify")
	d = checkDocument(t, "verify", out)
	if _, ok := d.Values["migration"]; ok {
		t.Errorf("a current KB's verify carries a migration record:\n%s", out)
	}
	if slices.ContainsFunc(d.items("refusals"), func(it map[string]any) bool { return it["remedy"] == "kbase refresh" }) {
		t.Errorf("verify after the refresh finds what a refresh clears:\n%s", out)
	}
}

// TestNewerFormatRefusedByEveryReader: a KB stamped at a newer major or minor
// version is refused by every subcommand that reads it, naming both versions
// and the remedy, and nothing is written; a newer patch is read as current.
func TestNewerFormatRefusedByEveryReader(t *testing.T) {
	values := writeValues(t, t.TempDir(), "insert.yaml", "entry:\n- title: A Result\n  rigor: 0.5\n  rationale: Shown.\n")
	readers := append([][]string{{"refresh"}, {"verify"},
		{"insert-claim-entry", "--values", values}, {"insert-claim-entry", "--no-refresh", "--values", values}}, queryCases...)
	for _, stamp := range []string{"4.0.0", "3.1.0"} {
		t.Run(stamp, func(t *testing.T) {
			repo := stageRepo(t, kb100Dir)
			entry := filepath.Join(repo, kb.KBDir, kb.EntryPointFile)
			writeValues(t, filepath.Dir(entry), kb.EntryPointFile, strings.Replace(tree(t, repo)["kb-root/entry-point.md"], `"1.0.0"`, `"`+stamp+`"`, 1))
			before := tree(t, repo)
			for _, args := range readers {
				out, code := kbase(t, repo, args...)
				d := checkDocument(t, args[0], out)
				items := d.items("refusals")
				if code != 1 || d.Outcome != "refused" || len(items) != 1 || items[0]["check"] != "kb-format" || items[0]["remedy"] != "update kbase" ||
					!strings.Contains(items[0]["detail"].(string), stamp) || !strings.Contains(items[0]["detail"].(string), kb.FormatVersion) {
					t.Errorf("%q over a KB at %s: exit %d\n%s", args, stamp, code, out)
				}
			}
			if after := tree(t, repo); !mapsEqualStrings(before, after) {
				t.Error("a refused reader wrote to the KB")
			}
		})
	}
	t.Run("a newer patch", func(t *testing.T) {
		repo := stageRepo(t, kb100Dir)
		if out, code := kbase(t, repo, "refresh"); code != 0 {
			t.Fatalf("refresh over 1.0.0: exit %d\n%s", code, out)
		}
		writeValues(t, filepath.Join(repo, kb.KBDir, kb.IndexDir), "format.yaml", "kb-format: \"3.0.9\"\n")
		if out, code := kbase(t, repo, "find", ""); code != 0 {
			t.Errorf("find over a KB at 3.0.9: exit %d\n%s", code, out)
		}
		out, code := kbase(t, repo, "refresh")
		if d := checkDocument(t, "refresh", out); code != 0 || !slices.Contains(anyStrings(d.Values["written"]), kb.FormatFile) {
			t.Errorf("refresh over a KB at 3.0.9: exit %d\n%s", code, out)
		}
		if stamp, _ := kb.DecodeFormatStamp([]byte(tree(t, repo)[kb.KBDir+"/"+kb.FormatFile])); stamp != kb.FormatVersion {
			t.Errorf("the save left the stamp at %q, want it rewritten to %s", stamp, kb.FormatVersion)
		}
	})
}

// TestARefusedConversionStampsNothing: a 1.0.0 KB whose index file is a
// symbolic link is refused, naming it, and the entry point keeps its stamp.
func TestARefusedConversionStampsNothing(t *testing.T) {
	repo := stageRepo(t, kb100Dir)
	outside := filepath.Join(t.TempDir(), "claims.yaml")
	if err := os.WriteFile(outside, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(repo, kb.KBDir, kb.IndexDir, "claims.yaml")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}

	out, code := kbase(t, repo, "refresh")
	d := checkDocument(t, "refresh", out)
	items := d.items("refusals")
	if code != 1 || d.Outcome != "refused" || len(items) != 1 || items[0]["check"] != kb.CheckSymlink || items[0]["path"] != link {
		t.Fatalf("refresh over a 1.0.0 KB with a symlinked index file: exit %d\n%s", code, out)
	}
	if stamp, _ := kb.FormatStamp(tree(t, repo)["kb-root/entry-point.md"]); stamp != "1.0.0" {
		t.Errorf("the refused conversion left the entry point stamped %q, want it unstamped at 1.0.0", stamp)
	}
	if data, err := os.ReadFile(outside); err != nil || string(data) != "{}\n" {
		t.Errorf("%s = %q (%v), want it untouched", outside, data, err)
	}
}

// kbaseStreams is kbase, also returning the stderr a person sees.
func kbaseStreams(t *testing.T, dir string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	t.Chdir(dir)
	out, errOut, err := executeRoot(t, args...)
	code = exitStatus(err, out, &bytes.Buffer{})
	if sub, _, ferr := rootCmd.Find(args); ferr == nil {
		if err := mcp.ResetFlags(sub); err != nil {
			t.Fatal(err)
		}
	}
	return out.String(), errOut.String(), code
}

// TestNoRefreshRefusedOnAnOlderKB: a write op skipping its refresh over a KB
// at 1.0.0 is refused, naming the refresh that migrates it, and writes
// nothing; with its refresh it lands and migrates the KB. Each call says once
// what it converted.
func TestNoRefreshRefusedOnAnOlderKB(t *testing.T) {
	repo := stageRepo(t, kb100Dir)
	values := writeValues(t, t.TempDir(), "insert.yaml", "entry:\n- title: A Result\n  rigor: 0.5\n  rationale: Shown.\n")
	before := tree(t, repo)
	out, stderr, code := kbaseStreams(t, repo, "insert-claim-entry", "--no-refresh", "--values", values)
	d := checkDocument(t, "insert-claim-entry", out)
	items := d.items("refusals")
	if code != 1 || len(items) != 1 || items[0]["check"] != "index" || items[0]["remedy"] != "kbase refresh" {
		t.Errorf("--no-refresh over 1.0.0: exit %d\n%s", code, out)
	}
	if m, _ := d.Values["migration"].(map[string]any); m["from"] != "1.0.0" || m["to"] != kb.FormatVersion {
		t.Errorf("the refused op's migration record is %v", m)
	}
	if n := strings.Count(stderr, "converted to "+kb.FormatVersion); n != 1 {
		t.Errorf("the sentence appears %d times on stderr, want once:\n%s", n, stderr)
	}
	if after := tree(t, repo); !mapsEqualStrings(before, after) {
		t.Error("the refused write op wrote to the KB")
	}
	out, stderr, code = kbaseStreams(t, repo, "insert-claim-entry", "--values", values)
	d = checkDocument(t, "insert-claim-entry", out)
	if code != 0 || d.Outcome != "done" || !slices.Contains(anyStrings(d.Values["removed"]), path.Join(kb.IndexDir, "depends-on.yaml")) {
		t.Errorf("the write op with its refresh over 1.0.0: exit %d\n%s", code, out)
	}
	if m, _ := d.Values["migration"].(map[string]any); m["from"] != "1.0.0" || m["to"] != kb.FormatVersion {
		t.Errorf("the saving op's migration record is %v", m)
	}
	if n := strings.Count(stderr, "converted to "+kb.FormatVersion); n != 1 {
		t.Errorf("the sentence appears %d times on stderr, want once:\n%s", n, stderr)
	}
	if stamp, _ := kb.DecodeFormatStamp([]byte(tree(t, repo)[kb.KBDir+"/"+kb.FormatFile])); stamp != kb.FormatVersion {
		t.Errorf("the write op's refresh left the stamp at %q", stamp)
	}
}

// registerEntry is one 2.0.x register entry under heading, its Rigor section
// holding fields.
func registerEntry(id, heading string, fields ...string) string {
	return "## " + heading + "\n<!-- id: " + id + " -->\n\n> **Leaf references:** *(none)*\n\n### Rigor\n" +
		strings.Join(fields, "\n") + "\n\n### Strength\n- strength: 0.5\n\n---\n\n"
}

// warnedKB200 is a 2.0.0 repository whose conversion warns of one thing of
// every kind the conversion drops or changes, and whose converted records
// hold a cycle that refresh's repair cuts.
func warnedKB200(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	marker := kb.StatementMarker
	writeRepo(t, repo, map[string]string{
		".git/HEAD":                  "ref: refs/heads/main\n",
		"kb-root/entry-point.md":     "---\nkind: entry-point\nkb-format: \"2.0.0\"\n---\n# KB\n\n- [A Lemma](a.md)\n",
		"kb-root/axioms.md":          "### Axiom S3: A labelling convention\n\nLabels are written as authored.\n",
		"kb-root/vol/index.md":       "---\nkind: index\nno-claim: true\n---\n# Volume\n",
		"kb-root/vol/claim-rigor.md": "# Volume\n\n" + registerEntry("clm-aaaaaa", "Claim A Again", "- rigor: 0.2"),
		"kb-root/claim-rigor.md": "# Claim Rigor\n\n---\n\n" +
			registerEntry("clm-aaaaaa", "Claim A", "- rigor: 0.5", "- quality: 0.4", "- rests-on:",
				"  - clm-bbbbbb — B (bearing 0.5) [low]", "  - clm-bbbbbb — B (bearing 1.0) [high]", "  - clm-zzzzzz — Nowhere (bearing 1.0)") +
			registerEntry("clm-bbbbbb", "Claim B", "- rigor: 0.9", "- rests-on:", "  - clm-aaaaaa — A (bearing 0.5)") +
			registerEntry("clm-cccccc", "Claim C", "- rigor: 0.7") +
			registerEntry("ana-aaaaaa", "An Old Name", "- rigor: 0.8") +
			registerEntry("ana-bbbbbb", "A Stray Lemma", "- rigor: 0.8"),
		"kb-root/a.md": "---\nkind: leaf\nclaims: [clm-aaaaaa, clm-yyyyyy]\nexperiment-nodes:\n  - exp-id: exp-aaaaaa\n    status: run\n" +
			"analysis-nodes:\n  - ana-id: ana-aaaaaa\n---\n# A Lemma\n\nText. " + marker("clm-xxxxxx") + "\n",
		"kb-root/b.md": "---\nkind: leaf\nclaims: [clm-aaaaaa]\nexperiment-nodes:\n  - exp-id: exp-aaaaaa\n    status: pending\n---\n# B\n\nSee clm-aaaaaa.\n",
		"kb-root/c.md": "---\nkind: leaf\nclaims: [clm-bbbbbb]\n---\nClaim B holds.\n",
		"kb-root/d.md": "---\nkind: leaf\nexperiments: [exp-aaaaaa]\n---\n# D\n\nAgain. " + marker("clm-aaaaaa") + "\n",
		"kb-root/e.md": "---\nkind: leaf\nexperiment-nodes:\n  - exp-id: exp-bbbbbb\n    status: run\n---\nNo heading.\n",
	})
	return repo
}

// TestEveryConversionWarningReachesTheCaller: each kind of thing a 2.0.x
// conversion drops or changes is one item of the migration record's warnings
// and one conversion-warning line on stderr, and the saving refresh reports
// the cycle its repair cut under its own warnings.
func TestEveryConversionWarningReachesTheCaller(t *testing.T) {
	repo := warnedKB200(t)
	out, stderr, code := kbaseStreams(t, repo, "refresh")
	d := checkDocument(t, "refresh", out)
	if code != 0 || d.Outcome != "done" {
		t.Fatalf("refresh over the 2.0.0 KB: exit %d\n%s", code, out)
	}
	m, _ := d.Values["migration"].(map[string]any)
	warnings, _ := m["warnings"].([]any)
	var details []string
	for _, w := range warnings {
		item, _ := w.(map[string]any)
		details = append(details, item["detail"].(string))
	}
	for _, want := range []string{
		"clm-zzzzzz — Nowhere (bearing 1.0)",
		`register text dropped: "- quality: 0.4"`,
		"supports clm-bbbbbb → clm-aaaaaa is authored with bearing 0.5",
		"a second register entry for clm-aaaaaa is dropped",
		`frontmatter key "experiments" dropped`,
		`frontmatter key "no-claim" holds a value of the wrong type`,
		"a second declaration of exp-aaaaaa is dropped",
		"analysis ana-bbbbbb is hosted by no leaf",
		"claims lists clm-yyyyyy, which names no claim",
		"claims lists clm-aaaaaa, whose marker is in kb-root/a.md",
		"clm-aaaaaa's marker is appended to the first heading",
		"clm-bbbbbb gets no marker: kb-root/c.md has no heading",
		`exp-bbbbbb's host has no heading, so it is titled "e"`,
		"exp-bbbbbb gets no marker: kb-root/e.md has no heading to place it after",
		"a marker for clm-xxxxxx, which names no node, is removed",
		"a marker for clm-aaaaaa is removed; its marker is in kb-root/a.md",
	} {
		if !slices.ContainsFunc(details, func(s string) bool { return strings.Contains(s, want) }) {
			t.Errorf("the migration record warns of nothing holding %q:\n%s", want, strings.Join(details, "\n"))
		}
		if !slices.ContainsFunc(strings.Split(stderr, "\n"), func(line string) bool {
			return strings.Contains(line, "conversion warning:") && strings.Contains(line, want)
		}) {
			t.Errorf("stderr has no conversion warning holding %q:\n%s", want, stderr)
		}
	}
	if len(d.items("warnings")) == 0 {
		t.Errorf("the refresh reports no warning of the cycle its repair cut:\n%s", out)
	}
}

// TestAMigrationSaveRestampsTheStampedDocuments: a save of a converted KB,
// through refresh or through a write op's refresh, leaves AGENTS.md and
// CONVENTIONS.md as a fresh stamp writes them — named for the repository, with
// the authored-KB scope pin where init made the KB, which no build record
// beside it shows — whatever they held: the old AGENTS.md's text decides
// nothing.
func TestAMigrationSaveRestampsTheStampedDocuments(t *testing.T) {
	values := writeValues(t, t.TempDir(), "insert.yaml", "entry:\n- title: A Result\n  rigor: 0.5\n  rationale: Shown.\n")
	for _, tc := range []struct {
		name   string
		args   []string
		built  bool
		agents string
		pin    string
	}{
		{"refresh, a KB init made", []string{"refresh"}, false, "# Old\n\nA corpus.\n", kbdocs.AuthoredKBPin},
		{"a write op, a KB a build made", []string{"insert-claim-entry", "--values", values}, true, "# Old\n\n" + kbdocs.AuthoredKBPin + "\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := stageRepo(t, kb100Dir)
			if !tc.built {
				for _, rel := range buildrecords.RecordFiles {
					if err := os.Remove(filepath.Join(repo, rel)); err != nil && !errors.Is(err, fs.ErrNotExist) {
						t.Fatal(err)
					}
				}
			}
			writeRepo(t, repo, map[string]string{"kb-root/AGENTS.md": tc.agents, "kb-root/CONVENTIONS.md": "# Old conventions\n"})
			if out, code := kbase(t, repo, tc.args...); code != 0 {
				t.Fatalf("%q over 1.0.0: exit %d\n%s", tc.args, code, out)
			}
			fresh := filepath.Join(t.TempDir(), kb.KBDir)
			if err := os.MkdirAll(fresh, 0o755); err != nil {
				t.Fatal(err)
			}
			if _, _, err := kbdocs.Stamp(kb.OnDisk(fresh), filepath.Base(repo), tc.pin); err != nil {
				t.Fatal(err)
			}
			got := tree(t, repo)
			for _, name := range kb.StampedFiles {
				want, err := os.ReadFile(filepath.Join(fresh, name))
				if err != nil {
					t.Fatal(err)
				}
				if got["kb-root/"+name] != string(want) {
					t.Errorf("%s after the save =\n%s\nwant a fresh stamp\n%s", name, got["kb-root/"+name], want)
				}
			}
		})
	}
}
