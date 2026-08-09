package prompt

import (
	"sort"
	"strings"
)

// Placeholder syntax. DOUBLE braces are the reserved form; single braces pass
// through untouched, because the material being interpolated is markdown and
// code, where lone braces are ordinary content. A name is a bare identifier —
// letters, digits, underscore, not starting with a digit — and anything else
// between `{{` and `}}` is a collision, reported rather than guessed at.
const (
	placeholderOpen  = "{{"
	placeholderClose = "}}"

	// maxPlaceholderName bounds the scan for a closing `}}` so a stray `{{`
	// in a long document reports a short snippet instead of swallowing the
	// rest of the file into the search.
	maxPlaceholderName = 64

	// snippetLen bounds the excerpt in ErrBadPlaceholder — enough to locate
	// the offender in the source, never enough to dump content.
	snippetLen = 24
)

// placeholder is one `{{name}}` occurrence: its byte span and its name.
type placeholder struct {
	start int // index of the opening brace
	end   int // index just past the closing brace
	name  string
}

// scanPlaceholders finds every placeholder in s, in order, and reports the
// first malformed `{{` sequence. It is the single grammar: Interpolate fills
// what it finds and ValidateDefinition rejects what it refuses, so a
// definition that validates is a definition that interpolates.
func scanPlaceholders(s string) ([]placeholder, error) {
	var found []placeholder
	for i := 0; i < len(s); {
		rel := strings.Index(s[i:], placeholderOpen)
		if rel < 0 {
			return found, nil
		}
		open := i + rel
		nameStart := open + len(placeholderOpen)

		limit := min(nameStart+maxPlaceholderName, len(s))
		closeRel := strings.Index(s[nameStart:limit], placeholderClose)
		if closeRel < 0 {
			return nil, ErrBadPlaceholder{Snippet: snippet(s[open:])}
		}
		name := s[nameStart : nameStart+closeRel]
		if !validPlaceholderName(name) {
			return nil, ErrBadPlaceholder{Snippet: snippet(s[open:])}
		}
		end := nameStart + closeRel + len(placeholderClose)
		found = append(found, placeholder{start: open, end: end, name: name})
		i = end
	}
	return found, nil
}

// Interpolate fills `{{name}}` placeholders in template from vars.
//
// It is loud in both directions: a placeholder with no supplied value and a
// supplied value no placeholder consumed are both errors, and both are
// reported together. A partially filled prompt is a prompt whose author and
// whose reader disagree about what it says, and that disagreement is silent
// until an output stage goes wrong.
//
// Substitution is single-pass: braces inside a substituted value are content,
// never a second round of interpolation.
func Interpolate(template string, vars map[string]string) (string, error) {
	found, err := scanPlaceholders(template)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.Grow(len(template))
	unknown := make(map[string]bool)
	used := make(map[string]bool, len(vars))
	prev := 0
	for _, p := range found {
		b.WriteString(template[prev:p.start])
		v, ok := vars[p.name]
		if !ok {
			unknown[p.name] = true
		} else {
			b.WriteString(v)
			used[p.name] = true
		}
		prev = p.end
	}
	b.WriteString(template[prev:])

	var unused []string
	for name := range vars {
		if !used[name] {
			unused = append(unused, name)
		}
	}
	if len(unknown) > 0 || len(unused) > 0 {
		return "", ErrInterpolate{Unknown: sortedKeys(unknown), Unused: sorted(unused)}
	}
	return b.String(), nil
}

func validPlaceholderName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '_':
		case c >= '0' && c <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}

// snippet returns a short single-line excerpt for an error message.
func snippet(s string) string {
	if len(s) > snippetLen {
		s = s[:snippetLen]
	}
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	return s
}

func sortedKeys(set map[string]bool) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return sorted(out)
}

func sorted(s []string) []string {
	sort.Strings(s)
	return s
}
