package main

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/gitledger"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/log"
	toolresult "kbase/internal/result"
)

// maintenanceEntryPoint is a current entry point listing a.md and carrying no
// frontmatter: the stamp stands in the stamp file beside the records.
const maintenanceEntryPoint = "# KB\n\n- [a](a.md)\n"

// maintenanceStampFile and maintenanceFormatStamp are the format stamp as a
// repository-relative path and the bytes the current kbase writes into it.
const (
	maintenanceStampFile   = kb.KBDir + "/" + kb.FormatFile
	maintenanceFormatStamp = "kb-format: \"" + kb.FormatVersion + "\"\n"
)

// recordFiles is g's two record files, as writeRepo takes them, composed by
// kb's encoder.
func recordFiles(g kb.Graph) map[string]string {
	return map[string]string{
		kb.KBDir + "/" + kb.GraphNodesFile: string(kb.EncodeNodes(g)),
		kb.KBDir + "/" + kb.GraphEdgesFile: string(kb.EncodeEdges(g)),
	}
}

// maintenanceRepo is a repository whose KB is the entry point, one leaf and
// records holding nothing.
func maintenanceRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	files := map[string]string{
		".git/HEAD":              "ref: refs/heads/main\n",
		"kb-root/entry-point.md": maintenanceEntryPoint,
		maintenanceStampFile:     maintenanceFormatStamp,
		"kb-root/a.md":           "[↑ KB](entry-point.md)\n\n# A\n",
	}
	maps.Copy(files, recordFiles(kb.Graph{}))
	writeRepo(t, repo, files)
	return repo
}

func runMaintenance(t *testing.T, run func(maintenanceOptions) (int, error), dir string) (int, string) {
	t.Helper()
	var out bytes.Buffer
	code, err := run(maintenanceOptions{WorkDir: dir, Stdout: &out, Stderr: &bytes.Buffer{}, Logger: log.Discard()})
	if err != nil {
		t.Fatal(err)
	}
	return code, out.String()
}

