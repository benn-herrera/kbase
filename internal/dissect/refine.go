package dissect

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"kbase/internal/config"
	"kbase/internal/pipeline"
	"kbase/internal/prompt"
	"kbase/internal/survey"
	"kbase/internal/text"
)

// This file is §5 step 2: the model-facing half of boundary refinement, wired
// through the real orchestrator (internal/pipeline) rather than around it.
//
// The shape §5 specifies is a serial scan — one call per boundary, each
// window loaded into the end of the context and overwriting the previous, so
// the context stays O(1) while the calls are O(n). That is why StagePlan
// emits ONE domain stream: a worker owns a domain and runs it serially, so a
// single stream IS the serial scan, and the per-call Content slot holding
// this boundary's window IS the overwrite.
//
// It is a refinement seam in the §12 sense: the mechanical cut list is
// already valid, so a model that answers badly, or not at all, costs quality
// and never correctness. The one failure that is not treated that way is the
// tripwire — see ErrVerifierDefect below and the package comment above it.

const (
	// menuLabelWords bounds the text a menu entry quotes from the document.
	// The entry exists so the model can tell one candidate from another, not
	// so it can read the section twice — the section is already in the
	// window above it.
	menuLabelWords = 12

	// unitSuffix names a boundary's artifact. One unit per boundary is what
	// makes a boundary individually resumable and individually degradable.
	unitSuffix = ".cut"
)

// stubDefinition and stubTaskDef are PLACEHOLDER prompt text.
//
// TODO(embedded-definitions): the real gemma-4-tuned definition, its
// `## CRITICAL` section and its eval belong to the embedded-definitions burst
// (ROADMAP). What is being exercised here is the seam — window, menu,
// verifier, baseline, retry, fallback — and none of that depends on the
// wording. Keeping a stub visible and marked is honest; inventing tuned text
// nobody evaluated would look like the real thing.
const (
	stubDefinition = "# Boundary refinement\n\n" +
		"You are given the end of one section, the start of the next, and a numbered list of the\n" +
		"positions the boundary between them may be moved to.\n\n" +
		"## CRITICAL\n\n" +
		"Answer with one number from the list and nothing else.\n"

	stubTaskDef = "Task: choose the position in the numbered list that best separates the two sections."
)

// Boundary is one adjudication: the mechanical cut between two sections, the
// window the model may move it within, and the candidates inside that window
// as a numbered menu.
//
// The menu is the model's whole vocabulary. It answers with a number, so a
// cut through the middle of a heading or a code fence is not a wrong answer
// it could give — it is not an answer it can express (§5).
type Boundary struct {
	// Unit is the store path of the artifact this boundary produces.
	Unit string
	// Index is the position in the cut list of the section that STARTS at
	// Cut, so cuts[Index-1] and cuts[Index] are the two neighbours.
	Index int
	// Cut is the mechanical cut: the baseline, and the window's centre.
	Cut int
	// Window is the clamp (Windows).
	Window Window
	// Menu is the candidates inside the window, in document order. Menu[0]
	// is choice 1.
	Menu []survey.CutCandidate
}

// Refiner is the boundary-refinement stage over one span's mechanical cut
// list. It is built once, when the stage's work is described, and is then
// immutable: every worker of the stage reads it and nothing writes to it.
type Refiner struct {
	src     []byte
	span    survey.Range
	cands   []survey.CutCandidate
	cuts    []survey.Range
	windows []Window
	def     prompt.Definition

	order  []Boundary
	byUnit map[string]Boundary
}

