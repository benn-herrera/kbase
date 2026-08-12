package pipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"kbase/internal/log"
	"kbase/internal/pipeline/crashpoint"
	"kbase/internal/version"
)

const (
	// StampSuffix names the stamp sidecar beside the artifact it stamps:
	// `leaves/intro.md` is stamped by `leaves/intro.md.stamp.json`
	// (ARCHITECTURE.md §9).
	//
	// Sidecar rather than one manifest, for three reasons that all reduce to
	// the same one — a manifest is shared mutable state:
	//   - Stage 5 fans out across domain workers. A manifest makes every
	//     worker's write a read-modify-write of one file, which is either a
	//     lock or a lost update.
	//   - The atomic unit of the design is (artifact, proof). A sidecar keeps
	//     those two files and nothing else in the unit; a torn manifest would
	//     put every unit's proof at risk to one bad write.
	//   - Absent-vs-Invalid falls out for free: no artifact and no sidecar is
	//     Absent, either one alone is Invalid. A manifest has to encode that
	//     distinction rather than exhibit it.
	// The cost is an inode per unit and a directory listing with twice the
	// entries, which is not a cost at this scale.
	StampSuffix = ".stamp.json"

	// stampSchema is the sidecar's format version. App version already
	// covers schema change in a release, but a development tree keeps one
	// version string across many edits, and a stamp from an older shape
	// would decode into the current struct with zeroed new fields — a false
	// Valid at exactly the moment the format is in flux. Bumping this is
	// what makes that case Invalid.
	stampSchema = 1

	// ArtifactFileMode is the mode of every file kbase writes under an output
	// directory. A job dir holds a user's corpus in derived form; it gets the
	// same owner-only posture as the log file and the provider key files.
	//
	// It is exported because `cmd` writes beside the store — a survey
	// artifact, a run record — and an unexported constant there would be a
	// second declaration of one fact (ARCHITECTURE.md §9).
	ArtifactFileMode = 0o600

	// ArtifactDirMode is the mode of directories kbase creates under an
	// output directory — owner-only, matching the files inside them.
	ArtifactDirMode = 0o700

	// tempSuffix starts the name of the temporary file a write lands in
	// before the rename. It sits in the destination directory so the rename
	// stays within one filesystem and is therefore atomic. The suffix keeps
	// the artifact's own name as the prefix, so a temp file left behind by a
	// killed run is identifiable at a glance.
	tempSuffix = ".tmp"
)

// Crashpoints around store writes. Between pre-rename and pre-stamp the
// artifact exists with no proof, which is precisely the state resume must
// verdict Invalid rather than trust; the harness kills here to prove it does.
var (
	cpPutPreRename = crashpoint.Register("pipeline.store.put.pre-rename")
	cpPutPreStamp  = crashpoint.Register("pipeline.store.put.pre-stamp")
	cpPutDone      = crashpoint.Register("pipeline.store.put.done")
)

// Verdict is the resume vocabulary (ARCHITECTURE.md §12): what an inspection
// of one artifact concluded. It is a string type so a verdict reads as itself
// in a log record.
//
// There are exactly three because reuse requires affirmative proof: an
// artifact is reused only on VerdictValid, and both other verdicts mean
// "produce it again". They stay distinct because the forensics differ —
// Absent is an interrupted run, Invalid is a run whose output can no longer
// be trusted, and a resume inventory full of the latter is a signal.
type Verdict string

const (
	// VerdictValid means the artifact is present and its stamp proves it
	// was produced by this app version from these exact inputs.
	VerdictValid Verdict = "valid"
	// VerdictAbsent means neither artifact nor stamp is there.
	VerdictAbsent Verdict = "absent"
	// VerdictInvalid means present but unproven: any doubt at all lands
	// here — missing or unparseable stamp, any hash mismatch, version
	// skew. Redoing a unit costs tokens; trusting an unproven one costs
	// the artifact's integrity.
	VerdictInvalid Verdict = "invalid"
)

// Input is one named hash a stage artifact was derived from — the source
// corpus's content hash, an upstream artifact's output hash, a stage's
// parameter digest. The name is what makes a mismatch diagnosable ("the
// skeleton changed" rather than "hash 3 differs").
type Input struct {
	Name string `json:"name"`
	Hash string `json:"hash"`
}

