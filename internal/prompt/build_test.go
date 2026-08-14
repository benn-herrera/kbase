package prompt

import (
	"crypto/sha256"
	"errors"
	"reflect"
	"strings"
	"testing"

	"kbase/internal/tokens"
)

// Shared fixture. Small enough to read, complete enough that every slot is
// populated — the golden test below is a byte-for-byte pin of what it renders.
const (
	testFrame = "You are part of a fixed pipeline.\n" +
		"Job: distill corpus X."

	testDefMD = "# Distiller\n" +
		"\n" +
		"Translate the span faithfully.\n" +
		"\n" +
		"## CRITICAL\n" +
		"\n" +
		"Reproduce every fact. Never summarize.\n"

	testTaskDef = "Task: distill section 3."
)

func baseSpec(t *testing.T) StageSpec {
	t.Helper()
	def, err := ParseDefinition(testDefMD)
	if err != nil {
		t.Fatalf("ParseDefinition: %v", err)
	}
	return StageSpec{JobFrame: testFrame, AgentDef: def, TaskDef: testTaskDef}
}

func baseInput() CallInput {
	return CallInput{
		StatusLines:        []string{"Sections done: 2/9", "Current file: guide/intro.md"},
		StageRef:           "Cross-file listing:\n- a.md\n- b.md",
		Content:            "The quick brown fox.",
		CallRef:            "Prior output ends: ...",
		AcceptanceCriteria: []string{"- cover lines 10-42", "- budget 800 tokens"},
	}
}

// newContext is the constructor with its error checked, so a test that is not
// about construction reads as one line.
func newContext(t *testing.T, spec StageSpec) *StageContext {
	t.Helper()
	sc, err := NewStageContext(spec)
	if err != nil {
		t.Fatalf("NewStageContext: %v", err)
	}
	return sc
}

func build(t *testing.T, sc *StageContext, in CallInput) BuiltCall {
	t.Helper()
	got, err := sc.Build(in)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return got
}

// TestBuildGoldenBytes pins the render: every delimiter, the trailer label and
// its fence. These bytes are prefix-cache boundaries — a change here
// invalidates every cached prefix in flight, so it must fail a test first.
func TestBuildGoldenBytes(t *testing.T) {
	const want = "You are part of a fixed pipeline.\n" +
		"Job: distill corpus X.\n" +
		"\n" +
		"# Distiller\n" +
		"\n" +
		"Translate the span faithfully.\n" +
		"\n" +
		"## CRITICAL\n" +
		"\n" +
		"Reproduce every fact. Never summarize.\n" +
		"\n" +
		"Task: distill section 3.\n" +
		"\n" +
		"Cross-file listing:\n" +
		"- a.md\n" +
		"- b.md\n" +
		"\n" +
		"Sections done: 2/9\n" +
		"Current file: guide/intro.md\n" +
		"\n" +
		"The quick brown fox.\n" +
		"\n" +
		"Prior output ends: ...\n" +
		"\n" +
		"REMINDER:\n" +
		"````\n" +
		"Reproduce every fact. Never summarize.\n" +
		"\n" +
		"- cover lines 10-42\n" +
		"- budget 800 tokens\n" +
		"````"

	got := build(t, newContext(t, baseSpec(t)), baseInput())
	if got.UserTurn != want {
		t.Errorf("UserTurn mismatch:\n got %q\nwant %q", got.UserTurn, want)
	}
}

// TestBuildDeterminism: same inputs, same bytes — and the same offsets and
// hashes, which the churn tripwire compares across calls.
func TestBuildDeterminism(t *testing.T) {
	sc := newContext(t, baseSpec(t))
	a := build(t, sc, baseInput())
	b := build(t, sc, baseInput())

	if a.UserTurn != b.UserTurn {
		t.Error("identical inputs produced different bytes")
	}
	if !reflect.DeepEqual(a.Offsets, b.Offsets) {
		t.Error("identical inputs produced different offsets")
	}
	if !reflect.DeepEqual(a.Hashes, b.Hashes) {
		t.Error("identical inputs produced different hashes")
	}
	if !reflect.DeepEqual(a.EstTokens, b.EstTokens) {
		t.Error("identical inputs produced different estimates")
	}
}

