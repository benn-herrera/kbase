// Package buildstore is the build's state, read: the state store's layout,
// its holder record and progress record, the stage vocabulary and the step
// table's shape, status and the commit trail's reading. It reaches no model
// and no caller, so whatever reads a build's state through it reaches no
// inference; the runner, internal/build, imports it.
package buildstore

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"kbase/internal/asks"
	"kbase/internal/buildrecords"
	"kbase/internal/gitledger"
	"kbase/internal/kb"
	"kbase/internal/result"
)

// Stage is one stage of the build: its id and what it is for.
type Stage struct {
	ID, Display string
}

// Stages is the build's stage vocabulary in walk order. The id "phase-5" is
// never given to a stage: the ledgers would read it as recorded.
var Stages = []Stage{
	{"start", "build started"},
	{"document-graph", "document tree derived"},
	{"spine-seed", "claim-graph spine seeded"},
	{"claims-declared", "declared claim graph"},
	{"claims-discovered", "claim discovery"},
	{"equations-minted", "equation nodes minted"},
	{"references-found", "unmarked references found"},
	{"claims-culled", "co-linear claims culled"},
	{"depends-attributed", "dependency attribution"},
	{"analyses-split", "analyses split"},
	{"phase-3a", "validation gate"},
	{"overview-drafted", "overview drafted"},
}

const (
	StageStart         = "start"
	StageDocumentGraph = "document-graph"
	StageSpineSeed     = "spine-seed"
)

// OwnedPaths are the root-relative paths a build commits and restores:
// kb-root and the build records beside it.
var OwnedPaths = append([]string{kb.KBDir}, buildrecords.RecordFiles...)

// The refusal and failure classes the build's state names in its items.
const (
	CheckUsage    = "usage"
	CheckWorktree = "worktree"
	CheckLock     = kb.CheckLock
	CheckStateDir = "state-dir"
	CheckTrail    = "trail"
	CheckError    = "error"
)

// Step is one step of a stage, and one unit of its progress. A step spends
// inference where it makes a model call under a system prompt, or where its
// whole work is a model call made inside the tool it runs; it calls a model,
// too, where part of its work does. What a step does is the runner's, keyed
// by its ID.
type Step struct {
	ID, Stage             string
	system                string
	spendsOwnInference    bool
	spendsInferenceInPart bool
}

func (s Step) spendsInference() bool { return s.spendsOwnInference || s.system != "" }

// CallsAModel is whether any of the step's work asks a model.
func (s Step) CallsAModel() bool { return s.spendsInference() || s.spendsInferenceInPart }

// applies is whether a step runs: a run spending no inference drops every
// step that would spend it and walks the rest.
func (s Step) applies(noInference bool) bool { return !noInference || !s.spendsInference() }

// Steps is the step table in walk order.
var Steps = []Step{
	{ID: "pre.preflight", Stage: StageStart},
	{ID: "dg.build", Stage: StageDocumentGraph},
	{ID: "seed.graph-init", Stage: "spine-seed"},
	{ID: "declared.build", Stage: "claims-declared"},
	{ID: "discover.build", Stage: "claims-discovered", spendsOwnInference: true},
	{ID: "equations.build", Stage: "equations-minted"},
	{ID: "unmarked.build", Stage: "references-found", spendsOwnInference: true},
	{ID: "cull.build", Stage: "claims-culled", spendsOwnInference: true},
	{ID: "depends.attribute", Stage: "depends-attributed", spendsInferenceInPart: true},
	{ID: "split.build", Stage: "analyses-split"},
	{ID: "p3a.gate", Stage: "phase-3a"},
	{ID: "p3a.stamp", Stage: "phase-3a"},
	{ID: "ov.docs", Stage: "overview-drafted", system: asks.OverviewSystem},
}

// StageSteps is a stage's steps that run in this build, and the ids of those
// it drops.
func StageSteps(stage string, noInference bool) (run []Step, dropped []string) {
	for _, s := range Steps {
		switch {
		case s.Stage != stage:
		case s.applies(noInference):
			run = append(run, s)
		default:
			dropped = append(dropped, s.ID)
		}
	}
	return run, dropped
}

