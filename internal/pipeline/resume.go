package pipeline

import (
	"errors"
	"fmt"
)

// upstreamInputPrefix namespaces an input derived from an upstream artifact,
// keeping it from ever colliding with a caller-supplied input name (the
// source corpus hash, a parameter digest). The path follows the prefix, so a
// mismatch reads as "artifact:survey.json changed".
const upstreamInputPrefix = "artifact:"

// OwedArtifact is one artifact a stage produces, described well enough to decide
// whether an existing copy can be trusted.
type OwedArtifact struct {
	// Path is the store-relative, slash-separated output path.
	Path string
	// Inputs are the named hashes the caller derives: source identity
	// (the corpus content hash), stage parameters, anything outside the
	// store that determines the output.
	Inputs []Input
	// Upstreams are store-relative paths of artifacts this unit consumed.
	// Their stamps' output hashes join Inputs, which is what makes the
	// chain a chain: a re-run upstream stage changes its output hash, and
	// every unit derived from it is Invalid without anyone having to
	// propagate an invalidation.
	Upstreams []string
}

// OwedArtifactResolver describes a stage's output units. It is called when the chain
// walk REACHES the stage, not at job setup.
//
// That is the whole of the dynamic chain: a stage's unit set is not always
// knowable before its upstream ran. Stage 3 emits the tree plan that defines
// stage 4's leaves, and stage 4's verified cut list defines stage 5's
// (ARCHITECTURE.md §4). A resolver therefore reads artifacts that earlier
// stages have already been proven to hold, which is why it can fail and why
// the failure is returned rather than swallowed into an empty stage — an
// empty stage is indistinguishable from a complete one.
type OwedArtifactResolver func() ([]OwedArtifact, error)

// Stage is one link of the chain: a name for forensics and a description of
// the units it must have produced to count as complete.
type Stage struct {
	// Name is the stage's name, for forensics.
	Name string

	// Units resolves the description when the walk reaches this stage.
	//
	// It must be deterministic. The walk calls it again while validating a
	// later stage against what this one produces, and a description that
	// changed in between would mean the scan verdicted one chain and the run
	// executed another. Plan.StageChain's resolvers are backed by a per-stage memo
	// of the caller's own resolution, so determinism costs nothing there.
	Units OwedArtifactResolver
}

// StageChain is the stage dependency chain in execution order, upstream first
// (survey → tree plan → cuts → leaves → summaries → links). Stages are
// pointers because a chain is walked, described and verdicted as one object
// across the scan and the run rather than copied between them.
type StageChain []*Stage

// Mode selects whether a scan may reuse anything.
type Mode string

const (
	// ModeResume inspects the store and reuses what it can prove.
	ModeResume Mode = "resume"
	// ModeFresh ignores the store entirely and rebuilds everything. The job
	// directory's prior contents are DISCARDED before the scan rather than
	// merely left unread (Coordinator.Run, ArtifactStore.discard), which is
	// what makes "unconditionally" true of wreckage a rebuild would not
	// otherwise touch. It is the --fresh flag, and the reason resume is
	// allowed to be strict: any doubt can be redone, and the user always has
	// a button that skips the question.
	//
	// The discard is the whole mechanism, and the scan is how the run MEASURES
	// it: a fresh scan reads verdicts exactly as a resume does, so over the
	// emptied directory every unit is Absent and the run's reused count is
	// zero because nothing was there — not because the mode declined to look.
	// A scan that read no verdicts would report zero either way, which makes
	// "0 reused" evidence of nothing (it was the tautology GO M-1 named).
	ModeFresh Mode = "fresh"
)

// OwedArtifactVerdict is one unit's inspection result. Reason is empty for
// VerdictValid and otherwise says, in one line without content, why the unit
// could not be proven.
type OwedArtifactVerdict struct {
	Path    string
	Verdict Verdict
	Reason  string
}

// ResumeScanResult is where a run picks up: the deepest provably valid prefix of
// the chain, and the per-unit picture inside the first stage that is not
// complete.
type ResumeScanResult struct {
	// Mode is the mode the scan ran in.
	Mode Mode
	// Stages is the length of the chain that was scanned.
	Stages int
	// ResumeStage is the index of the first incomplete stage — the restart
	// point. It equals Stages when every stage is complete.
	ResumeStage int
	// StageName is chain[ResumeStage].Name, empty when the chain is
	// complete. Carried so callers log the name without re-indexing.
	StageName string
	// Verdicts covers every unit of chain[ResumeStage], in chain order.
	// Units in later stages are not verdicted: their inputs are about to
	// change, so they are invalid by definition and inspecting them would
	// only produce reasons nobody should act on.
	Verdicts []OwedArtifactVerdict
	// Reused counts units proven valid across every stage inspected,
	// complete stages included.
	Reused int
	// Redo counts non-valid units in the resume stage.
	Redo int
}

