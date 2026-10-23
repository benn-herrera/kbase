package score

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
// run.yaml that names it, the lines its attempts append, and the calls they
// made.
const (
	runsDir      = "score"
	runFile      = "run.yaml"
	linesFile    = "ledger.jsonl"
	capturesFile = "captures.jsonl"
	itemsFile    = "items.jsonl"
	pidFile      = "pid"
	// ResultFile is the name of the run's result document, written beside its
	// ledger so a waiter reads the same document the run reported.
	ResultFile   = "result.yaml"
	ledgerFormat = 1
)

// KeptRuns is how many runs of each kind — real or dry — the state directory
// keeps: the run an invocation names and the newest others.
const KeptRuns = 16

// The kinds of run: the prefix kb.MintID gives a run's id, which is the kind a
// run directory is counted under.
const (
	// RunKindReal is a run that asks and lands.
	RunKindReal = "run"
	// RunKindDry is a --dry-run's listing, which asks nothing.
	RunKindDry = "dry"
)

// checkUsage is the check a refusal names where the request itself is wrong.
const checkUsage = "usage"

// checkError names a failure kbase has no narrower class for, as a run read
// back from its ledger reports a failed item.
const checkError = "error"

// RunsDir is where the runs of the state directory dir stand: score/, one
// directory per run.
func RunsDir(dir string) string { return filepath.Join(dir, runsDir) }

// RunHeader is what run.yaml records of a run: which run over which KB asks
// what, what answered it, with what model, and when it started. Format is the
// ledger layout's version, and AskSHA256 the ask the run's judgements answered;
// Create stamps both and no resume rewrites them, so a caller leaves them zero.
//
// BackEnd is what answered the run's items — a pool entry or the environment's
// endpoint — and every header of a run that asked carries it, so a reader never
// infers the source from an absence. A dry run asks nothing and resolves no back
// end, so its header carries none: the empty string is not one of the two, and
// an absent key is how a reader tells a run that asked nothing from one whose
// back end was unnamed.
type RunHeader struct {
	Run    string `yaml:"run"`
	KBRoot string `yaml:"kb_root"`
	Format int    `yaml:"format"`
	// Pass names the set the run walks: one pass, or PassBoth where the walk
	// covers the node set and then the edge set. A ledger line names its own
	// item's pass, so a both-pass run's lines say which class each judgement
	// came from.
	Pass string `yaml:"pass"`
	// AskSHA256 names the ask set the run's answers belong to: one digest over
	// every instruction the run can send — a rigor run's statement and method
	// renders, a bearing run's one, a both-pass run's statement, method and
	// bearing renders in that order — each rendered for an empty subject and
	// concatenated in that fixed order, which is every fixed byte of the
	// instructions and nothing that varies with an item. The record carries the
	// ask and not the table, because a changed weight re-prices what a judgement
	// is worth while a changed ask is a different judgement — which this records
	// so a corpus can read the two apart. It gates nothing: an update applies its
	// behaviour. A header written before the rigor pair carries one render's
	// digest, and is read as the one ask that run began under, which is what it
	// recorded.
	AskSHA256 string `yaml:"ask_sha256"`
	BackEnd   string `yaml:"backend,omitempty"`
	Provider  string `yaml:"provider"`
	Model     string `yaml:"model"`
	Started   string `yaml:"started"`
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
	Run     string `json:"run"`
	Pass    string `json:"pass"`
	Backend string `json:"backend"`
	identity
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
	dir string
	run string
	// pass is the set the header names. A re-price reads its classes from it;
	// a line records its own item's pass instead.
	pass string
	// backEnd is what answers this invocation's items: the back end each line
	// it appends records.
	backEnd string
	// f is the line file and captures the capture file, each opened by the
	// first line appended to it.
	f        *os.File
	captures *os.File
	lines    map[string]Entry
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
// The header's format and ask are stamped here, from the set it names, so no
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
// another KB root or set, or a ledger format this kbase does not read. A run
// the update came between — a changed ask, a changed weight, a header written
// before the ask was recorded — is continued rather than refused: the values a
// changed weight moves are re-derived from the recorded ids and landed through w,
// the write a landing uses, and the items after them are asked under this
// kbase's ask. A line a killed process left torn did not land: it is dropped,
// and the item it names is asked again; a capture it left torn is dropped too,
// so this run's first capture is not appended behind it.
func Resume(dir string, h RunHeader, w Write) (*Ledger, error) {
	l := &Ledger{dir: filepath.Join(RunsDir(dir), h.Run), lines: map[string]Entry{}}
	have, err := readHeader(l.dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, result.Refusal{{Check: checkUsage, Path: l.dir, Key: "--resume",
			Remedy: "start a new run without --resume", Detail: "no run stands here: " + runFile + " is missing"}}
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.Join(l.dir, runFile), err)
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
	if _, err := trimTorn(filepath.Join(l.dir, capturesFile)); err != nil {
		return nil, err
	}
	if err := l.reprice(w); err != nil {
		return nil, err
	}
	return l, nil
}

// refusalFor is what a run standing here cannot be resumed as, or nil where it
// can: the run must ask over this KB, at this ledger format, over this set. A
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
	a := l.line(it, e)
	if err := l.appendLine(&l.f, linesFile, a); err != nil {
		return err
	}
	l.lines[a.lineKey()] = a.entry()
	return nil
}

