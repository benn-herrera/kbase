package kb

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	// FrontmatterRE is the kb-frontmatter block; group 1 is its body. The
	// closer may be indented.
	FrontmatterRE   = PyRE(`(?s)<!--\s*kb-frontmatter\s*\n(.*?)\n[ \t]*-->`)
	fmBulletRE      = PyRE(`^\s*-\s+(.*)$`)
	anyClaimOrExpRE = PyRE(IDBody("clm", "exp"))

	// leafDeclarationREs are the frontmatter keys that originate a node in
	// the container carrying them; each may repeat.
	leafDeclarationREs = []*regexp.Regexp{
		PyRE(`(?m)^\s*-?\s*exp-id:\s*(` + IDBody("exp") + `)\s*$`),
		PyRE(`(?m)^\s*-?\s*sup-id:\s*(` + IDBody("sup") + `)\s*$`),
	}
)

// DeclaredNodeIDs is every exp- and sup- id a frontmatter block body
// declares, experiments first, each kind in block order.
func DeclaredNodeIDs(body string) []string {
	var out []string
	for _, re := range leafDeclarationREs {
		for _, m := range re.FindAllStringSubmatch(body, -1) {
			out = append(out, m[1])
		}
	}
	return out
}

// Value is one typed frontmatter value: an id list, a string or a boolean.
type Value struct {
	List   []string
	Str    string
	Bool   bool
	IsList bool
	IsBool bool
}

// Truthy is the value's Python truthiness.
func (v Value) Truthy() bool {
	switch {
	case v.IsList:
		return len(v.List) > 0
	case v.IsBool:
		return v.Bool
	}
	return v.Str != ""
}

// Items is the value iterated: an id list's ids, a string's characters. A
// boolean is not iterable.
func (v Value) Items() ([]string, error) {
	switch {
	case v.IsList:
		return v.List, nil
	case v.IsBool:
		return nil, fmt.Errorf("frontmatter value %v is not a list", v.Bool)
	}
	out := make([]string, 0, utf8.RuneCountInString(v.Str))
	for _, r := range v.Str {
		out = append(out, string(r))
	}
	return out, nil
}

// Frontmatter is a parsed block: nil where a document has none, empty where
// its block declares no field.
type Frontmatter map[string]Value

// Get is the field's value, ok false where the block does not declare it.
func (f Frontmatter) Get(key string) (Value, bool) {
	v, ok := f[key]
	return v, ok
}

// Kind is the block's kind: field where it is a string, else "".
func (f Frontmatter) Kind() string {
	if v, ok := f["kind"]; ok && !v.IsList && !v.IsBool {
		return v.Str
	}
	return ""
}

// ListOrEmpty is `fm.get(key, []) or ()` iterated.
func (f Frontmatter) ListOrEmpty(key string) ([]string, error) {
	v, ok := f[key]
	if !ok || !v.Truthy() {
		return nil, nil
	}
	return v.Items()
}

// FindFrontmatter is the byte offsets of the first block at or after pos:
// the whole match, then its body; nil where there is none.
func FindFrontmatter(text string, pos int) []int {
	m := FrontmatterRE.FindStringSubmatchIndex(text[pos:])
	if m == nil {
		return nil
	}
	for i := range m {
		m[i] += pos
	}
	return m
}

// StripFrontmatter is text with every block removed.
func StripFrontmatter(text string) string {
	var b strings.Builder
	pos := 0
	for {
		m := FindFrontmatter(text, pos)
		if m == nil {
			break
		}
		b.WriteString(text[pos:m[0]])
		pos = m[1]
	}
	b.WriteString(text[pos:])
	return b.String()
}

// ParseFrontmatter is the first block's fields, or nil where there is none.
func ParseFrontmatter(text string) Frontmatter {
	m := FindFrontmatter(text, 0)
	if m == nil {
		return nil
	}
	return frontmatterFields(text[m[2]:m[3]])
}

// frontmatterFields types every field of a block body. A value may span
// several lines, wrapped ([a,\n b]) or as a YAML-block list; both join into
// one id list.
func frontmatterFields(body string) Frontmatter {
	fields := Frontmatter{}
	lines := SplitLines(body)
	for i := 0; i < len(lines); {
		line := RStrip(lines[i])
		if line == "" || !strings.Contains(line, ":") {
			i++
			continue
		}
		end := FrontmatterFieldEnd(lines, i)
		key, _, _ := strings.Cut(line, ":")
		fields[Strip(key)] = frontmatterValue(joinFrontmatterValue(lines, i, end))
		i = end
	}
	return fields
}

func frontmatterValue(value string) Value {
	switch {
	case strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]"):
		return Value{IsList: true, List: findAllBounded(anyClaimOrExpRE, value)}
	case strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`):
		if len(value) < 2 {
			return Value{}
		}
		return Value{Str: value[1 : len(value)-1]}
	case value == "true" || value == "false":
		return Value{IsBool: true, Bool: value == "true"}
	}
	return Value{Str: value}
}

func joinFrontmatterValue(lines []string, start, end int) string {
	_, value, _ := strings.Cut(RStrip(lines[start]), ":")
	value = Strip(value)
	tail := lines[start+1 : end]
	if len(tail) == 0 {
		return value
	}
	if strings.HasPrefix(value, "[") {
		parts := []string{value}
		for _, l := range tail {
			parts = append(parts, Strip(l))
		}
		return strings.Join(parts, " ")
	}
	items := make([]string, len(tail))
	for i, l := range tail {
		items[i] = Strip(fmBulletRE.FindStringSubmatch(l)[1])
	}
	return "[" + strings.Join(items, ", ") + "]"
}

// FrontmatterFieldEnd is one past the last line of the field opening at
// start: the reader joins exactly this span and the writer replaces it.
func FrontmatterFieldEnd(lines []string, start int) int {
	_, value, _ := strings.Cut(RStrip(lines[start]), ":")
	value = Strip(value)
	i := start + 1
	if strings.HasPrefix(value, "[") && !strings.HasSuffix(value, "]") {
		for i < len(lines) && !strings.HasSuffix(Strip(lines[i]), "]") {
			i++
		}
		return min(i+1, len(lines))
	}
	if value == "" {
		for i < len(lines) && fmBulletRE.MatchString(lines[i]) {
			i++
		}
	}
	return i
}
