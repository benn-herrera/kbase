package migrate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

const (
	version090 = "0.9.0"
	version100 = "1.0.0"

	entryPointPath = "kb-root/entry-point.md"
	indexDir       = "kb-root/.index"
	formatKey      = "kb-format"
)

// buildRecordStems are the build records beside kb-root/, each .json at 0.9.0
// and .yaml at 1.0.0.
var buildRecordStems = []string{"kb-build-node-pass", "kb-build-classification", "kb-build-unmarked"}

// The 0.9.0 comment-block grammar, as kb_tools reads it with Python's
// whitespace: the first block's body, a field per "key:" line, a value
// wrapped across lines as "[a,\n b]" or continued as "- item" bullets.
const pyWhitespace = `\t\n\v\f\r\x1c-\x1f \x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}`

// scoreNumber is a score as the 0.9.0 pair readers took it, widened to the
// exponent form Python's float repr writes below 1e-4.
const scoreNumber = `-?[0-9]+(?:\.[0-9]+)?(?:[eE][-+]?[0-9]+)?`

var (
	commentBlockRE = regexp.MustCompile(`(?s)<!--[` + pyWhitespace + `]*kb-frontmatter[` + pyWhitespace + `]*\n(.*?)\n[ \t]*-->`)
	blockBulletRE  = regexp.MustCompile(`^[` + pyWhitespace + `]*-[` + pyWhitespace + `]+(.*)$`)
	pairScoreRE    = regexp.MustCompile(`^` + scoreNumber + `$`)
	plainSafeRE    = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_./-]*$`)
	// frontmatterKeyLineRE is a line opening a 1.0.0 frontmatter key, every
	// one of which is kebab-case.
	frontmatterKeyLineRE = regexp.MustCompile(`^[a-z][a-z0-9-]*:`)
	// The 0.9.0 node scans' pair lines: a claim id and its score, the bullet
	// optional; a supports score may be the pending literal.
	strengthensPairRE = regexp.MustCompile(`^[` + pyWhitespace + `]*(?:-[` + pyWhitespace + `]*)?(clm-[a-z0-9]{6})[` + pyWhitespace + `]*:[` + pyWhitespace + `]*(` + scoreNumber + `)[` + pyWhitespace + `]*$`)
	supportsPairRE    = regexp.MustCompile(`^[` + pyWhitespace + `]*(?:-[` + pyWhitespace + `]*)?(clm-[a-z0-9]{6})[` + pyWhitespace + `]*:[` + pyWhitespace + `]*(` + scoreNumber + `|\*pending\*)[` + pyWhitespace + `]*$`)
)

const pendingLiteral = "*pending*"

// yaml11Words are the plain scalars a YAML 1.1 reader takes as a boolean or
// null, compared lowercased.
var yaml11Words = map[string]bool{"y": true, "n": true, "yes": true, "no": true, "on": true, "off": true, "true": true, "false": true, "null": true}

func convert090To100(in Files) (Files, []string, error) {
	out := make(Files, len(in))
	var obsolete []string
	for _, p := range slices.Sorted(maps.Keys(in)) {
		b := in[p]
		switch {
		case isBuildRecord090(p):
			y, err := recordYAML(b)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %w", p, err)
			}
			out[strings.TrimSuffix(p, ".json")+".yaml"] = y
			obsolete = append(obsolete, p)
		case path.Dir(p) == indexDir && path.Ext(p) == ".jsonl":
			y, err := indexStream(b)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %w", p, err)
			}
			out[strings.TrimSuffix(p, ".jsonl")+".yaml"] = y
			obsolete = append(obsolete, p)
		case path.Ext(p) == ".md":
			d, err := documentYAMLFrontmatter(b, p == entryPointPath)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %w", p, err)
			}
			out[p] = d
		default:
			// A converted file takes its new path over any file standing there.
			if _, converted := out[p]; !converted {
				out[p] = b
			}
		}
	}
	return out, obsolete, nil
}

// SupersededStamp is the kb-format an entry point in the 0.9.0 form declares
// in its comment block, ok false where it declares none.
func SupersededStamp(entryPoint []byte) (string, bool) {
	m := commentBlockRE.FindSubmatch(entryPoint)
	if m == nil {
		return "", false
	}
	for _, f := range genericFields(splitLines(string(m[1]))) {
		if f.key == formatKey && f.value.Kind == yaml.ScalarNode && f.value.Value != "" {
			return f.value.Value, true
		}
	}
	return "", false
}

func isBuildRecord090(p string) bool { return slices.Contains(SupersededRecordPaths(), p) }

// SupersededRecordPaths is every repository-relative path a build record
// stood at under a superseded format.
func SupersededRecordPaths() []string {
	out := make([]string, len(buildRecordStems))
	for i, stem := range buildRecordStems {
		out[i] = stem + ".json"
	}
	return out
}

// yamlFrontmatter locates a document that already opens with YAML
// frontmatter — a "---" line, then the next "---" line, either ending in \n
// or \r\n, the first non-blank line between them opening a kebab-case key —
// as the offsets of its body and of the closing fence; ok false where it does
// not, a fenced block opening otherwise being prose.
func yamlFrontmatter(text string) (bodyStart, closer int, ok bool) {
	bodyStart, closer, ok = fencedBlock(text)
	if !ok {
		return 0, 0, false
	}
	for _, line := range strings.Split(text[bodyStart:closer], "\n") {
		if strings.TrimSpace(line) != "" {
			return bodyStart, closer, frontmatterKeyLineRE.MatchString(line)
		}
	}
	return bodyStart, closer, true
}

// fencedBlock is the offsets of the body and the closing fence of the block
// a "---" line opening text fences, whatever lies between.
func fencedBlock(text string) (bodyStart, closer int, ok bool) {
	opener := strings.TrimPrefix(text, "---")
	switch {
	case len(opener) == len(text):
		return 0, 0, false
	case strings.HasPrefix(opener, "\n"):
		bodyStart = 4
	case strings.HasPrefix(opener, "\r\n"):
		bodyStart = 5
	default:
		return 0, 0, false
	}
	for i := bodyStart; i < len(text); {
		if rest := text[i:]; rest == "---" || strings.HasPrefix(rest, "---\n") || strings.HasPrefix(rest, "---\r\n") {
			return bodyStart, i, true
		}
		j := strings.IndexByte(text[i:], '\n')
		if j < 0 {
			break
		}
		i += j + 1
	}
	return 0, 0, false
}

// stampYAMLFrontmatter is a document already in YAML frontmatter with
// kb-format set as its last key.
func stampYAMLFrontmatter(text string, bodyStart, closer int) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(text[bodyStart:closer]), &doc); err != nil {
		return nil, fmt.Errorf("frontmatter: %w", err)
	}
	fields := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	if len(doc.Content) == 1 {
		switch root := doc.Content[0]; {
		case root.Kind == yaml.MappingNode:
			fields = root
		case root.Kind != yaml.ScalarNode || root.ShortTag() != "!!null":
			return nil, fmt.Errorf("frontmatter is not a mapping")
		}
	}
	setStamp(fields)
	var b bytes.Buffer
	b.WriteString("---\n")
	if err := encodeYAML(&b, fields); err != nil {
		return nil, err
	}
	b.WriteString(text[closer:])
	return b.Bytes(), nil
}

// setStamp makes kb-format the mapping's last key, at version100.
func setStamp(fields *yaml.Node) {
	for i := 0; i+1 < len(fields.Content); i += 2 {
		if fields.Content[i].Value == formatKey {
			fields.Content = slices.Delete(fields.Content, i, i+2)
			break
		}
	}
	fields.Content = append(fields.Content, stringNode(formatKey), stringNode(version100))
}

// documentYAMLFrontmatter is doc with its comment block's fields moved into
// YAML frontmatter at the top and the block, with its line break, removed;
// stamp adds kb-format as the last key. A document already opening with YAML
// frontmatter is in the next form: it is returned unchanged, or stamped. A
// document with no block and no stamp is returned unchanged.
func documentYAMLFrontmatter(doc []byte, stamp bool) ([]byte, error) {
	text := string(doc)
	if bodyStart, closer, ok := yamlFrontmatter(text); ok {
		if !stamp {
			return doc, nil
		}
		return stampYAMLFrontmatter(text, bodyStart, closer)
	}
	m := commentBlockRE.FindStringSubmatchIndex(text)
	if m == nil && !stamp {
		return doc, nil
	}
	fields := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	rest := text
	if m != nil {
		var err error
		if fields, err = blockFields(text[m[2]:m[3]]); err != nil {
			return nil, err
		}
		end := m[1]
		if strings.HasPrefix(text[end:], "\r\n") {
			end += 2
		} else if strings.HasPrefix(text[end:], "\n") {
			end++
		}
		rest = text[:m[0]] + text[end:]
	}
	if stamp {
		setStamp(fields)
	}
	var b bytes.Buffer
	b.WriteString("---\n")
	if err := encodeYAML(&b, fields); err != nil {
		return nil, err
	}
	b.WriteString("---\n")
	b.WriteString(rest)
	return b.Bytes(), nil
}

// placedField is one key and value of the converted block, at the body line
// it stands for.
type placedField struct {
	line  int
	key   string
	value *yaml.Node
}

// genericFields is every field of a block body as kb's 0.9.0 field reader
// took it, each at the line its key opens. A list written inline stays a
// flow list; a bullet list becomes a block list, a "key: value" bullet a
// one-key mapping whose numeric value stays a number.
func genericFields(lines []string) []placedField {
	var out []placedField
	for i := 0; i < len(lines); {
		line := rstrip(lines[i])
		if line == "" || !strings.Contains(line, ":") {
			i++
			continue
		}
		end := blockFieldEnd(lines, i)
		key, value, _ := strings.Cut(line, ":")
		out = append(out, placedField{line: i, key: strip(key), value: blockValue(strip(value), lines[i+1:end])})
		i = end
	}
	return out
}

// nodeScan is one kind of node declaration as kb's 0.9.0 node scan read it:
// an opener key starts a node, and the member key and the pair-list key —
// leading dashes trimmed — attach to the kind's latest node whatever stands
// between; a pair line, bullet optional, joins the pair list while the list
// is open, and any other key line closes it.
type nodeScan struct {
	opener, member, pairsKey, listKey string
	pairRE                            *regexp.Regexp
}

var nodeScans = []nodeScan{
	{opener: "exp-id", member: "status", pairsKey: "strengthens", listKey: "experiment-nodes", pairRE: strengthensPairRE},
	{opener: "sup-id", pairsKey: "supports", listKey: "support-nodes", pairRE: supportsPairRE},
}

// nodeDecl is one declared node's keys in the order each first appears.
type nodeDecl struct {
	keys   []string
	values map[string]*yaml.Node
}

func (d *nodeDecl) set(key string, v *yaml.Node) {
	if _, ok := d.values[key]; !ok {
		d.keys = append(d.keys, key)
	}
	d.values[key] = v
}

// mapping is the node as one mapping, an empty pair list left out.
func (d *nodeDecl) mapping() *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, k := range d.keys {
		if v := d.values[k]; v.Kind != yaml.SequenceNode || len(v.Content) > 0 {
			m.Content = append(m.Content, stringNode(k), v)
		}
	}
	return m
}

// scan is the kind's declarations over a block's lines: the list of them,
// the line the first opens on, and every line the scan took.
func (s nodeScan) scan(lines []string) (list *yaml.Node, first int, taken map[int]bool) {
	taken = map[int]bool{}
	var decls []*nodeDecl
	inPairs := false
	for i, line := range lines {
		stripped := strip(line)
		if stripped == "" {
			continue
		}
		if m := s.pairRE.FindStringSubmatch(line); inPairs && m != nil && len(decls) > 0 {
			pairs := decls[len(decls)-1].values[s.pairsKey]
			pairs.Content = append(pairs.Content, &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map",
				Content: []*yaml.Node{stringNode(m[1]), scoreNode(m[2])}})
			taken[i] = true
			continue
		}
		k, v, ok := strings.Cut(stripped, ":")
		if !ok {
			continue
		}
		key, value := strip(strings.TrimLeft(strip(k), "- ")), strip(v)
		if key == s.pairsKey {
			inPairs = true
			if len(decls) > 0 {
				d := decls[len(decls)-1]
				if _, open := d.values[s.pairsKey]; !open {
					d.set(s.pairsKey, &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"})
				}
				taken[i] = true
			}
			continue
		}
		inPairs = false
		switch {
		case key == s.opener:
			if len(decls) == 0 {
				first = i
			}
			d := &nodeDecl{values: map[string]*yaml.Node{}}
			d.set(key, stringNode(value))
			decls = append(decls, d)
			taken[i] = true
		case key == s.member && s.member != "" && len(decls) > 0:
			decls[len(decls)-1].set(key, stringNode(value))
			taken[i] = true
		}
	}
	if len(decls) == 0 {
		return nil, 0, taken
	}
	list = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, d := range decls {
		list.Content = append(list.Content, d.mapping())
	}
	return list, first, taken
}

// scoreNode is a pair's score: the pending literal a string, a number a
// number.
func scoreNode(text string) *yaml.Node {
	if text == pendingLiteral {
		return stringNode(text)
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: numberTag(text), Value: text}
}

// blockFields is a comment block's fields in block order: each kind's node
// declarations, read as the 0.9.0 node scan read them, as one list of
// mappings standing where the kind's first declaration stood, and every
// other field as the 0.9.0 field reader read it, less the lines a node scan
// took.
func blockFields(body string) (*yaml.Node, error) {
	lines := splitLines(body)
	var placed []placedField
	taken := map[int]bool{}
	for _, s := range nodeScans {
		list, first, t := s.scan(lines)
		maps.Copy(taken, t)
		if list != nil {
			placed = append(placed, placedField{line: first, key: s.listKey, value: list})
		}
	}
	for _, f := range genericFields(lines) {
		if !taken[f.line] {
			placed = append(placed, f)
		}
	}
	slices.SortStableFunc(placed, func(a, b placedField) int { return a.line - b.line })
	fields := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, f := range placed {
		if err := addField(fields, f.key, f.value); err != nil {
			return nil, err
		}
	}
	return fields, nil
}

func addField(m *yaml.Node, key string, v *yaml.Node) error {
	for i := 0; i < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return fmt.Errorf("frontmatter key %q repeats; a YAML mapping holds each key once", key)
		}
	}
	m.Content = append(m.Content, stringNode(key), v)
	return nil
}

// blockFieldEnd is one past the last line of the field opening at start.
func blockFieldEnd(lines []string, start int) int {
	_, value, _ := strings.Cut(rstrip(lines[start]), ":")
	value = strip(value)
	i := start + 1
	if strings.HasPrefix(value, "[") && !strings.HasSuffix(value, "]") {
		for i < len(lines) && !strings.HasSuffix(strip(lines[i]), "]") {
			i++
		}
		return min(i+1, len(lines))
	}
	if value == "" {
		for i < len(lines) && blockBulletRE.MatchString(lines[i]) {
			i++
		}
	}
	return i
}

func blockValue(value string, tail []string) *yaml.Node {
	if len(tail) == 0 {
		return blockScalar(value)
	}
	if strings.HasPrefix(value, "[") {
		parts := []string{value}
		for _, l := range tail {
			parts = append(parts, strip(l))
		}
		return blockScalar(strings.Join(parts, " "))
	}
	list := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, l := range tail {
		item := strip(blockBulletRE.FindStringSubmatch(l)[1])
		k, v, isPair := strings.Cut(item, ":")
		if !isPair {
			list.Content = append(list.Content, stringNode(item))
			continue
		}
		score := stringNode(strip(v))
		if pairScoreRE.MatchString(score.Value) {
			score = &yaml.Node{Kind: yaml.ScalarNode, Tag: numberTag(score.Value), Value: score.Value}
		}
		list.Content = append(list.Content, &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map",
			Content: []*yaml.Node{stringNode(strip(k)), score}})
	}
	return list
}

// blockScalar types a one-line value as kb_tools does: "[…]" a list, a
// double-quoted string with its quotes stripped once, true or false a
// boolean, anything else the string as written.
func blockScalar(value string) *yaml.Node {
	switch {
	case strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]"):
		list := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
		if len(value) >= 2 {
			for _, item := range strings.Split(value[1:len(value)-1], ",") {
				if item = strip(item); item != "" {
					list.Content = append(list.Content, stringNode(item))
				}
			}
		}
		return list
	case strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`):
		if len(value) < 2 {
			return stringNode("")
		}
		return stringNode(value[1 : len(value)-1])
	case value == "true" || value == "false":
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: value}
	}
	return stringNode(value)
}

