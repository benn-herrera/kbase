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
	return StageSpec{SystemFrame: testFrame, AgentDef: def, TaskDef: testTaskDef}
}

func baseInput() CallInput {
	return CallInput{
		StatusLines:        []string{"Sections done: 2/9", "Current file: guide/intro.md"},
		RefA:               "Cross-file listing:\n- a.md\n- b.md",
		Content:            "The quick brown fox.",
		RefB:               "Prior output ends: ...",
		AcceptanceCriteria: []string{"- cover lines 10-42", "- budget 800 tokens"},
	}
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
		"Sections done: 2/9\n" +
		"Current file: guide/intro.md\n" +
		"\n" +
		"Cross-file listing:\n" +
		"- a.md\n" +
		"- b.md\n" +
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

	got := build(t, NewStageContext(baseSpec(t)), baseInput())
	if got.UserTurn != want {
		t.Errorf("UserTurn mismatch:\n got %q\nwant %q", got.UserTurn, want)
	}
}

// TestBuildDeterminism: same inputs, same bytes — and the same offsets and
// hashes, which the churn tripwire compares across calls.
func TestBuildDeterminism(t *testing.T) {
	sc := NewStageContext(baseSpec(t))
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
	got := build(t, NewStageContext(baseSpec(t)), baseInput())
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
	sc := NewStageContext(spec)
	got := build(t, sc, CallInput{StatusLines: []string{"Sections done: 2/9"}})

	want := testFrame + slotDelimiter + strings.TrimRight(testDefMD, "\n") +
		slotDelimiter + testTaskDef + slotDelimiter + "Sections done: 2/9"
	if got.UserTurn != want {
		t.Errorf("UserTurn mismatch:\n got %q\nwant %q", got.UserTurn, want)
	}
	if strings.Contains(got.UserTurn, "\n\n\n") {
		t.Error("empty slots left orphan delimiters in the turn")
	}
	for _, s := range []Slot{SlotRefA, SlotContent, SlotRefB, SlotReminder} {
		span := got.Offsets[s]
		if span.Start != span.End || span.Start != len(got.UserTurn) {
			t.Errorf("%s span = %+v, want an empty span at %d", s, span, len(got.UserTurn))
		}
		if got.Hashes[s] != sha256.Sum256(nil) {
			t.Errorf("%s does not hash as empty", s)
		}
	}
}

// TestBuildNormalizesSlotWhitespace: the delimiter scheme means exactly one
// blank line between slots regardless of how a caller's source ended, but
// leading indentation survives — it is structure in verbatim material.
func TestBuildNormalizesSlotWhitespace(t *testing.T) {
	sc := NewStageContext(baseSpec(t))
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
	in.RefB = "  \n\t\n"
	got = build(t, sc, in)
	if got.Hashes[SlotRefB] != sha256.Sum256(nil) {
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

			got := build(t, NewStageContext(spec), in)
			span := got.Offsets[SlotReminder]
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

// TestBuildTrailerOverCap: an authored section that validates on its own can
// still push the combined trailer over the cap once criteria are injected.
// That is refused at build time, never truncated.
func TestBuildTrailerOverCap(t *testing.T) {
	spec := baseSpec(t)
	spec.AgentDef.Critical = strings.TrimSpace(strings.Repeat("word ", CriticalWordCap-10))
	if err := ValidateDefinition("## CRITICAL\n\n" + spec.AgentDef.Critical + "\n"); err != nil {
		t.Fatalf("fixture section should validate on its own: %v", err)
	}

	in := baseInput()
	in.AcceptanceCriteria = []string{strings.TrimSpace(strings.Repeat("crit ", 20))}

	got, err := NewStageContext(spec).Build(in)
	var target ErrCriticalOverCap
	if !errors.As(err, &target) {
		t.Fatalf("err = %v, want ErrCriticalOverCap", err)
	}
	if target.Words != CriticalWordCap-10+20 {
		t.Errorf("Words = %d, want %d", target.Words, CriticalWordCap+10)
	}
	if got.UserTurn != "" {
		t.Error("a refused call must return nothing")
	}
}

func TestBuildBudgets(t *testing.T) {
	sc := func(t *testing.T, b Budgets) *StageContext {
		t.Helper()
		spec := baseSpec(t)
		spec.Budgets = b
		return NewStageContext(spec)
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
		budgets := Budgets{PerSlot: map[Slot]int{SlotRefA: 0}}
		if _, err := sc(t, budgets).Build(baseInput()); err == nil {
			t.Error("a filled slot budgeted at 0 tokens must be refused")
		}
		in := baseInput()
		in.RefA = ""
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

	t.Run("zero budgets check nothing", func(t *testing.T) {
		got := build(t, sc(t, Budgets{}), baseInput())
		if got.Warning != "" {
			t.Errorf("unexpected warning: %s", got.Warning)
		}
	})
}

// TestBuildHonorsEstimatorRatio: the budget check must move when calibration
// moves the ratio, or the estimator swap would be cosmetic.
func TestBuildHonorsEstimatorRatio(t *testing.T) {
	spec := baseSpec(t)
	spec.Est = tokens.Estimator{CharsPerToken: 1}
	spec.Budgets = Budgets{CallCeiling: 100}
	if _, err := NewStageContext(spec).Build(baseInput()); err == nil {
		t.Error("a one-byte-per-token estimator should blow a 100-token ceiling")
	}
}
