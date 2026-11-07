package write

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/kb"
	"kbase/internal/result"
)

// TestBatchStopsAtTheFirstApplyRefusal: a batch whose third and fourth of five
// entries would each be refused stops at the third, names it, and writes
// nothing.
func TestBatchStopsAtTheFirstApplyRefusal(t *testing.T) {
	root := seedKB(t)
	before := snapshot(t, root)
	res := run(t, root, EditClaim, "entry:\n"+
		"- id: "+seedClaim+"\n  rigor: 0.5\n"+
		"- id: "+seedClaim+"\n  rigor: 0.5\n"+
		"- id: clm-zzzzzz\n  rigor: 0.5\n"+
		"- id: clm-yyyyyy\n  rigor: 0.5\n"+
		"- id: "+seedClaim+"\n  rigor: 0.5\n")
	if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Entry != 3 {
		t.Fatalf("got %s %+v, want refused with one item naming entry 3", res.Outcome, res.Refusals)
	}
	if after := snapshot(t, root); !maps.Equal(before, after) {
		t.Error("a refused batch changed kb-root")
	}
}

// TestBatchRefusesAnEntryALaterOneWouldRepair: removing a claim another
// claim's rationale names is refused at that entry, though the next entry
// removes the naming claim too: each entry is checked against the records as it
// leaves them.
func TestBatchRefusesAnEntryALaterOneWouldRepair(t *testing.T) {
	root := seedKB(t)
	g := records(t, root)
	g.Claims = append(g.Claims, kb.ClaimNode{ID: "clm-bbbbbb", Title: "Naming", Rigor: f64(0.5), Rationale: "Restates " + seedClaim + "."})
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(kb.GraphNodesFile)), kb.EncodeNodes(g), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)
	res := run(t, root, RemoveNode, "entry:\n- id: "+seedClaim+"\n- id: clm-bbbbbb\n")
	if res.Outcome != result.Refused || len(res.Refusals) == 0 || res.Refusals[0].Entry != 1 {
		t.Fatalf("got %s %+v, want refused at entry 1", res.Outcome, res.Refusals)
	}
	if after := snapshot(t, root); !maps.Equal(before, after) {
		t.Error("a refused batch changed kb-root")
	}
}

// TestBatchDryRun: a dry run writes nothing and reports what the real run
// then does.
func TestBatchDryRun(t *testing.T) {
	root := seedKB(t)
	before := snapshot(t, root)
	values := "entry:\n- id: " + seedClaim + "\n  rigor: 0.25\n"
	dry := run(t, root, EditClaim, "dry-run: true\n"+values)
	if after := snapshot(t, root); !maps.Equal(before, after) {
		t.Fatal("a dry run changed kb-root")
	}
	real := run(t, root, EditClaim, values)
	if dry.Outcome != result.Done || real.Outcome != dry.Outcome {
		t.Fatalf("dry run %s %+v, real run %s %+v; want both done", dry.Outcome, dry.Refusals, real.Outcome, real.Refusals)
	}
	if !equalResultsBarDryRun(dry, real) {
		t.Errorf("dry run %+v, real run %+v; want equal but for dry-run", dry, real)
	}
}

// TestBatchCommitPreflight: a committing batch — commit: true in the batch,
// or Options.Commit as --commit sets it — reports Commit and runs Preflight
// once its entries and derivation are known, before the first write, told
// whether the batch is a dry run, a dry run's included; its error stops the
// batch with nothing written.
func TestBatchCommitPreflight(t *testing.T) {
	const values = "entry:\n- id: " + seedClaim + "\n  rigor: 0.25\n"
	for _, tc := range []struct {
		name, options string
		flag          bool
		preflightErr  error
		calls         int
		commit        bool
		outcome       string
		written       bool
	}{
		{"batch", "commit: true\n", false, nil, 1, true, result.Done, true},
		{"flag", "", true, nil, 1, true, result.Done, true},
		{"without commit", "", false, nil, 0, false, result.Done, true},
		{"dry run", "commit: true\ndry-run: true\n", false, nil, 1, true, result.Done, false},
		{"preflight fails", "commit: true\n", false, errors.New("a hook refused the commit"), 1, true, result.Failed, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := seedKB(t)
			before := snapshot(t, root)
			calls, told := 0, false
			res := Run(EditClaim, Options{KBRoot: root, Values: []byte(tc.options + values), Commit: tc.flag, Preflight: func(dryRun bool) error {
				calls++
				told = dryRun
				if !maps.Equal(before, snapshot(t, root)) {
					t.Error("kb-root changed before the pre-flight")
				}
				return tc.preflightErr
			}})
			if calls != tc.calls || res.Commit != tc.commit || res.Outcome != tc.outcome {
				t.Errorf("preflight ran %d times, commit %t, outcome %s %+v; want %d, %t, %s", calls, res.Commit, res.Outcome, res.Failures, tc.calls, tc.commit, tc.outcome)
			}
			if calls > 0 && told != res.DryRun {
				t.Errorf("preflight told dry run %t of a batch whose dry-run is %t", told, res.DryRun)
			}
			if written := !maps.Equal(before, snapshot(t, root)); written != tc.written {
				t.Errorf("kb-root written %t, want %t", written, tc.written)
			}
		})
	}
}

