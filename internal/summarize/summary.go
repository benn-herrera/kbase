package summarize

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// SchemaVersion identifies the artifact shape. Stage 8 reads it back off disk
// after a resume, so it says what shape it is rather than being guessed at
// from the fields present.
const SchemaVersion = "kbase.summary/1"

// Summary is one index node's model-written prose: the three string fields
// §4.2 leaves to a model, and nothing else.
//
// It is NEVER rendered Markdown. Stage 8 renders, so a summary regenerated
// later cannot half-rewrite a delivered page (I-12) — and the fields carry no
// link syntax and no node path, so the prose cannot misroute a reader either
// (I-2, I-4). What routes is the down-link list, and that comes from the tree
// plan.
type Summary struct {
	// Unit is the artifact's store path. It is not marshalled: the artifact
	// lives AT that path, and a copy of it inside the file would be a second
	// statement of where the file is.
	Unit string `json:"-"`

	Schema string `json:"schema"`
	// Framing is the 0–3 sentences under the H1. Empty is legal in the
	// grammar; this stage's verifier asks for one anyway (§6.6).
	Framing string `json:"framing"`
	// ConclusionsHeading is the model's own heading for the block below —
	// "Key Results" is not forced onto material that has none (§0.5, §4.5.5).
	ConclusionsHeading string `json:"conclusionsHeading"`
	// Conclusions is what this subtree establishes, decides or instructs.
	// EMPTY IS LEGAL (§4.5.6): an index over purely navigational material has
	// nothing to conclude, and an invented conclusion is worse than a missing
	// one.
	Conclusions string `json:"conclusions"`
}

// WriteJSON writes the artifact with the discipline every other derived file
// here uses: HTML escaping off (the prose quotes source material, which
// legitimately holds `<` and `&`), indented, one trailing newline.
func (s Summary) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		return fmt.Errorf("summarize: write %s: %w", s.Unit, err)
	}
	return nil
}

// ReadJSON decodes an artifact and refuses one this build does not understand.
//
// A decoder is not optional: the level above reads its children's summaries
// back off disk, and so does stage 8, both across a resume boundary where the
// reader cannot ask the writer what it meant.
func ReadJSON(r io.Reader) (Summary, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	var s Summary
	if err := dec.Decode(&s); err != nil {
		return Summary{}, fmt.Errorf("summarize: read artifact: %w", err)
	}
	if s.Schema != SchemaVersion {
		return Summary{}, fmt.Errorf("summarize: artifact declares schema %q, this build reads %q",
			s.Schema, SchemaVersion)
	}
	return s, nil
}

// Encode renders a verified summary for the store — the stage's
// pipeline.AskSpec.Encode.
func Encode(artifact any) ([]byte, error) {
	s, ok := artifact.(Summary)
	if !ok {
		return nil, fmt.Errorf("summarize: artifact is %T, not a Summary", artifact)
	}
	if s.Schema != SchemaVersion {
		return nil, fmt.Errorf("summarize: %s declares schema %q, this build writes %q",
			s.Unit, s.Schema, SchemaVersion)
	}
	var buf bytes.Buffer
	if err := s.WriteJSON(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