// TestBuildOffsetsAndHashesTrackContent: the offsets must actually locate the
// slot in the turn, and the hash must be over those same bytes — the two
// facts the prefix-stability property and the churn tripwire both rest on.
func TestBuildOffsetsAndHashesTrackContent(t *testing.T) {
	got := build(t, newContext(t, baseSpec(t)), baseInput())
	for _, s := range allSlots {
		span, ok := got.Offsets[s]
		if !ok {
			t.Fatalf("%s has no offset span", s)
		}
		text := got.UserTurn[span.Start:span.End]
		if h := sha256.Sum256([]byte(text)); h != got.Hashes[s] {
			t.Errorf("%s hash does not cover its own bytes", s)
		}
		var est tokens.Estimator
		if want := est.Estimate(text); got.EstTokens[s] != want {
			t.Errorf("%s estimate = %d, want %d", s, got.EstTokens[s], want)
		}
	}
	// The delimiters are part of what the model reads, so the total is more
	// than the sum of the slots.
	sum := 0
	for _, s := range allSlots {
		sum += got.EstTokens[s]
	}
	if got.EstTokens[SlotTotal] < sum {
		t.Errorf("total estimate %d is below the slot sum %d", got.EstTokens[SlotTotal], sum)
	}
}

// TestBuildEmptySlotsRenderAsNothing: an unfilled buffer is absent, not empty
// scaffolding — no orphan delimiters, and its span collapses to the point it
// would have occupied.
func TestBuildEmptySlotsRenderAsNothing(t *testing.T) {
	spec := baseSpec(t)
	spec.AgentDef.Critical = ""
	sc := newContext(t, spec)
	got := build(t, sc, CallInput{StatusLines: []string{"Sections done: 2/9"}})

	want := testFrame + slotDelimiter + strings.TrimRight(testDefMD, "\n") +
		slotDelimiter + testTaskDef + slotDelimiter + "Sections done: 2/9"
	if got.UserTurn != want {
		t.Errorf("UserTurn mismatch:\n got %q\nwant %q", got.UserTurn, want)
	}
	if strings.Contains(got.UserTurn, "\n\n\n") {
		t.Error("empty slots left orphan delimiters in the turn")
	}
	for _, s := range []Slot{SlotStageRef, SlotContent, SlotCallRef, SlotCriticalEcho} {
		if got.Hashes[s] != sha256.Sum256(nil) {
			t.Errorf("%s does not hash as empty", s)
		}
	}
	// An empty slot in the middle of a filled stack collapses to the byte it
	// would have started at — the end of the last slot above it, before the
	// delimiter that slot's presence would have added.
	if span := got.Offsets[SlotStageRef]; span.Start != span.End || span.Start != got.Offsets[SlotTaskDef].End {
		t.Errorf("%s span = %+v, want an empty span at %d", SlotStageRef, span, got.Offsets[SlotTaskDef].End)
	}
	for _, s := range []Slot{SlotContent, SlotCallRef, SlotCriticalEcho} {
		span := got.Offsets[s]
		if span.Start != span.End || span.Start != len(got.UserTurn) {
			t.Errorf("%s span = %+v, want an empty span at %d", s, span, len(got.UserTurn))
		}
	}
}

// TestBuildNormalizesSlotWhitespace: the delimiter scheme means exactly one
// blank line between slots regardless of how a caller's source ended, but
// leading indentation survives — it is structure in verbatim material.
func TestBuildNormalizesSlotWhitespace(t *testing.T) {
	sc := newContext(t, baseSpec(t))
	in := baseInput()
	in.Content = "\n\n    indented span\t\n\n"
	got := build(t, sc, in)

	if want := "    indented span"; got.UserTurn[got.Offsets[SlotContent].Start:got.Offsets[SlotContent].End] != want {
		t.Errorf("content slot = %q, want %q",
			got.UserTurn[got.Offsets[SlotContent].Start:got.Offsets[SlotContent].End], want)
	}
	if strings.Contains(got.UserTurn, "\n\n\n") {
		t.Error("slot whitespace leaked into the delimiters")
	}

	// A whitespace-only slot is an empty slot.
	in.CallRef = "  \n\t\n"
	got = build(t, sc, in)
	if got.Hashes[SlotCallRef] != sha256.Sum256(nil) {
		t.Error("whitespace-only slot should render as empty")
	}
}