// recordYAML is a JSON build record as a YAML document of the same structure,
// keys in their order.
func recordYAML(b []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	n, err := jsonNode(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("data after the record's JSON value")
	}
	var out bytes.Buffer
	if err := encodeYAML(&out, n); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// indexStream is a JSONL index file as a YAML stream: one document per
// record, "--- " and the record as a one-line JSON object, keys in their
// order — a YAML flow mapping a JSON reader takes by dropping the marker.
func indexStream(b []byte) ([]byte, error) {
	var out bytes.Buffer
	for i, line := range bytes.Split(b, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if line = bytes.TrimSpace(line); line[0] != '{' {
			return nil, fmt.Errorf("line %d: a record is a JSON object", i+1)
		}
		dec := json.NewDecoder(bytes.NewReader(line))
		dec.UseNumber()
		var rec bytes.Buffer
		if err := writeJSONValue(&rec, dec); err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		if _, err := dec.Token(); !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("line %d: data after the record's JSON value", i+1)
		}
		out.WriteString("--- ")
		out.WriteString(yamlUnsafeRunes.Replace(rec.String()))
		out.WriteByte('\n')
	}
	return out.Bytes(), nil
}

// yamlUnsafeRunes escapes what encoding/json writes raw but a YAML reader
// takes as a line break or refuses as unprintable: NEL and the other C1
// controls, DEL, the byte-order mark, and the noncharacters U+FFFE and
// U+FFFF. JSON writes no such character outside a string, so every
// replacement lands inside one.
var yamlUnsafeRunes = func() *strings.Replacer {
	var pairs []string
	for r := rune(0x7f); r <= 0x9f; r++ {
		pairs = append(pairs, string(r), fmt.Sprintf(`\u%04x`, r))
	}
	for _, r := range []rune{0xfeff, 0xfffe, 0xffff} {
		pairs = append(pairs, string(r), fmt.Sprintf(`\u%04x`, r))
	}
	return strings.NewReplacer(pairs...)
}()

