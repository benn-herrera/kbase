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

// The tracked KB at metadata format 0.9.0 — the results fixture's documents
// as they stood at 0.9.0, a leaf declaring nodes, the index and the build
// records — and the same KB at 1.0.0, as internal/migrate's golden pair holds
// them.
const (
	kb090Dir = "../internal/migrate/testdata/0.9.0"
	kb100Dir = "../internal/migrate/testdata/1.0.0"
)

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

// TestQueriesReadAnOlderKBAsItsConversion: a query over a KB loaded at an
// older format is refused once, naming the refresh that derives the index,
// and writes nothing; once both the 0.9.0 and the 1.0.0 KB are migrated by a
// refresh, every query answers with the same document.
func TestQueriesReadAnOlderKBAsItsConversion(t *testing.T) {
	older, current := stageRepo(t, kb090Dir), stageRepo(t, kb100Dir)
	before := tree(t, older)
	for _, c := range queryCases {
		out, code := kbase(t, older, c...)
		d := checkDocument(t, c[0], out)
		items := d.items("refusals")
		if code != 1 || d.Outcome != "refused" || len(items) != 1 || items[0]["check"] != "index" || items[0]["remedy"] != "kbase refresh" {
			t.Errorf("%q over the unsaved 0.9.0 conversion: exit %d\n%s", c, code, out)
		}
		// The document says what the load converted, once, after kb-root.
		m, _ := d.Values["migration"].(map[string]any)
		if m["from"] != kb.UnstampedFormatVersion || m["to"] != kb.FormatVersion || m["note"] == "" {
			t.Errorf("%q: migration record %v, want %s → %s with a note\n%s", c, m, kb.UnstampedFormatVersion, kb.FormatVersion, out)
		}
		if len(d.Keys) < 3 || d.Keys[0] != "outcome" || d.Keys[1] != "kb-root" || d.Keys[2] != "migration" {
			t.Errorf("%q: keys %q, want migration after kb-root", c, d.Keys)
		}
	}
	if after := tree(t, older); !mapsEqualStrings(before, after) {
		t.Error("a query over the 0.9.0 KB wrote to it")
	}
	for _, repo := range []string{older, current} {
		if out, code := kbase(t, repo, "refresh"); code != 0 {
			t.Fatalf("refresh: exit %d\n%s", code, out)
		}
	}
	for _, c := range queryCases {
		gotOlder, codeOlder := kbase(t, older, c...)
		gotCurrent, codeCurrent := kbase(t, current, c...)
		gotOlder = strings.ReplaceAll(gotOlder, older, "<repo>")
		gotCurrent = strings.ReplaceAll(gotCurrent, current, "<repo>")
		if gotOlder != gotCurrent || codeOlder != codeCurrent {
			t.Errorf("%q: over the converted 0.9.0 KB (exit %d)\n%s\nover the converted 1.0.0 KB (exit %d)\n%s",
				c, codeOlder, gotOlder, codeCurrent, gotCurrent)
		}
	}
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

// TestRefreshMigratesAnOlderKB: refresh over a KB at 0.9.0 writes every file
// the format covers at 1.0.0 — the documents' frontmatter, the index, the
// build records — stamps the entry point, and removes and reports the files
// only 0.9.0 used; a second refresh writes nothing, and verify finds nothing
// a refresh would clear. (The fixture is a format specimen, not a whole KB:
// its leaf cites claims no register keys, which verify reports either way.)
func TestRefreshMigratesAnOlderKB(t *testing.T) {
	repo, golden := stageRepo(t, kb090Dir), tree(t, kb100Dir)
	out, code := kbase(t, repo, "refresh")
	d := checkDocument(t, "refresh", out)
	if code != 0 || d.Outcome != "done" {
		t.Fatalf("refresh over 0.9.0: exit %d\n%s", code, out)
	}
	written, removed := anyStrings(d.Values["written"]), anyStrings(d.Values["removed"])
	wantRemoved := []string{"kb-build-classification.json", "kb-build-node-pass.json", "kb-build-unmarked.json"}
	for _, name := range []string{"cites", "claims", "depends-on", "strengthen-by", "subtree-aggregates", "supported-by"} {
		wantRemoved = append(wantRemoved, path.Join(kb.IndexDir, name+".jsonl"))
	}
	for _, name := range kb.IndexFiles {
		if !slices.Contains(written, path.Join(kb.IndexDir, kb.IndexFileName(name))) {
			t.Errorf("written %q lacks the index file %s", written, name)
		}
	}
	slices.Sort(wantRemoved)
	if got := slices.Sorted(slices.Values(removed)); !slices.Equal(got, wantRemoved) {
		t.Errorf("removed %q, want %q", got, wantRemoved)
	}
	for _, rel := range []string{"a.md", "b/c.md", kb.EntryPointFile} {
		if !slices.Contains(written, rel) {
			t.Errorf("written %q lacks %s", written, rel)
		}
	}
	if !slices.ContainsFunc(written, func(w string) bool { return filepath.IsAbs(w) && strings.HasSuffix(w, "/kb-build-node-pass.yaml") }) {
		t.Errorf("written %q lacks the node-pass record beside kb-root, by its absolute path", written)
	}
	files := tree(t, repo)
	for p, text := range files {
		if strings.HasSuffix(p, ".json") || strings.HasSuffix(p, ".jsonl") {
			t.Errorf("%s still stands after the migration", p)
		}
		if strings.HasSuffix(p, ".md") && strings.Contains(text, "<!-- kb-frontmatter") {
			t.Errorf("%s still carries a comment-block frontmatter", p)
		}
	}
	for _, p := range []string{"kb-root/a.md", "kb-build-node-pass.yaml", "kb-build-classification.yaml", "kb-build-unmarked.yaml"} {
		if files[p] != golden[p] {
			t.Errorf("%s after the migration =\n%s\nwant the converted form\n%s", p, files[p], golden[p])
		}
	}
	// The leaf declaring nodes is the one document the 2.0.0 form rewrites:
	// each experiment's pair list is its lifts, and every experiment carries
	// the local value 1.0.0 had no field for.
	leaf := files["kb-root/b/c.md"]
	for _, want := range []string{"    rigor: 1.0\n    lifts:\n", "    lifts:\n      - clm-0dtsyu: 0.5\n"} {
		if !strings.Contains(leaf, want) {
			t.Errorf("b/c.md lacks %q after the migration:\n%s", want, leaf)
		}
	}
	for _, old := range []string{"strengthens:", "supports:"} {
		if strings.Contains(leaf, old) {
			t.Errorf("b/c.md still carries %q:\n%s", old, leaf)
		}
	}
	if stamp, _, err := kb.FormatStamp(files["kb-root/entry-point.md"]); err != nil || stamp != kb.FormatVersion {
		t.Errorf("entry point stamp %q (%v), want %s:\n%s", stamp, err, kb.FormatVersion, files["kb-root/entry-point.md"])
	}
	again, code := kbase(t, repo, "refresh")
	if d := checkDocument(t, "refresh", again); code != 0 || d.Outcome != "unchanged" {
		t.Errorf("second refresh: exit %d\n%s", code, again)
	}
	out, _ = kbase(t, repo, "verify")
	for _, it := range checkDocument(t, "verify", out).items("refusals") {
		if it["remedy"] == "kbase refresh" {
			t.Errorf("verify after the migration finds what a refresh clears: %v", it)
		}
	}
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
	if stamp, _, err := kb.FormatStamp(tree(t, repo)["kb-root/entry-point.md"]); err != nil || stamp != kb.FormatVersion {
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
		repo := stageRepo(t, kb090Dir)
		if out, code := kbase(t, repo, "refresh"); code != 0 {
			t.Fatalf("refresh over 0.9.0: exit %d\n%s", code, out)
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
		if stamp, _, _ := kb.FormatStamp(tree(t, repo)["kb-root/entry-point.md"]); stamp != kb.FormatVersion {
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
	if stamp, _, _ := kb.FormatStamp(tree(t, repo)["kb-root/entry-point.md"]); stamp != "1.0.0" {
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
// at 0.9.0 is refused, naming the refresh that migrates it, and writes
// nothing; with its refresh it lands and migrates the KB. Each call says once
// what it converted.
func TestNoRefreshRefusedOnAnOlderKB(t *testing.T) {
	repo := stageRepo(t, kb090Dir)
	values := writeValues(t, t.TempDir(), "insert.yaml", "entry:\n- register: claim-rigor.md\n  title: A Result\n  rigor: 0.5\n  rationale: Shown.\n")
	before := tree(t, repo)
	out, stderr, code := kbaseStreams(t, repo, "insert-claim-entry", "--create", "--no-refresh", "--values", values)
	d := checkDocument(t, "insert-claim-entry", out)
	items := d.items("refusals")
	if code != 1 || len(items) != 1 || items[0]["check"] != "index" || items[0]["remedy"] != "kbase refresh" {
		t.Errorf("--no-refresh over 0.9.0: exit %d\n%s", code, out)
	}
	if m, _ := d.Values["migration"].(map[string]any); m["from"] != kb.UnstampedFormatVersion || m["to"] != kb.FormatVersion {
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
	if code != 0 || d.Outcome != "done" || !slices.Contains(anyStrings(d.Values["removed"]), "kb-build-node-pass.json") {
		t.Errorf("the write op with its refresh over 0.9.0: exit %d\n%s", code, out)
	}
	if m, _ := d.Values["migration"].(map[string]any); m["from"] != kb.UnstampedFormatVersion || m["to"] != kb.FormatVersion {
		t.Errorf("the saving op's migration record is %v", m)
	}
	if n := strings.Count(stderr, "converted to "+kb.FormatVersion); n != 1 {
		t.Errorf("the sentence appears %d times on stderr, want once:\n%s", n, stderr)
	}
	if stamp, _, _ := kb.FormatStamp(tree(t, repo)["kb-root/entry-point.md"]); stamp != kb.FormatVersion {
		t.Errorf("the write op's refresh left the stamp at %q", stamp)
	}
}
