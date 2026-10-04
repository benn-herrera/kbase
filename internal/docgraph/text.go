package docgraph

import (
	"html"
	"regexp"
	"strings"
	"unicode"

	"kbase/internal/kb"
)

// The rendering's own spellings, undone so a heading's words can be compared
// against the AST's and slugged into a path.
var (
	escapeRe      = regexp.MustCompile(`\\([^0-9A-Za-z\s])`)
	footnoteRefRe = regexp.MustCompile(`\[\^[^\]\s]*\]`)
	mathElementRe = regexp.MustCompile(`(?s)<math\b.*?</math\s*>`)
	mathSpanRe    = regexp.MustCompile("(?s)\\$`.*?`\\$")
	codeSpanRe    = regexp.MustCompile("`[^`\n]*`")
	autolinkRe    = regexp.MustCompile(`<((?:https?|ftp|mailto):[^<>\s]*)>`)
	linkTargetRe  = regexp.MustCompile(`\]\(\S*\)`)
	tagRe         = regexp.MustCompile(`(?s)<[^<>]*>`)
	entityRe      = regexp.MustCompile(`&(?:#[0-9]+|#[xX][0-9A-Fa-f]+|[A-Za-z][A-Za-z0-9]*);`)
)

// Placeholders for an escaped angle bracket, so the tag pass cannot read a
// literal < in the text as the start of one.
const escapedLT, escapedGT = "\x01", "\x02"

// deleted are emphasis and quotation marks, removed wherever they sit; edges
// are trimmed from a token's ends only.
const (
	deleted = "*_~“”‘’\"'"
	edges   = "`()[]{}<>.,;:!?|\\/#$&+=-–—…"
)

// plainTokens are the tokens of text that is already plain: an AST inline
// stream's own words.
func plainTokens(text string) []string { return split(text) }

// inlineMarkdownTokens are the tokens of one line of rendered Markdown with
// the writer's markup undone. It holds no fenced or indented code.
func inlineMarkdownTokens(text string) []string {
	text = escapeRe.ReplaceAllStringFunc(text, func(m string) string {
		switch c := m[1:]; c {
		case "<":
			return escapedLT
		case ">":
			return escapedGT
		default:
			return c
		}
	})
	text = footnoteRefRe.ReplaceAllString(text, " ")
	text = mathElementRe.ReplaceAllString(text, " ")
	text = mathSpanRe.ReplaceAllString(text, " ")
	text = codeSpanRe.ReplaceAllStringFunc(text, func(m string) string { return " " + m + " " })
	text = autolinkRe.ReplaceAllString(text, "$1")
	text = linkTargetRe.ReplaceAllString(text, "]")
	text = tagRe.ReplaceAllString(text, "")
	text = entityRe.ReplaceAllStringFunc(text, html.UnescapeString)
	text = strings.NewReplacer(escapedLT, "<", escapedGT, ">").Replace(text)
	return split(text)
}

func split(text string) []string {
	var out []string
	for _, chunk := range strings.Fields(text) {
		chunk = strings.Map(func(r rune) rune {
			if strings.ContainsRune(deleted, r) {
				return -1
			}
			return r
		}, chunk)
		if chunk = strings.Trim(chunk, edges); chunk != "" {
			out = append(out, chunk)
		}
	}
	return out
}

// plain is a rendered title as its words, for slugging.
func plain(title string) string { return strings.Join(inlineMarkdownTokens(title), " ") }

// githubAnchor is the fragment GitHub derives from a heading's text; the gfm
// writer discards a header's identifier, so a link into a heading spells this.
func githubAnchor(heading string) string {
	text := tagRe.ReplaceAllString(mathSpanRe.ReplaceAllString(heading, ""), "")
	text = escapeRe.ReplaceAllString(linkTargetRe.ReplaceAllString(text, "]"), "$1")
	var kept strings.Builder
	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || strings.ContainsRune(" -_", r) {
			kept.WriteRune(r)
		}
	}
	return strings.ReplaceAll(strings.TrimSpace(kept.String()), " ", "-")
}

const (
	slugMaxChars = 48
	untitledSlug = "untitled"
)

var nonSlugRe = regexp.MustCompile(`[^a-z0-9]+`)

// slug is one path segment from one title, never empty. Truncation trims back
// to the last hyphen so a segment never ends mid-word.
func slug(title string) string {
	flat := strings.Trim(nonSlugRe.ReplaceAllString(strings.ToLower(strings.TrimSpace(title)), "-"), "-")
	if len(flat) <= slugMaxChars {
		if flat == "" {
			return untitledSlug
		}
		return flat
	}
	cut := flat[:slugMaxChars]
	if i := strings.LastIndexByte(cut, '-'); i >= 0 {
		cut = cut[:i]
	}
	if cut = strings.Trim(cut, "-"); cut == "" {
		return untitledSlug
	}
	return cut
}

var (
	fenceRe = regexp.MustCompile("^( *)(`{3,}|~{3,})")
	quoteRe = regexp.MustCompile(`^ {0,3}(?:> ?)+`)
	atxRe   = regexp.MustCompile(`^ {0,3}(#{1,6})\s+(.*?)\s*#*\s*$`)
)

// delimiter is the fence state after line, and the submatch indices of the
// delimiter line is, nil where it is none. A fence closes on its own marker and
// nothing else.
func delimiter(line, fence string) (string, []int) {
	m := fenceRe.FindStringSubmatchIndex(line)
	if m == nil {
		return fence, nil
	}
	if fence == "" {
		run := line[m[4]:m[5]]
		return strings.Repeat(run[:1], len(run)), m
	}
	if strings.HasPrefix(strings.TrimLeftFunc(line, unicode.IsSpace), fence) {
		return "", m
	}
	return fence, nil
}

