package markdown

import (
	"bytes"
	"errors"

	"gopkg.in/yaml.v3"

	"kbase/internal/log"
	"kbase/internal/survey"
)

// frontMatterDelim opens and closes a YAML front-matter block. Front matter
// is this format's spelling of the artifact's neutral metadata range
// (survey.File.Metadata) — inside this package the Markdown word is the
// correct one.
const frontMatterDelim = "---"

// frontMatterFields are the labels a front-matter mapping carries. They are
// in the artifact because ARCHITECTURE.md §4 stage 3 consumes the survey and
// never the raw source: in a corpus whose H1 lives in front matter and whose
// body opens with prose — exactly the convention this branch exists because
// of — a file with no title here surveys as a path and a gist, which is thin
// material for designing a hierarchy out of.
//
// Unknown keys are ignored rather than recorded. Front matter is a site
// generator's configuration; the taxonomy stage needs the labels a human
// would recognize the document by, not `sidebar_position`.
type frontMatterFields struct {
	Title       string   `yaml:"title"`
	Description string   `yaml:"description"`
	Tags        []string `yaml:"tags"`
}

// normalized puts the values through the artifact's one string rule
// (survey.CapWords) and drops tags that carry nothing, so a `tags: [a, "", b]`
// block does not spend an artifact slot on an empty string.
func (f frontMatterFields) normalized() frontMatterFields {
	f.Title, f.Description = survey.CapWords(f.Title), survey.CapWords(f.Description)
	tags := make([]string, 0, len(f.Tags))
	for _, t := range f.Tags {
		if t = survey.CapWords(t); t != "" {
			tags = append(tags, t)
		}
	}
	f.Tags = nil
	if len(tags) > 0 {
		f.Tags = tags
	}
	return f
}

// bomUTF8 is the byte-order mark editors on Windows put at the head of a
// UTF-8 file.
var bomUTF8 = []byte{0xEF, 0xBB, 0xBF}

// frontMatter reports the byte range of a leading YAML front-matter block and
// the labels it carries: a `---` line at the start of the file, up to and
// including the next `---` line, whose contents parse as a YAML mapping.
//
// Three things it deliberately is not:
//
//   - Anchored at byte 0. A leading BOM is skipped before the delimiter
//     check — with it in the way, line 1 is not a thematic break and
//     CommonMark reads the whole YAML block as a setext H2 title, which is
//     precisely the misparse this function exists to prevent. The BOM is
//     folded INTO the recorded range rather than trimmed off it, so the
//     range still starts at 0 and the file's ranges still tile it exactly.
//   - Satisfied by delimiters alone. See parseFrontMatter.
//   - Willing to run past a missing close. Without a closing delimiter there
//     is no block: an unterminated one is a thematic break followed by prose,
//     which is what CommonMark says it is.
func frontMatter(src []byte, from string, lg log.Logger) (survey.Range, frontMatterFields, bool) {
	start := 0
	if bytes.HasPrefix(src, bomUTF8) {
		start = len(bomUTF8)
	}
	first, next := readLine(src, start)
	if !isFrontMatterDelim(first) {
		return survey.Range{}, frontMatterFields{}, false
	}
	for off := next; off < len(src); {
		line, after := readLine(src, off)
		if !isFrontMatterDelim(line) {
			off = after
			continue
		}
		fields, ok := parseFrontMatter(src[next:off])
		if !ok {
			// Not a rejection of the file — a decision that these bytes are
			// content. They go back to the Markdown parse, where the headings
			// and links inside them still count.
			lg.Debug("survey read a delimited block as content, not front matter",
				"file", from, "end", after, "reason", "not a YAML mapping")
			return survey.Range{}, frontMatterFields{}, false
		}
		lg.Debug("survey detected front matter", "file", from, "end", after,
			"titled", fields.Title != "", "tags", len(fields.Tags))
		return survey.Range{Start: 0, End: after}, fields, true
	}
	return survey.Range{}, frontMatterFields{}, false
}

// parseFrontMatter decides whether a delimited block really is front matter,
// and lifts its labels out if so.
//
// The rule is exact rather than heuristic: front matter is a YAML MAPPING, or
// it is empty. A document that opens with a thematic break and prose — `---`,
// a paragraph, another `---` used as a section rule — parses as a scalar, not
// a mapping, so it is content; without this gate its headings, links and
// gists would vanish into an opaque metadata range and no tiling check would
// notice, because an opaque range tiles perfectly well. What survives is the
// case where the opening prose genuinely reads `key: value`, which a human
// would call ambiguous too.
func parseFrontMatter(block []byte) (frontMatterFields, bool) {
	var doc yaml.Node
	if err := yaml.Unmarshal(block, &doc); err != nil {
		return frontMatterFields{}, false
	}
	// An empty block (`---` immediately followed by `---`) parses to no node
	// at all — the zero Kind. It is front matter carrying nothing, a shape
	// real corpora have, and nothing is lost by recording it as such.
	if doc.Kind == 0 {
		return frontMatterFields{}, true
	}
	node := &doc
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) == 0 {
			return frontMatterFields{}, true
		}
		node = node.Content[0]
	}
	if node.Kind != yaml.MappingNode {
		return frontMatterFields{}, false
	}

	var f frontMatterFields
	if err := node.Decode(&f); err != nil {
		// The block is already known to be a mapping, so the only failure
		// left is a key whose value is the wrong shape (`tags: sometimes`).
		// yaml fills in what it could, and the survey keeps that: a
		// mis-shaped optional label is not grounds for throwing away a block
		// both YAML and a human read as front matter.
		var typeErr *yaml.TypeError
		if !errors.As(err, &typeErr) {
			return frontMatterFields{}, false
		}
	}
	return f.normalized(), true
}

// readLine returns the line at off without its terminator, and the offset of
// the next line (len(src) at the last line, so callers always advance).
func readLine(src []byte, off int) ([]byte, int) {
	i := bytes.IndexByte(src[off:], '\n')
	if i < 0 {
		return src[off:], len(src)
	}
	return src[off : off+i], off + i + 1
}

func isFrontMatterDelim(line []byte) bool {
	return string(bytes.TrimRight(line, " \t\r")) == frontMatterDelim
}