// NewRefiner builds the stage over a mechanical cut list, with unitDir as the
// store directory its per-boundary artifacts land in.
//
// It verifies the mechanical list first, and that is not a formality. The
// baseline is what every failure falls back to, so a baseline that does not
// check out is a fallback that would emit unverified material; and the
// tripwire firing here — before a single call — is the offset pipeline being
// broken in a way no model interaction could have caused. Both are worth
// learning at stage setup rather than n calls later.
func NewRefiner(src []byte, span survey.Range, cands []survey.CutCandidate, cuts []survey.Range, unitDir string) (*Refiner, error) {
	if err := Verify(src, span, cands, cuts, nil); err != nil {
		return nil, fmt.Errorf("dissect: the mechanical cut list this stage would fall back to does not verify: %w", err)
	}
	def, err := prompt.ParseDefinition(stubDefinition)
	if err != nil {
		return nil, fmt.Errorf("dissect: parsing the refinement definition: %w", err)
	}

	r := &Refiner{
		src:     src,
		span:    span,
		cands:   cands,
		cuts:    cuts,
		windows: Windows(src, cuts),
		def:     def,
		byUnit:  make(map[string]Boundary, len(cuts)),
	}
	for i := 1; i < len(cuts); i++ {
		w := r.windows[i-1]
		b := Boundary{
			Unit:   fmt.Sprintf("%s/%04d%s", unitDir, i, unitSuffix),
			Index:  i,
			Cut:    cuts[i].Start,
			Window: w,
			Menu:   menu(cands, w),
		}
		r.order = append(r.order, b)
		r.byUnit[b.Unit] = b
	}
	return r, nil
}

// Boundaries returns the adjudications in document order.
func (r *Refiner) Boundaries() []Boundary { return r.order }

// StagePlan describes the stage for the coordinator: the role every worker
// runs, the stage-constant context, and one serial stream of boundary units.
//
// inputs are the named hashes every boundary artifact is stamped with — the
// source identity and the upstream artifacts this cut list was derived from.
// They are the caller's because the caller is the one that knows what job
// this span came out of.
func (r *Refiner) StagePlan(name string, inputs []pipeline.Input) *pipeline.StagePlan {
	tasks := make([]pipeline.Task, 0, len(r.order))
	for _, b := range r.order {
		tasks = append(tasks, pipeline.Task{
			Unit:       pipeline.Unit{Path: b.Unit, Inputs: inputs},
			Section:    name,
			SectionRef: r.sectionRef(),
			Input:      r.callInput(b),
		})
	}
	return &pipeline.StagePlan{
		Name: name,
		Role: pipeline.Role{
			Def: r.def,
			// The light tier: refinement is the parallel, checklist-shaped
			// work §10 maps to 26B-A4B.
			Tier:     config.TierLight,
			Verify:   r.verify,
			Encode:   encodeChoice,
			Baseline: r.baseline,
		},
		Spec: prompt.StageSpec{TaskDef: stubTaskDef},
		Streams: func() ([]pipeline.DomainStream, error) {
			// One stream, so the boundaries are walked in order by one
			// worker: §5's serial scan, and the reason each window can
			// overwrite the last.
			return []pipeline.DomainStream{{Domain: name, Tasks: tasks}}, nil
		},
	}
}

// sectionRef is reference buffer A for the stage: what is stable across every
// call of this span's scan.
func (r *Refiner) sectionRef() string {
	return fmt.Sprintf("Span [%d,%d): %d sections, %d boundaries to adjudicate.",
		r.span.Start, r.span.End, len(r.cuts), len(r.order))
}

// callInput is one boundary's per-call half of the prompt.
//
// The window goes in the Content slot — it is the material under work — and
// the menu in reference buffer B, which is §7's slot for transient material
// specific to the current content. The status lines are ordered most→least
// stable, so the line that changes every call is last.
func (r *Refiner) callInput(b Boundary) prompt.CallInput {
	return prompt.CallInput{
		StatusLines: []string{
			fmt.Sprintf("Span: [%d,%d)", r.span.Start, r.span.End),
			fmt.Sprintf("Boundary %d of %d", b.Index, len(r.order)),
		},
		Content:            string(r.src[b.Window.Lo:b.Window.Hi]),
		RefB:               renderMenu(r.src, b),
		AcceptanceCriteria: []string{"- answer with one number from the list, nothing else"},
	}
}