// codeIndent is how far past its container's content column a line sits to
// be an indented code block.
const codeIndent = 4

// listMarkerRe is a list item's marker and the gap after it, whose end is the
// content column the item opens.
var listMarkerRe = regexp.MustCompile(`^ *(?:[-*+]|\d+[.)]) +`)

// markdownTokens are the tokens of a rendering, verbatim content taken
// verbatim: the writer's two spellings of it, a fence and an indented block,
// are split off first and each side tokenized by its own rule.
func markdownTokens(text string) []string {
	var tokens []string
	for _, c := range verbatimSplit(text) {
		if c.verbatim {
			tokens = append(tokens, strings.Fields(c.text)...)
		} else {
			tokens = append(tokens, inlineMarkdownTokens(c.text)...)
		}
	}
	return tokens
}

type chunk struct {
	verbatim bool
	text     string
}

// verbatimSplit cuts text, blockquote markers off, into verbatim and other
// chunks; a fence's own marker lines carry nothing, and an open fence outranks
// an indented block.
func verbatimSplit(text string) []chunk {
	lines := kb.Lines(text)
	for i, l := range lines {
		lines[i] = quoteRe.ReplaceAllString(l, "")
	}
	var out []chunk
	var buffered []string
	fence := ""
	column := -1
	for n, line := range lines {
		if column >= 0 {
			if strings.TrimSpace(line) == "" || indentOf(line) >= column {
				buffered = append(buffered, line)
				continue
			}
			out = append(out, chunk{true, strings.Join(buffered, "\n")})
			buffered, column = nil, -1
		}
		if fence == "" {
			if c := codeColumn(lines, n); c >= 0 {
				if len(buffered) > 0 {
					out = append(out, chunk{false, strings.Join(buffered, "\n")})
				}
				buffered, column = []string{line}, c
				continue
			}
		}
		wasOpen := fence != ""
		var marker []int
		fence, marker = delimiter(line, fence)
		if marker != nil {
			if len(buffered) > 0 {
				out = append(out, chunk{wasOpen, strings.Join(buffered, "\n")})
				buffered = nil
			}
			continue
		}
		buffered = append(buffered, line)
	}
	if len(buffered) > 0 {
		out = append(out, chunk{fence != "" || column >= 0, strings.Join(buffered, "\n")})
	}
	return out
}

func indentOf(line string) int { return len(line) - len(strings.TrimLeft(line, " ")) }

// codeColumn is the column an indented code block opening at lines[n] holds
// to, or -1. Indented code cannot interrupt a paragraph, and it is indented
// from the content column of whatever holds it.
func codeColumn(lines []string, n int) int {
	line := lines[n]
	if strings.TrimSpace(line) == "" || indentOf(line) < codeIndent || (n > 0 && strings.TrimSpace(lines[n-1]) != "") {
		return -1
	}
	column := containerColumn(lines[:n], indentOf(line)) + codeIndent
	if indentOf(line) >= column {
		return column
	}
	return -1
}

func containerColumn(preceding []string, indent int) int {
	for i := len(preceding) - 1; i >= 0; i-- {
		line := preceding[i]
		if strings.TrimSpace(line) == "" || indentOf(line) >= indent {
			continue
		}
		if m := listMarkerRe.FindStringIndex(line); m != nil {
			return m[1]
		}
		return indentOf(line)
	}
	return 0
}

// unquoted is text with every blockquote marker removed, line by line.
func unquoted(text string) string {
	lines := kb.Lines(text)
	for i, l := range lines {
		lines[i] = quoteRe.ReplaceAllString(l, "")
	}
	return strings.Join(lines, "\n")
}

type fenced struct{ info, content string }

// fencedBlocks are every fenced block's info string and content, blockquote
// markers off and the content dedented by the opening fence's own
// indentation, which is a list item's and not the source's.
func fencedBlocks(text string) []fenced {
	var blocks []fenced
	fence, info := "", ""
	indent := 0
	var buffered []string
	for _, quoted := range kb.Lines(text) {
		line := quoteRe.ReplaceAllString(quoted, "")
		wasOpen := fence != ""
		var marker []int
		fence, marker = delimiter(line, fence)
		switch {
		case marker != nil && !wasOpen:
			indent = marker[3] - marker[2]
			info, buffered = strings.TrimSpace(line[marker[1]:]), nil
		case marker != nil:
			blocks = append(blocks, fenced{info, strings.Join(buffered, "\n")})
		case wasOpen:
			buffered = append(buffered, line[min(indent, indentOf(line)):])
		}
	}
	return blocks
}

type heading struct {
	level      int
	text       string
	start, end int
}

// headings are every ATX heading of the rendering outside a fence, with any
// continuation lines folded into the text.
func headings(lines []string) []heading {
	var found []heading
	fence := ""
	for n, line := range lines {
		var marker []int
		fence, marker = delimiter(quoteRe.ReplaceAllString(line, ""), fence)
		if marker != nil || fence != "" {
			continue
		}
		m := atxRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		end := n + 1
		for end < len(lines) && strings.TrimSpace(lines[end]) != "" && !atxRe.MatchString(lines[end]) && !fenceRe.MatchString(lines[end]) {
			end++
		}
		parts := []string{m[2]}
		for _, l := range lines[n+1 : end] {
			parts = append(parts, strings.TrimSpace(l))
		}
		found = append(found, heading{level: len(m[1]), text: strings.TrimSpace(strings.Join(parts, " ")), start: n, end: end})
	}
	return found
}