// commitKB commits everything in repo as it stands — kb-root/, any build
// record at the repository root, and a move update-format staged — git's
// identity and configuration fixed, so update-format finds it clean. repo is
// a repository as gitRepo or writeRepo makes one.
func commitKB(t *testing.T, repo string) {
	t.Helper()
	for k, v := range map[string]string{
		"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@invalid", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@invalid",
		"GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": filepath.Join(repo, ".git", "test-gitconfig"),
	} {
		t.Setenv(k, v)
	}
	for _, d := range []string{"objects", "refs/heads"} {
		if err := os.MkdirAll(filepath.Join(repo, ".git", d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	r, err := gitledger.Open(repo, []string{"."}, log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Commit("kb-root as the test stands it", ""); err != nil {
		t.Fatal(err)
	}
}

// updateFormatKB commits repo's kb-root/ and runs update-format over it,
// failing t unless it succeeds; it returns the result document.
func updateFormatKB(t *testing.T, repo string) string {
	t.Helper()
	commitKB(t, repo)
	code, out := runMaintenance(t, runUpdateFormat, repo)
	if code != 0 {
		t.Fatalf("update-format: exit %d\n%s", code, out)
	}
	return out
}

// TestVerifyRefusesAMalformedRecord: a hand-edited record the strict load
// refuses is a refused verify, one item per fault under the metadata class at
// the record file and line, naming the field and the values it allows — not a
// failure.
func TestVerifyRefusesAMalformedRecord(t *testing.T) {
	repo := maintenanceRepo(t)
	values := t.TempDir()
	run := func(want string, args ...string) resultDoc {
		t.Helper()
		out, code := kbase(t, repo, args...)
		d := checkDocument(t, args[0], out)
		if d.Outcome != want || code != toolresult.ExitCode(want) {
			t.Fatalf("%q: outcome %s, exit %d; want %s\n%s", args, d.Outcome, code, want, out)
		}
		return d
	}
	writeRepo(t, repo, map[string]string{"kb-root/a.md": "[↑ KB](entry-point.md)\n\n# A\n\nA result is shown.\n\nAn analysis lifts it.\n"})
	ids := run("done", "write", "--values", writeValues(t, values, "nodes.yaml", "entry:\n"+
		"- operation: add-claim\n  title: A Result\n  rigor: 0.5\n  rationale: Shown.\n  document: a.md\n  line: 5\n  snippet: A result is shown\n"+
		"- operation: add-analysis\n  title: An Analysis\n  rigor: 0.9\n  rationale: Lifts the result.\n  document: a.md\n  line: 7\n  snippet: An analysis lifts it\n")).entryIDs()
	run("done", "add-edge", "--values", writeValues(t, values, "edge.yaml",
		"entry:\n- source: "+ids[1]+"\n  target: "+ids[0]+"\n  relation: lifts\n  bearing: 0.5\n"))

	edges := filepath.Join(repo, kb.KBDir, filepath.FromSlash(kb.GraphEdgesFile))
	text, err := os.ReadFile(edges)
	if err != nil {
		t.Fatal(err)
	}
	const judged, faulty = "bearing: 0.5\n", "bearing: 2\n"
	faulted := strings.Replace(string(text), judged, faulty, 1)
	if faulted == string(text) {
		t.Fatalf("no %q in %s:\n%s", judged, kb.GraphEdgesFile, text)
	}
	if err := os.WriteFile(edges, []byte(faulted), 0o644); err != nil {
		t.Fatal(err)
	}

	out, code := kbase(t, repo, "verify")
	if code != toolresult.ExitCode("refused") {
		t.Fatalf("verify: exit %d, want refused\n%s", code, out)
	}
	d := checkDocument(t, "verify", out)
	items := d.items("refusals")
	if d.Outcome != "refused" || len(items) != 1 {
		t.Fatalf("outcome %q with %d refusals, want refused with one:\n%s", d.Outcome, len(items), out)
	}
	it := items[0]
	wantLine := 1 + strings.Count(faulted[:strings.Index(faulted, faulty)], "\n")
	if it["check"] != kb.CheckMetadata || it["path"] != kb.GraphEdgesFile || it["line"] != wantLine || it["key"] != "bearing" {
		t.Errorf("item = %v, want check %s at %s:%d key bearing", it, kb.CheckMetadata, kb.GraphEdgesFile, wantLine)
	}
	if _, has := it["allowed"]; has {
		t.Errorf("allowed = %v, want none for an open domain", it["allowed"])
	}
	if detail, _ := it["detail"].(string); !strings.Contains(detail, `a number in [0, 1], or "*pending*"`) {
		t.Errorf("detail = %q, want it to state the bearing domain", it["detail"])
	}
}

// TestUpdateFormatAndVerifyVerbs: a hosted claim whose register page stands
// nowhere is a refused verify naming the page, remedy the empty write batch;
// update-format over the 4.0.0 KB renders it, writes the absent maintained
// documents whole and leaves the entry point and a bare id as they stand; a
// second run over the committed result changes nothing; verify then finds only
// the bare id, remedy the add-citation entry naming it as snippet, and once
// that entry has run, nothing.
func TestUpdateFormatAndVerifyVerbs(t *testing.T) {
	repo := maintenanceRepo(t)
	rigor := 0.5
	files := recordFiles(kb.Graph{Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "A Result", Rigor: &rigor, Rationale: "Shown."}}})
	const leaf = "[↑ KB](entry-point.md)\n\n# A\n\nThe result.<!-- kbase-id: clm-aaaaaa -->\n\nAs clm-aaaaaa shows.\n"
	files["kb-root/a.md"] = leaf
	writeRepo(t, repo, files)
	sub := filepath.Join(repo, kb.KBDir)

	code, out := runMaintenance(t, runVerify, repo)
	if d := parseResult(t, out); code != 1 || d.Outcome != "refused" || !hasItem(d.items("refusals"), index.CheckRegisterFreshness, kb.RegisterFile, atTheCommandLine(index.EmptyBatchRemedy)) {
		t.Errorf("verify before update-format: exit %d, want the missing %s refused with remedy %q\n%s", code, kb.RegisterFile, atTheCommandLine(index.EmptyBatchRemedy), out)
	}
	commitKB(t, repo)
	code, out = runMaintenance(t, runUpdateFormat, sub)
	d := checkDocument(t, "update-format", out)
	if code != 0 || d.Outcome != "done" || !slices.Equal(anyStrings(d.Values["written"]), []string{kb.AgentsFile, kb.ConventionsFile, kb.ClaimGraphFile, kb.RegisterFile}) {
		t.Errorf("first update-format: exit %d, want the register page, the sheet and the two maintained documents written\n%s", code, out)
	}
	if slices.ContainsFunc(d.Keys, func(k string) bool { return k == "migration" || k == "bare-ids" }) {
		t.Errorf("update-format over a 4.0.0 KB carries the conversion's keys: %q", d.Keys)
	}
	if b, _ := os.ReadFile(filepath.Join(sub, kb.EntryPointFile)); string(b) != maintenanceEntryPoint {
		t.Errorf("update-format left the entry point as\n%s\nwant it as it stood:\n%s", b, maintenanceEntryPoint)
	}
	if b, _ := os.ReadFile(filepath.Join(sub, "a.md")); string(b) != leaf {
		t.Errorf("update-format rewrote the leaf holding a bare id:\n%s", b)
	}
	commitKB(t, repo)
	if code, out := runMaintenance(t, runUpdateFormat, repo); code != 0 || !strings.Contains(out, `outcome: "unchanged"`) {
		t.Errorf("second update-format: exit %d\n%s", code, out)
	}
	code, out = runMaintenance(t, runVerify, repo)
	if items := parseResult(t, out).items("refusals"); code != 1 || len(items) != 1 || items[0]["check"] != "bare-id" ||
		items[0]["remedy"] != "cite the node: `printf 'entry:\\n  - document: a.md\\n    id: clm-aaaaaa\\n    line: 7\\n    operation: add-citation\\n    snippet: clm-aaaaaa\\n' | kbase write`, which replaces the id with its naming-form citation" {
		t.Errorf("verify after update-format: exit %d, want the bare id alone, its remedy the add-citation entry\n%s", code, out)
	}
	if d := writeBatch(t, repo, "write", "entry:\n  - document: a.md\n    id: clm-aaaaaa\n    line: 7\n    operation: add-citation\n    snippet: clm-aaaaaa\n", false); d.Outcome != toolresult.Done {
		t.Fatalf("the bare id's remedy: %s\n%v", d.Outcome, d.Values)
	}
	if code, out := runMaintenance(t, runVerify, repo); code != 0 {
		t.Errorf("verify after the bare id's remedy: exit %d, want done\n%s", code, out)
	}
	if code, out := runMaintenance(t, runVerify, t.TempDir()); code != 1 || !strings.Contains(out, "not inside a git worktree") {
		t.Errorf("verify outside a worktree: exit %d\n%s", code, out)
	}
}

// TestVerifyStalePageClearedByTheEmptyBatch: a summary page that does not
// stand as the derivation renders it is verify's one register-freshness
// fault, its remedy the empty write batch; once that batch has run, verify
// finds nothing.
func TestVerifyStalePageClearedByTheEmptyBatch(t *testing.T) {
	repo := maintenanceRepo(t)
	files := recordFiles(kb.Graph{Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "A Result"}}})
	files["kb-root/a.md"] = "[↑ KB](entry-point.md)\n\n# A\n\nThe result.<!-- kbase-id: clm-aaaaaa -->\n"
	files["kb-root/"+kb.RegisterFile] = "# KB summary\n\nstale\n"
	writeRepo(t, repo, files)

	code, out := runMaintenance(t, runVerify, repo)
	if items := parseResult(t, out).items("refusals"); code != 1 || len(items) != 1 ||
		!hasItem(items, "register-freshness", kb.RegisterFile, atTheCommandLine(index.EmptyBatchRemedy)) {
		t.Fatalf("verify over the stale page: exit %d, want its one register-freshness fault, remedy %q\n%s", code, atTheCommandLine(index.EmptyBatchRemedy), out)
	}
	if d := writeBatch(t, repo, "write", "entry: []\n", false); d.Outcome != toolresult.Done {
		t.Fatalf("the empty batch: %s\n%v", d.Outcome, d.Values)
	}
	if code, out := runMaintenance(t, runVerify, repo); code != 0 || !strings.Contains(out, `outcome: "done"`) {
		t.Errorf("verify after the empty batch: exit %d, want done\n%s", code, out)
	}
}

