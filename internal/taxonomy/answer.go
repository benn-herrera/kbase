package taxonomy

import (
	"fmt"
	"strconv"
	"strings"

	"kbase/internal/text"
	"kbase/internal/treeplan"
)

// The answer transport: the model supplies VALUES on lines whose punctuation
// the machine owns, and this turns them into the typed answer the verifier
// checks (treeplan.GroupingAnswer).
//
// # Why this is a line grammar and not a document
//
// The model used to be asked for a JSON object, which made it the author of a
// document's syntax as well as of the judgement being asked for. Those are two
// different failure modes and only one of them is the question: a 4% rejection
// rate at 833 containers was document-construction failure — a missing brace, a
// trailing comma, a fenced block the extractor took the wrong end of — not a
// model that grouped badly. The container's whole structure is resolved and
// spoonfed in the prompt (see Designer.callInput and stubDefinition); what is
// open is which entries go together and what each group is called, and that is
// all a line here carries.
//
// # Forgiving of noise, strict on substance
//
// A line whose first field is not the keyword is not read at all, so a fence, a
// preamble sentence or a closing remark costs nothing. A line that IS a group
// line is held strictly: five fields, and entry numbers that are numbers. What
// the answer MEANS — the partition, the fan-out cap, the titles, the closed set
// of kinds — is treeplan.Verifier.CheckAnswer's, unchanged, and it stays the
// real net. Nothing here re-states one of its rules: an empty title, an empty
// scope, an unknown kind and an empty member list all parse and are refused
// there, by the one statement of each rule.
//
// # Model-facing vocabulary
//
// `kind` is "page" or "section", not the artifact's "leaf" and "index". It is
// the vocabulary every rejection note already uses ("these must be pages, not
// sections"), and a model told one word in the prompt and corrected with
// another is being asked to translate on the retry.
//
// # Members are 1-based
//
// The candidate list is numbered from 1, so the answer is too; the mapping to
// the 0-based indices treeplan.CheckAnswer partitions over happens here, in the
// one place that knows both spellings. An out-of-range number survives the
// mapping deliberately — the partition check is what refuses it, with a note.
const (
	// kindPage and kindSection are the model-facing flag values. The set is
	// closed at two: `annex` is not a model-facing flag, because a model that
	// can place material out of scope can hide its own failures (O-12, §2.8).
	kindPage    = "page"
	kindSection = "section"

	// groupKeyword opens a group line. It is the whole of what makes a line
	// substance rather than noise, so it is matched case-insensitively and
	// through whatever list bullet or emphasis a model wrapped it in
	// (trimMarkup) — none of which changes what the line says.
	groupKeyword = "group"

	// fieldSep separates a group line's fields. Two colons rather than one:
	// a title, a scope and a kind are all prose that may hold a colon, and a
	// separator a value can contain is a separator that eventually splits a
	// value.
	fieldSep = "::"

	// memberSep separates entry numbers inside the last field.
	memberSep = ","

	// groupFields is how many fields a group line has: the keyword, the
	// title, the scope, the kind and the entry numbers.
	groupFields = 5

	// noteLineWords caps the quotation of the offending line in the
	// MODEL-FACING note, so a retry names the line it is about without
	// spending a share of the trailer that belongs to the acceptance criteria
	// (prompt.maxCriteriaWords, §9). The cut is marked (text.CapWords), and
	// the operator-facing rendering carries the line whole. Bounding it here
	// rather than relying on the runner's own note cap to trim is §12's
	// bounded-by-construction rule: the fact must survive the cap, so the
	// quotation cannot be what pushes it out.
	noteLineWords = 5
)

// AnswerGrammar is the ONE statement of what a group line looks like: the
// definition renders it, the example below is one filled-in line of it, and
// parseAnswer reads exactly it. It is assembled from the same constants the
// parser splits on, so the sentence the model is shown and the sentence the
// machine enforces cannot drift apart.
//
// It is the answer-format declaration ARCHITECTURE.md §3 names — the single
// home an optional provider-side tightening (schema-constrained decoding, if
// one dialect ever clears the provider-contract bar) would be derived from
// rather than a second description of.
const (
	answerGrammar = groupKeyword + " " + fieldSep + " <title> " + fieldSep + " <scope> " + fieldSep +
		" " + kindPage + " or " + kindSection + " " + fieldSep + " <entry numbers>"

	answerExample = groupKeyword + " " + fieldSep + " Installing the appliance " + fieldSep +
		" what a reader does to get it running " + fieldSep + " " + kindPage + " " + fieldSep + " 1, 4, 7"
)

