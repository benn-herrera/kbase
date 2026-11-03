package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/buildrecords"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/kbdocs"
	"kbase/internal/kbload"
	"kbase/internal/mcp"
	toolresult "kbase/internal/result"
	"kbase/internal/write"
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
	{"find"}, {"find", "--title", "result"}, {"citations-of", "clm-0dtsyu"},
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

// TestA100KBUpdatesToTheCurrentFormat: a KB stamped 1.0.0 opens, verify
// reports its index stale with update-format as the remedy, and each stale
// summary page with update-format too, since the empty batch is refused while
// the conversion is pending; update-format
// writes it at the current format and verify then finds nothing update-format
// would clear.
func TestA100KBUpdatesToTheCurrentFormat(t *testing.T) {
	repo := stageRepo(t, kb100Dir)
	out, code := kbase(t, repo, "verify")
	d := checkDocument(t, "verify", out)
	if code != 1 || d.Outcome != "refused" {
		t.Fatalf("verify over 1.0.0: exit %d\n%s", code, out)
	}
	if m, _ := d.Values["migration"].(map[string]any); m["from"] != "1.0.0" || m["to"] != kb.FormatVersion {
		t.Errorf("verify over 1.0.0 reports migration %v", m)
	}
	if !slices.ContainsFunc(d.items("refusals"), func(it map[string]any) bool { return it["remedy"] == atTheCommandLine(index.UpdateFormatRemedy) }) {
		t.Errorf("verify over 1.0.0 finds no stale index:\n%s", out)
	}
	pages := 0
	for _, it := range d.items("refusals") {
		if it["check"] != "register-freshness" {
			continue
		}
		pages++
		if it["remedy"] != atTheCommandLine(index.UpdateFormatRemedy) {
			t.Errorf("verify over 1.0.0: a stale page's remedy is %q, refused until update-format runs; want update-format", it["remedy"])
		}
	}
	if pages == 0 {
		t.Errorf("verify over 1.0.0 finds no stale summary page:\n%s", out)
	}
	updateFormatKB(t, repo)
	if stamp, err := kb.DecodeFormatStamp([]byte(tree(t, repo)[kb.KBDir+"/"+kb.FormatFile])); err != nil || stamp != kb.FormatVersion {
		t.Errorf("update-format left the stamp at %q (%v), want %s", stamp, err, kb.FormatVersion)
	}
	out, _ = kbase(t, repo, "verify")
	d = checkDocument(t, "verify", out)
	if _, ok := d.Values["migration"]; ok {
		t.Errorf("a current KB's verify carries a migration record:\n%s", out)
	}
	if slices.ContainsFunc(d.items("refusals"), func(it map[string]any) bool { return it["remedy"] == atTheCommandLine(index.UpdateFormatRemedy) }) {
		t.Errorf("verify after update-format finds what update-format clears:\n%s", out)
	}
}

