package main

import (
	"bytes"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
)

// The tracked KB at metadata format 1.0.0: the results fixture's documents, a
// leaf declaring nodes, the index and the build records.
const kb100Dir = "../internal/migrate/testdata/1.0.0"

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
	{"cited-by", "clm-0dtsyu"}, {"referenced-by", "clm-0dtsyu"}, {"strength-below", "--max-strength", "0.9"}, {"subtree", ""},
	{"subtree", "b"}, {"weak-points"}, {"show", "clm-0dtsyu"}, {"show", "exp-k3m9q2"}, {"stats"},
	{"edge-census"}, {"edges"},
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

// TestA100KBRefreshesTo200: a KB stamped 1.0.0 opens, verify reports its
// index stale with the refresh that migrates it, a refresh writes it at
// 2.0.0 and verify then finds nothing a refresh would clear.
func TestA100KBRefreshesTo200(t *testing.T) {
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
	if stamp, err := kb.FormatStamp(tree(t, repo)["kb-root/entry-point.md"]); err != nil || stamp != kb.FormatVersion {
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
	values := writeValues(t, t.TempDir(), "insert.yaml", "entry:\n- register: claim-rigor.md\n  title: A Result\n  rigor: 0.5\n  rationale: Shown.\n")
	readers := append([][]string{{"refresh"}, {"verify"}, {"render-claim-graph"},
		{"insert-claim-entry", "--create", "--values", values}, {"insert-claim-entry", "--create", "--no-refresh", "--values", values}}, queryCases...)
	for _, stamp := range []string{"3.0.0", "2.1.0"} {
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
		entry := filepath.Join(repo, kb.KBDir, kb.EntryPointFile)
		writeValues(t, filepath.Dir(entry), kb.EntryPointFile, strings.Replace(tree(t, repo)["kb-root/entry-point.md"], `"`+kb.FormatVersion+`"`, `"2.0.9"`, 1))
		if out, code := kbase(t, repo, "find", ""); code != 0 {
			t.Errorf("find over a KB at 2.0.9: exit %d\n%s", code, out)
		}
		out, code := kbase(t, repo, "refresh")
		if d := checkDocument(t, "refresh", out); code != 0 || !slices.Contains(anyStrings(d.Values["written"]), kb.EntryPointFile) {
			t.Errorf("refresh over a KB at 2.0.9: exit %d\n%s", code, out)
		}
		if stamp, _ := kb.FormatStamp(tree(t, repo)["kb-root/entry-point.md"]); stamp != kb.FormatVersion {
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
	link := filepath.Join(repo, kb.KBDir, kb.IndexDir, kb.IndexFileName(kb.IndexClaims))
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

// TestRenderClaimGraphRefusesAMigratedKB: rendering the sheets of a KB loaded
// at an older format refuses once as a stale index, naming the format and the
// refresh that writes it, and writes nothing.
func TestRenderClaimGraphRefusesAMigratedKB(t *testing.T) {
	repo := stageRepo(t, kb100Dir)
	before := tree(t, repo)
	out, code := kbase(t, repo, "render-claim-graph")
	d := checkDocument(t, "render-claim-graph", out)
	items := d.items("refusals")
	if code != 1 || d.Outcome != "refused" || len(items) != 1 || items[0]["check"] != "index" ||
		items[0]["remedy"] != "kbase refresh" || !strings.Contains(items[0]["detail"].(string), "1.0.0") {
		t.Fatalf("render-claim-graph over an unsaved 1.0.0 conversion: exit %d\n%s", code, out)
	}
	if after := tree(t, repo); !mapsEqualStrings(before, after) {
		t.Error("a refused render wrote to the KB")
	}
}

// kbaseStreams is kbase, also returning the stderr a person sees.
func kbaseStreams(t *testing.T, dir string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	t.Chdir(dir)
	out, errOut, err := executeRoot(t, args...)
	code = exitStatus(err, out, &bytes.Buffer{})
	if sub, _, ferr := rootCmd.Find(args); ferr == nil {
		if err := resetFlags(sub); err != nil {
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
	values := writeValues(t, t.TempDir(), "insert.yaml", "entry:\n- register: claim-rigor.md\n  title: A Result\n  rigor: 0.5\n  rationale: Shown.\n")
	before := tree(t, repo)
	out, stderr, code := kbaseStreams(t, repo, "insert-claim-entry", "--create", "--no-refresh", "--values", values)
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
	out, stderr, code = kbaseStreams(t, repo, "insert-claim-entry", "--create", "--values", values)
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
	if stamp, _ := kb.FormatStamp(tree(t, repo)["kb-root/entry-point.md"]); stamp != kb.FormatVersion {
		t.Errorf("the write op's refresh left the stamp at %q", stamp)
	}
}
