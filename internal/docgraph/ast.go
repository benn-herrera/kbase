package docgraph

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"kbase/internal/kb"
)

// The AST is pandoc's JSON held generically: every node is an object with a
// "t" and, usually, a "c". Numbers stay json.Number so the document reaches
// the writer exactly as the reader produced it.
type node = map[string]any

// The node types this package branches on or builds. Every such site names
// its type through one of these, so handledNodeTypes is exactly the set the
// code handles; any other type reaches the writer untouched.
const (
	typeStr        = "Str"
	typeSpace      = "Space"
	typeSoftBreak  = "SoftBreak"
	typeLineBreak  = "LineBreak"
	typeEmph       = "Emph"
	typeStrong     = "Strong"
	typeSpan       = "Span"
	typeLink       = "Link"
	typeCite       = "Cite"
	typeMath       = "Math"
	typeNote       = "Note"
	typeRawInline  = "RawInline"
	typeImage      = "Image"
	typeCode       = "Code"
	typePara       = "Para"
	typePlain      = "Plain"
	typeHeader     = "Header"
	typeBlockQuote = "BlockQuote"
	typeDiv        = "Div"
	typeFigure     = "Figure"
	typeTable      = "Table"
	typeRawBlock   = "RawBlock"
	typeCodeBlock  = "CodeBlock"
	typeLineBlock  = "LineBlock"
)

var handledNodeTypes = map[string]bool{
	typeStr: true, typeSpace: true, typeSoftBreak: true, typeLineBreak: true, typeEmph: true,
	typeStrong: true, typeSpan: true, typeLink: true, typeCite: true, typeMath: true, typeNote: true,
	typeRawInline: true, typeImage: true, typeCode: true, typePara: true, typePlain: true,
	typeHeader: true, typeBlockQuote: true, typeDiv: true, typeFigure: true, typeTable: true,
	typeRawBlock: true, typeCodeBlock: true, typeLineBlock: true,
}

// displayMath is the MathType constructor of a display equation.
const displayMath = "DisplayMath"

// The metadata value constructors.
const (
	metaBlocks  = "MetaBlocks"
	metaInlines = "MetaInlines"
	metaString  = "MetaString"
	metaList    = "MetaList"
	metaMap     = "MetaMap"
)

func decodeAST(data []byte) (node, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var doc node
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("decoding pandoc's JSON: %w", err)
	}
	return doc, nil
}

func encodeAST(doc node) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("encoding the transformed AST: %w", err)
	}
	return buf.Bytes(), nil
}

func asNode(v any) (node, string) {
	n, ok := v.(node)
	if !ok {
		return nil, ""
	}
	t, _ := n["t"].(string)
	return n, t
}