// equalResultsBarDryRun is whether a and b are equal but for their dry-run
// flag.
func equalResultsBarDryRun(a, b Result) bool {
	if !a.DryRun || b.DryRun {
		return false
	}
	a.DryRun = false
	return reflect.DeepEqual(a, b)
}

// TestBatchEmptyRederives: an empty batch writes no record and brings a stale
// summary page back to what the derivation renders.
func TestBatchEmptyRederives(t *testing.T) {
	root := seedKB(t)
	if res := run(t, root, EditClaim, "entry:\n- id: "+seedClaim+"\n  rigor: 0.25\n"); res.Outcome != result.Done {
		t.Fatalf("seeding write: %s %+v %+v", res.Outcome, res.Refusals, res.Failures)
	}
	page := filepath.Join(root, "kb-summary.md")
	rendered, err := os.ReadFile(page)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(page, append(rendered, "A hand edit.\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	nodes := recordFileText(t, root, kb.GraphNodesFile)
	res := run(t, root, EditClaim, "entry: []\n")
	if res.Outcome != result.Done {
		t.Fatalf("got %s %+v %+v, want done", res.Outcome, res.Refusals, res.Failures)
	}
	if got, _ := os.ReadFile(page); string(got) != string(rendered) {
		t.Errorf("kb-summary.md is\n%s\nwant\n%s", got, rendered)
	}
	if recordFileText(t, root, kb.GraphNodesFile) != nodes {
		t.Error("an empty batch rewrote the node records")
	}
}

// TestBatchDerivationFailureWritesNoRecord: where the derivation cannot be
// written — a summary page's position is a directory — the records the
// entries changed are not written either.
func TestBatchDerivationFailureWritesNoRecord(t *testing.T) {
	root := seedKB(t)
	blocked := filepath.Join(root, "kb-summary.md")
	if err := os.MkdirAll(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(blocked, "keep"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	nodes := recordFileText(t, root, kb.GraphNodesFile)
	res := run(t, root, EditClaim, "entry:\n- id: "+seedClaim+"\n  rigor: 0.25\n")
	if res.Outcome == result.Done || res.Outcome == result.Unchanged {
		t.Fatalf("got %s, want the derivation's failure", res.Outcome)
	}
	if recordFileText(t, root, kb.GraphNodesFile) != nodes {
		t.Error("the node records were written though the derivation was not")
	}
}

// TestCheckLine: a batch's prose value is one line read collapsed; a line
// break, U+0008 and U+000C are refused, and "" only where the key allows it.
func TestCheckLine(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		allowEmpty  bool
		want        string
		refused     bool
	}{
		{"collapsed", "  a \t b  c  ", false, "a b c", false},
		{"LF", "a\nb", true, "", true},
		{"CR", "a\rb", true, "", true},
		{"backspace", "\beta", true, "", true},
		{"form feed", "\frac", true, "", true},
		{"empty allowed", "", true, "", false},
		{"blank allowed", "   ", true, "", false},
		{"empty refused", "", false, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: tc.value}
			got, err := checkLine(tc.allowEmpty)(n, "title")
			if (err != nil) != tc.refused {
				t.Fatalf("checkLine(%q) = %v, %v; refused %t wanted", tc.value, got, err, tc.refused)
			}
			if err == nil && got != tc.want {
				t.Errorf("checkLine(%q) = %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}

// TestBatchMixed: kbase write's batch mixes ops, each entry naming its own,
// and applies them in the order given, never reordered: an add-claim of the
// title an edit-claim before it gave up takes that title, and an edit-claim to
// the title an add-claim before it took is refused.
func TestBatchMixed(t *testing.T) {
	edit := func(title string) string {
		return "- operation: edit-claim\n  id: " + seedClaim + "\n  title: " + title + "\n"
	}
	add := func(title string) string {
		return "- operation: add-claim\n  title: " + title + "\n  rationale: Shown.\n  document: leaf.md\n  line: 11\n  snippet: bound is tight\n"
	}
	t.Run("in order", func(t *testing.T) {
		root := seedKB(t)
		res := run(t, root, "write", "entry:\n"+edit("Renamed Result")+add("Seed Result"))
		if res.Outcome != result.Done || len(res.Entries) != 2 {
			t.Fatalf("got %s %+v", res.Outcome, res.Refusals)
		}
		id := res.Entries[1].ID
		c := record(records(t, root).Claims, func(n kb.ClaimNode) bool { return n.ID == id })
		if c == nil || c.Title != "Seed Result" {
			t.Errorf("the added claim's record is %+v, want title Seed Result under %s", c, id)
		}
		want := []EntryResult{{Entry: 1, Operation: EditClaim, Outcome: result.Done, Names: []string{seedClaim}, ID: seedClaim},
			{Entry: 2, Operation: AddClaim, Outcome: result.Done, ID: id}}
		if !reflect.DeepEqual(res.Entries, want) {
			t.Errorf("entries %+v, want %+v", res.Entries, want)
		}
	})
	t.Run("never reordered", func(t *testing.T) {
		root := seedKB(t)
		before := snapshot(t, root)
		res := run(t, root, "write", "entry:\n"+add("Renamed Result")+edit("Renamed Result"))
		if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Entry != 2 || res.Refusals[0].Key != "title" {
			t.Fatalf("got %s %+v, want refused at entry 2's title", res.Outcome, res.Refusals)
		}
		if after := snapshot(t, root); !maps.Equal(before, after) {
			t.Error("a refused batch changed kb-root")
		}
	})
	t.Run("shape errors all at once", func(t *testing.T) {
		root := seedKB(t)
		before := snapshot(t, root)
		res := run(t, root, "write", "entry:\n- operation: no-such-op\n- operation: edit-claim\n  id: "+seedClaim+"\n  rigor: 2\n"+
			"- operation: edit-claim\n  rigor: 0.5\n- edit-claim: 0.5\n")
		var entries []int
		for _, r := range res.Refusals {
			entries = append(entries, r.Entry)
		}
		if res.Outcome != result.Refused || len(entries) != 4 || entries[0] != 1 || entries[1] != 2 || entries[2] != 3 || entries[3] != 4 {
			t.Fatalf("got %s %+v, want one refusal per entry 1–4", res.Outcome, res.Refusals)
		}
		if after := snapshot(t, root); !maps.Equal(before, after) {
			t.Error("a refused batch changed kb-root")
		}
	})
}

// TestBatchOutcomeFollowsTheEntries: a batch whose entries are each done is
// done, though the second restores what the first changed and no path differs
// from the start.
func TestBatchOutcomeFollowsTheEntries(t *testing.T) {
	root := seedKB(t)
	if res := run(t, root, "write", "entry: []\n"); res.Outcome != result.Done {
		t.Fatalf("deriving first: %s %+v %+v", res.Outcome, res.Refusals, res.Failures)
	}
	res := run(t, root, "write", "entry:\n"+
		"- operation: edit-claim\n  id: "+seedClaim+"\n  title: Another Title\n"+
		"- operation: edit-claim\n  id: "+seedClaim+"\n  title: Seed Result\n")
	if len(res.Entries) != 2 || res.Entries[0].Outcome != result.Done || res.Entries[1].Outcome != result.Done {
		t.Fatalf("entries %+v, want both done", res.Entries)
	}
	if res.Outcome != result.Done {
		t.Errorf("batch outcome %s with every entry done, want done", res.Outcome)
	}
}

// TestBatchReportsEveryShapeError: an entry refused at shape does not hide a
// shape error found across the entries that parsed.
func TestBatchReportsEveryShapeError(t *testing.T) {
	root := seedKB(t)
	res := run(t, root, "write", "entry:\n"+
		"- operation: edit-claim\n  id: "+seedClaim+"\n"+
		"- operation: edit-claim\n  id: "+seedClaim+"\n  bogus: 1\n")
	var entries []int
	for _, r := range res.Refusals {
		entries = append(entries, r.Entry)
	}
	if len(entries) != 2 {
		t.Errorf("refusals at entries %v, want both shape errors, entries 1 (no field) and 2 (unknown key)", entries)
	}
}

// TestBatchRetriesWhereThePreflightChangedAFile: a pre-commit hook that edits
// a file the batch is about to replace turns the batch into retry, and the
// hook's bytes stay.
func TestBatchRetriesWhereThePreflightChangedAFile(t *testing.T) {
	root := seedKB(t)
	nodes := filepath.Join(root, filepath.FromSlash(kb.GraphNodesFile))
	var hooked []byte
	res := Run(EditClaim, Options{KBRoot: root, Values: []byte("commit: true\nentry:\n- id: " + seedClaim + "\n  rigor: 0.25\n"),
		Preflight: func(bool) error {
			data, err := os.ReadFile(nodes)
			if err != nil {
				return err
			}
			hooked = append(data, "# edited by a hook\n"...)
			return os.WriteFile(nodes, hooked, 0o644)
		}})
	if res.Outcome != result.Retry || len(res.Refusals) != 1 || res.Refusals[0].Check != kb.CheckLock {
		t.Fatalf("got %s %+v, want retry with one lock item", res.Outcome, res.Refusals)
	}
	if got, err := os.ReadFile(nodes); err != nil || string(got) != string(hooked) {
		t.Errorf("nodes.yaml is %q (%v), want the hook's bytes", got, err)
	}
}

// TestBatchDryRunFlag: the batch's dry-run reads as Run reads it, and
// is false where the batch does not parse that far.
func TestBatchDryRunFlag(t *testing.T) {
	for _, tc := range []struct {
		name, values string
		want         bool
	}{
		{"true", "dry-run: true\nentry: []\n", true},
		{"false", "dry-run: false\nentry: []\n", false},
		{"absent", "entry: []\n", false},
		{"refused entries", "dry-run: true\nentry: 3\n", true},
		{"not a batch", "[", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := BatchDryRun(EditClaim, []byte(tc.values)); got != tc.want {
				t.Errorf("got %t, want %t", got, tc.want)
			}
		})
	}
}