func TestBuildTrailer(t *testing.T) {
	for _, tc := range []struct {
		name     string
		critical string
		criteria []string
		dualOff  bool
		want     string // trailer bytes; "" means no trailer at all
	}{
		{
			name:     "critical and criteria",
			critical: "Never summarize.",
			criteria: []string{"- cover lines 10-42"},
			want:     "REMINDER:\n````\nNever summarize.\n\n- cover lines 10-42\n````",
		},
		{
			name:     "critical only",
			critical: "Never summarize.",
			want:     "REMINDER:\n````\nNever summarize.\n````",
		},
		{
			name:     "criteria only",
			criteria: []string{"- cover lines 10-42"},
			want:     "REMINDER:\n````\n- cover lines 10-42\n````",
		},
		{
			name: "neither renders no trailer",
			want: "",
		},
		{
			// Dual-render off is the eval harness's A/B: the recency
			// restatement goes, the injected criteria stay.
			name:     "dual render off drops critical, keeps criteria",
			critical: "Never summarize.",
			criteria: []string{"- cover lines 10-42"},
			dualOff:  true,
			want:     "REMINDER:\n````\n- cover lines 10-42\n````",
		},
		{
			name:     "dual render off with no criteria renders no trailer",
			critical: "Never summarize.",
			dualOff:  true,
			want:     "",
		},
		{
			name:     "blank criteria lines are dropped",
			criteria: []string{"", "  ", "- cover lines 10-42"},
			want:     "REMINDER:\n````\n- cover lines 10-42\n````",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec := baseSpec(t)
			spec.AgentDef.Critical = tc.critical
			spec.DisableDualRender = tc.dualOff
			in := baseInput()
			in.AcceptanceCriteria = tc.criteria

			got := build(t, newContext(t, spec), in)
			span := got.Offsets[SlotCriticalEcho]
			if trailer := got.UserTurn[span.Start:span.End]; trailer != tc.want {
				t.Errorf("trailer = %q, want %q", trailer, tc.want)
			}
			if tc.want == "" && strings.Contains(got.UserTurn, reminderLabel) {
				t.Error("empty trailer still emitted the REMINDER label")
			}
			// Slot 2 always renders the definition as authored, dual
			// render or not.
			if !strings.Contains(got.UserTurn, criticalHeading) {
				t.Error("slot 2 lost the authored CRITICAL section")
			}
		})
	}
}

// words returns n whitespace-separated words for the cap fixtures.
func words(w string, n int) string { return strings.TrimSpace(strings.Repeat(w+" ", n)) }

// TestBuildDevGateGuaranteesRuntimePass is the property the split cap exists
// for: a definition at the dev-time limit, plus criteria at their limit, must
// build. Before the split, a near-cap section shipped in an immutable binary
// and then failed every call that carried any criteria at all.
func TestBuildDevGateGuaranteesRuntimePass(t *testing.T) {
	md := "## CRITICAL\n\n" + words("word", authoredCriticalCap) + "\n"
	if err := ValidateDefinition(md); err != nil {
		t.Fatalf("a definition at the authored cap must validate: %v", err)
	}
	def, err := ParseDefinition(md)
	if err != nil {
		t.Fatalf("ParseDefinition: %v", err)
	}
	spec := baseSpec(t)
	spec.AgentDef = def

	in := baseInput()
	in.AcceptanceCriteria = []string{words("crit", maxCriteriaWords)}
	if _, err := newContext(t, spec).Build(in); err != nil {
		t.Errorf("dev-gated definition + conforming criteria must build: %v", err)
	}
}