// TestNewerFormatRefusedByEveryReader: a KB stamped at a newer major or minor
// version is refused by every subcommand that reads it, naming both versions
// and the remedy, and nothing is written; a newer patch is read as current.
func TestNewerFormatRefusedByEveryReader(t *testing.T) {
	values := writeValues(t, t.TempDir(), "add.yaml", "entry:\n- label: Result2026\n  title: A Result\n  rationale: Shown.\n")
	readers := append([][]string{{"update-format"}, {"verify"},
		{"add-work", "--values", values}, {"add-work", "--dry-run", "--values", values}}, queryCases...)
	for _, stamp := range []string{"5.0.0", "4.1.0"} {
		t.Run(stamp, func(t *testing.T) {
			repo := stageRepo(t, kb100Dir)
			entry := filepath.Join(repo, kb.KBDir, kb.EntryPointFile)
			writeValues(t, filepath.Dir(entry), kb.EntryPointFile, strings.Replace(tree(t, repo)["kb-root/entry-point.md"], `"1.0.0"`, `"`+stamp+`"`, 1))
			commitKB(t, repo)
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
		updateFormatKB(t, repo)
		writeValues(t, filepath.Join(repo, kb.KBDir, kb.IndexDir), "format.yaml", "kb-format: \"4.0.9\"\n")
		if out, code := kbase(t, repo, "find"); code != 0 {
			t.Errorf("find over a KB at 4.0.9: exit %d\n%s", code, out)
		}
		commitKB(t, repo)
		out, code := kbase(t, repo, "update-format")
		if d := checkDocument(t, "update-format", out); code != 0 || !slices.Contains(anyStrings(d.Values["written"]), kb.FormatFile) {
			t.Errorf("update-format over a KB at 4.0.9: exit %d\n%s", code, out)
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
	commitKB(t, repo)

	out, code := kbase(t, repo, "update-format")
	d := checkDocument(t, "update-format", out)
	items := d.items("refusals")
	if code != 1 || d.Outcome != "refused" || len(items) != 1 || items[0]["check"] != kb.CheckSymlink || items[0]["path"] != link {
		t.Fatalf("update-format over a 1.0.0 KB with a symlinked index file: exit %d\n%s", code, out)
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

// pendingConversionValues is one values document per write op the caller may
// issue, each an entry the op's vocabulary accepts over the score-kb fixture.
var pendingConversionValues = map[string]string{
	"add-analysis":    "- title: An Analysis\n  rationale: Shown.\n  document: a.md\n  line: 1\n  snippet: The\n",
	"add-axiom":       "- label: S9\n  title: An Axiom\n",
	"add-citation":    "- document: a.md\n  line: 1\n  snippet: The\n  id: clm-aaaaaa\n",
	"add-claim":       "- title: A Result\n  rationale: Shown.\n  document: a.md\n  line: 1\n  snippet: The\n",
	"add-edge":        "- source: clm-bbbbbb\n  target: clm-aaaaaa\n  relation: refutes\n",
	"add-experiment":  "- title: A Trial\n  status: run\n  document: a.md\n  line: 1\n  snippet: The\n",
	"add-work":        "- label: Probe2026\n  title: A Work\n  rationale: Cited.\n",
	"edit-analysis":   "- id: ana-aaaaaa\n  title: An Analysis\n",
	"edit-axiom":      "- id: axm-s10000\n  title: An Axiom\n",
	"edit-claim":      "- id: clm-aaaaaa\n  title: A Result\n",
	"edit-edge":       "- source: clm-bbbbbb\n  target: clm-aaaaaa\n  relation: supports\n  bearing: 0.5\n",
	"edit-experiment": "- id: exp-aaaaaa\n  status: run\n",
	"edit-work":       "- id: wrk-p00000\n  title: A Work\n",
	"move-node":       "- id: clm-aaaaaa\n  document: a.md\n  line: 1\n  snippet: The\n",
	"remove-edge":     "- source: clm-bbbbbb\n  target: clm-aaaaaa\n  relation: supports\n",
	"remove-node":     "- id: clm-aaaaaa\n",
	"restore-demoted": "- source: clm-bbbbbb\n  target: clm-aaaaaa\n  relation: demoted\n",
}

// TestEveryWriteRefusedWhileAConversionIsPending: over the committed 3.0.0
// fixture, every write op — a dry run or not — is refused, check
// index and remedy kbase update-format, writing nothing, each call saying once
// on stderr what the load converted; a query answers as converted, carrying
// the migration record.
func TestEveryWriteRefusedWhileAConversionIsPending(t *testing.T) {
	repo := fixtureRepo(t, "score-kb")
	values := t.TempDir()
	before := tree(t, repo)
	for _, op := range write.Ops() {
		entry, ok := pendingConversionValues[op]
		if !ok {
			t.Errorf("%s: no values entry to issue it with", op)
			continue
		}
		file := writeValues(t, values, op+".yaml", "entry:\n"+entry)
		for _, args := range [][]string{{op, "--values", file}, {op, "--dry-run", "--values", file}} {
			out, stderr, code := kbaseStreams(t, repo, args...)
			d := checkDocument(t, op, out)
			items := d.items("refusals")
			if code != 1 || len(items) != 1 || items[0]["check"] != index.CheckIndex || items[0]["remedy"] != atTheCommandLine(index.UpdateFormatRemedy) {
				t.Errorf("%q over 3.0.0: exit %d, want the pending conversion refused\n%s", args, code, out)
			}
			if n := strings.Count(stderr, "converted to "+kb.FormatVersion); n != 1 {
				t.Errorf("%q: the sentence appears %d times on stderr, want once:\n%s", args, n, stderr)
			}
		}
	}
	if after := tree(t, repo); !mapsEqualStrings(before, after) {
		t.Error("a refused write op wrote to the KB")
	}
	out, code := kbase(t, repo, "find")
	d := checkDocument(t, "find", out)
	if m, _ := d.Values["migration"].(map[string]any); code != 0 || m["from"] != "3.0.0" || m["to"] != kb.FormatVersion {
		t.Errorf("find over 3.0.0: exit %d, want it answered as converted\n%s", code, out)
	}
}

// registerEntry is one 2.0.x register entry under heading, its Rigor section
// holding fields.
func registerEntry(id, heading string, fields ...string) string {
	return "## " + heading + "\n<!-- id: " + id + " -->\n\n> **Leaf references:** *(none)*\n\n### Rigor\n" +
		strings.Join(fields, "\n") + "\n\n### Strength\n- strength: 0.5\n\n---\n\n"
}

// warnedKB200 is a 2.0.0 repository whose conversion warns of one thing of
// every kind the conversion drops or changes.
func warnedKB200(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	marker := func(id string) string { return "<!-- claim-rigor: " + id + " -->" } // the 2.0.0 spelling
	writeRepo(t, repo, map[string]string{
		".git/HEAD":                  "ref: refs/heads/main\n",
		"kb-root/entry-point.md":     "---\nkind: entry-point\nkb-format: \"2.0.0\"\n---\n# KB\n\n- [A Lemma](a.md)\n",
		"kb-root/axioms.md":          "### Axiom S3: A labelling convention\n\nLabels are written as authored.\n",
		"kb-root/vol/index.md":       "---\nkind: index\nno-claim: true\n---\n# Volume\n",
		"kb-root/vol/claim-rigor.md": "# Volume\n\n" + registerEntry("clm-aaaaaa", "Claim A Again", "- rigor: 0.2"),
		"kb-root/claim-rigor.md": "# Claim Rigor\n\n---\n\n" +
			registerEntry("clm-aaaaaa", "Claim A", "- rigor: 0.5", "- quality: 0.4", "- rests-on:",
				"  - clm-bbbbbb — B (bearing 0.5) [low]", "  - clm-bbbbbb — B (bearing 1.0) [high]", "  - clm-zzzzzz — Nowhere (bearing 1.0)") +
			registerEntry("clm-bbbbbb", "Claim B", "- rigor: 0.9") +
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

// TestUpdateFormatListsKBTextAsItStands: the KB text update-format's
// conversion lists carry — a no-edge or strengthen-by entry's id and text, a
// bare id — is written as it stands on both surfaces, even where it holds
// what reads as an argument marker.
func TestUpdateFormatListsKBTextAsItStands(t *testing.T) {
	marked := func(s string) string { return s + " " + string(rune(0xE000)) + "-x" + string(rune(0xE001)) }
	id, text := marked("clm-aaaaaa"), marked("Holds the bound")
	_, lists := listConversion([]kb.MigrationWarning{
		{List: kbload.ListBareIDs, Path: "a.md", Line: 3, ID: id},
		{List: kbload.ListNoEdge, ID: id, Text: text},
		{List: kbload.ListStrengthenBy, ID: id, Text: text},
	})
	for _, overMCP := range []bool{false, true} {
		var out bytes.Buffer
		if err := toolresult.Emit(toolresult.Spelled(&out, mcp.Spelling(rootCmd, overMCP)), toolresult.Done, lists...); err != nil {
			t.Fatal(err)
		}
		d := parseResult(t, out.String())
		for _, l := range []string{kbload.ListBareIDs, kbload.ListNoEdge, kbload.ListStrengthenBy} {
			items := d.items(l)
			if len(items) != 1 || items[0]["id"] != id || (l != kbload.ListBareIDs && items[0]["text"] != text) {
				t.Errorf("over MCP %v, %s = %v, want the id %q and text %q as they stand", overMCP, l, items, id, text)
			}
		}
	}
}

// TestEveryConversionWarningReachesTheCaller: each kind of thing a 2.0.x
// conversion drops or changes is one item of the migration record's warnings
// and one conversion-warning line on stderr.
func TestEveryConversionWarningReachesTheCaller(t *testing.T) {
	repo := warnedKB200(t)
	commitKB(t, repo)
	out, stderr, code := kbaseStreams(t, repo, "update-format")
	d := checkDocument(t, "update-format", out)
	if code != 0 || d.Outcome != "done" {
		t.Fatalf("update-format over the 2.0.0 KB: exit %d\n%s", code, out)
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
}

// TestUpdateFormatRewritesTheMaintainedBlocks: update-format over a converted
// KB leaves an unfenced AGENTS.md as it stands, listing it under unfenced;
// writes an absent CONVENTIONS.md whole, as a fresh stamp writes it — named for
// the repository, with the authored-KB scope pin where init made the KB, which
// no build record beside it shows; and creates no README.md.
func TestUpdateFormatRewritesTheMaintainedBlocks(t *testing.T) {
	repo := stageRepo(t, kb100Dir)
	for _, rel := range buildrecords.RecordFiles {
		if err := os.Remove(filepath.Join(repo, buildrecords.RootRecordFile(rel))); err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Fatal(err)
		}
	}
	const agents = "# Old\n\nA corpus.\n"
	writeRepo(t, repo, map[string]string{"kb-root/AGENTS.md": agents})
	if err := os.Remove(filepath.Join(repo, kb.KBDir, kb.ConventionsFile)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	d := checkDocument(t, "update-format", updateFormatKB(t, repo))
	if got := anyStrings(d.Values["unfenced"]); !slices.Equal(got, []string{kb.AgentsFile}) {
		t.Errorf("unfenced = %q, want the AGENTS.md without the marker pair", got)
	}
	if !slices.Contains(anyStrings(d.Values["written"]), kb.ConventionsFile) || slices.Contains(anyStrings(d.Values["written"]), kb.AgentsFile) {
		t.Errorf("written = %q, want CONVENTIONS.md and not AGENTS.md", d.Values["written"])
	}
	fresh := filepath.Join(t.TempDir(), kb.KBDir)
	if err := os.MkdirAll(fresh, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := kbdocs.Stamp(kb.OnDisk(fresh), filepath.Base(repo), kbdocs.AuthoredKBPin); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join(fresh, kb.ConventionsFile))
	if err != nil {
		t.Fatal(err)
	}
	got := tree(t, repo)
	if got["kb-root/"+kb.ConventionsFile] != string(want) {
		t.Errorf("%s =\n%s\nwant a fresh stamp\n%s", kb.ConventionsFile, got["kb-root/"+kb.ConventionsFile], want)
	}
	if got["kb-root/"+kb.AgentsFile] != agents {
		t.Errorf("the unfenced AGENTS.md was rewritten:\n%s", got["kb-root/"+kb.AgentsFile])
	}
	if _, ok := got["kb-root/"+kbdocs.OverviewFile]; ok {
		t.Errorf("update-format created %s", kbdocs.OverviewFile)
	}
}

// TestUpdateFormatOverA300KBListsTheConversion: update-format over a
// committed copy of the 3.0.0 fixture — its copy given a no-edge text, a
// strengthen-by list and a bare id in prose — converts it, re-derives it and
// lists the converter's reports under bare-ids, no-edge and strengthen-by,
// each kind under its own key alone and none among migration's warnings, and
// cites the bare id in the leaf.
func TestUpdateFormatOverA300KBListsTheConversion(t *testing.T) {
	repo := fixtureRepo(t, "score-kb")
	nodes := filepath.Join(repo, kb.KBDir, filepath.FromSlash(kb.GraphNodesFile))
	text := tree(t, repo)[kb.KBDir+"/"+kb.GraphNodesFile]
	const blank = "  rationale: \"Read off the leaf's own statement.\"\n  no-edge: \"\"\n  strengthen-by: []\n"
	if !strings.Contains(text, blank) {
		t.Fatalf("the fixture's clm-aaaaaa record no longer reads\n%s", blank)
	}
	writeValues(t, filepath.Dir(nodes), filepath.Base(nodes), strings.Replace(text, blank,
		"  rationale: \"Read off the leaf's own statement.\"\n  no-edge: \"Stands apart from the trial.\"\n  strengthen-by: [\"Rerun the trial.\"]\n", 1))
	leaf := filepath.Join(repo, kb.KBDir, "a.md")
	writeValues(t, filepath.Dir(leaf), "a.md", tree(t, repo)[kb.KBDir+"/a.md"]+"\nThe second rests on clm-aaaaaa.\n")

	d := checkDocument(t, "update-format", updateFormatKB(t, repo))
	if formatOf(t, repo) != kb.FormatVersion {
		t.Errorf("the KB stands at %s, want %s", formatOf(t, repo), kb.FormatVersion)
	}
	bare, _ := d.Values["bare-ids"].([]any)
	if item, _ := firstOf(bare).(map[string]any); len(bare) != 1 || item["path"] != "a.md" || item["id"] != "clm-aaaaaa" || item["line"] != 10 {
		t.Errorf("bare-ids = %v, want the one bare id in a.md", d.Values["bare-ids"])
	}
	for list, want := range map[string]string{"no-edge": "Stands apart from the trial.", "strengthen-by": "Rerun the trial."} {
		got, _ := d.Values[list].([]any)
		if item, _ := firstOf(got).(map[string]any); len(got) != 1 || item["id"] != "clm-aaaaaa" || item["text"] != want {
			t.Errorf("%s = %v, want clm-aaaaaa's dropped %q", list, d.Values[list], want)
		}
	}
	migration, _ := d.Values["migration"].(map[string]any)
	for _, w := range migration["warnings"].([]any) {
		if detail, _ := w.(map[string]any)["detail"].(string); strings.Contains(detail, "clm-aaaaaa") {
			t.Errorf("migration warns of what the result lists under its own key: %s", detail)
		}
	}
	if commits, ok := d.Values["commits"].([]any); !ok || len(commits) != 0 {
		t.Errorf("commits = %v, want none without commit", d.Values["commits"])
	}
	nodesAfter := tree(t, repo)[kb.KBDir+"/"+kb.GraphNodesFile]
	ids, _ := d.Values["ids"].([]any)
	var from []string
	for _, i := range ids {
		item, _ := i.(map[string]any)
		to, _ := item["to"].(string)
		f, _ := item["from"].(string)
		from = append(from, f)
		if kind, ok := kb.NodeIDKind(to); !ok || (kind != kb.NodeKindWork && kind != kb.NodeKindAxiom) || to == item["from"] ||
			!strings.Contains(nodesAfter, `- id: "`+to+`"`) {
			t.Errorf("ids item %v, want a 3.0.0 id and the minted work or axiom id the records now carry", item)
		}
	}
	if !slices.Equal(from, []string{"axm-S1", "wrk-aaaaaa"}) {
		t.Errorf("ids name %q, want the fixture's axiom and work", from)
	}
	if got := tree(t, repo)[kb.KBDir+"/a.md"]; strings.Contains(got, "rests on clm-aaaaaa.") {
		t.Errorf("the bare id stands uncited in a.md:\n%s", got)
	}
}

// TestUpdateFormatMovesTheRootBuildRecords: over a 3.0.0 KB whose unmarked
// record stands committed at the repository root, update-format refuses while
// that record is dirty, naming it and moving nothing; once it is clean, the
// conversion moves it into kb-root/.build/ with git mv, its bytes unchanged,
// so git holds the move as a staged rename, and the result reports the move:
// the root path, repository-relative, under removed, and the new path,
// kb-root-relative, under written.
func TestUpdateFormatMovesTheRootBuildRecords(t *testing.T) {
	repo := fixtureRepo(t, "score-kb")
	from, to := buildrecords.RootRecordFile(buildrecords.UnmarkedFile), buildrecords.UnmarkedFile
	const record = "about: the unmarked record\nplanned: null\npairs: []\n"
	writeRepo(t, repo, map[string]string{from: record})
	commitKB(t, repo)

	writeRepo(t, repo, map[string]string{from: record + "# edited\n"})
	code, out := runMaintenance(t, runUpdateFormat, repo)
	d := checkDocument(t, "update-format", out)
	if items := d.items("refusals"); code != 1 || len(items) != 1 || items[0]["check"] != checkDirtyPaths || items[0]["path"] != from {
		t.Errorf("update-format over a dirty %s: exit %d, want one dirty-paths item naming it\n%s", from, code, out)
	}
	if got := tree(t, repo); got[from] != record+"# edited\n" || got[to] != "" || formatOf(t, repo) == kb.FormatVersion {
		t.Errorf("the refused update-format moved or converted something: %s = %q, %s = %q, format %s", from, got[from], to, got[to], formatOf(t, repo))
	}

	writeRepo(t, repo, map[string]string{from: record})
	code, out = runMaintenance(t, runUpdateFormat, repo)
	if code != 0 {
		t.Fatalf("update-format over the clean record: exit %d\n%s", code, out)
	}
	d = checkDocument(t, "update-format", out)
	if removed := anyStrings(d.Values["removed"]); !slices.Contains(removed, from) {
		t.Errorf("removed = %q, want the moved record's root path %s", removed, from)
	}
	if moved := strings.TrimPrefix(to, kb.KBDir+"/"); !slices.Contains(anyStrings(d.Values["written"]), moved) {
		t.Errorf("written = %q, want the moved record's new path %s", d.Values["written"], moved)
	}
	got := tree(t, repo)
	if _, stands := got[from]; stands || got[to] != record {
		t.Errorf("after update-format %s stands %t and %s = %q, want the record moved with its bytes", from, stands, to, got[to])
	}
	status, err := exec.Command("git", "-C", repo, "status", "--porcelain=v1", "--", from, to).Output()
	if err != nil {
		t.Fatal(err)
	}
	if want := "R  " + from + " -> " + to + "\n"; string(status) != want {
		t.Errorf("git status = %q, want the staged rename %q", status, want)
	}
}

// TestARefusedConversionMovesNoRecord: over a 3.0.0 KB whose unmarked record
// stands committed at the repository root and whose conversion drops a
// no-edge text, a load error that passes the open — a kb-root/CLAUDE.md that
// is not the redirect, refused before the derivation, or a cycle among the
// scoring edges, which the derivation refuses — refuses update-format with
// the record still at the root and the worktree clean, the dropped text among
// migration's warnings and no list of its own.
func TestARefusedConversionMovesNoRecord(t *testing.T) {
	const record = "about: the unmarked record\nplanned: null\npairs: []\n"
	nodes, edges := kb.KBDir+"/"+kb.GraphNodesFile, kb.KBDir+"/"+kb.GraphEdgesFile
	const blank = "  rationale: \"The argument carries the trial's result to the second result.\"\n  no-edge: \"\"\n"
	const dropped = "Stands apart from the trial."
	for _, tc := range []struct {
		name string
		// fault is the files that make the load fail, given the fixture's.
		fault func(got map[string]string) map[string]string
	}{
		{"a CLAUDE.md that is not the redirect", func(map[string]string) map[string]string {
			return map[string]string{kb.KBDir + "/" + kb.AgentsRedirectFile: "# Notes kept here\n"}
		}},
		{"a cycle among the scoring edges", func(got map[string]string) map[string]string {
			return map[string]string{edges: got[edges] + "- relation: \"supports\"\n  source: \"clm-bbbbbb\"\n  target: \"clm-aaaaaa\"\n" +
				"  context: \"\"\n  bearing: \"*pending*\"\n  manually-scored: false\n"}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := fixtureRepo(t, "score-kb")
			from := buildrecords.RootRecordFile(buildrecords.UnmarkedFile)
			got := tree(t, repo)
			if !strings.Contains(got[nodes], blank) {
				t.Fatalf("the fixture's ana-aaaaaa record no longer reads\n%s", blank)
			}
			files := tc.fault(got)
			files[from] = record
			files[nodes] = strings.Replace(got[nodes], blank, strings.Replace(blank, `no-edge: ""`, `no-edge: "`+dropped+`"`, 1), 1)
			writeRepo(t, repo, files)
			commitKB(t, repo)

			code, out := runMaintenance(t, runUpdateFormat, repo)
			d := checkDocument(t, "update-format", out)
			if d.Outcome != toolresult.Refused || code != toolresult.ExitCode(toolresult.Refused) {
				t.Fatalf("update-format: outcome %s, exit %d; want refused\n%s", d.Outcome, code, out)
			}
			if got := tree(t, repo); got[from] != record {
				t.Errorf("the refused update-format moved %s: %q", from, got[from])
			}
			status, err := exec.Command("git", "-C", repo, "status", "--porcelain").Output()
			if err != nil {
				t.Fatal(err)
			}
			if len(status) != 0 {
				t.Errorf("git status after the refused update-format = %q, want a clean worktree", status)
			}
			migration, _ := d.Values["migration"].(map[string]any)
			warnings, _ := migration["warnings"].([]any)
			if !slices.ContainsFunc(warnings, func(w any) bool {
				detail, _ := w.(map[string]any)["detail"].(string)
				return strings.Contains(detail, dropped)
			}) {
				t.Errorf("migration's warnings = %v, want the dropped no-edge text %q", warnings, dropped)
			}
			if _, listed := d.Values[kbload.ListNoEdge]; listed {
				t.Errorf("the refused update-format lists %s: %v", kbload.ListNoEdge, d.Values[kbload.ListNoEdge])
			}
		})
	}
}

// firstOf is items' first element, nil where it holds none.
func firstOf(items []any) any {
	if len(items) == 0 {
		return nil
	}
	return items[0]
}
