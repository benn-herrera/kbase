package kb

import (
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// SupportsKey is a write op's values key, not a frontmatter key: the pair list
// of an experiment's supports edges, at insert and through add-pairs.
const SupportsKey = "supports"

const fence = "---"

// fenceAt is whether a line at offset i of text is exactly the frontmatter
// fence, and the offset just past its line break (len(text) where it ends the
// text without one).
func fenceAt(text string, i int) (next int, ok bool) {
	if !strings.HasPrefix(text[i:], fence) {
		return 0, false
	}
	switch rest := text[i+len(fence):]; {
	case rest == "":
		return len(text), true
	case strings.HasPrefix(rest, "\n"):
		return i + len(fence) + 1, true
	case strings.HasPrefix(rest, "\r\n"):
		return i + len(fence) + 2, true
	}
	return 0, false
}

// FindFrontmatter locates a document's YAML frontmatter, which opens the
// document with a "---" line and ends at the next "---" line, either fence
// line ending in \n or \r\n: text[m[0]:m[1]] is the block from the opening
// fence through the closing one, its line break excluded, and
// text[m[2]:m[3]] the YAML between them, the break before the closer
// excluded. It is nil where text does not open with a fence line, never
// closes it, or fences a block whose first non-blank line opens no
// kebab-case key: a Markdown thematic break opening prose.
func FindFrontmatter(text string) []int {
	m := fencedBlock(text)
	if m == nil || !opensWithKey(text[m[2]:m[3]]) {
		return nil
	}
	return m
}

// frontmatterKeyLineRE is a line opening a frontmatter key, every one of
// which is kebab-case.
var frontmatterKeyLineRE = regexp.MustCompile(`^[a-z][a-z0-9-]*:`)

// opensWithKey is whether body's first non-blank line opens a key; a body
// with none is an empty frontmatter.
func opensWithKey(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) != "" {
			return frontmatterKeyLineRE.MatchString(line)
		}
	}
	return true
}

// fencedBlock is the offsets of text's opening fence line through the next
// fence line, as FindFrontmatter gives them, whatever lies between.
func fencedBlock(text string) []int {
	bodyStart, ok := fenceAt(text, 0)
	if !ok || bodyStart == len(text) && !strings.HasSuffix(text, "\n") {
		return nil
	}
	for i := bodyStart; i < len(text); {
		if _, ok := fenceAt(text, i); ok {
			bodyEnd := bodyStart
			if i > bodyStart {
				bodyEnd = i - 1
				if bodyEnd > bodyStart && text[bodyEnd-1] == '\r' {
					bodyEnd--
				}
			}
			return []int{0, i + len(fence), bodyStart, bodyEnd}
		}
		j := strings.IndexByte(text[i:], '\n')
		if j < 0 {
			break
		}
		i += j + 1
	}
	return nil
}

// FrontmatterEnd is the offset of the first line after the frontmatter's
// closing fence, 0 where text opens with none: where the document's body,
// up-link line first, begins.
func FrontmatterEnd(text string) int {
	m := FindFrontmatter(text)
	if m == nil {
		return 0
	}
	if next, ok := fenceAt(text, m[1]-len(fence)); ok {
		return next
	}
	return m[1]
}

// StripFrontmatter is text with its frontmatter, and the closing fence's line
// break, removed.
func StripFrontmatter(text string) string { return text[FrontmatterEnd(text):] }

// Value is one typed frontmatter value: a string, a boolean, or a list.
type Value struct {
	Str    string
	Bool   bool
	IsList bool
	IsBool bool
}

// Frontmatter is a parsed block: nil where a document has none, empty where
// its block declares no field.
type Frontmatter map[string]Value

// Str is the field's value where it is a string, else "".
func (f Frontmatter) Str(key string) string {
	if v, ok := f[key]; ok && !v.IsList && !v.IsBool {
		return v.Str
	}
	return ""
}

// ParseFrontmatter is the document's frontmatter fields, or nil where it has
// none. YAML that does not read as a mapping is a MalformedError.
func ParseFrontmatter(text string) (Frontmatter, error) {
	m := FindFrontmatter(text)
	if m == nil {
		return nil, nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(text[m[2]:m[3]]), &doc); err != nil {
		return nil, malformed("frontmatter: %v", err)
	}
	if len(doc.Content) == 0 {
		return Frontmatter{}, nil
	}
	root := doc.Content[0]
	if root.Kind == yaml.ScalarNode && root.ShortTag() == "!!null" {
		return Frontmatter{}, nil
	}
	if root.Kind != yaml.MappingNode {
		return nil, malformed("frontmatter: line %d: not a mapping of keys to values", root.Line)
	}
	return mappingFields(root)
}

func mappingFields(n *yaml.Node) (Frontmatter, error) {
	out := Frontmatter{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, val := n.Content[i], n.Content[i+1]
		if key.Kind != yaml.ScalarNode {
			return nil, malformed("frontmatter: line %d: a key that is not a string", key.Line)
		}
		if _, repeated := out[key.Value]; repeated {
			return nil, malformed("frontmatter: line %d: key %q repeats", key.Line, key.Value)
		}
		v, err := fieldValue(val)
		if err != nil {
			return nil, err
		}
		out[key.Value] = v
	}
	return out, nil
}

func fieldValue(n *yaml.Node) (Value, error) {
	if n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	switch n.Kind {
	case yaml.ScalarNode:
		switch n.ShortTag() {
		case "!!null":
			return Value{}, nil
		case "!!bool":
			b, err := strconv.ParseBool(strings.ToLower(n.Value))
			if err != nil {
				return Value{}, malformed("frontmatter: line %d: %q is not a boolean", n.Line, n.Value)
			}
			return Value{IsBool: true, Bool: b}, nil
		}
		return Value{Str: n.Value}, nil
	case yaml.SequenceNode:
		v := Value{IsList: true}
		for _, item := range n.Content {
			if item.Kind == yaml.AliasNode {
				item = item.Alias
			}
			switch item.Kind {
			case yaml.ScalarNode:
			case yaml.MappingNode:
				if _, err := mappingFields(item); err != nil {
					return Value{}, err
				}
			default:
				return Value{}, malformed("frontmatter: line %d: a list item that is neither a value nor a mapping", item.Line)
			}
		}
		return v, nil
	}
	return Value{}, malformed("frontmatter: line %d: a value that is a mapping; only a list holds mappings", n.Line)
}
