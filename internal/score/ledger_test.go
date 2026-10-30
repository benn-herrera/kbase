package score

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"kbase/internal/asks"
	"kbase/internal/buildstore"
	"kbase/internal/kb"
	"kbase/internal/query"
	"kbase/internal/result"
)

// rigorAskDigest is the digest of the ask set a rigor run can send, computed here
// rather than through the package's own helper: the test states the header's ask
// independently of the code that stamps it — the statement instruction and then
// the method one, each rendered for an empty subject, concatenated and hashed
// once.
func rigorAskDigest(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	for _, ask := range []func(string) (string, error){asks.ScoreRigorStatement, asks.ScoreRigorMethod} {
		rendered, err := ask("")
		if err != nil {
			t.Fatal(err)
		}
		b.WriteString(rendered)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// header is the run.yaml the tests' runs are opened with.
func header() RunHeader {
	return RunHeader{Run: "run-1", KBRoot: "/repo/kb-root", Pass: query.PassRigor, BackEnd: "provider",
		Provider: "local", Model: "small", Started: "2026-10-20T10:00:00Z"}
}

// node is one rigor item.
func node(kind, id string) query.Item {
	return query.Item{Node: &query.ScoringNode{Kind: kind, ID: id}}
}

// edge is one bearing item.
func edge(source, target, relation string) query.Item {
	return query.Item{Edge: &query.ScoringEdge{Source: source, Target: target, Relation: relation}}
}

// kept is the ledger opened for a fresh run, closed at the test's end.
func kept(t *testing.T, dir string) *Ledger {
	t.Helper()
	l, err := Create(dir, header())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	return l
}

// reopened is the ledger of the run in dir, read back as a resume reads it; a
// re-priced value lands through a seam that drops it, for the tests that make no
// claim about the KB.
func reopened(t *testing.T, dir string) *Ledger {
	t.Helper()
	return resumed(t, dir, landsAll)
}

// landsAll is a write seam that takes every value and keeps none of them.
func landsAll(values []Landing) []error { return make([]error, len(values)) }

// resumed is reopened with w as the write a re-priced value lands through.
func resumed(t *testing.T, dir string, w Write) *Ledger {
	t.Helper()
	l, err := Resume(dir, header(), w, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	return l
}

// landing is one value a resume's re-pricing wrote back, and the item it landed
// on.
type landing struct {
	item  query.Item
	value string
}

// landingOn is a write seam collecting what it is asked to land, in the order it
// is asked; err, where set, is what it returns for every value.
func landingOn(err error) (*[]landing, Write) {
	var got []landing
	return &got, func(values []Landing) []error {
		errs := make([]error, len(values))
		for i, v := range values {
			got = append(got, landing{item: v.Item, value: kb.BearingText(v.Value)})
			errs[i] = err
		}
		return errs
	}
}

// lines is a ledger file's records, each line parsed; a line that does not
// parse fails the test.
func lines(t *testing.T, path string) []map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("%s holds a line that does not parse: %q", path, line)
		}
		out = append(out, rec)
	}
	return out
}

