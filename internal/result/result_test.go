package result

import (
	"bytes"
	"reflect"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/argmark"
)

func TestEmitRoundTrips(t *testing.T) {
	var buf bytes.Buffer
	err := Emit(&buf, Done,
		Field{"whole", 1.0}, Field{"tiny", 0.00001}, Field{"big", 1e21}, Field{"half", 0.5},
		Field{"none", nil}, Field{"flag", true}, Field{"count", 3}, Field{"word", "true"},
		Field{"bands", Record{{"*pending*", 2}, {"ok to build on", 0}}},
		Field{"rows", []Record{{{"id", "clm-aaaaaa"}, {"fraction", "*pending*"}}}})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := yaml.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("%v\n%s", err, buf.String())
	}
	want := map[string]any{
		"outcome": Done, "whole": 1.0, "tiny": 0.00001, "big": 1e21, "half": 0.5, "none": nil, "flag": true, "count": 3,
		"word":  "true",
		"bands": map[string]any{"*pending*": 2, "ok to build on": 0},
		"rows":  []any{map[string]any{"id": "clm-aaaaaa", "fraction": "*pending*"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip:\n got %#v\nwant %#v\n%s", got, want, buf.String())
	}
}

func TestEmitEndsClosingItemsWithTheOutcomesAsk(t *testing.T) {
	given := func() []Item {
		return []Item{
			{Check: "error", Detail: "the call failed"},
			{Check: "error", Detail: "the call failed."},
			{Check: "error"},
			{Check: "error", Detail: "the provider refused the key; no one asks for it"},
		}
	}
	details := func(items []Item) []string {
		out := make([]string, len(items))
		for i, it := range items {
			out[i] = it.Detail
		}
		return out
	}
	for _, tc := range []struct {
		outcome, closing, other, ask string
	}{
		{Retry, RefusalsKey, FailuresKey, RetryAsk},
		{Failed, FailuresKey, RefusalsKey, FailedAsk},
	} {
		for _, wrap := range []struct {
			name string
			of   func([]Item) any
		}{
			{"[]Item", func(it []Item) any { return it }},
			{"Items", func(it []Item) any { return Items(it) }},
		} {
			t.Run(tc.outcome+"/"+wrap.name, func(t *testing.T) {
				closing, other := given(), given()
				var buf bytes.Buffer
				if err := Emit(&buf, tc.outcome, Field{tc.other, wrap.of(other)}, Field{tc.closing, wrap.of(closing)}); err != nil {
					t.Fatal(err)
				}
				var doc struct {
					Refusals []Item `yaml:"refusals"`
					Failures []Item `yaml:"failures"`
				}
				if err := yaml.Unmarshal(buf.Bytes(), &doc); err != nil {
					t.Fatalf("%v\n%s", err, buf.String())
				}
				got := map[string][]Item{RefusalsKey: doc.Refusals, FailuresKey: doc.Failures}
				want := []string{
					"the call failed; " + tc.ask,
					"the call failed; " + tc.ask,
					tc.ask,
					"the provider refused the key; no one asks for it; " + tc.ask,
				}
				if d := details(got[tc.closing]); !reflect.DeepEqual(d, want) {
					t.Errorf("closing details:\n got %q\nwant %q", d, want)
				}
				if d := details(got[tc.other]); !reflect.DeepEqual(d, details(given())) {
					t.Errorf("items under %s changed: %q", tc.other, d)
				}
				if !reflect.DeepEqual(closing, given()) || !reflect.DeepEqual(other, given()) {
					t.Errorf("Emit changed the caller's items: %#v %#v", closing, other)
				}
			})
		}
	}
}

func TestEmitLeavesOtherOutcomesItemsAlone(t *testing.T) {
	items := []Item{{Check: "error", Detail: "the call failed."}, {Check: "usage"}}
	for _, outcome := range []string{Done, Unchanged, Bounded, Refused, Cancelled} {
		for _, key := range []string{RefusalsKey, FailuresKey} {
			fields := []Field{{"state-dir", "/s"}, {key, items}}
			var got, want bytes.Buffer
			if err := Emit(&got, outcome, fields...); err != nil {
				t.Fatal(err)
			}
			if err := Write(&want, append([]Field{{"outcome", outcome}}, fields...)...); err != nil {
				t.Fatal(err)
			}
			if got.String() != want.String() {
				t.Errorf("%s under %s:\n got %s\nwant %s", key, outcome, got.String(), want.String())
			}
		}
	}
}

func TestFloatTextIsPositional(t *testing.T) {
	for in, want := range map[float64]string{1: "1.0", 0.00001: "0.00001", 0.55: "0.55", 1e21: "1000000000000000000000.0"} {
		if got := floatText(in); got != want {
			t.Errorf("floatText(%v) = %q, want %q", in, got, want)
		}
	}
}

// TestADocumentIsSpelledByTheWriterItGoesTo: a document written on a spelled
// writer spells every string by it, items and lists included; one written on
// any other writer keeps the marked form, and Reemit spells that record as
// Write would have, byte for byte; the first spelling a writer carries holds.
func TestADocumentIsSpelledByTheWriterItGoesTo(t *testing.T) {
	fields := []Field{{"note", "pass " + argmark.Flag("state-dir")}, {"words", []string{argmark.Positional("id")}},
		{RefusalsKey, []Item{{Key: argmark.Flag("limit"), Detail: argmark.Flag("limit") + " needs " + argmark.Flag("sort")}}}}
	mcp := argmark.Spelling{MCP: true}
	var spelled, recorded, reemitted bytes.Buffer
	if err := Emit(Spelled(&spelled, mcp), Refused, fields...); err != nil {
		t.Fatal(err)
	}
	if err := Emit(&recorded, Refused, fields...); err != nil {
		t.Fatal(err)
	}
	if err := Reemit(Spelled(Spelled(&reemitted, mcp), argmark.Spelling{}), recorded.Bytes()); err != nil {
		t.Fatal(err)
	}
	want := "outcome: \"refused\"\nnote: \"pass `state-dir`\"\nwords:\n    - \"id\"\nrefusals:\n    - key: \"limit\"\n      detail: \"`limit` needs `sort`\"\n"
	if spelled.String() != want {
		t.Errorf("spelled:\n%s\nwant:\n%s", spelled.String(), want)
	}
	if !argmark.Marked(recorded.String()) {
		t.Errorf("a document on an unspelled writer lost its markers:\n%s", recorded.String())
	}
	if reemitted.String() != want {
		t.Errorf("reemitted:\n%s\nwant:\n%s", reemitted.String(), want)
	}
}

func TestAnErrorSpellsItsArgumentsAtTheCommandLine(t *testing.T) {
	err := Refusal{{Detail: argmark.Flag("limit") + " needs " + argmark.Flag("sort")}, {Detail: "no " + argmark.Positional("id")}}
	if got, want := err.Error(), "--limit needs --sort; no <id>"; got != want {
		t.Errorf("Error = %q, want %q", got, want)
	}
}
