// Package survey is pipeline stage 2 (ARCHITECTURE.md §4): the per-file
// structural inventory the taxonomy stage consumes instead of raw source.
//
// It is entirely mechanical — heading tree, section token sizes, link graph,
// first-paragraph gists — and produces a compact artifact with two properties
// the rest of the pipeline leans on:
//
//   - Every structural fact is a byte offset into the ingested source
//     (ARCHITECTURE.md §5). The survey copies no content except gists, which
//     are routing hints, and titles, which are labels; a consumer that wants
//     text reads the offsets back out of the source it already holds.
//   - The same corpus produces byte-identical JSON. No maps on the marshal
//     path, no timestamps, no absolute paths — provenance is the corpus
//     content hash, not where the machine happened to keep the files.
//
// Section ranges tile each file exactly: front matter, preamble, and the
// heading tree together cover every byte, once. Survey verifies this before
// returning, because the dissector (stage 4) inherits the property, and a
// gap introduced here would surface as silently missing source in a leaf.
package survey

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/yuin/goldmark"

	"kbase/internal/ingest"
	"kbase/internal/log"
	"kbase/internal/tokens"
)

// SchemaVersion identifies the artifact shape. Consumers (the taxonomy
// stage, a later survey-of-surveys rollup) check it rather than guessing
// from the fields present, and a shape change bumps it.
const SchemaVersion = "kbase.survey/1"

// Artifact is a whole-corpus survey: the totals a planner needs up front,
// then one entry per file in path order.
type Artifact struct {
	Schema string `json:"schema"`
	Corpus Totals `json:"corpus"`
	Files  []File `json:"files"`
}

// Totals is the corpus-level roll-up. ContentHash is the source identity
// (ingest.Corpus.ContentHash) — the artifact's only provenance, and the only
// thing tying it to the bytes it describes.
type Totals struct {
	ContentHash string     `json:"contentHash"`
	Files       int        `json:"files"`
	Bytes       int        `json:"bytes"`
	Tokens      int        `json:"tokens"`
	Sections    int        `json:"sections"`
	Links       LinkTotals `json:"links"`
}

// LinkTotals counts outbound links by kind across the corpus. Unresolved is
// the interesting one: it is the corpus's broken-reference budget, and the
// link stage (§4 stage 8) will need to know about every one of them.
type LinkTotals struct {
	Internal   int `json:"internal"`
	Unresolved int `json:"unresolved"`
	External   int `json:"external"`
	Anchor     int `json:"anchor"`
}

// File is one document's inventory.
//
// FrontMatter, Preamble, and Sections partition the file's bytes in that
// order. Preamble is the content before the first heading — for a file with
// no headings at all it is the whole document, which is why it is a Section
// rather than a bare range: it carries the same token count and gist a
// heading section does, and the taxonomy stage treats it the same way.
//
// Title, Description and Tags are the front-matter labels, absent when the
// file has no front matter or the block does not carry them. They are the
// only fields here that are neither an offset nor derived from the body: in
// the Docusaurus/Hugo shape the block exists for, they are what the document
// calls itself, and stage 3 has no other way to learn it.
type File struct {
	Path        string    `json:"path"`
	SHA256      string    `json:"sha256"`
	Bytes       int       `json:"bytes"`
	Tokens      int       `json:"tokens"`
	Title       string    `json:"title,omitempty"`
	Description string    `json:"description,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	Gist        string    `json:"gist,omitempty"`
	FrontMatter *Range    `json:"frontMatter,omitempty"`
	Preamble    *Section  `json:"preamble,omitempty"`
	Sections    []Section `json:"sections,omitempty"`
	Links       []Link    `json:"links,omitempty"`
}

// Range is a half-open byte range [Start, End) into a file's source bytes.
type Range struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Section is one heading and everything under it, up to the next heading of
// the same or a higher level.
//
// Start is the first byte of the heading line itself, not of its text, so a
// section's range includes the heading that names it — a leaf extracted by
// offset carries its own title. End is exclusive. Children nest by level and
// their ranges tile [first child Start, End); the bytes between Start and the
// first child are the section's own body.
//
// Tokens covers the whole subtree, which is the number the taxonomy stage
// compares against a per-call budget when deciding whether a section can be
// one leaf.
type Section struct {
	// Level is the heading level, 1–6; the preamble section carries 0,
	// which is what marks it as headingless.
	Level    int       `json:"level"`
	Title    string    `json:"title,omitempty"`
	Start    int       `json:"start"`
	End      int       `json:"end"`
	Tokens   int       `json:"tokens"`
	Gist     string    `json:"gist,omitempty"`
	Children []Section `json:"children,omitempty"`
}

// LinkKind classifies an outbound link by where it points.
type LinkKind string

const (
	// LinkInternal resolves to another document in the corpus; Path names it.
	LinkInternal LinkKind = "internal"
	// LinkUnresolved is corpus-relative but names no ingested document —
	// recorded, never dropped, for the link stage to adjudicate.
	LinkUnresolved LinkKind = "unresolved"
	// LinkExternal carries a scheme or a host: off-corpus, nobody's problem
	// here.
	LinkExternal LinkKind = "external"
	// LinkAnchor is a fragment with no path: a jump within the same file.
	LinkAnchor LinkKind = "anchor"
)

// Link is one distinct outbound reference from a file. Repeats of the same
// target collapse to one entry — the taxonomy stage reads a graph, and edge
// multiplicity is not part of it.
type Link struct {
	Kind LinkKind `json:"kind"`
	// Target is the destination exactly as written in the source.
	Target string `json:"target"`
	// Path is the resolved corpus-relative document, set for LinkInternal.
	Path string `json:"path,omitempty"`
	// Fragment is the `#fragment` part, without the `#`.
	Fragment string `json:"fragment,omitempty"`
	// Image is true when the reference is an image rather than a link.
	Image bool `json:"image,omitempty"`
}