// TestBuildTrailerOverCap: each half of the trailer is refused against its own
// reserved share, and the two failures are different types — the orchestrator
// can fix its criteria and can do nothing about a shipped definition.
func TestBuildTrailerOverCap(t *testing.T) {
	t.Run("authored section over its share", func(t *testing.T) {
		spec := baseSpec(t)
		// One word over: a user-adapted copy that never met the dev gate.
		spec.AgentDef.Critical = words("word", authoredCriticalCap+1)

		got, err := newContext(t, spec).Build(baseInput())
		var target ErrCriticalOverCap
		if !errors.As(err, &target) {
			t.Fatalf("err = %v, want ErrCriticalOverCap", err)
		}
		if target.Words != authoredCriticalCap+1 || target.Cap != authoredCriticalCap {
			t.Errorf("got %d words / cap %d, want %d / %d",
				target.Words, target.Cap, authoredCriticalCap+1, authoredCriticalCap)
		}
		if got.UserTurn != "" {
			t.Error("a refused call must return nothing")
		}
		if strings.Contains(err.Error(), "word word") {
			t.Error("error message must not echo the section content")
		}
	})

	t.Run("criteria over their share", func(t *testing.T) {
		in := baseInput()
		in.AcceptanceCriteria = []string{words("crit", maxCriteriaWords+1)}

		got, err := newContext(t, baseSpec(t)).Build(in)
		var target ErrCriteriaOverCap
		if !errors.As(err, &target) {
			t.Fatalf("err = %v, want ErrCriteriaOverCap", err)
		}
		if target.Words != maxCriteriaWords+1 || target.Cap != maxCriteriaWords {
			t.Errorf("got %d words / cap %d, want %d / %d",
				target.Words, target.Cap, maxCriteriaWords+1, maxCriteriaWords)
		}
		if got.UserTurn != "" {
			t.Error("a refused call must return nothing")
		}
	})

	t.Run("the two shares do not fund each other", func(t *testing.T) {
		// A short section leaves words unspent; the criteria still cannot
		// spend them, or the dev gate stops being a sufficient condition.
		spec := baseSpec(t)
		spec.AgentDef.Critical = "Never summarize."
		in := baseInput()
		in.AcceptanceCriteria = []string{words("crit", maxCriteriaWords+1)}

		var target ErrCriteriaOverCap
		if _, err := newContext(t, spec).Build(in); !errors.As(err, &target) {
			t.Errorf("err = %v, want ErrCriteriaOverCap", err)
		}
	})
}

