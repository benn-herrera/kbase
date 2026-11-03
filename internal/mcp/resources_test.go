package mcp

import (
	"errors"
	"slices"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/mcp/protocol"
	"kbase/internal/result"
)

// stubResources answers List with what a test supplies and reads nothing.
type stubResources struct {
	listed []protocol.Resource
	err    error
}

func (s stubResources) List() ([]protocol.Resource, error) { return s.listed, s.err }

func (s stubResources) Read(protocol.Resource) (string, error) { return "", nil }

// TestResourceFamiliesListSkipsOnlyRefusals: a family refusing what it lists
// from contributes no entries and leaves the rest listing, while any other
// error is the list's and stops it.
func TestResourceFamiliesListSkipsOnlyRefusals(t *testing.T) {
	hints := []protocol.Resource{{URI: ModelHintsURI}}
	refusing := stubResources{err: result.Refusal{{Check: kb.CheckKBRoot, Path: "kb-root"}}}
	families := resourceFamilies{{Resources: refusing}, {Resources: stubResources{listed: hints}}}
	if listed, err := families.List(); err != nil || !slices.Equal(listed, hints) {
		t.Errorf("a refusing family: listed %v, %v; want %v", listed, err, hints)
	}
	failing := stubResources{err: errors.New("the KB's index is unreadable")}
	families = resourceFamilies{{Resources: stubResources{listed: hints}}, {Resources: failing}}
	if listed, err := families.List(); !errors.Is(err, failing.err) || listed != nil {
		t.Errorf("a failing family: listed %v, %v; want its error and no list", listed, err)
	}
}