// parseAnswer reads a response into the typed answer, one group per matching
// line and nothing else read.
//
// A response with no group line in it at all is the one whole-response
// rejection left: the model answered something other than the question.
func parseAnswer(response string) (treeplan.GroupingAnswer, error) {
	var out treeplan.GroupingAnswer
	for n, line := range strings.Split(response, "\n") {
		fields, ok := groupLine(line)
		if !ok {
			continue
		}
		if len(fields) != groupFields {
			return treeplan.GroupingAnswer{}, lineRejection(n, line, "five fields per group line —")
		}
		members, err := memberList(fields[4])
		if err != nil {
			return treeplan.GroupingAnswer{}, lineRejection(n, line, "entry numbers are plain numbers —")
		}
		out.Groups = append(out.Groups, treeplan.AnswerGroup{
			Title:   fields[1],
			Scope:   fields[2],
			Kind:    nodeKind(fields[3]),
			Members: members,
		})
	}
	if len(out.Groups) == 0 {
		return treeplan.GroupingAnswer{}, treeplan.RejectionError{
			Reason: "answer one " + groupKeyword + " line per group"}
	}
	return out, nil
}

// groupLine reports whether a line is a group line and returns its trimmed
// fields if it is.
//
// The keyword is what decides, and it is read through trimMarkup so a bullet, a
// bold run or a stray backtick around it is noise rather than a lost group. The
// VALUES are trimmed of whitespace only: what a model put inside a title is the
// title.
func groupLine(line string) ([]string, bool) {
	fields := strings.Split(line, fieldSep)
	if !strings.EqualFold(trimMarkup(fields[0]), groupKeyword) {
		return nil, false
	}
	for i := range fields {
		fields[i] = strings.TrimSpace(fields[i])
	}
	return fields, true
}

// trimMarkup strips the decoration a model wraps a keyword or a label in — a
// list bullet, emphasis, a code span, a blockquote marker — leaving the word
// itself. It is deliberately applied only where a KEYWORD is matched, never to
// a value: a title that starts with an asterisk is a title.
func trimMarkup(s string) string { return strings.Trim(s, " \t\r-*`#>_") }

// memberList reads the entry-number field: 1-based numbers, comma-separated,
// mapped onto the candidate slice's own 0-based indices.
//
// An empty field yields no members, which CheckAnswer refuses in its own words
// ("a group holds no entries"). A trailing comma is noise and is skipped;
// anything else that is not a number fails the line.
func memberList(field string) ([]int, error) {
	var out []int
	for _, tok := range strings.Split(field, memberSep) {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		n, err := strconv.Atoi(tok)
		if err != nil {
			return nil, err
		}
		out = append(out, n-1)
	}
	return out, nil
}

// lineRejection is a defective group line, in the type's two renderings: the
// operator gets the line's ordinal and the line whole, and the model gets the
// mechanical fact with the line quoted after it, capped so the fact survives
// the runner's note budget.
func lineRejection(n int, line, fact string) error {
	return treeplan.RejectionError{
		Subject: fmt.Sprintf("line %d: %s", n+1, strings.TrimSpace(line)),
		Reason:  fact + " " + text.CapWords(line, noteLineWords),
	}
}

// nodeKind maps the model's flag onto the artifact's node kind. An unknown
// value maps to itself, so the rejection comes from CheckAnswer's own closed
// set ("a group is neither a page nor a section") rather than from a second
// spelling of that rule here.
func nodeKind(flag string) treeplan.Kind {
	switch strings.ToLower(strings.TrimSpace(flag)) {
	case kindPage:
		return treeplan.KindLeaf
	case kindSection:
		return treeplan.KindIndex
	default:
		return treeplan.Kind(flag)
	}
}
