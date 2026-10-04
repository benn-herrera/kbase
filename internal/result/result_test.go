package result

import (
	"bytes"
	"reflect"
	"testing"

	"go.yaml.in/yaml/v3"
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

func TestFloatTextIsPositional(t *testing.T) {
	for in, want := range map[float64]string{1: "1.0", 0.00001: "0.00001", 0.55: "0.55", 1e21: "1000000000000000000000.0"} {
		if got := floatText(in); got != want {
			t.Errorf("floatText(%v) = %q, want %q", in, got, want)
		}
	}
}