// Stamp is the on-disk proof beside an artifact: what produced it, from
// what, and what came out. It is the whole of the resume state — there is no
// journal and no cursor, so validity is decidable by inspection with no
// knowledge of how the previous run died.
//
// Its JSON encoding is deterministic: fixed field order, inputs sorted by
// name at write, no maps anywhere.
type Stamp struct {
	// Schema is stampSchema at write time.
	Schema int `json:"schema"`
	// Version is the app version that wrote the artifact. Embedded prompts
	// are implied by app version (§3), so a skew invalidates outputs whose
	// prompt bytes we can no longer reproduce.
	Version string `json:"version"`
	// Path is the store-relative path this stamp belongs to. It is
	// self-describing on purpose: a sidecar that names a different artifact
	// than the one it sits beside means the layout was rearranged under us,
	// which is structural incoherence rather than a stale unit.
	Path string `json:"path"`
	// Inputs are the named hashes the artifact was derived from, sorted by
	// name.
	Inputs []Input `json:"inputs"`
	// Output is the sha256 of the artifact's bytes, which catches the
	// truncation and tampering an input-only stamp would miss.
	Output string `json:"output"`
}

// Store is stage-artifact custody rooted at one job directory — in a real
// run, the `temp-work` tree kbase created under the output directory (see
// TempWork). Every path it takes is store-relative and slash-separated, so a
// chain description reads the same on every platform, and the same relative
// path names the scratch copy and its delivered counterpart.
type Store struct {
	root string
	lg   log.Logger
}

// NewStore returns a store rooted at jobDir, logging through lg (which must
// be non-nil; log.Discard covers a caller with nothing to hand it). The
// directory is created lazily by the first write, so constructing a store
// touches no disk — a resume scan over a job dir that does not exist yet is a
// legitimate call that should report Absent, not fail.
func NewStore(jobDir string, lg log.Logger) *Store {
	return &Store{root: jobDir, lg: lg}
}

// HashBytes returns the hex sha256 of b — the one hash function on both
// sides of every comparison the store makes, exported so callers derive
// input hashes the same way.
func HashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// Put writes data to rel and its stamp beside it, atomically.
//
// The stamp is built here rather than accepted from the caller: app version
// and output hash are facts of the write, and a caller that could supply them
// could stamp a lie. The caller supplies only what it knows and the store
// cannot — the named input hashes (see Store.resolveInputs, which derives
// them for a chain unit so the writer and the verifier never derive them
// differently).
//
// Order is the contract: data lands first, the stamp second. A kill in
// between leaves an artifact with no proof, which verify reports Invalid and
// resume redoes. The reverse order could leave proof for bytes that were
// never written, which is the one failure this design must not have.
//
// Both writes are temp+rename+fsync. The rename is atomic against a
// concurrent reader with or without the sync; the sync is what keeps the new
// name from coming back pointing at an empty file after a power loss. The
// directory entry itself is durable only per the platform's own guarantees —
// kbase does not fsync directories — and that hole is SAFE rather than
// merely acknowledged, because of the ordering above: whichever directory
// entry a power loss drops, the outcome is one this design already redoes.
// Lose the artifact's entry and the stamp stands alone (Invalid); lose the
// stamp's and the artifact stands alone (Invalid); lose both and the unit is
// Absent. There is no ordering of losses that produces a stamp proving bytes
// that are not there, so every partial outcome lands on redo and none on a
// false Valid. Cloud-synced output directories are a different matter and are
// documented unsupported (§12): they break the rename/inode assumptions
// underneath all of this.
func (s *Store) Put(rel string, data []byte, inputs []Input) error {
	abs, err := s.resolve(rel)
	if err != nil {
		return err
	}

	stamp := Stamp{
		Schema:  stampSchema,
		Version: version.Current,
		Path:    rel,
		Inputs:  sortedInputs(inputs),
		Output:  HashBytes(data),
	}
	encoded, err := stamp.encode()
	if err != nil {
		return fmt.Errorf("pipeline: stamp %s: %w", rel, err)
	}

	if err := writeAtomic(abs, data, cpPutPreRename); err != nil {
		return fmt.Errorf("pipeline: write %s: %w", rel, err)
	}
	crashpoint.At(cpPutPreStamp)
	if err := writeAtomic(abs+StampSuffix, encoded, ""); err != nil {
		return fmt.Errorf("pipeline: write stamp for %s: %w", rel, err)
	}
	crashpoint.At(cpPutDone)

	s.lg.Debug("pipeline artifact written", "path", rel, "bytes", len(data), "output", stamp.Output)
	return nil
}