// ResolveStage is the stage name names, by id or display name, whitespace
// collapsed and case ignored.
func ResolveStage(name string) (Stage, bool) {
	wanted := strings.ToLower(strings.Join(strings.Fields(name), " "))
	for _, s := range Stages {
		if wanted == strings.ToLower(s.ID) || wanted == strings.ToLower(s.Display) {
			return s, true
		}
	}
	return Stage{}, false
}

// RecordedStages is each stage the trail records, with its newest boundary
// commit. An id outside the vocabulary reads as no stage.
func RecordedStages(trail []gitledger.Entry) map[string]string {
	recorded := map[string]string{}
	for _, e := range trail {
		known := slices.ContainsFunc(Stages, func(s Stage) bool { return s.ID == e.Stage })
		if known && recorded[e.Stage] == "" {
			recorded[e.Stage] = e.Commit
		}
	}
	return recorded
}

// TrailRefusal is the item refusing a trail that records a stage after one it
// leaves unrecorded, in walk order: a build walking another stage order wrote
// it, and walking the gap would run a stage over the products of stages meant
// to follow it. None where the recorded stages are a prefix of the walk.
func TrailRefusal(repoRoot string, recorded map[string]string) result.Refusal {
	gap := ""
	for _, s := range Stages {
		switch {
		case recorded[s.ID] == "" && gap == "":
			gap = s.ID
		case recorded[s.ID] != "" && gap != "":
			return result.Refusal{{Check: CheckTrail, Path: repoRoot, Remedy: "start over from a commit before the trail",
				Detail: fmt.Sprintf("the trail records %s but not %s: a build walking another stage order wrote it", s.ID, gap)}}
		}
	}
	return nil
}

// Retry is a live holder of what the run needs: the next invocation may find
// it gone, and is told to re-issue.
type Retry []result.Item

func (r Retry) Error() string { return result.Items(r).Error() }

// Failure is a row whose own checks found its products disagreeing, or a
// product a later step found wrong, every finding named. A row that leaves it
// empty has its stage name the report holding the findings.
type Failure []result.Item

func (f Failure) Error() string {
	if len(f) == 0 {
		return "a check failed"
	}
	return result.Items(f).Error()
}

// Cancelled is a build stopped on request, the unit in flight abandoned.
type Cancelled struct{ Stage, Unit string }

func (c Cancelled) Error() string { return fmt.Sprintf("cancelled in %s at %s", c.Stage, c.Unit) }

// Ending is how a build ended: its outcome, and the items or the unit in
// flight that say why where it did not succeed.
type Ending struct {
	Outcome string
	items   []result.Item
	at      Cancelled
}

// EndingOf classifies a walk's error; success is the outcome given.
func EndingOf(success string, err error) Ending {
	var (
		r result.Refusal
		t Retry
		f Failure
		c Cancelled
	)
	switch {
	case err == nil:
		return Ending{Outcome: success}
	case errors.As(err, &r):
		return Ending{Outcome: result.Refused, items: r}
	case errors.As(err, &t):
		return Ending{Outcome: result.Retry, items: t}
	case errors.As(err, &c):
		return Ending{Outcome: result.Cancelled, at: c}
	case errors.As(err, &f):
		return Ending{Outcome: result.Failed, items: f}
	}
	return Ending{Outcome: result.Failed, items: []result.Item{{Check: CheckError, Detail: err.Error()}}}
}

// Fields is the key that closes the result document, none on success.
func (e Ending) Fields() []result.Field {
	switch e.Outcome {
	case result.Refused, result.Retry:
		return []result.Field{{Key: result.RefusalsKey, Value: e.items}}
	case result.Failed:
		return []result.Field{{Key: result.FailuresKey, Value: e.items}}
	case result.Cancelled:
		return []result.Field{{Key: result.CancelledKey, Value: result.Record{{Key: "stage", Value: e.at.Stage}, {Key: "unit", Value: e.at.Unit}}}}
	}
	return nil
}

// Event is the progress record's end of the invocation that ended so.
func (e Ending) Event() Event {
	return Event{Event: EventEnd, Outcome: e.Outcome, Items: e.items, Stage: e.at.Stage, Unit: e.at.Unit}
}