// verify maps a response back to a byte offset and checks the cut list it
// would produce.
//
// The check is the WHOLE list with this boundary moved, run through the same
// Verify every other caller uses, rather than a per-boundary re-implementation
// of membership, clamp, minimum size and tripwire. One implementation is the
// point: a second one would be a second thing to keep true, and the two would
// disagree on exactly the day it mattered.
func (r *Refiner) verify(unit, response string) (any, error) {
	b, ok := r.byUnit[unit]
	if !ok {
		return nil, fmt.Errorf("dissect: %s is not a boundary of this span", unit)
	}
	n, err := parseChoice(response, len(b.Menu))
	if err != nil {
		return nil, RejectionError{Offset: b.Cut, Reason: err.Error()}
	}
	at := b.Menu[n-1].Offset

	moved := make([]survey.Range, len(r.cuts))
	copy(moved, r.cuts)
	moved[b.Index-1].End, moved[b.Index].Start = at, at

	if err := Verify(r.src, r.span, r.cands, moved, r.windows); err != nil {
		var defect OffsetDefectError
		if errors.As(err, &defect) {
			// Hand the runner the classification this package is the only
			// one that can make: no answer to the question we asked could
			// land here, so it is not the model that is wrong.
			return nil, fmt.Errorf("%w: %w", pipeline.ErrVerifierDefect, defect)
		}
		return nil, err
	}
	return Choice{Unit: unit, Offset: at}, nil
}

// baseline is the mechanical cut for a boundary — the §3 fallback that was
// already valid, and was verified as a whole list at NewRefiner.
func (r *Refiner) baseline(unit string) any {
	// An unknown unit cannot arrive through the coordinator (the units and
	// this table are built from one list), and a Baseline has no way to
	// refuse. The zero offset is one encodeChoice rejects, so the case
	// surfaces as a loud write failure rather than as a plausible artifact.
	return Choice{Unit: unit, Offset: r.byUnit[unit].Cut}
}

// Choice is one adjudicated boundary: the offset the cut settled on, whether
// the model moved it or the baseline stood.
type Choice struct {
	Unit   string
	Offset int
}

// encodeChoice renders a boundary artifact: the offset, one decimal line.
//
// A cut is one number, and a number is what stage 5 reads back. Whether the
// model moved it is not in the bytes because it is not a property of the cut
// — the coordinator counts degraded units and the log names them, which is
// where a question about model performance is answered.
func encodeChoice(artifact any) ([]byte, error) {
	c, ok := artifact.(Choice)
	if !ok {
		return nil, fmt.Errorf("dissect: artifact is %T, not a Choice", artifact)
	}
	if c.Offset <= 0 {
		return nil, fmt.Errorf("dissect: %s has no cut offset; it is not a boundary of this span", c.Unit)
	}
	return fmt.Appendf(nil, "%d\n", c.Offset), nil
}

// menu is the candidates inside a window, in document order.
func menu(cands []survey.CutCandidate, w Window) []survey.CutCandidate {
	var out []survey.CutCandidate
	for _, c := range cands {
		if c.Offset > w.Hi {
			break
		}
		if w.Contains(c.Offset) {
			out = append(out, c)
		}
	}
	return out
}

// renderMenu numbers the menu from 1 and labels each entry with its kind and
// the line it starts, capped.
//
// No byte offsets: the model emits no raw offsets (§5), so showing it any
// would be an invitation to answer with one. The number is the whole of what
// it may say back.
func renderMenu(src []byte, b Boundary) string {
	var sb strings.Builder
	sb.WriteString("Candidate positions:\n")
	for i, c := range b.Menu {
		fmt.Fprintf(&sb, "%d. %s: %s\n", i+1, c.Kind, label(src, c.Offset))
	}
	return sb.String()
}

// label quotes the line a candidate starts, capped to a few words.
func label(src []byte, off int) string {
	end := off
	for end < len(src) && src[end] != '\n' {
		end++
	}
	return text.CapWords(string(src[off:end]), menuLabelWords)
}

// parseChoice reads the menu number out of a response.
//
// It takes the first run of digits it finds, so "2", "2." and "Answer: 2" all
// land — a small tier says the number in a sentence often enough that
// refusing the sentence would spend a retry on punctuation. Everything else
// is a rejection: no digits, a number outside the menu, or a number the menu
// does not have.
func parseChoice(response string, size int) (int, error) {
	digits := ""
	for i := 0; i < len(response); i++ {
		if response[i] >= '0' && response[i] <= '9' {
			digits += string(response[i])
			continue
		}
		if digits != "" {
			break
		}
	}
	if digits == "" {
		return 0, fmt.Errorf("no menu number in the response")
	}
	n, err := strconv.Atoi(digits)
	if err != nil || n < 1 || n > size {
		return 0, fmt.Errorf("choice %s is not one of the %d listed positions", digits, size)
	}
	return n, nil
}