// Get returns the bytes of the artifact at rel — Put's counterpart, through
// the same path validation, so a chain description that could not have
// written a path cannot read one either.
//
// It reads and does not prove. Proof is the stamp's job and the scan's, and
// the two are already joined: a stage that consumes an upstream artifact names
// it in Unit.Upstreams, which puts that artifact's output hash in this unit's
// stamp, so a consumer that read bytes nobody proved produces a unit nothing
// will verdict Valid. Folding a verification in here would be the second
// derivation of a proof the store already has exactly one of.
//
// A missing artifact comes back wrapping fs.ErrNotExist, so "not there yet" is
// a case a caller can test for rather than a string it has to match.
func (s *Store) Get(rel string) ([]byte, error) {
	abs, err := s.resolve(rel)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("pipeline: read %s: %w", rel, err)
	}
	return data, nil
}

// verify inspects the artifact at rel against the inputs it should have been
// derived from and returns a verdict, a short reason for the log, and an
// error.
//
// The reason is always populated for a non-Valid verdict and is the text the
// resume forensics record. It carries identity and arithmetic, never content.
// An unreadable file or an unparseable stamp is folded into the reason rather
// than returned as an error, because at this seam an I/O failure and a hash
// mismatch mean the same thing operationally — the artifact is not proven, so
// it is produced again — and the write that follows will fail loudly with the
// real cause if the filesystem is genuinely broken.
//
// The error return is reserved for structural incoherence: the store
// contradicting itself in a way redoing the unit cannot fix (see
// IncoherentStoreError). It refuses the resume rather than verdicting it.
func (s *Store) verify(rel string, want []Input) (Verdict, string, error) {
	abs, err := s.resolve(rel)
	if err != nil {
		return VerdictInvalid, err.Error(), nil
	}

	// A directory where an artifact belongs is not a unit to redo: the
	// write that would "fix" it cannot succeed. That is layout, not
	// staleness.
	if info, err := os.Stat(abs); err == nil && info.IsDir() {
		return VerdictInvalid, "", IncoherentStoreError{Path: rel, Reason: "a directory sits where an artifact belongs"}
	}

	data, dataErr := os.ReadFile(abs)
	stamp, stampErr := s.readStamp(rel)
	switch {
	case errors.Is(dataErr, fs.ErrNotExist) && errors.Is(stampErr, fs.ErrNotExist):
		return VerdictAbsent, "no artifact and no stamp", nil
	case errors.Is(dataErr, fs.ErrNotExist):
		return VerdictInvalid, "stamp with no artifact beside it", nil
	case dataErr != nil:
		return VerdictInvalid, fmt.Sprintf("artifact unreadable: %v", dataErr), nil
	case errors.Is(stampErr, fs.ErrNotExist):
		return VerdictInvalid, "artifact with no stamp", nil
	case stampErr != nil:
		return VerdictInvalid, fmt.Sprintf("stamp unusable: %v", stampErr), nil
	}

	// A sidecar naming a different artifact than the one it sits beside
	// means the layout moved under us — a renamed directory, a job dir
	// assembled from two runs. Nothing about redoing this unit makes the
	// rest of the store trustworthy again.
	if stamp.Path != rel {
		return VerdictInvalid, "", IncoherentStoreError{
			Path:   rel,
			Reason: fmt.Sprintf("its stamp claims to belong to %q", stamp.Path),
		}
	}
	if stamp.Schema != stampSchema {
		return VerdictInvalid, fmt.Sprintf("stamp schema %d, this build writes %d", stamp.Schema, stampSchema), nil
	}
	if stamp.Version != version.Current {
		return VerdictInvalid, fmt.Sprintf("written by kbase %s, running %s", stamp.Version, version.Current), nil
	}
	if got := HashBytes(data); got != stamp.Output {
		return VerdictInvalid, "artifact bytes do not match their own stamp", nil
	}
	if reason := compareInputs(stamp.Inputs, sortedInputs(want)); reason != "" {
		return VerdictInvalid, reason, nil
	}
	return VerdictValid, "", nil
}