// capture records the call one attempt at it made, in captures.jsonl: the closed
// text whole, the prompt sent and the reply returned, beside e's outcome. It is
// written before the attempt's ledger line, and only for an attempt that reached
// the Ask seam. A failed attempt's reason is the capture's error, where the
// ledger line carries it as its reply and keeps no text of the model's.
func (l *Ledger) capture(it query.Item, e Entry, c call) error {
	rec := captureRecord{Run: l.run, Pass: passOf(it), identity: identityOf(it), Class: c.class.name,
		Text: c.closed.text, Lines: capturedLines(c.closed.lines), Prompt: c.prompt, PromptSHA256: e.PromptSHA256,
		Reply: c.reply, Model: e.Model, Backend: l.backEnd, Started: e.Started, Ended: e.Ended}
	if e.Status != StatusDone {
		rec.Error = e.Reply
	}
	return l.appendLine(&l.captures, capturesFile, rec)
}

// captureRecord is one line of a run's captures.jsonl: the call one attempt
// made, whole enough to ask it again — the item and its class, the closed text
// and its lines as the call numbered them, the prompt sent, and what the Ask
// seam returned. Reply is "" where the call failed, and Error is the call's, the
// parse's or the write's failure, absent on a landing.
type captureRecord struct {
	Run  string `json:"run"`
	Pass string `json:"pass"`
	identity
	Class        string         `json:"class"`
	Text         string         `json:"text"`
	Lines        []capturedLine `json:"lines"`
	Prompt       string         `json:"prompt"`
	PromptSHA256 string         `json:"prompt_sha256"`
	Reply        string         `json:"reply"`
	Error        string         `json:"error,omitempty"`
	Model        string         `json:"model"`
	Backend      string         `json:"backend"`
	Started      string         `json:"started"`
	Ended        string         `json:"ended"`
}

// capturedLine is one line of a closed text as a capture records it: the fields
// of line, under the names the record spells.
type capturedLine struct {
	No   int    `json:"no"`
	End  string `json:"end"`
	From int    `json:"from"`
	To   int    `json:"to"`
}

// capturedLines is a closed text's lines as a capture records them.
func capturedLines(lines []line) []capturedLine {
	out := make([]capturedLine, len(lines))
	for i, l := range lines {
		out[i] = capturedLine{No: l.no, End: l.end, From: l.from, To: l.to}
	}
	return out
}

// readComplete is the JSONL file at path to its last newline, and the file's
// whole size. A line is complete once its newline landed; what a killed process
// left after the last one never became a record.
func readComplete(path string) (complete []byte, size int, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	return b[:bytes.LastIndexByte(b, '\n')+1], len(b), nil
}

