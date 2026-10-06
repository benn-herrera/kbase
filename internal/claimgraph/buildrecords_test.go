package claimgraph

import (
	"encoding/json"
	"reflect"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/buildrecords"
	"kbase/internal/kb"
	"kbase/internal/migrate"
)

// TestEncodeRecordAsTheMigrationWrites pins encodeRecord to the bytes the
// 0.9.0 → 1.0.0 migration writes for the same record, so a migrated build
// record is the writer's own, and checks each reads back as written.
func TestEncodeRecordAsTheMigrationWrites(t *testing.T) {
	ls, ps := string(rune(0x2028)), string(rune(0x2029))
	type text struct {
		T string `json:"t" yaml:"t"`
	}
	for _, tc := range []struct {
		name string
		in   any
		back func() any
	}{
		{"control characters", text{"a\"b\\c\n\r\t\b\f\x01\x1f"}, func() any { return &text{} }},
		{"characters JSON writes raw", text{"<>& é\x7f" + ls + ps}, func() any { return &text{} }},
		{"an escaped backslash before a separator's digits", text{"\\" + "u2028"}, func() any { return &text{} }},
		{"strings YAML would take for something else", text{"yes"}, func() any { return &text{} }},
		{"empty lists and null", buildrecords.UnmarkedRecord{About: "x", Pairs: []buildrecords.CandidateEntry{}}, func() any { return &buildrecords.UnmarkedRecord{} }},
		{"an empty mapping", buildrecords.NodePassRecord{About: "x", Leaves: map[string]buildrecords.LeafEntry{}}, func() any { return &buildrecords.NodePassRecord{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := encodeRecord(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			js, err := json.Marshal(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			const p = "kb-build-unmarked.json"
			out, _, err := migrate.Chain(kb.UnstampedFormatVersion, kb.FormatVersion, migrate.Files{p: js})
			if err != nil {
				t.Fatal(err)
			}
			if want := out[buildrecords.UnmarkedFile]; string(got) != string(want) {
				t.Errorf("encodeRecord =\n%s\nthe migration writes\n%s", got, want)
			}
			back := tc.back()
			if err := yaml.Unmarshal(got, back); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(reflect.ValueOf(back).Elem().Interface(), tc.in) {
				t.Errorf("reads back as %+v, want %+v", back, tc.in)
			}
		})
	}
}

// TestNodePassWrittenAsKbTools checks the writer's normalization: leaves by
// path, verdicts by line, nil lists written as [].
func TestNodePassWrittenAsKbTools(t *testing.T) {
	cause := CauseNoLetter
	got, err := nodePassBytes(buildrecords.NodePassRecord{Leaves: map[string]buildrecords.LeafEntry{
		"b.md": {State: ReadLanded, Verdicts: []buildrecords.ParagraphVerdict{{Line: 9, Verdict: JudgementDefaulted, Cause: &cause}, {Line: 2, Verdict: JudgementNotAClaim}}},
		"a.md": {State: ReadUnread},
	}})
	want := "about: \"" + nodePassAbout + "\"\nleaves:\n" +
		"  a.md:\n    state: unread\n    outcome: null\n    claims: []\n    verdicts: []\n" +
		"  b.md:\n    state: landed\n    outcome: null\n    claims: []\n    verdicts:\n" +
		"      - line: 2\n        verdict: not-a-claim\n        cause: null\n" +
		"      - line: 9\n        verdict: defaulted\n        cause: no-letter\n"
	if err != nil || string(got) != want {
		t.Errorf("nodePassBytes = %s, %v; want %s", got, err, want)
	}
}
