package mcp

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"kbase/internal/buildstore"
	"kbase/internal/config"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/mcp/protocol"
	"kbase/internal/result"
	"kbase/internal/score"
)

// The resources' URIs. A claim-graph sheet's is SheetURIPrefix followed by its
// kb-root-relative slash path; the model guidance, the build trail and the
// build's state are one resource each.
const (
	SheetURIPrefix = "kbase://sheet/"
	ModelHintsURI  = "kbase://model-hints"
	BuildTrailURI  = "kbase://build-trail"
	BuildURI       = "kbase://build"
)

// ResourceFamily is one resource source and the URI prefix it answers, bound
// when the server serves: bind is its resources over what the server serves,
// and the subscription a client may take to it where it offers one.
type ResourceFamily struct {
	prefix string
	bind   func(serving) (protocol.Resources, *protocol.Subscription)
}

// boundFamily is a family's resources as one server serves them, and the URI
// prefix it answers.
type boundFamily struct {
	protocol.Resources
	prefix string
}

// SheetResources offers the claim-graph sheets of the KB open finds from the
// served directory. open returns no source where no KB is found yet, and a
// result.Refusal where the KB found will not open.
func SheetResources(open func(dir string) (*kb.Source, error)) ResourceFamily {
	return ResourceFamily{prefix: SheetURIPrefix, bind: func(s serving) (protocol.Resources, *protocol.Subscription) {
		return sheetResources{dir: s.dir, open: open}, nil
	}}
}

type sheetResources struct {
	dir  string
	open func(dir string) (*kb.Source, error)
}

func (r sheetResources) List() ([]protocol.Resource, error) {
	src, err := r.open(r.dir)
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
	src, err := r.open(r.dir)
	if err != nil {
		return "", err
	}
	if src == nil {
		return "", fmt.Errorf("no KB is found from %s", r.dir)
	}
	data, err := src.ReadFile(src.KBPath(strings.TrimPrefix(sheet.URI, SheetURIPrefix)))
	return string(data), err
}

// ModelHintsResource serves the [models] guidance the config template carries:
// the rows the config.toml template is written with, as text that needs no KB.
func ModelHintsResource() ResourceFamily {
	return ResourceFamily{prefix: ModelHintsURI, bind: func(serving) (protocol.Resources, *protocol.Subscription) {
		return modelHintsResource{}, nil
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

// RewindBuildTool is the one tool no subcommand stands behind: the build's
// commit trail of the repository dir is in, and for a target boundary the
// commands that reset the repository to it and re-issue the build. It reads
// the tree, the state store and history, and writes none of them.
func RewindBuildTool(dir string) protocol.Tool {
	return protocol.Tool{
		Name: "rewind-build",
		Description: "Every stage boundary of a build is a commit in the repository; this tool lists " +
			"them itself, no separate query, and for a target boundary, commands that reset the " +
			"repository to it and re-issue the build. A build rewound to one resumes by re-issuing " +
			"`build` with the newest boundary's inputs — the same volume roots and `--bibliography` " +
			"files, same order; other inputs refused (`check: inputs`); read-only, it changes nothing.",
		InputSchema: protocol.Schema{
			Properties: map[string]protocol.Property{
				"target":     {Type: "string", Description: "the boundary to reset to, by stage id or display name"},
				stateDirFlag: {Type: "string", Description: "the build's state store (default: $XDG_STATE_HOME/kbase/<key>)"},
			},
			Required: []string{},
		},
		ReadOnly: true,
		Call: func(args map[string]any) (string, map[string]any, bool, error) {
			target, _ := args["target"].(string)
			stateDir, _ := args[stateDirFlag].(string)
			doc, err := rewindDocument(buildstore.RewindOptions{RepoRoot: dir, StateDir: stateDir, Target: target})
			if err != nil {
				return "", nil, false, err
			}
			return ToolResult([]byte(doc))
		},
	}
}

// rewindDocument is buildstore.Rewind's document rendered: the one document the
// rewind-build tool answers with and the build-trail resource reads.
func rewindDocument(opts buildstore.RewindOptions) (string, error) {
	outcome, fields := buildstore.Rewind(opts)
	var out bytes.Buffer
	if err := result.Emit(&out, outcome, fields...); err != nil {
		return "", fmt.Errorf("rendering the build trail: %w", err)
	}
	return out.String(), nil
}

// BuildTrailResource serves the build trail of the repository the served
// directory is in, as the rewind-build tool reports it with no target named.
func BuildTrailResource() ResourceFamily {
	return ResourceFamily{prefix: BuildTrailURI, bind: func(s serving) (protocol.Resources, *protocol.Subscription) {
		return buildTrailResource{s.dir}, nil
	}}
}

type buildTrailResource struct{ dir string }

// List offers the trail whether or not a state store or a boundary exists: the
// document says what the repository holds, and reading it is how a caller
// learns the repository holds nothing.
func (buildTrailResource) List() ([]protocol.Resource, error) {
	return []protocol.Resource{{URI: BuildTrailURI, Name: "Build trail", MimeType: "application/yaml"}}, nil
}

func (r buildTrailResource) Read(protocol.Resource) (string, error) {
	return rewindDocument(buildstore.RewindOptions{RepoRoot: r.dir})
}

// BuildResource serves the build's state as the status tool reports it, for
// the store of the build the server launched last — the served repository's
// default store where it launched none — and the subscription that announces
// each run's end.
func BuildResource() ResourceFamily {
	return ResourceFamily{prefix: BuildURI, bind: func(s serving) (protocol.Resources, *protocol.Subscription) {
		return buildResource{s.last}, buildSubscription(s.last, s.lg)
	}}
}

type buildResource struct{ last *lastBuild }

// List offers the state whether or not a build ever ran: the document says so,
// and a read is how a caller learns it.
func (buildResource) List() ([]protocol.Resource, error) {
	return []protocol.Resource{{URI: BuildURI, Name: "Build state", MimeType: "application/yaml"}}, nil
}

// Read is the status document for the store, and it never fails: a store no
// build wrote holds no run and reads state none, and a served directory no
// worktree holds reads the refusal status answers with.
func (r buildResource) Read(protocol.Resource) (string, error) {
	dir, _ := r.last.store()
	return statusDocument(buildstore.MonitorOptions{StateDir: dir, WorkDir: r.last.served})
}

// statusDocument is score.Status's document rendered: the one document the
// status tool answers with and the build resource reads, the score run's where
// the store stands at one.
func statusDocument(opts buildstore.MonitorOptions) (string, error) {
	outcome, fields := score.Status(opts)
	var out bytes.Buffer
	if err := result.Emit(&out, outcome, fields...); err != nil {
		return "", fmt.Errorf("rendering the build's state: %w", err)
	}
	return out.String(), nil
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
// since one resource's URI may lead another's, as kbase://build leads
// kbase://build-trail.
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