func TestBuildBudgets(t *testing.T) {
	sc := func(t *testing.T, b Budgets) *StageContext {
		t.Helper()
		spec := baseSpec(t)
		spec.Budgets = b
		return newContext(t, spec)
	}

	t.Run("per-slot refusal", func(t *testing.T) {
		got, err := sc(t, Budgets{PerSlot: map[Slot]int{SlotContent: 1}}).Build(baseInput())
		var target ErrOverBudget
		if !errors.As(err, &target) {
			t.Fatalf("err = %v, want ErrOverBudget", err)
		}
		if target.Slot != SlotContent || target.Budget != 1 || target.Estimate <= 1 {
			t.Errorf("got %+v, want slot 6 over a budget of 1", target)
		}
		if got.UserTurn != "" {
			t.Error("a refused call must return nothing")
		}
		if strings.Contains(err.Error(), "quick brown fox") {
			t.Error("error message must not echo slot content")
		}
	})

	t.Run("zero budget requires an empty slot", func(t *testing.T) {
		budgets := Budgets{PerSlot: map[Slot]int{SlotStageRef: 0}}
		if _, err := sc(t, budgets).Build(baseInput()); err == nil {
			t.Error("a filled slot budgeted at 0 tokens must be refused")
		}
		in := baseInput()
		in.StageRef = ""
		if _, err := sc(t, budgets).Build(in); err != nil {
			t.Errorf("an empty slot budgeted at 0 tokens must pass, got %v", err)
		}
	})

	t.Run("ceiling refusal reports the total", func(t *testing.T) {
		_, err := sc(t, Budgets{CallCeiling: 5}).Build(baseInput())
		var target ErrOverBudget
		if !errors.As(err, &target) {
			t.Fatalf("err = %v, want ErrOverBudget", err)
		}
		if target.Slot != SlotTotal || target.Budget != 5 {
			t.Errorf("got %+v, want the call total over a ceiling of 5", target)
		}
	})

	t.Run("target overrun is advisory", func(t *testing.T) {
		got := build(t, sc(t, Budgets{CallTarget: 5, CallCeiling: DefaultCallCeiling}), baseInput())
		if got.Warning == "" {
			t.Fatal("over-target call carried no warning")
		}
		if got.UserTurn == "" {
			t.Error("over-target call must still be usable")
		}
	})

	t.Run("under target and ceiling is silent", func(t *testing.T) {
		got := build(t, sc(t, Budgets{CallTarget: DefaultCallTarget, CallCeiling: DefaultCallCeiling}), baseInput())
		if got.Warning != "" {
			t.Errorf("unexpected warning: %s", got.Warning)
		}
	})

	t.Run("zero budgets still take the default ceiling", func(t *testing.T) {
		got := build(t, sc(t, Budgets{}), baseInput())
		if got.Warning != "" {
			t.Errorf("unexpected warning: %s", got.Warning)
		}

		// A stage that set no budgets is not unbudgeted: the §9 ceiling
		// applies. The absurd ratio stands in for a huge call without
		// allocating one — the refusal is what is under test, not the size.
		spec := baseSpec(t)
		spec.Est = tokens.Estimator{CharsPerToken: 0.001}
		var target ErrOverBudget
		if _, err := newContext(t, spec).Build(baseInput()); !errors.As(err, &target) {
			t.Fatalf("err = %v, want ErrOverBudget", err)
		}
		if target.Slot != SlotTotal || target.Budget != DefaultCallCeiling {
			t.Errorf("got %+v, want the call total over the default ceiling %d",
				target, DefaultCallCeiling)
		}
	})

	t.Run("a negative ceiling is the explicit opt-out", func(t *testing.T) {
		spec := baseSpec(t)
		spec.Est = tokens.Estimator{CharsPerToken: 0.001}
		spec.Budgets = Budgets{CallCeiling: -1}
		if _, err := newContext(t, spec).Build(baseInput()); err != nil {
			t.Errorf("a negative ceiling must budget nothing, got %v", err)
		}
	})

	t.Run("a SlotTotal per-slot budget is refused at construction", func(t *testing.T) {
		spec := baseSpec(t)
		spec.Budgets = Budgets{PerSlot: map[Slot]int{SlotTotal: 1000}}
		_, err := NewStageContext(spec)
		var target ErrReservedBudgetKey
		if !errors.As(err, &target) {
			t.Fatalf("err = %v, want ErrReservedBudgetKey", err)
		}
		if target.Slot != SlotTotal {
			t.Errorf("Slot = %v, want %v", target.Slot, SlotTotal)
		}
	})

	// The frozen-slots claim covers the budgets too: the caller's map is
	// cloned, so mutating it after construction changes nothing.
	t.Run("per-slot budgets are frozen at construction", func(t *testing.T) {
		budgets := Budgets{PerSlot: map[Slot]int{SlotContent: 1}}
		context := sc(t, budgets)
		delete(budgets.PerSlot, SlotContent)

		if _, err := context.Build(baseInput()); err == nil {
			t.Error("deleting a budget after construction must not disarm it")
		}
	})
}

// TestBuildHonorsEstimatorRatio: the budget check must move when calibration
// moves the ratio, or the estimator swap would be cosmetic.
func TestBuildHonorsEstimatorRatio(t *testing.T) {
	spec := baseSpec(t)
	spec.Est = tokens.Estimator{CharsPerToken: 1}
	spec.Budgets = Budgets{CallCeiling: 100}
	if _, err := newContext(t, spec).Build(baseInput()); err == nil {
		t.Error("a one-byte-per-token estimator should blow a 100-token ceiling")
	}
}
