package main

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/log"
	toolresult "kbase/internal/result"
)

// maintenanceEntryPoint is a 3.0.0 entry point listing a.md and carrying no
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
	run("done", "refresh")
	claim := anyStrings(run("done", "insert-claim-entry", "--values",
		writeValues(t, values, "claim.yaml", "entry:\n- title: A Result\n  rigor: 0.5\n  rationale: Shown.\n")).Values["ids"])[0]
	run("done", "insert-analysis-entry", "--values", writeValues(t, values, "analysis.yaml",
		"entry:\n- title: An Analysis\n  rigor: 0.9\n  rationale: Lifts the result.\n  lifts:\n  - id: "+claim+"\n    bearing: 0.5\n"))

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
	if allowed, _ := it["allowed"].([]any); len(allowed) != 1 || allowed[0] != `a number in [0, 1], or "*pending*"` {
		t.Errorf("allowed = %v, want the bearing domain alone", it["allowed"])
	}
}

// TestRefreshAndVerifyVerbs: a hosted claim whose register page stands nowhere
// is a refused verify naming the page, remedy kbase refresh; refresh renders it
// and changes nothing else; a second refresh changes nothing; verify is
// then green.
func TestRefreshAndVerifyVerbs(t *testing.T) {
	repo := maintenanceRepo(t)
	rigor := 0.5
	files := recordFiles(kb.Graph{Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "A Result", Rigor: &rigor, Rationale: "Shown."}}})
	files["kb-root/a.md"] = "[↑ KB](entry-point.md)\n\n# A\n\nThe result. <!-- claim-rigor: clm-aaaaaa -->\n"
	writeRepo(t, repo, files)
	sub := filepath.Join(repo, kb.KBDir)

	code, out := runMaintenance(t, runVerify, repo)
	if d := parseResult(t, out); code != 1 || d.Outcome != "refused" || !hasItem(d.items("refusals"), index.CheckRegisterFreshness, kb.RegisterFile, index.RefreshRemedy) {
		t.Errorf("verify before refresh: exit %d, want the missing %s refused with remedy %q\n%s", code, kb.RegisterFile, index.RefreshRemedy, out)
	}
	if code, out := runMaintenance(t, runRefresh, sub); code != 0 || !strings.Contains(out, `outcome: "done"`) || !strings.Contains(out, `- "`+kb.RegisterFile+`"`) {
		t.Errorf("first refresh: exit %d\n%s", code, out)
	}
	if b, _ := os.ReadFile(filepath.Join(sub, kb.EntryPointFile)); string(b) != maintenanceEntryPoint {
		t.Errorf("the first refresh left the entry point as\n%s\nwant it as it stood:\n%s", b, maintenanceEntryPoint)
	}
	if code, out := runMaintenance(t, runRefresh, repo); code != 0 || !strings.Contains(out, `outcome: "unchanged"`) {
		t.Errorf("second refresh: exit %d\n%s", code, out)
	}
	if code, out := runMaintenance(t, runVerify, repo); code != 0 || !strings.Contains(out, `outcome: "done"`) {
		t.Errorf("verify after refresh: exit %d\n%s", code, out)
	}
	if code, out := runMaintenance(t, runVerify, t.TempDir()); code != 1 || !strings.Contains(out, "not inside a git worktree") {
		t.Errorf("verify outside a worktree: exit %d\n%s", code, out)
	}
}

// TestADegradedKBIsRefusedUntilRefreshRepairsIt: an edge naming no node
// degrades the load. export-latex is refused, check degraded with the remedy
// kbase refresh, writing nothing; verify reports the fault; refresh rewrites
// the records without the edge and lists its loss under warnings; verify is
// then green.
func TestADegradedKBIsRefusedUntilRefreshRepairsIt(t *testing.T) {
	repo := maintenanceRepo(t)
	writeRepo(t, repo, recordFiles(kb.Graph{Scorable: []kb.ScorableEdge{{Relation: kb.RelationSupports,
		Source: "clm-aaaaaa", Target: "clm-bbbbbb", Bearing: kb.Bearing{Pending: true}}}}))

	for _, args := range [][]string{{"export-latex"}, {"verify"}} {
		out, code := kbase(t, repo, args...)
		d := checkDocument(t, args[0], out)
		if code != toolresult.ExitCode("refused") || !hasItem(d.items("refusals"), kb.CheckDegraded, kb.GraphEdgesFile, index.RefreshRemedy) {
			t.Errorf("%s over a degraded KB: exit %d, want refused naming the fault with remedy %q\n%s", args[0], code, index.RefreshRemedy, out)
		}
	}
	if _, err := os.Stat(filepath.Join(repo, exportOutDefault)); !os.IsNotExist(err) {
		t.Errorf("a refused export wrote %s (%v)", exportOutDefault, err)
	}

	out, code := kbase(t, repo, "refresh")
	d := checkDocument(t, "refresh", out)
	warnings := d.items("warnings")
	if code != 0 || d.Outcome != "done" || len(warnings) != 1 || warnings[0]["path"] != kb.GraphEdgesFile ||
		!slices.Contains(anyStrings(d.Values["written"]), kb.GraphEdgesFile) {
		t.Fatalf("refresh over a degraded KB: exit %d, want done rewriting %s with one warning\n%s", code, kb.GraphEdgesFile, out)
	}
	if b, err := os.ReadFile(filepath.Join(repo, kb.KBDir, filepath.FromSlash(kb.GraphEdgesFile))); err != nil || string(b) != string(kb.EncodeEdges(kb.Graph{})) {
		t.Errorf("the repaired %s reads %q (%v), want no record", kb.GraphEdgesFile, b, err)
	}
	if out, code := kbase(t, repo, "verify"); code != 0 {
		t.Errorf("verify after the repair: exit %d\n%s", code, out)
	}
	if out, code := kbase(t, repo, "refresh"); code != 0 || len(checkDocument(t, "refresh", out).items("warnings")) != 0 {
		t.Errorf("a refresh over the repaired KB: exit %d, want no warning\n%s", code, out)
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