// Complete reports whether the chain is already fully built — nothing to do.
func (r ResumeScanResult) Complete() bool { return r.ResumeStage >= r.Stages }

// ResumeScan walks the chain upstream→downstream and returns the resume boundary.
//
// A stage is complete when every one of its units is VerdictValid; the walk
// stops at the first stage that is not. That is the deepest-provably-valid
// prefix: everything before it stands on proof, everything after it has
// inputs that are about to change. Within the stopping stage, units are
// verdicted individually, so a stage that died two thirds of the way through
// redoes one third.
//
// Forensics go through the logger: one summary record, plus one debug record
// per unit to be redone carrying the reason. A resume that quietly rebuilds
// everything and a resume that reuses everything look identical from the
// outside otherwise, and the difference is the whole value of the feature.
//
// Errors are refusals, never verdicts: a chain description defect
// (BadPathError) or a store that contradicts the chain (IncoherentStoreError,
// whose message names the remedy — --fresh wherever the contradiction is the
// job directory's own contents).
//
// Both modes walk identically. ModeFresh differs only in what it is pointed
// at: its caller discarded the job directory first (see ModeFresh), so the
// same walk finds nothing to prove and every count it reports is a reading of
// that directory rather than a property of the mode.
//
// A stage past the boundary is never described: it is downstream of the
// frontier by definition, its inputs are about to change, and under the
// dynamic chain its description may not exist yet. The run resolves it when
// it reaches it (see Coordinator.Run).
func (s *ArtifactStore) ResumeScan(chain StageChain, mode Mode) (ResumeScanResult, error) {
	if len(chain) == 0 {
		return ResumeScanResult{}, errors.New("pipeline: resume scan over an empty stage chain")
	}
	switch mode {
	case ModeResume, ModeFresh:
	default:
		return ResumeScanResult{}, fmt.Errorf("pipeline: unknown resume mode %q (want %s or %s)", mode, ModeResume, ModeFresh)
	}

	res := ResumeScanResult{Mode: mode, Stages: len(chain), ResumeStage: len(chain)}
	for i, stage := range chain {
		units, err := s.resolveStage(chain, i)
		if err != nil {
			return ResumeScanResult{}, err
		}
		verdicts := make([]OwedArtifactVerdict, 0, len(units))
		valid := 0
		for _, u := range units {
			v, err := s.verdict(u)
			if err != nil {
				return ResumeScanResult{}, err
			}
			if v.Verdict == VerdictValid {
				valid++
			}
			verdicts = append(verdicts, v)
		}
		res.Reused += valid
		if valid == len(units) {
			continue
		}

		res.ResumeStage = i
		res.StageName = stage.Name
		res.Verdicts = verdicts
		res.Redo = len(verdicts) - valid
		break
	}

	for _, v := range res.Verdicts {
		if v.Verdict == VerdictValid {
			continue
		}
		s.lg.Debug("resume will redo unit",
			"stage", res.StageName, "path", v.Path, "verdict", string(v.Verdict), "reason", v.Reason)
	}
	if res.Complete() {
		s.lg.Info("resume found every stage complete",
			"mode", string(mode), "stages", len(chain), "reused", res.Reused)
		return res, nil
	}
	s.lg.Info("resume boundary found",
		"mode", string(mode), "stages", len(chain), "stage", res.StageName,
		"stage_index", res.ResumeStage, "reused", res.Reused, "redo", res.Redo)
	return res, nil
}

// verdict inspects one unit. An input set that cannot even be derived — an
// upstream artifact with no readable stamp — is the unit's own doubt, not a
// refusal: it means the thing this unit was built from is not proven either,
// and the answer to doubt is to redo.
func (s *ArtifactStore) verdict(u OwedArtifact) (OwedArtifactVerdict, error) {
	inputs, err := s.resolveInputs(u)
	if err != nil {
		var inc IncoherentStoreError
		if errors.As(err, &inc) {
			return OwedArtifactVerdict{}, err
		}
		return OwedArtifactVerdict{Path: u.Path, Verdict: VerdictInvalid, Reason: err.Error()}, nil
	}
	v, reason, err := s.verify(u.Path, inputs)
	if err != nil {
		return OwedArtifactVerdict{}, err
	}
	return OwedArtifactVerdict{Path: u.Path, Verdict: v, Reason: reason}, nil
}