// readStamp returns the stamp sidecar for rel. A missing sidecar surfaces as
// an error wrapping fs.ErrNotExist, which is how verify tells absent from
// unparseable.
func (s *Store) readStamp(rel string) (Stamp, error) {
	abs, err := s.resolve(rel)
	if err != nil {
		return Stamp{}, err
	}
	raw, err := os.ReadFile(abs + StampSuffix)
	if err != nil {
		return Stamp{}, err
	}
	var st Stamp
	if err := json.Unmarshal(raw, &st); err != nil {
		return Stamp{}, fmt.Errorf("parse stamp for %s: %w", rel, err)
	}
	return st, nil
}

// validatePath refuses anything that is not a plain path inside the job dir
// and returns the platform-native form of it. The refusal is not defensive
// politeness: chain descriptions are code, and a path that escapes the root
// or collides with the stamp namespace is a defect that would otherwise land
// as a write outside the job dir. It is separate from resolve so a chain
// description can be checked before any store exists.
func validatePath(rel string) (string, error) {
	if rel == "" {
		return "", BadPathError{Path: rel, Reason: "empty"}
	}
	local := filepath.FromSlash(rel)
	if !filepath.IsLocal(local) {
		return "", BadPathError{Path: rel, Reason: "not a path inside the job directory"}
	}
	// Two spellings of one file ("a/../b" and "b") would defeat the chain's
	// uniqueness check and stamp the same artifact twice under different
	// names, so only the canonical spelling is a path here.
	if local != filepath.Clean(local) {
		return "", BadPathError{Path: rel, Reason: "not in canonical form"}
	}
	if strings.HasSuffix(rel, StampSuffix) {
		return "", BadPathError{Path: rel, Reason: "the " + StampSuffix + " suffix is reserved for stamps"}
	}
	// The lockfile is the store's own namespace too, and the consequence of
	// missing it is worse than a collision: Put would rename an artifact over
	// the live lock of the running job, Release would then succeed on a file
	// it no longer owns, and a second writer could take the job dir out from
	// under this one.
	if rel == LockFileName {
		return "", BadPathError{Path: rel, Reason: "the name " + LockFileName + " is reserved for the job lock"}
	}
	return local, nil
}

// resolve turns a store-relative path into an absolute one under the job dir.
func (s *Store) resolve(rel string) (string, error) {
	local, err := validatePath(rel)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.root, local), nil
}

