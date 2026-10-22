package score

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"kbase/internal/asks"
	"kbase/internal/atomicfile"
	"kbase/internal/kb"
	"kbase/internal/query"
	"kbase/internal/result"
)

// The statuses a ledger line records.
const (
	// StatusDone is an item whose value reached the KB.
	StatusDone = "done"
	// StatusFailed is a call or a write that failed; nothing was written.
	StatusFailed = "failed"
	// StatusPending is what items.jsonl records of every item: a dry run's
	// listing, an attempt never made.
	StatusPending = "pending"
)

// The ledger's layout: one directory per run under the state directory, the
// run.yaml that names it, and the lines its attempts append.
const (
	runsDir   = "score"
	runFile   = "run.yaml"
	linesFile = "ledger.jsonl"
	itemsFile = "items.jsonl"
	pidFile   = "pid"
	// ResultFile is the name of the run's result document, written beside its
	// ledger so a waiter reads the same document the run reported.
	ResultFile   = "result.yaml"
	ledgerFormat = 1
)

// checkUsage is the check a refusal names where the request itself is wrong.
const checkUsage = "usage"

// checkError names a failure kbase has no narrower class for, as a run read
// back from its ledger reports a failed item.
const checkError = "error"

// RunsDir is where the runs of the state directory dir stand: score/, one
// directory per run, and the run lock beside them.
func RunsDir(dir string) string { return filepath.Join(dir, runsDir) }

// RunHeader is what run.yaml records of a run: which run over which KB asks
// what, what answered it, with what model, and when it started. Format is the
// ledger layout's version, and AskSHA256 the ask the run's judgements answered;
// Create stamps both and no resume rewrites them, so a caller leaves them zero.
//
// BackEnd is what answered the run's items — a pool entry, the environment's
// endpoint or a harness — and every header of a run that asked carries it, so a
// reader never infers the source from an absence. A dry run asks nothing and
// resolves no back end, so its header carries none: the empty string is not one
// of the three, and an absent key is how a reader tells a run that asked nothing
// from one whose back end was unnamed.
//
// Command is present under a harness alone: the argv that invoked it — how the
// run was invoked, never the text of one item. CommandOverride names the
// environment variable that supplied that argv where one did, and is empty where
// kbase composed the command itself, so a run whose command nobody chose says so
// in its own record.
type RunHeader struct {
	Run    string `yaml:"run"`
	KBRoot string `yaml:"kb_root"`
	Format int    `yaml:"format"`
	Pass   string `yaml:"pass"`
	// AskSHA256 names the ask set the run's answers belong to: one digest over
	// every instruction the pass can send — a rigor run's statement and method
	// renders, a bearing run's one — each rendered for an empty subject and
	// concatenated in that fixed order, which is every fixed byte of the
	// instructions and nothing that varies with an item. The record carries the
	// ask and not the table, because a changed weight re-prices what a judgement
	// is worth while a changed ask is a different judgement — which this records
	// so a corpus can read the two apart. It gates nothing: an update applies its
	// behaviour. A header written before the rigor pair carries one render's
	// digest, and is read as the one ask that run began under, which is what it
	// recorded.
	AskSHA256       string   `yaml:"ask_sha256"`
	BackEnd         string   `yaml:"backend,omitempty"`
	Provider        string   `yaml:"provider"`
	Model           string   `yaml:"model"`
	Command         []string `yaml:"command,omitempty"`
	CommandOverride string   `yaml:"command_override,omitempty"`
	Started         string   `yaml:"started"`
}

// Entry is what one attempt at an item records: its status and the evidence
// behind it. The line's run, pass and item identity are the ledger's, and
// Append takes the item's.
//
// Faults and Moot are a reply's parsed charges: the answer's own, and the lines
// a pending answer made moot. They are recorded only where the value landed —
// an empty list always carries the model's positive answer that it found no
// fault — and a failed attempt records neither, nor any text of the reply.
type Entry struct {
	Status       string
	Value        string // the value as it landed
	Faults       []Charge
	Moot         []Charge
	Reply        string // the model's reply where it parsed, or the reason on a failed line
	Model        string
	PromptSHA256 string // the digest of the text the call carried
	Started      string // RFC 3339
	Ended        string // RFC 3339
}