// TestCreateWritesTheRunHeader: a run stands at <state-dir>/score/<run>/ and its
// run.yaml names the run, the KB, the pass, the provider, the model, the start
// and the ledger format.
func TestCreateWritesTheRunHeader(t *testing.T) {
	dir := t.TempDir()
	l := kept(t, dir)
	if want := filepath.Join(dir, "score", "run-1"); l.Dir() != want {
		t.Errorf("the run's directory = %s, want %s", l.Dir(), want)
	}
	b, err := os.ReadFile(filepath.Join(l.Dir(), "run.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := yaml.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"run": "run-1", "kb_root": "/repo/kb-root", "format": 1,
		"pass": query.PassRigor, "ask_sha256": rigorAskDigest(t),
		"backend": "provider", "provider": "local", "model": "small",
		"started": "2026-10-20T10:00:00Z"}
	if !maps.Equal(got, want) {
		t.Errorf("run.yaml = %v, want %v", got, want)
	}
	// The digest is the pass's ask: a bearing run records another.
	bearing, err := Create(t.TempDir(), RunHeader{Run: "run-2", KBRoot: "/repo/kb-root",
		Pass: query.PassBearing, BackEnd: "provider", Provider: "local", Model: "small", Started: "2026-10-20T10:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	defer bearing.Close()
	b, err = os.ReadFile(filepath.Join(bearing.Dir(), runFile))
	if err != nil {
		t.Fatal(err)
	}
	var other map[string]any
	if err := yaml.Unmarshal(b, &other); err != nil {
		t.Fatal(err)
	}
	if other["ask_sha256"] == want["ask_sha256"] {
		t.Error("the bearing run records the rigor run's ask")
	}
}

// TestAskDigestCoversEveryInstructionOfThePass: a rigor run sends two
// instructions, so its header is one digest over both — an update to either moves
// it, and neither instruction's own digest is the header's — a bearing run's is
// its one render, and a both-pass run's is one digest over the three, in the
// order its walk takes them.
func TestAskDigestCoversEveryInstructionOfThePass(t *testing.T) {
	statement, err := asks.ScoreRigorStatement("")
	if err != nil {
		t.Fatal(err)
	}
	method, err := asks.ScoreRigorMethod("")
	if err != nil {
		t.Fatal(err)
	}
	got, err := askDigest(query.PassRigor)
	if err != nil {
		t.Fatal(err)
	}
	if want := rigorAskDigest(t); got != want {
		t.Errorf("the rigor run's ask = %q, want the one digest over both instructions %q", got, want)
	}
	for name, one := range map[string]string{"statement": statement, "method": method} {
		if got == digestOf(one) {
			t.Errorf("the header records the %s instruction's own digest, so an update to the other would not move it", name)
		}
	}

	bearing, err := asks.ScoreBearing("")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := askDigest(query.PassBearing); err != nil || got != digestOf(bearing) {
		t.Errorf("the bearing run's ask = %q, %v; want the digest of its one instruction", got, err)
	}

	// A run walking both sends all three: the statement, the method and then
	// the bearing instruction, each rendered once.
	both, err := askDigest(query.PassBoth)
	if err != nil {
		t.Fatal(err)
	}
	if want := digestOf(statement + method + bearing); both != want {
		t.Errorf("the both-pass run's ask = %q, want one digest over the three instructions in order %q", both, want)
	}
}

// TestLineCarriesTheAttempt: one attempt is one line, its identity from the
// item, its run and pass from the item's own class, and its outcome and evidence
// from the attempt.
func TestLineCarriesTheAttempt(t *testing.T) {
	dir := t.TempDir()
	bh := header()
	bh.Pass = query.PassBearing
	l, err := Create(dir, bh)
	if err != nil {
		t.Fatal(err)
	}
	it := edge("clm-1", "clm-2", kb.RelationSupports)
	entry := Entry{Status: StatusDone, Value: "0.65",
		Faults: []Charge{{ID: kb.FaultTransfer, Passage: passageSource, Lines: "12", Citation: "A span", Offset: 9}},
		Reply:  "flt-transfer: 12\n", Model: "small",
		PromptSHA256: "deadbeef", Started: "2026-10-20T10:00:01Z", Ended: "2026-10-20T10:00:02Z"}
	if err := l.Append(it, entry); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	got := lines(t, filepath.Join(l.Dir(), linesFile))
	want := []map[string]any{{
		"run": "run-1", "pass": query.PassBearing, "backend": "provider", "source": "clm-1", "target": "clm-2",
		"relation": kb.RelationSupports, "status": "done",
		"faults": []any{map[string]any{"id": kb.FaultTransfer, "passage": passageSource, "lines": "12", "citation": "A span", "offset": float64(9)}},
		"value":  "0.65", "reply": "flt-transfer: 12\n",
		"model": "small", "prompt_sha256": "deadbeef",
		"started": "2026-10-20T10:00:01Z", "ended": "2026-10-20T10:00:02Z",
	}}
	if len(got) != 1 || !reflect.DeepEqual(got[0], want[0]) {
		t.Errorf("the line = %v, want %v", got, want)
	}
	// A landed answer of `none` writes its empty list, so the record carries the
	// model's positive statement that it found no fault; a failed attempt
	// writes no faults at all and no text.
	nl := kept(t, t.TempDir())
	if err := nl.Append(node(kb.NodeKindClaim, "clm-3"), Entry{Status: StatusDone, Value: "1.0", Reply: "none"}); err != nil {
		t.Fatal(err)
	}
	if err := nl.Append(node(kb.NodeKindClaim, "clm-4"), Entry{Status: StatusFailed, Reply: "line 1 is not a charge line"}); err != nil {
		t.Fatal(err)
	}
	recs := lines(t, filepath.Join(nl.Dir(), linesFile))
	if faults, ok := recs[0]["faults"].([]any); !ok || len(faults) != 0 {
		t.Errorf("the `none` line = %v, want an empty faults list", recs[0])
	}
	if _, carried := recs[1]["faults"]; carried {
		t.Errorf("the failed line carries a faults list: %v", recs[1])
	}
}

// TestLineRecordsMootBesideAPendingAnswer: a reply carrying `flt-pending` beside
// charge lines records the pending judgement and those lines as moot — parsed the
// same way, priced nowhere — and lands the pending literal.
func TestLineRecordsMootBesideAPendingAnswer(t *testing.T) {
	dir := t.TempDir()
	l := kept(t, dir)
	claim := node(kb.NodeKindClaim, "clm-1")
	entry := Entry{Status: StatusDone, Value: kb.PendingLiteral, Reply: "flt-pending\nflt-overreach: A span",
		Faults: []Charge{{ID: kb.FaultPending}},
		Moot:   []Charge{{ID: kb.FaultOverreach, Passage: passageStatement, Citation: "A span", Offset: 4}}}
	if err := l.Append(claim, entry); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	rec := lines(t, filepath.Join(l.Dir(), linesFile))[0]
	if rec["value"] != kb.PendingLiteral {
		t.Errorf("the line's value = %v, want the pending literal", rec["value"])
	}
	faults, _ := rec["faults"].([]any)
	if len(faults) != 1 || faults[0].(map[string]any)["id"] != kb.FaultPending {
		t.Errorf("the line's faults = %v, want the one pending record", rec["faults"])
	}
	moot, _ := rec["moot"].([]any)
	if len(moot) != 1 || moot[0].(map[string]any)["id"] != kb.FaultOverreach {
		t.Errorf("the line's moot = %v, want the charge the pending answer made moot", rec["moot"])
	}
}

// TestResumedRunRecordsWhatAskedEachLine: every line names the back end that
// asked its item, and a resumed run on the other back end leaves the header's
// opening alone while the line it appends names the one that asked now.
func TestResumedRunRecordsWhatAskedEachLine(t *testing.T) {
	dir := t.TempDir()
	l, err := Create(dir, header())
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Append(node(kb.NodeKindClaim, "clm-1"), Entry{Status: StatusDone, Value: "0.5", Model: "small"}); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	first := lines(t, filepath.Join(l.Dir(), linesFile))[0]
	if first["backend"] != "provider" || first["model"] != "small" {
		t.Errorf("the line = %v, want the provider that asked it", first)
	}

	// A resume over the same run on the environment's endpoint: the header
	// keeps the provider run's opening, and the line this invocation appends
	// names the environment.
	env := header()
	env.BackEnd, env.Provider, env.Model = "environment", "", "judge"
	back, err := Resume(dir, env, landsAll, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := back.Append(node(kb.NodeKindClaim, "clm-2"), Entry{Status: StatusDone, Value: "0.6", Model: "judge"}); err != nil {
		t.Fatal(err)
	}
	if err := back.Close(); err != nil {
		t.Fatal(err)
	}
	second := lines(t, filepath.Join(l.Dir(), linesFile))[1]
	if second["backend"] != "environment" || second["model"] != "judge" {
		t.Errorf("the resumed line = %v, want the back end that asked it", second)
	}
	b, err := os.ReadFile(filepath.Join(l.Dir(), runFile))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := yaml.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["backend"] != "provider" || got["provider"] != "local" || got["model"] != "small" {
		t.Errorf("run.yaml = %v, want the run's opening left as it was written", got)
	}
}

// TestResumeReadsTheLastStatusPerItem: a resumed run reads back the last status
// each item landed, and none for an item the ledger never recorded.
func TestResumeReadsTheLastStatusPerItem(t *testing.T) {
	dir := t.TempDir()
	l := kept(t, dir)
	claim := node(kb.NodeKindClaim, "clm-1")
	failed := Entry{Status: StatusFailed, Reply: "timeout", Model: "small", PromptSHA256: "aa"}
	if err := l.Append(claim, failed); err != nil {
		t.Fatal(err)
	}
	if err := l.Append(claim, Entry{Status: StatusDone, Value: "0.85"}); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}

	back := reopened(t, dir)
	if got := back.Status(claim); got != StatusDone {
		t.Errorf("the claim's last status = %q, want %q: the last line answers", got, StatusDone)
	}
	if got := back.Status(node(kb.NodeKindClaim, "clm-9")); got != "" {
		t.Errorf("an item with no line = %q, want none", got)
	}
}

// TestLastIsTheLastLinesEntry: the entry the ledger hands back for an item is
// the whole evidence its last line carries — status, value, reply, model and
// digest, not its status alone — and it is the same entry before the file is
// closed and after a resume reads it back. An item with no line has none.
func TestLastIsTheLastLinesEntry(t *testing.T) {
	dir := t.TempDir()
	l := kept(t, dir)
	claim := node(kb.NodeKindClaim, "clm-1")
	if err := l.Append(claim, Entry{Status: StatusFailed, Reply: "the register refused it", Model: "small", PromptSHA256: "aa"}); err != nil {
		t.Fatal(err)
	}
	want := Entry{Status: StatusDone, Value: "0.85", Faults: []Charge{{ID: kb.FaultOverreach, Passage: passageStatement, Citation: "A span", Offset: 4}},
		Reply: "flt-overreach: A span", Model: "small", PromptSHA256: "bb"}
	if err := l.Append(claim, want); err != nil {
		t.Fatal(err)
	}
	if got, ok := l.Last(claim); !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("Last(claim) = %+v, %t; want the last append's %+v", got, ok, want)
	}
	if _, ok := l.Last(node(kb.NodeKindClaim, "clm-9")); ok {
		t.Error("an item with no line has an entry")
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if got, ok := reopened(t, dir).Last(claim); !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("the line read back = %+v, %t; want %+v", got, ok, want)
	}
}

// TestResumeAfterAnUpdateContinues: a run whose header records an ask other than
// the one this kbase would put to the model — or which records none, as a run
// written before the field does — is resumed rather than refused: the lines it
// landed are read back, the item after them is asked and recorded, and the
// header keeps the digest it was written with, which is the record of the
// instruction its own judgements answered.
func TestResumeAfterAnUpdateContinues(t *testing.T) {
	for _, tc := range []struct {
		name string
		ask  string
	}{
		{"another ask", strings.Repeat("0", 64)},
		{"no ask at all", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			l := kept(t, dir)
			// One overreach is what the running table gives 0.85, so the resume
			// moves no value: this case is the ask alone.
			first := node(kb.NodeKindClaim, "clm-1")
			if err := l.Append(first, landedEntry("0.85", kb.FaultOverreach)); err != nil {
				t.Fatal(err)
			}
			if err := l.Close(); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(l.Dir(), runFile)
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var h RunHeader
			if err := yaml.Unmarshal(b, &h); err != nil {
				t.Fatal(err)
			}
			h.AskSHA256 = tc.ask
			if b, err = yaml.Marshal(h); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, b, 0o666); err != nil {
				t.Fatal(err)
			}

			back := reopened(t, dir)
			if got := back.Status(first); got != StatusDone {
				t.Errorf("the landed item = %q, want %q: the resume read the run back", got, StatusDone)
			}
			next := node(kb.NodeKindClaim, "clm-2")
			if err := back.Append(next, Entry{Status: StatusDone, Value: "1.0", Reply: "none"}); err != nil {
				t.Fatalf("the resumed run does not go on: %v", err)
			}
			if err := back.Close(); err != nil {
				t.Fatal(err)
			}
			lines := lines(t, filepath.Join(l.Dir(), linesFile))
			if len(lines) != 2 || lines[1]["value"] != "1.0" {
				t.Errorf("the ledger = %v, want the resumed item recorded behind the run's own", lines)
			}
			// The digest is the record of what the run's judgements answered:
			// the resume leaves it as the run wrote it.
			b, err = os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var after RunHeader
			if err := yaml.Unmarshal(b, &after); err != nil {
				t.Fatal(err)
			}
			if after.AskSHA256 != tc.ask {
				t.Errorf("the resumed run rewrote the header's ask to %q, want the %q it recorded", after.AskSHA256, tc.ask)
			}
		})
	}
}