// TestUpdateFormatWithNoDotReportsNotDrawn: with no dot on PATH update-format
// draws nothing, and its result says so in the sheet value and names the
// cause.
func TestUpdateFormatWithNoDotReportsNotDrawn(t *testing.T) {
	repo := maintenanceRepo(t)
	commitKB(t, repo)
	t.Setenv("PATH", gitOnlyPath(t))
	code, out := runMaintenance(t, runUpdateFormat, repo)
	if code != 0 {
		t.Fatalf("update-format with no dot: exit %d\n%s", code, out)
	}
	for _, want := range []string{`sheet: "not-drawn"`, `dot: "not found on PATH"`} {
		if !strings.Contains(out, want) {
			t.Errorf("update-format with no dot lacks %s\n%s", want, out)
		}
	}
}

// TestUpdateFormatRefusesADirtyKBRoot: an uncommitted change under kb-root/
// is refused, naming each path, and nothing is written.
func TestUpdateFormatRefusesADirtyKBRoot(t *testing.T) {
	repo := maintenanceRepo(t)
	commitKB(t, repo)
	writeRepo(t, repo, map[string]string{"kb-root/a.md": "[↑ KB](entry-point.md)\n\n# A, edited\n", "kb-root/b.md": "# B\n", "outside.md": "not kbase's\n"})
	code, out := runMaintenance(t, runUpdateFormat, repo)
	d := checkDocument(t, "update-format", out)
	var paths []string
	for _, it := range d.items("refusals") {
		if it["check"] == checkDirtyPaths {
			paths = append(paths, it["path"].(string))
		}
	}
	if code != 1 || !slices.Equal(paths, []string{kb.KBDir + "/a.md", kb.KBDir + "/b.md"}) {
		t.Errorf("update-format over a dirty kb-root/: exit %d, want a dirty-paths item for each changed path under kb-root/ alone\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(repo, kb.KBDir, kb.AgentsFile)); !os.IsNotExist(err) {
		t.Errorf("the refused update-format wrote %s (%v)", kb.AgentsFile, err)
	}
}

// TestALoadErrorIsRefusedByEveryLoadingVerb: an edge naming no node is a load
// error. export-latex, verify and update-format are each refused, check
// metadata naming the edge file with the remedy in git, and none writes
// anything.
func TestALoadErrorIsRefusedByEveryLoadingVerb(t *testing.T) {
	repo := maintenanceRepo(t)
	writeRepo(t, repo, recordFiles(kb.Graph{Scorable: []kb.ScorableEdge{{Relation: kb.RelationSupports,
		Source: "clm-aaaaaa", Target: "clm-bbbbbb", Bearing: kb.Bearing{Pending: true}}}}))
	commitKB(t, repo)
	edges := filepath.Join(repo, kb.KBDir, filepath.FromSlash(kb.GraphEdgesFile))
	before, err := os.ReadFile(edges)
	if err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{{"export-latex"}, {"verify"}, {"update-format"}} {
		out, code := kbase(t, repo, args...)
		d := checkDocument(t, args[0], out)
		if code != toolresult.ExitCode("refused") || !hasItem(d.items("refusals"), kb.CheckMetadata, kb.GraphEdgesFile, kb.ConsistentRecordsRemedy) {
			t.Errorf("%s over a load error: exit %d, want refused naming the fault with remedy %q\n%s", args[0], code, kb.ConsistentRecordsRemedy, out)
		}
	}
	if _, err := os.Stat(filepath.Join(repo, exportOutDefault)); !os.IsNotExist(err) {
		t.Errorf("a refused export wrote %s (%v)", exportOutDefault, err)
	}
	if after, err := os.ReadFile(edges); err != nil || string(after) != string(before) {
		t.Errorf("a refused update-format rewrote %s: %q (%v)", kb.GraphEdgesFile, after, err)
	}
}

// hasItem is whether items holds one of check about path with remedy.
func hasItem(items []map[string]any, check, path, remedy string) bool {
	for _, it := range items {
		if it["check"] == check && it["path"] == path && it["remedy"] == remedy {
			return true
		}
	}
	return false
}