// attempt is one item's line in ledger.jsonl. It carries what answered its item
// — the back end beside the requested model — so a mixed or resumed run's lines
// say what answered each one.
//
// Faults is a pointer so that an empty list is written: a landed line always
// carries the charges it records, and `[]` is the model's answer that it found
// no fault. A failed line carries neither faults nor text.
type attempt struct {
	Run          string    `json:"run"`
	Pass         string    `json:"pass"`
	Backend      string    `json:"backend"`
	Kind         string    `json:"kind,omitempty"`
	ID           string    `json:"id,omitempty"`
	Source       string    `json:"source,omitempty"`
	Target       string    `json:"target,omitempty"`
	Relation     string    `json:"relation,omitempty"`
	Status       string    `json:"status"`
	Faults       *[]Charge `json:"faults,omitempty"`
	Moot         []Charge  `json:"moot,omitempty"`
	Value        string    `json:"value,omitempty"`
	Reply        string    `json:"reply,omitempty"`
	Model        string    `json:"model,omitempty"`
	PromptSHA256 string    `json:"prompt_sha256,omitempty"`
	Started      string    `json:"started,omitempty"`
	Ended        string    `json:"ended,omitempty"`
}

// Ledger is one run's record: the run.yaml that names it and a line per
// attempt. It is a record, not a cache — a fresh run reads none of it back and
// asks every item again.
//
// The back end a line records is the one this invocation resolved, not the
// header's: run.yaml is written once and never rewritten, so a resumed run's
// header names the first invocation's back end while each line names the one
// that asked it.
type Ledger struct {
	dir  string
	run  string
	pass string
	// backEnd is what answers this invocation's items: the back end each line
	// it appends records.
	backEnd string
	f       *os.File
	lines   map[string]Entry
	// order is every complete line a resume read back, in the file's order: a
	// re-derivation rewrites the file, and the last status per item does not say
	// where a line stands.
	order []attempt
	// moved is what this resume's re-pricing landed differently from what its
	// lines recorded: the count says whether the KB holds a write this
	// invocation must publish.
	moved []rePricedLine
}

// Create starts the ledger of a run in the state directory dir, at
// dir/score/<run>/, and writes its run.yaml. The line file is not created
// until an attempt appends one: a run that asks nothing writes no ledger. A run
// directory that already stands is not reused, so a fresh run never appends
// behind a recorded one.
//
// The header's format and ask are stamped here, from the pass it names, so no
// caller can open a run without the ask its judgements answer. The templates are
// embedded, so a render that fails is a defect of the build and not a user's
// refusal: the error is returned rather than a run written with no digest.
func Create(dir string, h RunHeader) (*Ledger, error) {
	l := &Ledger{dir: filepath.Join(RunsDir(dir), h.Run), run: h.Run, pass: h.Pass,
		backEnd: h.BackEnd, lines: map[string]Entry{}}
	if err := os.MkdirAll(RunsDir(dir), 0o777); err != nil {
		return nil, err
	}
	if err := os.Mkdir(l.dir, 0o777); err != nil {
		return nil, fmt.Errorf("start run %s: %w", h.Run, err)
	}
	ask, err := askDigest(h.Pass)
	if err != nil {
		return nil, err
	}
	h.Format, h.AskSHA256 = ledgerFormat, ask
	b, err := yaml.Marshal(h)
	if err != nil {
		return nil, err
	}
	if err := atomicfile.Write(filepath.Join(l.dir, runFile), b, nil); err != nil {
		return nil, err
	}
	return l, nil
}

// Resume opens the ledger of an existing run in the state directory dir for
// appending, its last status per item read back. It refuses a run.yaml naming
// another KB root or pass, or a ledger format this kbase does not read. A run
// the update came between — a changed ask, a changed weight, a header written
// before the ask was recorded — is continued rather than refused: the values a
// changed weight moves are re-derived from the recorded ids and landed through w,
// the write a landing uses, and the items after them are asked under this
// kbase's ask. A line a killed process left torn did not land: it is dropped,
// and the item it names is asked again.
func Resume(dir string, h RunHeader, w Write) (*Ledger, error) {
	l := &Ledger{dir: filepath.Join(RunsDir(dir), h.Run), lines: map[string]Entry{}}
	path := filepath.Join(l.dir, runFile)
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, result.Refusal{{Check: checkUsage, Path: l.dir, Key: "--resume",
			Remedy: "start a new run without --resume", Detail: "no run stands here: " + runFile + " is missing"}}
	}
	if err != nil {
		return nil, err
	}
	var have RunHeader
	if err := yaml.Unmarshal(b, &have); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if refusal := l.refusalFor(have, h); refusal != nil {
		return nil, refusal
	}
	// The header's own facts are h's, not the file's: the file names the run's
	// opening, and this invocation's lines name what asks now.
	l.run, l.pass, l.backEnd = have.Run, have.Pass, h.BackEnd
	if err := l.readBack(); err != nil {
		return nil, err
	}
	if err := l.reprice(w); err != nil {
		return nil, err
	}
	return l, nil
}