// TestResumeRepricesFromTheRecordedIDs: a landed value the running table no
// longer gives for its recorded ids is re-derived from those ids, landed through
// the write a landing uses, and rewritten in the ledger's own file, and the
// run's directory gains a record of what moved. run.yaml is left exactly as
// Create wrote it, and a re-derivation that moves nothing lands nothing and
// records nothing.
func TestResumeRepricesFromTheRecordedIDs(t *testing.T) {
	dir := t.TempDir()
	l := kept(t, dir)
	// One line whose value its class's table contradicts, one it agrees with,
	// and one a run landed under the old single rigor table: the ids alone
	// decide, the way a weight that moved between invocations does. The third
	// carries a method id on a claim, which the claim's own table does not hold,
	// so the resume prices it at the statement maximum — a run landed under one
	// table re-prices under its items' classes.
	stale := node(kb.NodeKindClaim, "clm-1")
	moved := Entry{Status: StatusDone, Value: "0.30",
		Faults: []Charge{{ID: kb.FaultOverreach, Passage: passageStatement, Citation: "A span", Offset: 4}}, Reply: "flt-overreach: A span"}
	if err := l.Append(stale, moved); err != nil {
		t.Fatal(err)
	}
	if err := l.Append(node(kb.NodeKindClaim, "clm-2"), landedEntry("0.85", kb.FaultEquivocation)); err != nil {
		t.Fatal(err)
	}
	oldTable := node(kb.NodeKindClaim, "clm-3")
	if err := l.Append(oldTable, landedEntry("0.65", kb.FaultClaimedStep)); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(l.Dir(), runFile))
	if err != nil {
		t.Fatal(err)
	}

	landed, land := landingOn(nil)
	back := resumed(t, dir, land)
	defer back.Close()
	if got, _ := back.Last(stale); got.Value != "0.85" {
		t.Errorf("the re-derived value = %q, want the table's 0.85 for one overreach", got.Value)
	}
	if got, _ := back.Last(node(kb.NodeKindClaim, "clm-2")); got.Value != "0.85" {
		t.Errorf("a line whose value stood = %q, want 0.85", got.Value)
	}
	if got, _ := back.Last(oldTable); got.Value != "1.0" {
		t.Errorf("the line the old table priced = %q, want the claim's own table's 1.0", got.Value)
	}
	// The values the tables moved reached the write the run lands through, in
	// the order the lines stand.
	wantLandings := []landing{
		{item: node(kb.NodeKindClaim, "clm-1"), value: "0.85"},
		{item: node(kb.NodeKindClaim, "clm-3"), value: "1.0"},
	}
	if !reflect.DeepEqual(*landed, wantLandings) {
		t.Errorf("the re-pricing landed %+v, want %+v", *landed, wantLandings)
	}
	if got := back.Repriced(); got != 2 {
		t.Errorf("Repriced() = %d, want the two values the resume moved", got)
	}
	after, err := os.ReadFile(filepath.Join(l.Dir(), runFile))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("the re-pricing rewrote run.yaml:\n%s\n---\n%s", before, after)
	}
	record, err := os.ReadFile(filepath.Join(l.Dir(), repricedFile))
	if err != nil {
		t.Fatalf("the re-pricing left no %s: %v", repricedFile, err)
	}
	var got rePriced
	if err := yaml.Unmarshal(record, &got); err != nil {
		t.Fatal(err)
	}
	want := []rePricedLine{
		{Item: "clm-1", From: "0.30", To: "0.85"},
		{Item: "clm-3", From: "0.65", To: "1.0"},
	}
	if got.Run != "run-1" || !slices.Equal(got.Moved, want) {
		t.Errorf("the record = %+v, want run-1 with %+v", got, want)
	}

	// A second resume over the rewritten ledger moves nothing: it lands nothing,
	// writes no ledger and records nothing more.
	if err := os.Remove(filepath.Join(l.Dir(), repricedFile)); err != nil {
		t.Fatal(err)
	}
	back.Close()
	none, again := landingOn(nil)
	still := resumed(t, dir, again)
	defer still.Close()
	if len(*none) != 0 || still.Repriced() != 0 {
		t.Errorf("a re-derivation that moved nothing landed %+v and counted %d", *none, still.Repriced())
	}
	if _, err := os.Stat(filepath.Join(l.Dir(), repricedFile)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a re-derivation that moved nothing recorded %v", err)
	}
}