// writeJSONValue writes the next JSON value dec holds to b on one line, object
// keys in their order, numbers as written, separators as Python's json.dumps
// writes them.
func writeJSONValue(b *bytes.Buffer, dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	switch t := tok.(type) {
	case json.Delim:
		closer := map[json.Delim]string{'{': "}", '[': "]"}[t]
		b.WriteString(t.String())
		for first := true; dec.More(); first = false {
			if !first {
				b.WriteString(", ")
			}
			if t == '{' {
				k, err := dec.Token()
				if err != nil {
					return err
				}
				if err := writeJSONString(b, k.(string)); err != nil {
					return err
				}
				b.WriteString(": ")
			}
			if err := writeJSONValue(b, dec); err != nil {
				return err
			}
		}
		if _, err := dec.Token(); err != nil {
			return err
		}
		b.WriteString(closer)
	case string:
		return writeJSONString(b, t)
	case json.Number:
		b.WriteString(t.String())
	case bool:
		fmt.Fprint(b, t)
	case nil:
		b.WriteString("null")
	}
	return nil
}

func writeJSONString(b *bytes.Buffer, s string) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return err
	}
	b.Write(bytes.TrimSuffix(buf.Bytes(), []byte("\n")))
	return nil
}

// jsonNode is the next JSON value dec holds as a YAML node, object keys in
// their order and numbers as written.
func jsonNode(dec *json.Decoder) (*yaml.Node, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		if t == '{' {
			n = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		}
		for dec.More() {
			if n.Kind == yaml.MappingNode {
				k, err := dec.Token()
				if err != nil {
					return nil, err
				}
				n.Content = append(n.Content, stringNode(k.(string)))
			}
			v, err := jsonNode(dec)
			if err != nil {
				return nil, err
			}
			n.Content = append(n.Content, v)
		}
		if _, err := dec.Token(); err != nil {
			return nil, err
		}
		return n, nil
	case string:
		return stringNode(t), nil
	case json.Number:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: numberTag(t.String()), Value: t.String()}, nil
	case bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: fmt.Sprint(t)}, nil
	case nil:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}, nil
	}
	return nil, fmt.Errorf("unexpected JSON token %v", tok)
}