// refusalFor is what a run standing here cannot be resumed as, or nil where it
// can: the run must ask over this KB, at this ledger format, in this pass. A
// header recording another ask is not one of them, nor is one recording none:
// the digest says which instruction answered which judgement, so a corpus
// spanning an update can be read honestly, and a resume continues under this
// kbase's ask because an update applies its behaviour.
func (l *Ledger) refusalFor(have, want RunHeader) error {
	if have.Format == ledgerFormat && have.KBRoot == want.KBRoot && have.Pass == want.Pass {
		return nil
	}
	return result.Refusal{{Check: checkUsage, Path: l.dir, Key: "--resume",
		Remedy: "start a new run without --resume",
		Detail: fmt.Sprintf("the run here asks %s over %s at ledger format %d; this one asks %s over %s at ledger format %d",
			have.Pass, have.KBRoot, have.Format, want.Pass, want.KBRoot, ledgerFormat)}}
}

// Dir is the run's directory: the run.yaml that names it and the lines that
// record it.
func (l *Ledger) Dir() string { return l.dir }

// Repriced is how many landed values this resume re-derived differently and
// wrote back to the KB, zero where the run's values all stood.
func (l *Ledger) Repriced() int { return len(l.moved) }

// Status is the last status the ledger records for it, or "" where it records
// none: an item the run has not attempted.
func (l *Ledger) Status(it query.Item) string { return l.lines[itemKey(it)].Status }

// Last is the entry the ledger's last line records for it, and false where it
// records none: the status, and the evidence the line carries behind it.
func (l *Ledger) Last(it query.Item) (Entry, bool) {
	e, ok := l.lines[itemKey(it)]
	return e, ok
}

// Append records one attempt at it. The line lands after the item's write and
// validation, never before: a crash between the two re-asks an item whose
// write is idempotent, where the reverse would skip a value that never landed.
func (l *Ledger) Append(it query.Item, e Entry) error {
	if l.f == nil {
		f, err := os.OpenFile(filepath.Join(l.dir, linesFile), os.O_WRONLY|os.O_APPEND|os.O_CREATE, atomicfile.CreateMode)
		if err != nil {
			return err
		}
		l.f = f
	}
	a := l.line(it, e)
	b, err := json.Marshal(a)
	if err != nil {
		return err
	}
	if _, err := l.f.Write(append(b, '\n')); err != nil {
		return err
	}
	if err := l.f.Sync(); err != nil {
		return err
	}
	l.lines[a.lineKey()] = a.entry()
	return nil
}

// WriteItems writes the run's item list, items.jsonl: one pending line per
// item of the set the run enumerated, so a reader counts the whole run —
// status and a waiter read it — rather than only the attempts made so far.
func (l *Ledger) WriteItems(items []query.Item) error {
	var b bytes.Buffer
	for _, it := range items {
		a, err := json.Marshal(l.line(it, Entry{Status: StatusPending}))
		if err != nil {
			return err
		}
		b.Write(append(a, '\n'))
	}
	return atomicfile.Write(filepath.Join(l.dir, itemsFile), b.Bytes(), nil)
}

// WritePID records the process walking this run, so a waiter can find the run
// by the pid it was given. A resumed run overwrites the record with its own
// process: the pid names the invocation in flight, not the run's first.
func (l *Ledger) WritePID(pid int) error {
	return atomicfile.Write(filepath.Join(l.dir, pidFile), []byte(strconv.Itoa(pid)), nil)
}

// ClearResult removes a result an earlier invocation left: a run in progress is
// not finished, and a stale result would report an outcome that has not happened.
func (l *Ledger) ClearResult() error {
	err := os.Remove(filepath.Join(l.dir, ResultFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// WriteResultDocument writes the run's result document, its outcome and fields
// as the verb emits them, to the run directory dir so a waiter reads the same
// document the run reported.
func WriteResultDocument(dir, outcome string, fields []result.Field) error {
	var b bytes.Buffer
	if err := result.Emit(&b, outcome, fields...); err != nil {
		return err
	}
	return atomicfile.Write(filepath.Join(dir, ResultFile), b.Bytes(), nil)
}

// RunByPID is the run in the state directory dir whose process is pid, or
// os.ErrNotExist where no run names it.
func RunByPID(dir string, pid int) (string, error) {
	entries, err := os.ReadDir(RunsDir(dir))
	if errors.Is(err, os.ErrNotExist) {
		return "", os.ErrNotExist
	}
	if err != nil {
		return "", err
	}
	want := strconv.Itoa(pid)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(RunsDir(dir), e.Name(), pidFile))
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(b)) == want {
			return e.Name(), nil
		}
	}
	return "", os.ErrNotExist
}