func list(v any) []any {
	l, _ := v.([]any)
	return l
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

// attr is an Attr's identifier, classes and key-value pairs.
func attr(v any) (string, []string, map[string]string) {
	a := list(v)
	if len(a) < 3 {
		return "", nil, nil
	}
	var classes []string
	for _, c := range list(a[1]) {
		classes = append(classes, str(c))
	}
	kv := map[string]string{}
	for _, pair := range list(a[2]) {
		if p := list(pair); len(p) == 2 {
			kv[str(p[0])] = str(p[1])
		}
	}
	return str(a[0]), classes, kv
}

func newAttr(id string, classes []string, kv [][2]string) []any {
	cs := make([]any, len(classes))
	for i, c := range classes {
		cs[i] = c
	}
	pairs := make([]any, len(kv))
	for i, p := range kv {
		pairs[i] = []any{p[0], p[1]}
	}
	return []any{id, cs, pairs}
}

// structural are the Div classes that are pandoc's or citeproc's own; any
// other class is the name of an environment the author wrote.
var structural = map[string]bool{
	"center": true, "tabular": true, "titlepage": true, "thebibliography": true,
	"references": true, "csl-bib-body": true, "hanging-indent": true, "csl-entry": true,
}

const titlepageClass = "titlepage"

// authored is the first class of a Div that is none of pandoc's own, and
// whether the Div is the title page.
func authored(classes []string) (string, bool) {
	name := ""
	for _, c := range classes {
		if c == titlepageClass {
			return "", true
		}
		if name == "" && !structural[c] {
			name = c
		}
	}
	return name, false
}

// rewrite walks v bottom-up and splices fn's replacement for each node in a
// list; fn returns false to keep a node.
func rewrite(v any, fn func(node, string) ([]any, bool)) any {
	switch x := v.(type) {
	case []any:
		out := make([]any, 0, len(x))
		for _, e := range x {
			e = rewrite(e, fn)
			if n, t := asNode(e); t != "" {
				if repl, ok := fn(n, t); ok {
					out = append(out, repl...)
					continue
				}
			}
			out = append(out, e)
		}
		return out
	case node:
		for k, val := range x {
			x[k] = rewrite(val, fn)
		}
		return x
	}
	return v
}

// transform reshapes the reader's AST into what the writer renders: every
// citation wrapped in pandoc's own citation markup carrying its keys, each
// author-declared Div a labelled blockquote, the title page dropped, and
// display maths lifted out of emphasis. Metadata is walked with the body, so
// an abstract's citations are wrapped too.
func transform(doc node, names map[string]string, keysOnly bool) {
	fn := func(n node, t string) ([]any, bool) {
		switch t {
		case typeCite:
			return []any{citationSpan(n, keysOnly)}, true
		case typeDiv:
			return labelledBlock(n, names)
		case typeEmph:
			return liftDisplayMath(n)
		}
		return nil, false
	}
	doc["blocks"] = rewrite(doc["blocks"], fn)
	doc["meta"] = rewrite(doc["meta"], fn)
}

func citeKeys(n node) []string {
	var keys []string
	for _, c := range list(list(n["c"])[0]) {
		cit, _ := c.(node)
		keys = append(keys, str(cit["citationId"]))
	}
	return keys
}

// citationSpan wraps a Cite in <span class="citation" data-cites="...">. With
// no bibliography no citeproc ran and a Cite renders as nothing, so the span
// carries the keys themselves, parenthesised as citeproc parenthesises.
func citationSpan(n node, keysOnly bool) node {
	keys := citeKeys(n)
	a := newAttr("", []string{"citation"}, [][2]string{{"data-cites", strings.Join(keys, " ")}})
	content := []any{n}
	if keysOnly {
		content = []any{node{"t": typeStr, "c": "(" + strings.Join(keys, "; ") + ")"}}
	}
	return node{"t": typeSpan, "c": []any{a, content}}
}

// blockLabel is the name an author's Div's label line carries — the
// environment's declared display name, else its class — "" for a Div that is
// not the author's, and whether the Div is the title page.
func blockLabel(classes []string, names map[string]string) (string, bool) {
	name, titlepage := authored(classes)
	if display, ok := names[name]; ok && name != "" {
		name = display
	}
	return name, titlepage
}

// labelNameRe is the name a label line must carry for kb_tools' reader to
// read it: inventory.LABEL_LINE_RE's environment group.
var labelNameRe = regexp.MustCompile(`^` + kb.LabelName + `$`)

// labelLineRead is whether kb_tools' reader reads the label line
// labelledBlock writes for a block named name. It reads one only at the
// margin, which the writer keeps exactly where every container above the
// block is a Div that is not the author's: any other container indents its
// content, quotes it, or renders it as HTML.
func labelLineRead(name string, atMargin bool) bool {
	return atMargin && labelNameRe.MatchString(name)
}

// labelledBlock reshapes an author's Div into a blockquote whose first line
// is the block's name, carrying the Div's identifier on a Span around it.
func labelledBlock(n node, names map[string]string) ([]any, bool) {
	c := list(n["c"])
	id, classes, _ := attr(c[0])
	name, titlepage := blockLabel(classes, names)
	if titlepage {
		return []any{}, true
	}
	if name == "" {
		return nil, false
	}
	var label any = node{"t": typeStrong, "c": []any{node{"t": typeStr, "c": name}}}
	if id != "" {
		label = node{"t": typeSpan, "c": []any{newAttr(id, nil, nil), []any{label}}}
	}
	blocks := append([]any{node{"t": typePlain, "c": []any{label}}}, list(c[1])...)
	return []any{node{"t": typeBlockQuote, "c": blocks}}, true
}

func isDisplayMath(v any) bool {
	n, t := asNode(v)
	if t != typeMath {
		return false
	}
	_, mt := asNode(list(n["c"])[0])
	return mt == displayMath
}

var whitespace = map[string]bool{typeSpace: true, typeSoftBreak: true, typeLineBreak: true}

// liftDisplayMath takes display equations out of an Emph, keeping the words
// either side emphasised: the gfm writer otherwise glues the emphasis
// delimiter to the fence, and a fence that does not open is no fence.
func liftDisplayMath(n node) ([]any, bool) {
	content := list(n["c"])
	if !slices.ContainsFunc(content, isDisplayMath) {
		return nil, false
	}
	var lifted, group []any
	worded := false
	flush := func() {
		if worded {
			lifted = append(lifted, node{"t": typeEmph, "c": group})
		} else {
			lifted = append(lifted, group...)
		}
		group, worded = nil, false
	}
	for _, inline := range content {
		if isDisplayMath(inline) {
			flush()
			lifted = append(lifted, inline)
			continue
		}
		group = append(group, inline)
		_, t := asNode(inline)
		worded = worded || !whitespace[t]
	}
	flush()
	return lifted, true
}

// Metadata keys split by kb_tools' closed classification: content reaches the
// tree, apparatus is elided. A key the rendering's metadata block carries in
// neither stops the build.
var (
	contentKeys   = map[string]bool{"abstract": true, "title": true}
	apparatusKeys = map[string]bool{"address": true, "author": true, "date": true, "bibliography": true}
)

// MetadataKeyError is metadata the classification does not cover.
type MetadataKeyError struct{ Keys []string }

func (e MetadataKeyError) Error() string {
	return fmt.Sprintf("metadata key(s) %s are neither content nor apparatus", strings.Join(e.Keys, ", "))
}

// RawBlockError is a raw block the reader handed back, whose treatment
// nothing has measured.
type RawBlockError struct{ Format, Text string }

func (e RawBlockError) Error() string {
	text := e.Text
	if len(text) > 200 {
		text = text[:200]
	}
	return fmt.Sprintf("raw %q block, no measured treatment: %q", e.Format, text)
}

// header is one document heading: its level and identifier, and the Header
// node itself, whose words are read once the transform has reshaped them into
// what the writer renders.
type header struct {
	level      int
	identifier string
	node       node
}

func (h header) tokens() []string { return inlineTokens(list(h.node["c"])[2]) }

// outline is what the walk of the blocks yields beside the records: the
// header sequence the rendering is cut against, the section each label is
// declared in (-1 before the first header), and which labels a link may spell
// as a fragment.
type outline struct {
	headers        []header
	labelSection   map[string]int
	headerLabels   map[string]bool
	anchoredLabels map[string]bool
}

// mathLabel is \label{...} as it survives verbatim inside maths source.
var mathLabel = regexp.MustCompile(`\\label\s*\{([^}]*)\}`)

// identified are the blocks whose identifier a \ref can name. A Table's id
// does not survive the gfm writer, so it is never a fragment.
var identified = map[string]bool{typeDiv: true, typeFigure: true, typeSpan: true, typeTable: true}

func sortedKeys(n node) []string {
	keys := make([]string, 0, len(n))
	for k := range n {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func atoi(v any) int {
	if n, ok := v.(json.Number); ok {
		i, err := n.Int64()
		if err == nil {
			return int(i)
		}
	}
	return 0
}

// runBreakers end a text run: each renders as something other than its own
// words, so the words either side are not contiguous in the output.
var runBreakers = map[string]bool{typeMath: true, typeRawInline: true, typeImage: true, typeCode: true, typeNote: true}

// inlineRuns are the maximal contiguous word runs of an inline stream.
func inlineRuns(v any) [][]string {
	var runs [][]string
	var pending strings.Builder
	flush := func() {
		if tokens := plainTokens(pending.String()); len(tokens) > 0 {
			runs = append(runs, tokens)
		}
		pending.Reset()
	}
	var descend func(any)
	descend = func(v any) {
		switch x := v.(type) {
		case []any:
			for _, e := range x {
				descend(e)
			}
		case node:
			t, _ := x["t"].(string)
			switch {
			case t == typeStr:
				pending.WriteString(str(x["c"]))
			case whitespace[t]:
				pending.WriteString(" ")
			case runBreakers[t]:
				flush()
			default:
				if c, ok := x["c"]; ok {
					descend(c)
				}
			}
		}
	}
	descend(v)
	flush()
	return runs
}

func inlineTokens(v any) []string {
	var out []string
	for _, run := range inlineRuns(v) {
		out = append(out, run...)
	}
	return out
}

// leafBlockTypes are the blocks that hold inlines directly; every other block
// is a container, so a run never spans a list item or a table cell.
var leafBlockTypes = map[string]bool{
	typePara: true, typePlain: true, typeHeader: true, typeLineBlock: true, typeCodeBlock: true, typeRawBlock: true,
}

// leafBlocks are the leaf blocks under v in document order, each followed by
// the blocks of the notes it carries: the writer sets a note's text at the
// foot, so it reaches the rendering too.
func leafBlocks(v any) []node {
	var out []node
	switch x := v.(type) {
	case []any:
		for _, e := range x {
			out = append(out, leafBlocks(e)...)
		}
	case node:
		if t, _ := x["t"].(string); leafBlockTypes[t] {
			return append(append(out, x), leafBlocks(notesWithin(x))...)
		}
		for _, k := range sortedKeys(x) {
			out = append(out, leafBlocks(x[k])...)
		}
	}
	return out
}

func notesWithin(v any) []any {
	var found []any
	var descend func(any)
	descend = func(v any) {
		switch x := v.(type) {
		case []any:
			for _, e := range x {
				descend(e)
			}
		case node:
			if t, _ := x["t"].(string); t == typeNote {
				found = append(found, list(x["c"])...)
				return
			}
			for _, k := range sortedKeys(x) {
				descend(x[k])
			}
		}
	}
	descend(v)
	return found
}

// textRuns are a leaf block's maximal contiguous word runs; a code block's
// text is one run, split on whitespace.
func textRuns(block node) [][]string {
	if t, _ := block["t"].(string); t == typeCodeBlock {
		return [][]string{strings.Fields(str(list(block["c"])[1]))}
	}
	return inlineRuns(block["c"])
}

// metaEntry is one content-bearing value under meta: where it sits, the
// top-level key it sits under, and its word runs.
type metaEntry struct {
	path, key string
	runs      [][]string
}

// metaEntries are every content-bearing value of meta, found by constructor
// at any depth, so a list of inlines is reached the way a block value is.
func metaEntries(meta node) []metaEntry {
	var entries []metaEntry
	record := func(path, key string, runs [][]string) {
		var kept [][]string
		for _, r := range runs {
			if len(r) > 0 {
				kept = append(kept, r)
			}
		}
		if len(kept) > 0 {
			entries = append(entries, metaEntry{path: path, key: key, runs: kept})
		}
	}
	var descend func(v any, path, key string)
	descend = func(v any, path, key string) {
		n, t := asNode(v)
		switch t {
		case metaBlocks:
			var runs [][]string
			for _, b := range leafBlocks(n["c"]) {
				runs = append(runs, textRuns(b)...)
			}
			record(path, key, runs)
		case metaInlines:
			record(path, key, inlineRuns(n["c"]))
		case metaString:
			record(path, key, [][]string{plainTokens(str(n["c"]))})
		case metaList:
			for i, m := range list(n["c"]) {
				descend(m, fmt.Sprintf("%s[%d]", path, i), key)
			}
		case metaMap:
			if m, ok := n["c"].(node); ok {
				for _, name := range sortedKeys(m) {
					descend(m[name], path+"."+name, key)
				}
			}
		}
	}
	for _, name := range sortedKeys(meta) {
		descend(meta[name], name, name)
	}
	return entries
}

// mathTexts are every Math element's LaTeX in blocks and in the content
// metadata keys, in document order; apparatus reaches no document, so its
// maths has none to reach.
func mathTexts(doc node) []string {
	var found []string
	var descend func(any)
	descend = func(v any) {
		switch x := v.(type) {
		case []any:
			for _, e := range x {
				descend(e)
			}
		case node:
			if t, _ := x["t"].(string); t == typeMath {
				found = append(found, str(list(x["c"])[1]))
				return
			}
			for _, k := range sortedKeys(x) {
				descend(x[k])
			}
		}
	}
	descend(doc["blocks"])
	meta, _ := doc["meta"].(node)
	for _, k := range sortedKeys(meta) {
		if contentKeys[k] {
			descend(meta[k])
		}
	}
	return found
}
