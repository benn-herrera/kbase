package skeleton

import (
	"bytes"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// composeFixture is one non-trivial skeleton: a split group, a nested index,
// and an annex, so the round-trip covers every field of every type.
func composeFixture(t *testing.T) (*Verifier, Skeleton) {
	t.Helper()
	v, art := verifierFor(t, testParams(),
		docSpec{path: "big.md", title: "Big", secs: []secSpec{
			{title: "Long", paras: 8, words: 60},
			{title: "Short", paras: 1, words: 20},
		}},
		oneSectionDoc("ref/api.md", "API"))
	plan := Plan{Title: "The Corpus", Scope: "everything under one roof", Children: []PlanNode{
		indexNode("Chapters",
			leafFor(art, "big.md", 0, "Long Chapter"),
			leafFor(art, "big.md", 1, "Short Chapter")),
	}}
	s, err := v.Compose(plan, []Annex{{Prefix: "ref", Convention: "ref/<name>.md, e.g. ref/api.md"}})
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	return v, s
}

// The artifact is byte-identical for identical input. Stage 3 is resumable and
// stage 9 re-checks a tree it did not build; both need the same corpus and the
// same parameters to produce the same bytes.
func TestArtifactIsByteDeterministic(t *testing.T) {
	var first bytes.Buffer
	_, s := composeFixture(t)
	if err := s.WriteJSON(&first); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	for range 3 {
		var again bytes.Buffer
		_, s2 := composeFixture(t)
		if err := s2.WriteJSON(&again); err != nil {
			t.Fatalf("WriteJSON: %v", err)
		}
		if !bytes.Equal(first.Bytes(), again.Bytes()) {
			t.Fatalf("the same inputs produced different artifacts:\n%s\n---\n%s",
				first.String(), again.String())
		}
	}
}

func TestArtifactRoundTrips(t *testing.T) {
	v, s := composeFixture(t)

	var buf bytes.Buffer
	if err := s.WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	got, err := ReadJSON(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if !reflect.DeepEqual(got, s) {
		t.Fatalf("round trip changed the artifact:\n got %+v\nwant %+v", got, s)
	}
	// The decoded artifact passes the same check the composed one did — which
	// is what stage 9 is relying on when it re-checks a resumed job's tree.
	if err := v.Check(got); err != nil {
		t.Fatalf("the decoded artifact does not verify: %v", err)
	}
	// And it re-encodes to the same bytes.
	var again bytes.Buffer
	if err := got.WriteJSON(&again); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), again.Bytes()) {
		t.Fatal("decode/encode is not the identity on the artifact's bytes")
	}
}

func TestReadJSONRefusesWhatThisBuildCannotRead(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{"another schema", `{"schema":"kbase.skeleton/99","corpusHash":"x","budgets":{},"nodes":[],"groups":[]}`},
		{"no schema", `{"corpusHash":"x","budgets":{},"nodes":[],"groups":[]}`},
		{"a field this build has never heard of",
			`{"schema":"kbase.skeleton/1","corpusHash":"x","budgets":{},"nodes":[],"groups":[],"related":[]}`},
		{"not json", `nodes: []`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ReadJSON(strings.NewReader(tc.body)); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}

// The artifact carries no interior boundary of a split group, by construction
// (I-1, F-2): the skeleton owns which bytes, the cut list owns where inside
// them the boundaries fall. The check is on the BYTES rather than on the
// struct, because a field added later would pass a struct-shaped assertion.
func TestArtifactCarriesNoInteriorBoundary(t *testing.T) {
	v, s := composeFixture(t)

	var multi Group
	for _, g := range s.Groups {
		if g.Parts > 1 {
			multi = g
		}
	}
	if multi.ID == "" {
		t.Fatal("the fixture has no multi-part group; it is meant to have one")
	}
	cuts, err := v.split(multi.Source)
	if err != nil {
		t.Fatalf("split: %v", err)
	}

	var buf bytes.Buffer
	if err := s.WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	body := buf.String()
	for i := 1; i < len(cuts); i++ {
		at := cuts[i].Start
		if at == multi.Source.Start || at == multi.Source.End {
			continue
		}
		for _, form := range []string{
			`"start": ` + strconv.Itoa(at), `"end": ` + strconv.Itoa(at),
		} {
			if strings.Contains(body, form) {
				t.Errorf("the artifact states interior boundary %d (%s); stage 4 owns that offset", at, form)
			}
		}
	}
}
