package pipeline

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kbase/internal/log"
)

// TempWorkDirName is the transient half of an output directory
// (ARCHITECTURE.md §12, "Temporary work"). Everything a run produces that is
// not a delivered artifact — stage artifacts, their stamps, the job lock, the
// residue of a killed write — lives under `<out>/temp-work/`, and nothing
// transient lives anywhere else. Not a system temporary directory either: an
// interrupted run's intermediates are what its resume reads, and a location
// the OS may clear between runs would make resume a coin flip.
//
// The name is load-bearing rather than cosmetic. Store.sweep deletes every
// file its root holds that the chain does not account for, and what makes
// that safe is not a marker file or a heuristic — it is that the root is a
// directory kbase itself created for this purpose, one level below whatever
// the operator named. `--out notes` can no longer reach `notes/`; it reaches
// `notes/temp-work/`, which was empty until this run made it.
const TempWorkDirName = "temp-work"

// TempWork is one run's scratch space under an output directory: the store
// every stage writes through, and the teardown that removes it.
//
// It MIRRORS the output tree. Scratch for `<out>/ch1/sec2/` lives at
// `<out>/temp-work/ch1/sec2/`, created on demand by the first write that
// needs it (Store.Put's writeAtomic does the MkdirAll), so a store-relative
// path means the same thing on both sides and no second layout has to be
// kept in step with the first.
//
// Teardown is asymmetric on purpose: a successful run removes it, a failed or
// interrupted one keeps it. That is one rule rather than a cleanup step with
// an exception, it is the same "litter around a broken job is evidence"
// reading Store.sweep already takes, and it is what leaves a resume something
// to read. Discard is the caller's call to make, because only the caller
// knows whether the delivered artifacts made it out.
type TempWork struct {
	root  string
	store *Store
	lg    log.Logger
}

// OpenTempWork creates `<out>/temp-work` and returns the scratch space rooted
// there. The output directory is created too if it is missing — a job that
// delivers into it needs it either way, and creating both here keeps the two
// facts in one place.
func OpenTempWork(out string, lg log.Logger) (*TempWork, error) {
	if strings.TrimSpace(out) == "" {
		return nil, errors.New("pipeline: temporary work needs an output directory to sit under")
	}
	root := filepath.Join(out, TempWorkDirName)
	if err := os.MkdirAll(root, ArtifactDirMode); err != nil {
		return nil, fmt.Errorf("pipeline: create %s: %w", root, err)
	}
	return &TempWork{root: root, store: NewStore(root, lg), lg: lg}, nil
}

// Root is the temp-work directory itself, for a message that has to name it.
func (w *TempWork) Root() string { return w.root }

// Store is the artifact custody rooted at this scratch space — the one a
// Coordinator runs against.
func (w *TempWork) Store() *Store { return w.store }

// Discard removes the scratch space. It is called only after a run that
// succeeded and delivered what it owed; every other outcome keeps the tree so
// a resume, or a human, can read it.
func (w *TempWork) Discard() error {
	if err := os.RemoveAll(w.root); err != nil {
		return fmt.Errorf("pipeline: remove %s: %w", w.root, err)
	}
	w.lg.Debug("pipeline removed the run's temporary work", "path", w.root)
	return nil
}