// encode renders the stamp as its on-disk bytes: indented JSON with a
// trailing newline, so a sidecar is readable when someone is looking at a
// job dir trying to work out why a resume redid everything.
func (st Stamp) encode() ([]byte, error) {
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// sortedInputs returns inputs ordered by name, leaving the caller's slice
// alone — a sort in place would reorder a slice the caller still holds and
// may reuse for the next unit.
func sortedInputs(in []Input) []Input {
	out := slices.Clone(in)
	slices.SortFunc(out, func(a, b Input) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// compareInputs returns "" when two sorted input lists agree, and otherwise a
// reason naming the first disagreement. Names are reported, hashes are not:
// which input changed is the diagnosis, and 64 hex characters in a log line
// is noise.
func compareInputs(got, want []Input) string {
	for i := range max(len(got), len(want)) {
		switch {
		case i >= len(got):
			return fmt.Sprintf("stamp is missing input %q", want[i].Name)
		case i >= len(want):
			return fmt.Sprintf("stamp carries unexpected input %q", got[i].Name)
		case got[i].Name != want[i].Name:
			return fmt.Sprintf("stamp input %q where %q was expected", got[i].Name, want[i].Name)
		case got[i].Hash != want[i].Hash:
			return fmt.Sprintf("input %q changed", got[i].Name)
		}
	}
	return ""
}

// writeAtomic writes data to abs through a temporary file in the same
// directory, syncs it, and renames it into place. cp, when non-empty, is a
// crashpoint fired with the temporary file written and the rename not yet
// done.
func writeAtomic(abs string, data []byte, cp string) error {
	dir := filepath.Dir(abs)
	if err := os.MkdirAll(dir, ArtifactDirMode); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(abs)+tempSuffix+"*")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}

	// A real process kill before the rename leaves the temporary file on
	// disk; an in-process simulated one would run this cleanup as the panic
	// unwinds and erase the evidence. So on the crossing that kills, the
	// cleanup stands down and the residue survives, as it would in the case
	// being modeled.
	keep := false
	defer func() {
		if !keep {
			os.Remove(tmp.Name())
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", abs, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync %s: %w", abs, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", abs, err)
	}
	// os.CreateTemp opens at 0600, which is the mode these files want, so
	// there is nothing to chmod before the name becomes visible.

	if cp != "" {
		crashpoint.AtStaging(cp, func() { keep = true })
	}
	if err := os.Rename(tmp.Name(), abs); err != nil {
		return fmt.Errorf("install %s: %w", abs, err)
	}
	return nil
}

// sweep removes every file under the job directory that the chain does not
// account for: the temp-file residue a killed write left behind, and the
// artifacts of a previous run whose plan named different paths. Neither is
// visible to the resume scan, which only inspects paths the chain names, so
// without this they accumulate in the tree the emit step draws from.
//
// WHERE it runs is what makes deleting safe, and it is structural rather than
// a check: the store's root is the `temp-work` directory kbase itself created
// under the output directory (TempWork), so the sweep cannot reach an
// operator's files because it is not standing anywhere near them. The
// tripwire below is a cheap second reading of the same fact — one string
// comparison against the day someone hands this a root that was never ours.
//
// WHEN it runs is the whole design. Under the dynamic chain a stage's units
// are not described until the stage is reached (see resolveStage), so a sweep
// at job setup would know only the first stage's paths and would delete a
// prior run's stage-4 and stage-5 artifacts — the exact artifacts the resume
// exists to reuse — before their stages ever resolved. So sweeping happens at
// the END of a run that described every stage of its chain, which is the one
// moment the accounted-for set is both complete and final. A run that stopped
// early sweeps nothing, which is also the right answer twice over: its later
// stages are still unresolved, and the litter around a broken job is
// evidence.
//
// Removal failures are logged and not returned: a file that will not delete
// is untidiness, and failing a completed job over it would turn a successful
// run into an error.
func (s *Store) sweep(chain Chain) error {
	if filepath.Base(s.root) != TempWorkDirName {
		return fmt.Errorf("pipeline: refusing to sweep %s: a run's artifacts live in the %s directory "+
			"kbase creates under the output directory, and this is not one", s.root, TempWorkDirName)
	}
	keep := map[string]bool{LockFileName: true}
	for i := range chain {
		units, err := chain[i].Units()
		if err != nil {
			return fmt.Errorf("pipeline: stage %s: its units could not be described: %w", chain[i].Name, err)
		}
		for _, u := range units {
			local, err := validatePath(u.Path)
			if err != nil {
				return err
			}
			keep[local] = true
			keep[local+StampSuffix] = true
		}
	}

	removed := 0
	err := filepath.WalkDir(s.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(s.root, path)
		if err != nil {
			return err
		}
		if keep[rel] {
			return nil
		}
		if rmErr := os.Remove(path); rmErr != nil {
			s.lg.Warn("pipeline could not sweep an unaccounted file", "path", rel, "error", rmErr)
			return nil
		}
		removed++
		s.lg.Debug("pipeline swept an unaccounted file", "path", rel)
		return nil
	})
	if err != nil {
		return fmt.Errorf("pipeline: sweep %s: %w", s.root, err)
	}
	if removed > 0 {
		s.lg.Info("pipeline swept the job directory", "removed", removed)
	}
	return nil
}

// BadPathError reports a store-relative path that is not one: empty,
// absolute, escaping the job directory, or colliding with the stamp
// namespace. It is a defect in the chain description, not a user error.
type BadPathError struct {
	Path   string
	Reason string
}

func (e BadPathError) Error() string {
	return fmt.Sprintf("pipeline: %q is not a usable artifact path: %s", e.Path, e.Reason)
}

// IncoherentStoreError refuses a resume outright: the job directory
// contradicts the chain being run, in a way no amount of redoing units
// resolves. The message names the remedy, because the remedy is always
// available — resume is an optimization and --fresh is always sufficient.
type IncoherentStoreError struct {
	Path   string
	Reason string
}

func (e IncoherentStoreError) Error() string {
	return fmt.Sprintf("pipeline: cannot resume: %s — %s; rerun with --fresh to rebuild from scratch",
		e.Path, e.Reason)
}