// RunStatus is a run read back from its records: the items by the status the
// ledger last records for each, the items whose value landed, and the items
// that failed with the reason their line recorded.
type RunStatus struct {
	Run      string
	Pass     string
	Counts   Counts
	Scored   []string
	Failures []result.Item
}

// ReadRun reads the run in the state directory dir without opening it for
// appending: its header, its item list and its ledger lines. The item list is
// the set the run enumerated; a line the list does not carry is an item a
// resume asked outside it, and is counted too, the ledger being the record.
func ReadRun(dir, run string) (RunStatus, error) {
	runDir := filepath.Join(RunsDir(dir), run)
	var h RunHeader
	b, err := os.ReadFile(filepath.Join(runDir, runFile))
	if err != nil {
		return RunStatus{}, err
	}
	if err := yaml.Unmarshal(b, &h); err != nil {
		return RunStatus{}, err
	}
	items, err := readAttempts(filepath.Join(runDir, itemsFile))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return RunStatus{}, err
	}
	lines, err := readAttempts(filepath.Join(runDir, linesFile))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return RunStatus{}, err
	}
	last := make(map[string]attempt, len(lines))
	for _, a := range lines {
		last[a.lineKey()] = a
	}
	st := RunStatus{Run: h.Run, Pass: h.Pass}
	seen := make(map[string]bool, len(items))
	count := func(a attempt, ok bool) {
		st.Counts.Total++
		if !ok {
			st.Counts.Remaining++
			return
		}
		switch a.Status {
		case StatusDone:
			st.Counts.Done++
			st.Scored = append(st.Scored, a.item())
		case StatusFailed:
			st.Counts.Failed++
			st.Failures = append(st.Failures, result.Item{Check: checkError,
				Detail: a.item() + ": " + ReplySummary(a.Reply)})
		default:
			st.Counts.Remaining++
		}
	}
	for _, it := range items {
		key := it.lineKey()
		if seen[key] {
			continue
		}
		seen[key] = true
		a, ok := last[key]
		count(a, ok)
	}
	for _, a := range lines {
		key := a.lineKey()
		if seen[key] {
			continue
		}
		seen[key] = true
		count(a, true)
	}
	return st, nil
}

// readAttempts reads one JSONL file of attempts; a line no attempt reads as is
// dropped.
func readAttempts(path string) ([]attempt, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []attempt
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<16), 1<<24)
	for sc.Scan() {
		var a attempt
		if json.Unmarshal(sc.Bytes(), &a) == nil {
			out = append(out, a)
		}
	}
	return out, sc.Err()
}

// Close releases the line file, where the run appended one.
func (l *Ledger) Close() error {
	if l.f == nil {
		return nil
	}
	return l.f.Close()
}

// readBack reads the lines a resume continues: the last status per item. The
// caller holds the run lock, so nothing else appends while a torn tail is
// dropped.
func (l *Ledger) readBack() error {
	path := filepath.Join(l.dir, linesFile)
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	// A line is complete once its newline landed. What a killed process left
	// after the last newline never became a record, and is dropped here so this
	// run's first line is not appended behind it.
	complete := int64(bytes.LastIndexByte(b, '\n')) + 1
	if complete < int64(len(b)) {
		if err := os.Truncate(path, complete); err != nil {
			return err
		}
	}
	for _, line := range bytes.Split(b[:complete], []byte{'\n'}) {
		var a attempt
		if json.Unmarshal(line, &a) != nil {
			continue
		}
		l.order = append(l.order, a)
		l.lines[a.lineKey()] = a.entry()
	}
	return nil
}

