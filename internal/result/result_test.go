package result

import (
	"bytes"
	"io"
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

type namedCount uint16

func TestWriteEncodesEveryIntegerKindAsAnInt(t *testing.T) {
	encoded := func(v any) string {
		var buf bytes.Buffer
		if err := Write(&buf, Field{"n", v}); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	for _, tc := range []struct {
		name string
		v    any
		as   int
	}{
		{"int8", int8(-7), -7}, {"int16", int16(300), 300}, {"int32", int32(70000), 70000},
		{"int64", int64(1) << 40, 1 << 40}, {"uint", uint(5), 5}, {"uint8", uint8(255), 255},
		{"uint16", uint16(65535), 65535}, {"uint32", uint32(1) << 31, 1 << 31},
		{"uint64", uint64(1) << 40, 1 << 40}, {"named", namedCount(9), 9}, {"zero uint", uint(0), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, want := encoded(tc.v), encoded(tc.as); got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
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

// TestAResultIsSpelledByTheWriterItGoesTo: a result written on a spelled
// writer spells every string by it, items and lists included; one written on
// any other writer keeps the marked form, and Reemit spells that record as
// Write would have, byte for byte; the first spelling a writer carries holds.
func TestAResultIsSpelledByTheWriterItGoesTo(t *testing.T) {
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
		t.Errorf("a result on an unspelled writer lost its markers:\n%s", recorded.String())
	}
	if reemitted.String() != want {
		t.Errorf("reemitted:\n%s\nwant:\n%s", reemitted.String(), want)
	}
}

// TestUnderlyingPassesBytesUnspelled: text holding a marker, written on the
// writer Underlying finds past a Spelled one, or on a writer that is not
// Spelled, arrives as written.
func TestUnderlyingPassesBytesUnspelled(t *testing.T) {
	text := "key-" + argmark.Flag("limit") + "\n"
	for _, tc := range []struct {
		name string
		wrap func(*bytes.Buffer) io.Writer
	}{
		{"spelled", func(b *bytes.Buffer) io.Writer { return Spelled(b, argmark.Spelling{MCP: true}) }},
		{"plain", func(b *bytes.Buffer) io.Writer { return b }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if _, err := io.WriteString(Underlying(tc.wrap(&buf)), text); err != nil {
				t.Fatal(err)
			}
			if buf.String() != text {
				t.Errorf("wrote %q, want %q", buf.String(), text)
			}
		})
	}
}

func TestAnErrorSpellsItsArgumentsAtTheCommandLine(t *testing.T) {
	err := Refusal{{Detail: argmark.Flag("limit") + " needs " + argmark.Flag("sort")}, {Detail: "no " + argmark.Positional("id")}}
	if got, want := err.Error(), "--limit needs --sort; no <id>"; got != want {
		t.Errorf("Error = %q, want %q", got, want)
	}
}

// TestWithWarningsPlacesThemBeforeTheClosingKey: the warnings go last among
// the verb's own keys, before a closing key where the fields end on one, and
// the fields given are left as they were.
func TestWithWarningsPlacesThemBeforeTheClosingKey(t *testing.T) {
	warnings := Field{Key: WarningsKey}
	for _, tc := range []struct {
		name string
		keys []string
		want []string
	}{
		{"no fields", nil, []string{WarningsKey}},
		{"no closing key", []string{"a", "b"}, []string{"a", "b", WarningsKey}},
		{"refusals", []string{"a", RefusalsKey}, []string{"a", WarningsKey, RefusalsKey}},
		{"failures", []string{"a", FailuresKey}, []string{"a", WarningsKey, FailuresKey}},
		{"cancelled", []string{"a", CancelledKey}, []string{"a", WarningsKey, CancelledKey}},
		{"closing key alone", []string{RefusalsKey}, []string{WarningsKey, RefusalsKey}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fields := make([]Field, len(tc.keys), len(tc.keys)+1)
			for i, k := range tc.keys {
				fields[i] = Field{Key: k}
			}
			var got []string
			for _, f := range WithWarnings(fields, warnings) {
				got = append(got, f.Key)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("keys = %q, want %q", got, tc.want)
			}
			for i, k := range tc.keys {
				if fields[i].Key != k {
					t.Errorf("the fields given became %v", fields)
				}
			}
		})
	}
}