// trimTorn is the JSONL file at path to its last newline, and drops what follows
// it from the file, so the next line appended is not joined to a torn one. No
// file standing is none. The caller holds the store's holder lock, so no run
// appends while the tail is dropped.
func trimTorn(path string) ([]byte, error) {
	b, size, err := readComplete(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(b) < size {
		if err := os.Truncate(path, int64(len(b))); err != nil {
			return nil, err
		}
	}
	return b, nil
}

// appendLine appends v as one JSON line to the run directory's file name,
// opening it into *f on first use, and syncs it: a line that returns has landed.
func (l *Ledger) appendLine(f **os.File, name string, v any) error {
	if *f == nil {
		opened, err := os.OpenFile(filepath.Join(l.dir, name), os.O_WRONLY|os.O_APPEND|os.O_CREATE, atomicfile.CreateMode)
		if err != nil {
			return err
		}
		*f = opened
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := (*f).Write(append(b, '\n')); err != nil {
		return err
	}
	return (*f).Sync()
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

// NewestRun is the run in the state directory dir whose header records the
// latest start, equal starts ordered as the prune orders them, and that stamp.
// A run directory whose run.yaml is absent, does not read as a header, or
// records a start this kbase cannot read is skipped: it left no record of when
// it started, and is no error. No run standing is "" and "".
func NewestRun(dir string) (run, started string, err error) {
	starts, err := dirStarts(RunsDir(dir), runFile)
	if err != nil || len(starts) == 0 {
		return "", "", err
	}
	newest := slices.MaxFunc(starts, compareStarts)
	return newest.name, newest.started, nil
}

// dirStart is one directory under a parent and the start its header records.
type dirStart struct {
	name, started string
	at            time.Time
}

// compareStarts orders two directories oldest first: by start, and equal starts
// by name, the later name newer.
func compareStarts(a, b dirStart) int {
	if c := a.at.Compare(b.at); c != 0 {
		return c
	}
	return strings.Compare(a.name, b.name)
}

// dirStarts is every directory under parent whose header — the file of that name
// inside it — records a start this kbase reads, in the parent's listing order. A
// directory whose header is absent, does not read as YAML, or records a start
// that does not parse left no record of when it started and is left out. No
// parent standing is no directory.
func dirStarts(parent, header string) ([]dirStart, error) {
	entries, err := os.ReadDir(parent)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []dirStart
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(parent, e.Name(), header))
		if err != nil {
			continue
		}
		var h struct {
			Started string `yaml:"started"`
		}
		if yaml.Unmarshal(b, &h) != nil {
			continue
		}
		at, err := time.Parse(time.RFC3339, h.Started)
		if err != nil {
			continue
		}
		out = append(out, dirStart{name: e.Name(), started: h.Started, at: at})
	}
	return out, nil
}

// PruneRuns keeps, of the runs in the state directory dir, the run keep names —
// created or resumed — and the newest KeptRuns-1 others of its kind, and
// removes the rest of that kind whole. A run's kind is its id's prefix, so a
// dry run never removes a real run nor a real run a dry one.
func PruneRuns(dir, keep string) error {
	return pruneByStart(RunsDir(dir), runFile, keep)
}

// pruneByStart keeps, of the directories under parent of keep's kind, keep and
// the newest KeptRuns-1 others in compareStarts' order, and removes the rest
// whole. A directory dirStarts leaves out has no start to order by, and stands.
func pruneByStart(parent, header, keep string) error {
	starts, err := dirStarts(parent, header)
	if err != nil {
		return err
	}
	kind := runKind(keep)
	var others []dirStart
	for _, s := range starts {
		if s.name != keep && runKind(s.name) == kind {
			others = append(others, s)
		}
	}
	slices.SortFunc(others, compareStarts)
	for _, s := range others[:max(0, len(others)-(KeptRuns-1))] {
		if err := os.RemoveAll(filepath.Join(parent, s.name)); err != nil {
			return fmt.Errorf("remove run %s: %w", s.name, err)
		}
	}
	return nil
}

// runKind is a run directory's kind: its name's prefix, before the first hyphen.
func runKind(name string) string {
	kind, _, _ := strings.Cut(name, "-")
	return kind
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
	h, err := readHeader(runDir)
	if err != nil {
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

// readHeader is the run.yaml in the run directory runDir.
func readHeader(runDir string) (RunHeader, error) {
	var h RunHeader
	b, err := os.ReadFile(filepath.Join(runDir, runFile))
	if err != nil {
		return h, err
	}
	return h, yaml.Unmarshal(b, &h)
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

// Close releases the line file and the capture file, where the run appended
// to either.
func (l *Ledger) Close() error {
	var errs []error
	for _, f := range []*os.File{l.f, l.captures} {
		if f != nil {
			errs = append(errs, f.Close())
		}
	}
	return errors.Join(errs...)
}

// readBack reads the lines a resume continues: the last status per item, a torn
// tail dropped.
func (l *Ledger) readBack() error {
	b, err := trimTorn(filepath.Join(l.dir, linesFile))
	if err != nil {
		return err
	}
	for _, line := range bytes.Split(b, []byte{'\n'}) {
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
	a := attempt{Run: l.run, Pass: passOf(it), Backend: l.backEnd, identity: identityOf(it), Status: e.Status,
		Value: e.Value, Reply: e.Reply, Model: e.Model, PromptSHA256: e.PromptSHA256,
		Started: e.Started, Ended: e.Ended}
	if e.Status == StatusDone {
		faults := e.Faults
		if faults == nil {
			faults = []Charge{}
		}
		a.Faults, a.Moot = &faults, e.Moot
	}
	return a
}

// identity is an item as a record's line names it: a node's kind and id, or an
// edge's source, target and relation, the other's keys omitted. The ledger's
// lines and the captures carry the one shape, so a capture joins to its line.
type identity struct {
	Kind     string `json:"kind,omitempty"`
	ID       string `json:"id,omitempty"`
	Source   string `json:"source,omitempty"`
	Target   string `json:"target,omitempty"`
	Relation string `json:"relation,omitempty"`
}

// identityOf is it as a line names it.
func identityOf(it query.Item) identity {
	switch {
	case it.Node != nil:
		return identity{Kind: it.Node.Kind, ID: it.Node.ID}
	case it.Edge != nil:
		return identity{Source: it.Edge.Source, Target: it.Edge.Target, Relation: it.Edge.Relation}
	}
	return identity{}
}

// passOf is the pass an item is a subject of: a node's is rigor, an edge's
// bearing. A line records its own item's rather than the run's set, so one
// walk over both says which class each judgement came from; a single-pass
// run's lines all name the run's pass, every item being its subject.
func passOf(it query.Item) string {
	switch {
	case it.Node != nil:
		return query.PassRigor
	case it.Edge != nil:
		return query.PassBearing
	}
	return ""
}

// itemKey is it as the ledger names it.
func itemKey(it query.Item) string { return identityOf(it).lineKey() }

// lineKey is an item's identity joined so no two items share one: a node's
// kind and id, then an edge's source, target and relation.
func (id identity) lineKey() string {
	return strings.Join([]string{id.Kind, id.ID, id.Source, id.Target, id.Relation}, "\x00")
}

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
	// name is the class as a capture records it.
	name string
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
	statementClass = class{name: "statement", ask: asks.ScoreRigorStatement, table: kb.RigorStatement}
	methodClass    = class{name: "method", ask: asks.ScoreRigorMethod, table: kb.RigorMethod}
	bearingClass   = class{name: "bearing", edge: true, ask: asks.ScoreBearing, table: kb.BearingFaults}
)

// passFacts is a run's set as it reads it: which class an item takes, and every
// class the set can send, in the fixed order its digest concatenates them. A
// pass is read here and nowhere else, so a call's instruction and its table are
// one derivation. A both-pass run sends all three classes, each item taking the
// one its own set gives it.
type passFacts struct {
	classOf func(it query.Item) (class, error)
	classes []class
}

// rigorClassOf is the class a rigor item takes: a claim's own statement, an
// analysis's or an experiment's method. A work's rigor is judged by hand and an
// axiom's is 1.0 by rule, so neither is a subject, and an edge is no rigor item.
func rigorClassOf(it query.Item) (class, error) {
	if it.Node == nil {
		return class{}, fmt.Errorf("the item %s is no %s subject", ItemName(it), query.PassRigor)
	}
	switch it.Node.Kind {
	case kb.NodeKindClaim:
		return statementClass, nil
	case kb.NodeKindAnalysis, kb.NodeKindExperiment:
		return methodClass, nil
	}
	return class{}, fmt.Errorf("the node %s is no %s subject: a work's rigor is judged by hand and an axiom's is 1.0 by rule", it.Node.ID, query.PassRigor)
}

// bearingClassOf is the class a bearing item takes: a scoring edge's link is
// judged on the edge itself, whatever its ends are typed, and a node is no
// bearing item.
func bearingClassOf(it query.Item) (class, error) {
	if it.Edge == nil {
		return class{}, fmt.Errorf("the item %s is no %s subject", ItemName(it), query.PassBearing)
	}
	return bearingClass, nil
}

// passFactsOf is pass's facts, or an error where no pass is named that. The
// class is derived from the item here, once: a rigor call's instruction and its
// closed set are the node's class's, a bearing call's the edge's, and an item
// the pass does not walk gets neither.
func passFactsOf(pass string) (passFacts, error) {
	switch pass {
	case query.PassRigor:
		return passFacts{classOf: rigorClassOf, classes: []class{statementClass, methodClass}}, nil
	case query.PassBearing:
		return passFacts{classOf: bearingClassOf, classes: []class{bearingClass}}, nil
	case query.PassBoth:
		return passFacts{
			classOf: func(it query.Item) (class, error) {
				if it.Edge != nil {
					return bearingClassOf(it)
				}
				return rigorClassOf(it)
			},
			classes: []class{statementClass, methodClass, bearingClass},
		}, nil
	}
	return passFacts{}, fmt.Errorf("no score pass is named %q", pass)
}

// askDigest is the digest of every instruction a run can send, each rendered for
// an empty subject and concatenated in the run's fixed order: the statement ask
// and then the method ask for a rigor run, the one bearing ask alone for a
// bearing run, and the statement, the method and then the bearing ask for a
// both-pass run. Rendering substitutes and never inspects the text, and no
// template carries an element keyed on the item's length or shape, so an empty
// subject covers every fixed byte of the instructions and nothing that varies
// with one. One field covers the set, because separate digests would agree or
// differ together.
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
func (id identity) item() string { return ItemName(id.asItem()) }

// asItem is the line's identity as the item a re-priced value lands on.
func (id identity) asItem() query.Item {
	if id.ID != "" {
		return query.Item{Node: &query.ScoringNode{Kind: id.Kind, ID: id.ID}}
	}
	return query.Item{Edge: &query.ScoringEdge{Source: id.Source, Target: id.Target, Relation: id.Relation}}
}
