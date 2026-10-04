// Package prepass holds the source scans that run before pandoc parses a
// volume root, in one registry. Each entry names the reader-level hole it
// fills and reports a census of what it found. Nothing else massages source.
package prepass

import (
	"regexp"
	"strings"
	"unicode"
)

// Outcome is the source pandoc reads and what the scans learned from it.
type Outcome struct {
	// Source is the volume root with every readable environment declaration
	// removed.
	Source string
	// TheoremNames maps a \newtheorem internal name to its display name.
	TheoremNames map[string]string
	// Censuses holds one entry per registry entry, in registry order.
	Censuses []Census
}

// Census is what one scan found.
type Census struct {
	Pass  string `json:"pass"`
	Hole  string `json:"hole"`
	Found int    `json:"found"`
	// Unread lists the declarations the scan could not read, each left where
	// the author wrote it.
	Unread []Unread `json:"unread,omitempty"`
}

// Unread is one declaration located the way a person looks it up.
type Unread struct {
	Line int    `json:"line"`
	Text string `json:"text"`
}

type entry struct {
	name  string
	hole  string
	apply func(*Outcome) Census
}

// registry is every pre-pass, in the order they run. Adding one is an
// ARCHITECTURE change.
var registry = []entry{{
	name: "theorem_display_names",
	hole: "pandoc consumes the preamble, so a \\newtheorem's display name reaches no AST node; " +
		"the Div carries only the internal name",
	apply: theoremDisplayNames,
}, {
	name: "strip_environment_declarations",
	hole: "pandoc expands a \\newenvironment as defined during parsing, so the author's " +
		"environment name never reaches a Div",
	apply: stripEnvironmentDeclarations,
}}

// Apply runs every registry entry over a volume root's text.
func Apply(source string) Outcome {
	out := Outcome{Source: source, TheoremNames: map[string]string{}}
	for _, e := range registry {
		c := e.apply(&out)
		c.Pass, c.Hole = e.name, e.hole
		out.Censuses = append(out.Censuses, c)
	}
	return out
}

// theoremDeclaration is \newtheorem in every form amsthm admits: starred, a
// shared counter in a leading optional argument, a subordinate counter in a
// trailing one (which the pattern stops before). Group 1 is the internal
// name, group 2 the display name.
var theoremDeclaration = regexp.MustCompile(`\\newtheorem\s*\*?\s*\{([^}]*)\}\s*(?:\[[^\]]*\])?\s*\{([^}]*)\}`)

// translationMacro is a babel translation macro standing in for the display
// word, \protect\theoremname; its stem is the word.
var translationMacro = regexp.MustCompile(`^\\(?:protect\s*\\)?([a-zA-Z]+)name$`)

func theoremDisplayNames(o *Outcome) Census {
	for _, m := range theoremDeclaration.FindAllStringSubmatch(o.Source, -1) {
		if display, ok := displayName(strings.TrimSpace(m[2])); ok {
			o.TheoremNames[strings.TrimSpace(m[1])] = display
		}
	}
	return Census{Found: len(o.TheoremNames)}
}

// displayName is the declared name as a word, or false where it is markup
// this cannot resolve or empty.
func displayName(declared string) (string, bool) {
	if m := translationMacro.FindStringSubmatch(declared); m != nil {
		return capitalize(m[1]), true
	}
	if declared == "" || strings.Contains(declared, `\`) {
		return "", false
	}
	return declared, true
}

func capitalize(s string) string {
	r := []rune(strings.ToLower(s))
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// declaration is the keyword of a declaration whose expansion would erase the
// author's environment name, up to the brace opening its name group.
var declaration = regexp.MustCompile(`\\(?:re)?newenvironment\s*\*?\s*\{`)

func stripEnvironmentDeclarations(o *Outcome) Census {
	source := o.Source
	var kept strings.Builder
	var census Census
	position := 0
	for {
		loc := declaration.FindStringIndex(source[position:])
		if loc == nil {
			break
		}
		start, nameGroup := position+loc[0], position+loc[1]-1
		end, ok := declarationEnd(source, nameGroup)
		if !ok {
			census.Unread = append(census.Unread, unread(source, start))
			kept.WriteString(source[position:nameGroup])
			position = nameGroup
			continue
		}
		census.Found++
		kept.WriteString(source[position:start])
		position = end
	}
	kept.WriteString(source[position:])
	o.Source = kept.String()
	return census
}

// declarationEnd is the offset just past the declaration whose name group
// begins at start: the name, up to two optional arguments, then the begin
// and end bodies.
func declarationEnd(source string, start int) (int, bool) {
	cursor, ok := skipGroup(source, start)
	if !ok {
		return 0, false
	}
	cursor = skipOptional(source, cursor)
	cursor = skipOptional(source, cursor)
	for range 2 {
		if cursor, ok = skipGroup(source, cursor); !ok {
			return 0, false
		}
	}
	return cursor, true
}

func unread(source string, start int) Unread {
	line := strings.Count(source[:start], "\n") + 1
	end := strings.IndexByte(source[start:], '\n')
	if end < 0 {
		end = len(source) - start
	}
	return Unread{Line: line, Text: strings.TrimRightFunc(source[start:start+end], unicode.IsSpace)}
}

// skipGroup is the offset just past the braced group at start. A % comments
// out the rest of its line, so a brace inside one does not count.
func skipGroup(source string, start int) (int, bool) {
	cursor := skipIgnorable(source, start)
	if cursor >= len(source) || source[cursor] != '{' {
		return 0, false
	}
	depth := 0
	for cursor < len(source) {
		switch source[cursor] {
		case '\\':
			cursor += 2
			continue
		case '%':
			cursor = endOfLine(source, cursor)
			continue
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return cursor + 1, true
			}
		}
		cursor++
	}
	return 0, false
}

func skipOptional(source string, start int) int {
	cursor := skipIgnorable(source, start)
	if cursor >= len(source) || source[cursor] != '[' {
		return start
	}
	closing := strings.IndexByte(source[cursor:], ']')
	if closing < 0 {
		return start
	}
	return cursor + closing + 1
}

// skipIgnorable passes whitespace and line comments; LyX separates a
// declaration's parts with a comment.
func skipIgnorable(source string, start int) int {
	cursor := start
	for cursor < len(source) {
		switch source[cursor] {
		case ' ', '\t', '\r', '\n':
			cursor++
		case '%':
			cursor = endOfLine(source, cursor)
		default:
			return cursor
		}
	}
	return cursor
}

func endOfLine(source string, start int) int {
	i := strings.IndexByte(source[start:], '\n')
	if i < 0 {
		return len(source)
	}
	return start + i + 1
}