// resolveInputs derives a unit's full input set: the caller's named hashes
// plus one per upstream artifact, carrying that artifact's recorded output
// hash. The result is sorted, and it is the SAME function both sides of the
// proof use — the writer passes it to Put, the scan passes it to verify — so
// a stamp can never be written under one derivation and checked under
// another.
func (s *ArtifactStore) resolveInputs(u OwedArtifact) ([]Input, error) {
	inputs := make([]Input, 0, len(u.Inputs)+len(u.Upstreams))
	inputs = append(inputs, u.Inputs...)
	for _, up := range u.Upstreams {
		stamp, err := s.readStamp(up)
		if err != nil {
			return nil, fmt.Errorf("upstream %s is unproven: %w", up, err)
		}
		if stamp.Path != up {
			return nil, IncoherentStoreError{
				Path:   up,
				Reason: fmt.Sprintf("its stamp claims to belong to %q", stamp.Path),
				Remedy: remedyFresh,
			}
		}
		inputs = append(inputs, Input{Name: upstreamInputPrefix + up, Hash: stamp.Output})
	}
	return sortedInputs(inputs), nil
}

// resolveStage describes chain[i] and checks the description against every
// stage before it. It is the one place a stage becomes concrete: the scan
// drives it to the resume boundary, the run drives it the rest of the way,
// and a stage is described the same way whichever of the two got there first.
//
// The earlier stages are walked again rather than accumulated across calls.
// They are already described — resolution is strictly in order — so this
// re-derives at most nine slices from memoized resolutions, which is cheaper
// than a second piece of walk state that the scan starts and the run has to
// finish in step.
func (s *ArtifactStore) resolveStage(chain StageChain, i int) ([]OwedArtifact, error) {
	var (
		seen     = map[string]bool{} // every unit path described so far
		produced = map[string]bool{} // unit paths from strictly earlier stages
		units    []OwedArtifact
	)
	for j := 0; j <= i; j++ {
		var err error
		if units, err = chain[j].Units(); err != nil {
			return nil, fmt.Errorf("pipeline: stage %s: its units could not be described: %w", chain[j].Name, err)
		}
		if err := s.validateStage(units, seen, produced); err != nil {
			return nil, err
		}
		for _, u := range units {
			produced[u.Path] = true
		}
	}
	return units, nil
}

// validateStage checks one stage's description against what the chain has
// described so far: seen is every path described before this stage
// (uniqueness), produced is every path from a STRICTLY earlier stage
// (upstream legality). It adds this stage's paths to seen; the caller adds
// them to produced once the whole stage has passed.
//
// An upstream no earlier stage produces is not a defect by itself. Under the
// dynamic chain a stage may legitimately consume an artifact this chain never
// produces — a tree plan a previous planning epoch left behind — so the store
// is asked whether it holds a stamp for it. An unstamped one refuses the run:
// nothing this chain does would ever produce it, which makes it structural
// incoherence rather than a unit to redo.
//
// A unit may not name a SIBLING as an upstream. A stage's units run
// concurrently across the worker pool, so "earlier in the same stage" is not
// an ordering, and a chain that assumed it would be a race with a plausible
// stamp on the far side of it.
func (s *ArtifactStore) validateStage(units []OwedArtifact, seen, produced map[string]bool) error {
	for _, u := range units {
		if _, err := validatePath(u.Path); err != nil {
			return err
		}
		if seen[u.Path] {
			return BadPathError{Path: u.Path, Reason: "produced by more than one unit"}
		}
		seen[u.Path] = true
	}
	for _, u := range units {
		for _, up := range u.Upstreams {
			switch {
			case produced[up]:
			case seen[up]:
				return BadPathError{
					Path:   u.Path,
					Reason: fmt.Sprintf("names upstream %q from its own stage, whose units run concurrently", up),
				}
			default:
				if _, err := s.readStamp(up); err != nil {
					return IncoherentStoreError{
						Path: up,
						Reason: fmt.Sprintf("unit %q consumes it, no earlier stage produces it, "+
							"and the store holds no stamp for it", u.Path),
						Remedy: remedyUnproduced,
					}
				}
			}
		}
	}
	return nil
}