// line is it as the ledger's line records it: the identity from the item, the
// run's own fields from the header, and the attempt's outcome from e. The
// charges are a landed line's alone: a failed attempt records the reason it
// failed and keeps no text of the reply, so it carries neither faults nor moot.
func (l *Ledger) line(it query.Item, e Entry) attempt {
	a := attempt{Run: l.run, Pass: l.pass, Backend: l.backEnd, Status: e.Status,
		Value: e.Value, Reply: e.Reply, Model: e.Model, PromptSHA256: e.PromptSHA256,
		Started: e.Started, Ended: e.Ended}
	if e.Status == StatusDone {
		faults := e.Faults
		if faults == nil {
			faults = []Charge{}
		}
		a.Faults, a.Moot = &faults, e.Moot
	}
	switch {
	case it.Node != nil:
		a.Kind, a.ID = it.Node.Kind, it.Node.ID
	case it.Edge != nil:
		a.Source, a.Target, a.Relation = it.Edge.Source, it.Edge.Target, it.Edge.Relation
	}
	return a
}

// itemKey is it as the ledger names it.
func itemKey(it query.Item) string {
	switch {
	case it.Node != nil:
		return lineKey(it.Node.Kind, it.Node.ID, "", "", "")
	case it.Edge != nil:
		return lineKey("", "", it.Edge.Source, it.Edge.Target, it.Edge.Relation)
	}
	return ""
}

// lineKey is an item's identity joined so no two items share one: a node's
// kind and id, then an edge's source, target and relation.
func lineKey(kind, id, source, target, relation string) string {
	return strings.Join([]string{kind, id, source, target, relation}, "\x00")
}

func (a attempt) lineKey() string { return lineKey(a.Kind, a.ID, a.Source, a.Target, a.Relation) }

// entry is the line as the ledger hands it back: its status and the evidence
// behind it, which is what Append stored and what a resume reads.
func (a attempt) entry() Entry {
	return Entry{Status: a.Status, Value: a.Value, Faults: a.faults(), Moot: a.Moot,
		Reply: a.Reply, Model: a.Model, PromptSHA256: a.PromptSHA256,
		Started: a.Started, Ended: a.Ended}
}

// faults is the charges the line records, nil where it records none.
func (a attempt) faults() []Charge {
	if a.Faults == nil {
		return nil
	}
	return *a.Faults
}

// class is the pair one item's call is: the instruction it carries and the table
// its reply is priced under. A node's class fixes both at once, so what an item
// is asked and what its charges cost cannot come apart, and the composer, the
// parse's closed set and the price all read this one fact.
type class struct {
	// edge is what the class is judged on: a scoring edge's two ends and its
	// context for a bearing call, a node's own title and statement for either
	// rigor call.
	edge  bool
	ask   func(subject string) (string, error)
	table kb.FaultTable
}

// The three classes: a claim's own text is a statement, an analysis's or an
// experiment's a method, and a scoring edge's link is a bearing whatever its ends
// are typed.
var (
	statementClass = class{ask: asks.ScoreRigorStatement, table: kb.RigorStatement}
	methodClass    = class{ask: asks.ScoreRigorMethod, table: kb.RigorMethod}
	bearingClass   = class{edge: true, ask: asks.ScoreBearing, table: kb.BearingFaults}
)

// passFacts is one pass as a run reads it: which class an item takes, and every
// class the pass can send, in the fixed order its digest concatenates them. A
// pass is read here and nowhere else, so a call's instruction and its table are
// one derivation.
type passFacts struct {
	classOf func(it query.Item) (class, error)
	classes []class
}

// passFactsOf is pass's facts, or an error where no pass is named that. The
// class is derived from the item here, once: a rigor call's instruction and its
// closed set are the node's class's, and an item the pass does not walk gets
// neither.
func passFactsOf(pass string) (passFacts, error) {
	switch pass {
	case query.PassRigor:
		return passFacts{
			classOf: func(it query.Item) (class, error) {
				if it.Node == nil {
					return class{}, fmt.Errorf("the item %s is no %s subject", ItemName(it), pass)
				}
				switch it.Node.Kind {
				case kb.NodeKindClaim:
					return statementClass, nil
				case kb.NodeKindAnalysis, kb.NodeKindExperiment:
					return methodClass, nil
				}
				return class{}, fmt.Errorf("the node %s is no %s subject: a work's rigor is judged by hand and an axiom's is 1.0 by rule", it.Node.ID, pass)
			},
			classes: []class{statementClass, methodClass},
		}, nil
	case query.PassBearing:
		return passFacts{
			classOf: func(it query.Item) (class, error) {
				if it.Edge == nil {
					return class{}, fmt.Errorf("the item %s is no %s subject", ItemName(it), pass)
				}
				return bearingClass, nil
			},
			classes: []class{bearingClass},
		}, nil
	}
	return passFacts{}, fmt.Errorf("no score pass is named %q", pass)
}