func numberTag(text string) string {
	if strings.ContainsAny(text, ".eE") {
		return "!!float"
	}
	return "!!int"
}

// stringNode is s as a string scalar, written plain only where no YAML 1.1 or
// 1.2 reader could take it for anything else, double-quoted otherwise: the
// quoted form escapes every line break, so no string spans two lines.
func stringNode(s string) *yaml.Node {
	n := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}
	if !plainSafeRE.MatchString(s) || yaml11Words[strings.ToLower(s)] {
		n.Style = yaml.DoubleQuotedStyle
	}
	return n
}

func encodeYAML(w io.Writer, n *yaml.Node) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(n); err != nil {
		return err
	}
	return enc.Close()
}

// splitLines is Python's str.splitlines(): no trailing empty line, \r\n one
// break.
func splitLines(text string) []string {
	var out []string
	start := 0
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		next := i + size
		if isLineBreak(r) {
			out = append(out, text[start:i])
			if r == '\r' && next < len(text) && text[next] == '\n' {
				next++
			}
			start = next
		}
		i = next
	}
	if start < len(text) {
		out = append(out, text[start:])
	}
	return out
}

func isLineBreak(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// isSpace is Python's str.isspace() for one character.
func isSpace(r rune) bool {
	switch {
	case r >= '\t' && r <= '\r', r >= 0x1c && r <= ' ', r == 0x85, r == 0xa0, r == 0x1680,
		r >= 0x2000 && r <= 0x200a, r == 0x2028, r == 0x2029, r == 0x202f, r == 0x205f, r == 0x3000:
		return true
	}
	return false
}

func strip(s string) string  { return strings.TrimFunc(s, isSpace) }
func rstrip(s string) string { return strings.TrimRightFunc(s, isSpace) }