// TestResumeRepriceFailsWhereTheWriteIsRefused: a re-priced value the KB's write
// refuses fails the resume, naming the item, and leaves the ledger as it stood —
// no line claims a value the KB did not take — so the next resume re-derives the
// same move.
func TestResumeRepriceFailsWhereTheWriteIsRefused(t *testing.T) {
	dir := t.TempDir()
	l := kept(t, dir)
	claim := node(kb.NodeKindClaim, "clm-1")
	if err := l.Append(claim, landedEntry("0.30", kb.FaultOverreach)); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	_, refused := landingOn(errors.New("the register is read-only"))
	_, err := Resume(dir, header(), refused, nil)
	if err == nil || !strings.Contains(err.Error(), "re-price clm-1") || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("Resume over a refused re-price = %v, want the item and the write's reason", err)
	}
	recs := lines(t, filepath.Join(l.Dir(), linesFile))
	if len(recs) != 1 || recs[0]["value"] != "0.30" {
		t.Errorf("the ledger = %v, want the value its line recorded, untouched", recs)
	}
	if _, err := os.Stat(filepath.Join(l.Dir(), repricedFile)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a failed re-price recorded a move: %v", err)
	}
}

// TestResumeLeavesASkippedItemUnrepriced: a landed line whose item the resume
// skips — its value marked manually scored since it landed — is not re-derived:
// nothing lands on it and its line keeps its value, while a line beside it the
// table contradicts still moves.
func TestResumeLeavesASkippedItemUnrepriced(t *testing.T) {
	dir := t.TempDir()
	l := kept(t, dir)
	manual, model := node(kb.NodeKindClaim, "clm-1"), node(kb.NodeKindClaim, "clm-2")
	for _, it := range []query.Item{manual, model} {
		if err := l.Append(it, landedEntry("0.30", kb.FaultOverreach)); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	landed, land := landingOn(nil)
	back, err := Resume(dir, header(), land, func(it query.Item) bool { return itemKey(it) == itemKey(manual) })
	if err != nil {
		t.Fatal(err)
	}
	defer back.Close()
	if want := []landing{{item: model, value: "0.85"}}; !reflect.DeepEqual(*landed, want) {
		t.Errorf("the re-pricing landed %+v, want %+v alone", *landed, want)
	}
	if got, _ := back.Last(manual); got.Value != "0.30" {
		t.Errorf("the skipped item's line = %q, want the 0.30 it landed", got.Value)
	}
}

// TestResumeFoldsTheResultsAStoppedRunLeft: the result files a killed walk left
// fold on the next resume, before it re-prices and with no ask for them: an
// answered value lands, its capture and its line are recorded in the order of
// the walk that wrote them, and the files are removed. A result whose attempt
// the ledger already records is removed alone. A result for an item the resume
// skips lands nothing and records no line, its capture still appended, and the
// walk then records the item skipped.
func TestResumeFoldsTheResultsAStoppedRunLeft(t *testing.T) {
	src, ix, _ := runSet(t)
	dir := t.TempDir()
	l := kept(t, dir)
	answered, recorded, failed := node(kb.NodeKindClaim, "clm-aaaaaa"), node(kb.NodeKindClaim, "clm-bbbbbb"), node(kb.NodeKindClaim, "clm-cccccc")
	manual, moved := node(kb.NodeKindAnalysis, "ana-aaaaaa"), node(kb.NodeKindClaim, "clm-eeeeee")
	bounded := func(e Entry, digest string) Entry {
		e.Model, e.PromptSHA256, e.Started, e.Ended = "small", digest, "2026-10-20T10:01:00Z", "2026-10-20T10:01:05Z"
		return e
	}
	// The ledger as the killed invocation's start left it: a line the running
	// table re-prices, and the line of an attempt whose result file still
	// stands.
	if err := l.Append(moved, landedEntry("0.30", kb.FaultOverreach)); err != nil {
		t.Fatal(err)
	}
	recordedEntry := bounded(landedEntry("0.85", kb.FaultOverreach), "bb")
	if err := l.Append(recorded, recordedEntry); err != nil {
		t.Fatal(err)
	}
	// The result files, named by place in the walk: the fold takes them in that
	// order, 3 before 10, and not in their names' order.
	for _, r := range []struct {
		at int
		it query.Item
		e  Entry
	}{
		{10, answered, bounded(landedEntry("0.85", kb.FaultOverreach), "aa")},
		{0, recorded, recordedEntry},
		{12, manual, bounded(Entry{Status: StatusDone, Value: "1.0", Reply: "none"}, "dd")},
		{3, failed, bounded(Entry{Status: StatusFailed, Reply: "the provider closed the connection"}, "cc")},
	} {
		c := captureRecord{Run: l.run, Pass: passOf(r.it), identity: identityOf(r.it), PromptSHA256: r.e.PromptSHA256,
			Started: r.e.Started, Ended: r.e.Ended}
		if err := l.writeResult(r.at, resultRecord{Line: l.line(r.it, r.e), Capture: c}); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}

	skip := func(it query.Item) bool { return itemKey(it) == itemKey(manual) }
	landed, land := landingOn(nil)
	back, err := Resume(dir, header(), land, skip)
	if err != nil {
		t.Fatal(err)
	}
	defer back.Close()
	// The fold's landing comes before the re-price's.
	if want := []landing{{item: answered, value: "0.85"}, {item: moved, value: "0.85"}}; !reflect.DeepEqual(*landed, want) {
		t.Errorf("the resume landed %+v, want %+v", *landed, want)
	}
	if got := fieldOf(t, filepath.Join(l.Dir(), linesFile), "id"); !slices.Equal(got, []string{"clm-eeeeee", "clm-bbbbbb", "clm-cccccc", "clm-aaaaaa"}) {
		t.Errorf("the ledger's lines name %q, want the folded results behind the run's own, in walk order", got)
	}
	if got := fieldOf(t, filepath.Join(l.Dir(), capturesFile), "id"); !slices.Equal(got, []string{"clm-cccccc", "clm-aaaaaa", "ana-aaaaaa"}) {
		t.Errorf("the captures name %q, want every folded result's but the one already recorded, in walk order", got)
	}
	if _, err := os.Stat(filepath.Join(l.Dir(), resultsDir)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the fold left the result files: %v", err)
	}
	for it, want := range map[query.Item]string{answered: StatusDone, failed: StatusFailed, manual: ""} {
		if got := back.Status(it); got != want {
			t.Errorf("%s folded as %q, want %q", itemID(it), got, want)
		}
	}

	// The walk after the fold asks only the item whose result failed, and records
	// the skipped one skipped; its lines land behind the re-priced file.
	ask, prompts := asking(t, []asked{{title: "1| Claim C", reply: "none"}})
	res, err := (&Runner{Source: src, Index: ix, Items: []query.Item{answered, recorded, failed, manual}, Pass: query.PassRigor,
		Launch: sequentialLaunch(ask), Write: land, Skip: skip, Ledger: back, Model: "small"}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(*prompts) != 1 || res.Counts != (Counts{Total: 4, Done: 3, Skipped: 1}) {
		t.Errorf("the walk made %d calls, counts %+v; want the failed item asked alone and the skipped one recorded", len(*prompts), res.Counts)
	}
	if got := fieldOf(t, filepath.Join(l.Dir(), linesFile), "status"); !slices.Equal(got[len(got)-2:], []string{StatusDone, StatusSkipped}) {
		t.Errorf("the ledger's lines record %q, want the walk's two lines last", got)
	}
}

// landedEntry is a landed entry recording one charge of id.
func landedEntry(value, id string) Entry {
	return Entry{Status: StatusDone, Value: value, Reply: id + ": A span",
		Faults: []Charge{{ID: id, Passage: passageStatement, Citation: "A span", Offset: 4}}}
}

// TestResumeDropsTheTornLine: a line a kill left half-written never landed, so
// its item is asked again — and the line this resume writes for it is not glued
// behind the torn text.
func TestResumeDropsTheTornLine(t *testing.T) {
	dir := t.TempDir()
	l := kept(t, dir)
	done := node(kb.NodeKindClaim, "clm-1")
	if err := l.Append(done, Entry{Status: StatusDone, Value: "0.5"}); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	torn := node(kb.NodeKindClaim, "clm-2")
	f, err := os.OpenFile(filepath.Join(l.Dir(), linesFile), os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"run":"run-1","id":"clm-2","status":"do`); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	back := reopened(t, dir)
	if got := back.Status(done); got != StatusDone {
		t.Errorf("the complete line's item = %q, want %q: a torn tail drops only itself", got, StatusDone)
	}
	if got := back.Status(torn); got != "" {
		t.Errorf("the torn line's item = %q, want none: the item is asked again", got)
	}
	if err := back.Append(torn, Entry{Status: StatusDone, Value: "0.25"}); err != nil {
		t.Fatal(err)
	}
	if err := back.Close(); err != nil {
		t.Fatal(err)
	}
	if got := lines(t, filepath.Join(l.Dir(), linesFile)); len(got) != 2 {
		t.Errorf("the ledger holds %d records, want the surviving line and the re-asked one", len(got))
	}
}

// TestResumeRefusesAnotherRun: a run naming another KB root, another pass or a
// ledger this kbase does not read is refused, and so is a run that stands
// nowhere.
func TestResumeRefusesAnotherRun(t *testing.T) {
	dir := t.TempDir()
	kept(t, dir).Close()
	future := filepath.Join(dir, "score", "run-2")
	if err := os.MkdirAll(future, 0o755); err != nil {
		t.Fatal(err)
	}
	futureRun, err := yaml.Marshal(RunHeader{Run: "run-2", KBRoot: "/repo/kb-root", Format: ledgerFormat + 1,
		Pass: query.PassRigor, Provider: "local", Model: "small", Started: "2026-10-20T10:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(future, runFile), futureRun, 0o666); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		h    RunHeader
		want string
	}{
		{"another KB root", RunHeader{Run: "run-1", KBRoot: "/elsewhere/kb-root", Pass: query.PassRigor}, "/elsewhere/kb-root"},
		{"another pass", RunHeader{Run: "run-1", KBRoot: "/repo/kb-root", Pass: query.PassBearing}, "bearing"},
		{"a newer ledger", RunHeader{Run: "run-2", KBRoot: "/repo/kb-root", Pass: query.PassRigor}, "ledger format 2"},
		{"no run", RunHeader{Run: "run-9", KBRoot: "/repo/kb-root", Pass: query.PassRigor}, "is missing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Resume(dir, tc.h, landsAll, nil)
			var ref result.Refusal
			if !errors.As(err, &ref) {
				t.Fatalf("Resume(%s) = %v, want a refusal", tc.name, err)
			}
			if len(ref) != 1 || ref[0].Check != result.CheckUsage || ref[0].Key != "--resume" || !strings.Contains(ref[0].Detail, tc.want) {
				t.Errorf("the refusal = %v; want check %s, key --resume, detail naming %q", ref, result.CheckUsage, tc.want)
			}
		})
	}
}

// TestDryRunWritesTheItemList: a dry run leaves run.yaml and items.jsonl, one
// pending line per item, and no ledger.
func TestDryRunWritesTheItemList(t *testing.T) {
	dir := t.TempDir()
	l := kept(t, dir)
	items := []query.Item{node(kb.NodeKindClaim, "clm-1"), edge("clm-1", "clm-2", kb.RelationSupports)}
	if err := l.WriteItems(items); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(l.Dir(), linesFile)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a dry run wrote %s: %v", linesFile, err)
	}
	got := lines(t, filepath.Join(l.Dir(), itemsFile))
	if len(got) != 2 || got[0]["id"] != "clm-1" || got[1]["relation"] != kb.RelationSupports ||
		got[0]["status"] != StatusPending || got[1]["status"] != StatusPending {
		t.Errorf("items.jsonl = %v, want one pending line per item in set order", got)
	}
}

// standing is one run directory a retention case stands before the prune: its
// name, and its run.yaml's start — "" for a directory with no run.yaml,
// "garbage" for one that does not read as a header.
type standing struct{ name, started string }

// series is n runs of kind, named in order and each started a minute after the
// last from the minute at.
func series(kind string, n, at int) []standing {
	out := make([]standing, n)
	for i := range out {
		out[i] = standing{name: fmt.Sprintf("%s-%06d", kind, i), started: minute(at + i)}
	}
	return out
}

// minute is a start stamp m minutes into a fixed hour.
func minute(m int) string {
	return time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC).Add(time.Duration(m) * time.Minute).Format(time.RFC3339)
}

// TestNewestRunOrdersEqualStartsAsThePruneDoes: of two runs recording the same
// start, the newest is the later name — the one the prune keeps.
func TestNewestRunOrdersEqualStartsAsThePruneDoes(t *testing.T) {
	dir := t.TempDir()
	for _, run := range []string{"run-aaaaaa", "run-bbbbbb"} {
		h := header()
		h.Run = run
		l, err := Create(dir, h)
		if err != nil {
			t.Fatal(err)
		}
		l.Close()
	}
	run, started, err := NewestRun(dir)
	if err != nil {
		t.Fatal(err)
	}
	if run != "run-bbbbbb" || started != header().Started {
		t.Errorf("NewestRun = %s at %s, want run-bbbbbb at %s", run, started, header().Started)
	}
}

// TestPruneRunsKeepsTheNewestOfEachKind: an invocation keeps the run it names and
// the newest buildstore.KeptRuns-1 others of its own kind, by run.yaml's start
// with equal starts ordered by name, and removes the rest of that kind; another
// kind's runs and a directory with no readable start stand.
func TestPruneRunsKeepsTheNewestOfEachKind(t *testing.T) {
	tied := series("run", buildstore.KeptRuns, 10)
	tied[1].started = tied[0].started
	for _, tc := range []struct {
		name     string
		standing []standing
		keep     string
		removed  []string
	}{
		{
			name:     "a seventeenth real run removes the oldest real run",
			standing: series("run", buildstore.KeptRuns, 0),
			keep:     "run-new",
			removed:  []string{"run-000000"},
		},
		{
			name:     "a seventeenth dry run removes the oldest dry run and no real run",
			standing: append(series("run", buildstore.KeptRuns, 0), series("dry", buildstore.KeptRuns, 100)...),
			keep:     "dry-new",
			removed:  []string{"dry-000000"},
		},
		{
			name:     "resuming the oldest of seventeen keeps it and removes the second-oldest",
			standing: series("run", buildstore.KeptRuns+1, 0),
			keep:     "run-000000",
			removed:  []string{"run-000001"},
		},
		{
			name:     "equal starts prune the earlier name",
			standing: tied,
			keep:     "run-new",
			removed:  []string{"run-000000"},
		},
		{
			name: "a directory without a readable run.yaml stands",
			standing: append(series("run", buildstore.KeptRuns, 10),
				standing{name: "run-absent"}, standing{name: "run-garbage", started: "garbage"}),
			keep:    "run-new",
			removed: []string{"run-000000"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, s := range tc.standing {
				run := filepath.Join(RunsDir(dir), s.name)
				switch s.started {
				case "":
					if err := os.MkdirAll(run, 0o777); err != nil {
						t.Fatal(err)
					}
				case "garbage":
					if err := os.MkdirAll(run, 0o777); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(run, runFile), []byte("started: [not a stamp\n"), 0o666); err != nil {
						t.Fatal(err)
					}
				default:
					h := header()
					h.Run, h.Started = s.name, s.started
					l, err := Create(dir, h)
					if err != nil {
						t.Fatal(err)
					}
					l.Close()
				}
			}
			if err := PruneRuns(dir, tc.keep); err != nil {
				t.Fatal(err)
			}
			for _, s := range tc.standing {
				_, err := os.Stat(filepath.Join(RunsDir(dir), s.name))
				if gone := errors.Is(err, os.ErrNotExist); gone != slices.Contains(tc.removed, s.name) {
					t.Errorf("%s removed %v, want %v", s.name, gone, !gone)
				}
			}
		})
	}
}

