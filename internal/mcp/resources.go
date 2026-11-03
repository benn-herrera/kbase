package mcp

import (
	"errors"
	"fmt"
	"strings"

	"kbase/internal/config"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/mcp/protocol"
	"kbase/internal/result"
)

// The resources' URIs. A claim-graph sheet's is SheetURIPrefix followed by its
// kb-root-relative slash path; the model guidance is one resource.
const (
	SheetURIPrefix = "kbase://sheet/"
	ModelHintsURI  = "kbase://model-hints"
)

// ResourceFamily is one resource source and the URI prefix it answers, bound
// when the server serves: bind is its resources over what the server serves.
type ResourceFamily struct {
	prefix string
	bind   func(serving) protocol.Resources
}

// boundFamily is a family's resources as one server serves them, and the URI
// prefix it answers.
type boundFamily struct {
	protocol.Resources
	prefix string
}

// SheetResources offers the claim-graph sheets of the KB open finds from the
// located project, and none where the server located none. open returns no
// source where no KB is found yet, and a result.Refusal where the KB found
// will not open.
func SheetResources(open func(dir string) (*kb.Source, error)) ResourceFamily {
	return ResourceFamily{prefix: SheetURIPrefix, bind: func(s serving) protocol.Resources {
		return sheetResources{located: s.located, open: open}
	}}
}

type sheetResources struct {
	located Located
	open    func(dir string) (*kb.Source, error)
}

// source is the located project's KB: none where there is no located project
// or no KB is found there yet.
func (r sheetResources) source() (*kb.Source, error) {
	root, ok := r.located.Root()
	if !ok {
		return nil, nil
	}
	return r.open(root)
}

func (r sheetResources) List() ([]protocol.Resource, error) {
	src, err := r.source()
	if err != nil || src == nil {
		return nil, err
	}
	_, present, err := index.SheetPaths(src)
	if err != nil {
		return nil, err
	}
	var sheets []protocol.Resource
	for _, rel := range present {
		sheets = append(sheets, protocol.Resource{URI: SheetURIPrefix + rel, Name: rel, MimeType: "image/svg+xml"})
	}
	return sheets, nil
}

func (r sheetResources) Read(sheet protocol.Resource) (string, error) {
	src, err := r.source()
	if err != nil {
		return "", err
	}
	if src == nil {
		return "", errors.New("no KB is found in the server's located project")
	}
	data, err := src.ReadFile(src.KBPath(strings.TrimPrefix(sheet.URI, SheetURIPrefix)))
	return string(data), err
}

// ModelHintsResource serves the [models] guidance the config template carries:
// the rows the config.toml template is written with, as text that needs no KB.
func ModelHintsResource() ResourceFamily {
	return ResourceFamily{prefix: ModelHintsURI, bind: func(serving) protocol.Resources {
		return modelHintsResource{}
	}}
}

type modelHintsResource struct{}

func (modelHintsResource) List() ([]protocol.Resource, error) {
	return []protocol.Resource{{URI: ModelHintsURI, Name: "Model hints", MimeType: "text/markdown"}}, nil
}

// Read returns the whole guidance: it is one document, and List offers no
// other resource of this family.
func (modelHintsResource) Read(protocol.Resource) (string, error) {
	return config.ModelHintsMarkdown(), nil
}

// resourceFamilies serves the families as one Resources: List is each family's
// in order, and Read goes to the family whose prefix the URI carries.
type resourceFamilies []boundFamily

// List is every family that lists, in order. A family refusing what it lists
// from — a KB that exists but will not open — contributes no entries and the
// rest still list: the refusal is a fact about the KB, which status and verify
// report when asked, not a fault in the list. Any other error is a fault, and
// is returned as the family raised it, so joining families changes what is
// listed, not what any one of them answers.
func (f resourceFamilies) List() ([]protocol.Resource, error) {
	var listed []protocol.Resource
	for _, family := range f {
		one, err := family.List()
		var refusal result.Refusal
		if errors.As(err, &refusal) {
			continue
		}
		if err != nil {
			return nil, err
		}
		listed = append(listed, one...)
	}
	return listed, nil
}

// Read goes to the family serving the URI: the one whose prefix is longest,
// since one resource's URI may lead another's.
func (f resourceFamilies) Read(res protocol.Resource) (string, error) {
	i := -1
	for j, family := range f {
		if strings.HasPrefix(res.URI, family.prefix) && (i < 0 || len(family.prefix) > len(f[i].prefix)) {
			i = j
		}
	}
	if i < 0 {
		return "", fmt.Errorf("no source serves %q", res.URI)
	}
	return f[i].Read(res)
}