// askDigest is the digest of every instruction a pass can send, each rendered for
// an empty subject and concatenated in the pass's fixed order: the statement ask
// and then the method ask for a rigor run, the one bearing ask alone for a bearing
// run. Rendering substitutes and never inspects the text, and no template carries
// an element keyed on the item's length or shape, so an empty subject covers every
// fixed byte of the instructions and nothing that varies with one. One field
// covers the set, because separate digests would agree or differ together.
func askDigest(pass string) (string, error) {
	facts, err := passFactsOf(pass)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, cl := range facts.classes {
		prompt, err := cl.ask("")
		if err != nil {
			return "", err
		}
		b.WriteString(prompt)
	}
	return digestOf(b.String()), nil
}

// rePriced is the record the run's directory gains where a resume re-derived a
// landed value and landed it: the run, when, and the lines whose value the
// running table now gives differently. run.yaml is written once and is not the
// place for it.
type rePriced struct {
	Run      string         `yaml:"run"`
	Repriced string         `yaml:"repriced"`
	Moved    []rePricedLine `yaml:"moved"`
}

// rePricedLine is one line a re-derivation moved: the item as the ledger names
// it, the value the line recorded and the value the table gives it.
type rePricedLine struct {
	Item string `yaml:"item"`
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

// repricedFile is the record a re-pricing leaves in the run's directory.
const repricedFile = "repriced.yaml"

// reprice re-derives every landed value from the ids its line records, under the
// table the line's own class holds, lands each value a re-derivation moves
// through w — the write a landing uses, under the lock that landing holds — and
// rewrites the lines it moved. It asks no model: the model never saw a weight, the
// ids are the whole input of the price, and a weight that changed between
// invocations re-prices a run rather than making it incomparable. A run whose
// values all stand writes nothing.
//
// A moved value is landed before its line records it, so the record never claims
// a value the KB did not take: a refused write fails the resume with the ledger
// as it stood, and the next resume re-derives the same move, the write being
// idempotent.
func (l *Ledger) reprice(w Write) error {
	facts, err := passFactsOf(l.pass)
	if err != nil {
		return err
	}
	var moved []rePricedLine
	for i := range l.order {
		a := &l.order[i]
		if a.Status != StatusDone || a.Faults == nil {
			// A line this build wrote always records its charges; one that
			// records none carries no input to re-derive from, and is left as it
			// stands.
			continue
		}
		// The price is the item's class's, derived from the line the way a call's
		// is: a run landed under one rigor table re-prices under the class's.
		cl, err := facts.classOf(a.asItem())
		if err != nil {
			return fmt.Errorf("re-price %s: %w", a.item(), err)
		}
		ids := make([]string, 0, len(a.faults()))
		for _, c := range a.faults() {
			ids = append(ids, c.ID)
		}
		value := cl.table.Price(ids)
		want := kb.BearingText(value)
		if a.Value == want {
			continue
		}
		if err := w(a.asItem(), value); err != nil {
			return fmt.Errorf("re-price %s: %w", a.item(), err)
		}
		moved = append(moved, rePricedLine{Item: a.item(), From: a.Value, To: want})
		a.Value = want
		l.lines[a.lineKey()] = a.entry()
	}
	l.moved = moved
	if len(moved) == 0 {
		return nil
	}
	var b bytes.Buffer
	for _, a := range l.order {
		line, err := json.Marshal(a)
		if err != nil {
			return err
		}
		b.Write(append(line, '\n'))
	}
	if err := atomicfile.Write(filepath.Join(l.dir, linesFile), b.Bytes(), nil); err != nil {
		return err
	}
	record, err := yaml.Marshal(rePriced{Run: l.run, Repriced: time.Now().UTC().Format(time.RFC3339), Moved: moved})
	if err != nil {
		return err
	}
	return atomicfile.Write(filepath.Join(l.dir, repricedFile), record, nil)
}

// item is the item as the ledger names it in prose: the documents' own spelling,
// so a `repriced.yaml` line joins to the result's `scored` without a translation.
func (a attempt) item() string { return ItemName(a.asItem()) }

// asItem is the line's identity as the item a re-priced value lands on.
func (a attempt) asItem() query.Item {
	if a.ID != "" {
		return query.Item{Node: &query.ScoringNode{Kind: a.Kind, ID: a.ID}}
	}
	return query.Item{Edge: &query.ScoringEdge{Source: a.Source, Target: a.Target, Relation: a.Relation}}
}