// TestPruneRunsKeepsTheNewestSixteenScoreCaptures: of 18 captured score runs, each a
// .stdout.yaml and .stderr.log pair, the newest 16 pairs stand, by name; a build
// capture beside them stands.
func TestPruneRunsKeepsTheNewestSixteenScoreCaptures(t *testing.T) {
	dir := t.TempDir()
	reports := filepath.Join(dir, buildstore.ReportsDir)
	if err := os.MkdirAll(reports, 0o777); err != nil {
		t.Fatal(err)
	}
	stems := make([]string, 18)
	for i := range stems {
		stems[i] = fmt.Sprintf("%s20260101T0000%02dZ-r", CapturePrefix, i)
	}
	files := []string{buildstore.CapturePrefix + "old.stdout.yaml"}
	for _, stem := range stems {
		files = append(files, stem+".stdout.yaml", stem+".stderr.log")
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(reports, f), nil, 0o666); err != nil {
			t.Fatal(err)
		}
	}
	if err := PruneRuns(dir, "run-new"); err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		_, err := os.Stat(filepath.Join(reports, f))
		gone := errors.Is(err, os.ErrNotExist)
		oldest := strings.HasPrefix(f, stems[0]) || strings.HasPrefix(f, stems[1])
		if gone != oldest {
			t.Errorf("%s removed %v, want %v", f, gone, oldest)
		}
	}
}