// Survey inventories every document in the corpus.
//
// It fails rather than degrades: a file whose sections do not tile is a defect
// in this package, and returning a plausible-looking artifact would push the
// consequences into a stage that cannot see the cause.
//
// lg carries the decisions the artifact records only the outcome of — which
// blocks were read as front matter, which links resolved by fold — plus one
// warning for the corpus's unresolved-link total, which is the number stage 8
// eventually has to answer for.
func Survey(corpus ingest.Corpus, est tokens.Estimator, lg log.Logger) (Artifact, error) {
	// One parser for the whole corpus. goldmark's parser holds no
	// per-document state (each Parse builds its own context), so this is a
	// setup cost paid once rather than per file.
	p := goldmark.DefaultParser()

	art := Artifact{
		Schema: SchemaVersion,
		Corpus: Totals{ContentHash: corpus.ContentHash, Files: len(corpus.Units)},
		Files:  make([]File, 0, len(corpus.Units)),
	}
	for _, u := range corpus.Units {
		f, err := surveyFile(p, u, corpus, est, lg)
		if err != nil {
			return Artifact{}, err
		}
		art.Files = append(art.Files, f)

		art.Corpus.Bytes += f.Bytes
		art.Corpus.Tokens += f.Tokens
		if f.Preamble != nil {
			art.Corpus.Sections++
		}
		art.Corpus.Sections += countSections(f.Sections)
		for _, l := range f.Links {
			switch l.Kind {
			case LinkInternal:
				art.Corpus.Links.Internal++
			case LinkUnresolved:
				art.Corpus.Links.Unresolved++
			case LinkExternal:
				art.Corpus.Links.External++
			case LinkAnchor:
				art.Corpus.Links.Anchor++
			}
		}
	}
	if n := art.Corpus.Links.Unresolved; n > 0 {
		lg.Warn("survey found unresolved corpus links", "count", n, "files", len(art.Files))
	}
	return art, nil
}

// WriteJSON writes the artifact as indented JSON with a trailing newline.
//
// HTML escaping is off: gists and titles come from Markdown and legitimately
// contain `<` and `&`, and escaping them makes the artifact harder to read
// without making it any more valid. Determinism is unaffected either way —
// struct field order is fixed, and nothing here marshals a map.
func (a Artifact) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(a); err != nil {
		return fmt.Errorf("survey: write artifact: %w", err)
	}
	return nil
}

// countSections totals a section forest, children included.
func countSections(secs []Section) int {
	n := 0
	for _, s := range secs {
		n += 1 + countSections(s.Children)
	}
	return n
}
