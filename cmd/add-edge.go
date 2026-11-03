package main

import (
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"kbase/internal/kb"
	"kbase/internal/write"
)

func addEdgeCommand() *cobra.Command {
	var lines []string
	for _, l := range relationLines() {
		lines = append(lines, l.help())
	}
	return writeOpCommand(write.AddEdge,
		"add one edge between two standing nodes",
		`add-edge adds one edge, stored in the direction it reads: A supports B means
A's strength bears on B. Many edges are many entries. Its "source" and
"target" name nodes standing where the entry applies; a minted id is known
only from its add's result, so a node whose add mints its id takes its edges
in a later batch.

The relations, by the kinds of source and target each joins:
`+strings.Join(lines, ";\n")+`.
A relation that does not join the two nodes' kinds is refused, "allowed"
naming the relations that do.

`+manuallyScoredHelp("bearing", false, " of a "+markedRelations()+"\nedge")+`
`+oneLineHelp("context")+`

An edge of that "source", "target" and "relation" already standing is
refused, whatever its values (edge exists; remedy: edit-edge, which changes
them). A scoring edge (`+proseSeries(kb.ScoringRelations, "or")+`)
that would close a cycle among the scoring edges is refused
(check: dependency-cycle), the detail naming the cycle in edge direction:
remove another edge of it first. A "relation" of demoted is refused:
restore-demoted puts a demoted edge back. Where the build demoted this edge,
the add writes it with the add's own values and deletes that demoted record.

The result gives each entry's "source", "target" and "relation".`)
}

// relationLine is one relation as add-edge's help states its pairing: it
// joins each of sources to each of targets, both in kb.NodeKinds order.
type relationLine struct {
	relation         string
	sources, targets []string
}

// relationLines is a line for each relation of kb.Relations that write.Joins
// joins for some pair of kb.NodeKinds, in kb.Relations order.
func relationLines() []relationLine {
	var out []relationLine
	for _, r := range kb.Relations {
		l := relationLine{relation: r}
		for _, k := range kb.NodeKinds {
			if slices.ContainsFunc(kb.NodeKinds, func(target string) bool { return write.Joins(r, k, target) }) {
				l.sources = append(l.sources, k)
			}
			if slices.ContainsFunc(kb.NodeKinds, func(source string) bool { return write.Joins(r, source, k) }) {
				l.targets = append(l.targets, k)
			}
		}
		if len(l.sources) > 0 {
			out = append(out, l)
		}
	}
	return out
}

// help is the line as add-edge's help lists it, a relation outside the
// scoring relations noted as carrying no strength.
func (l relationLine) help() string {
	line := "- " + l.relation + ": from " + kindsHelp(l.sources) + " to " + kindsHelp(l.targets)
	if !kb.Scoring(l.relation) {
		line += ", carrying no strength"
	}
	return line
}

// kindsHelp is kinds as prose names one of them, "any node" where they are
// every kind.
func kindsHelp(kinds []string) string {
	if len(kinds) == len(kb.NodeKinds) {
		return "any node"
	}
	var named []string
	for _, k := range kinds {
		named = append(named, kb.KindWithArticle(k))
	}
	return proseSeries(named, "or")
}

// markedRelations is the relations whose edge a supplied bearing marks
// manually scored, as prose lists them: the scoring relations but founds,
// whose record carries no mark.
func markedRelations() string {
	return proseSeries(slices.DeleteFunc(slices.Clone(kb.ScoringRelations), func(r string) bool {
		return r == kb.RelationFounds
	}), "or")
}

// manuallyScoredHelp is the sentence a help gives where a value of key marks
// its record manually scored, as its argument's description says: what that
// mark does to a score run, and what leaving key out does — an add leaves the
// record for score to judge, scope naming the records score judges ("" where
// it judges every one), an edit keeps the mark. It wraps where it begins a
// line.
func manuallyScoredHelp(key string, edit bool, scope string) string {
	marked := `A score run does not re-judge a "` + key + `" marked manually scored unless told
to`
	if edit {
		return marked + `; an edit leaving "` + key + `" out keeps the mark as it stands.`
	}
	return marked + `: leave "` + key + `" out` + scope + ` for score to judge it.`
}

// oneLineHelp is the sentence a help gives of the prose keys an op takes,
// each quoted: what a line break and a whitespace run in one do. It begins a
// line.
func oneLineHelp(prose ...string) string {
	quoted := make([]string, len(prose))
	for i, key := range prose {
		quoted[i] = `"` + key + `"`
	}
	return "A line break in " + proseSeries(quoted, "or") + ` is refused, and any other whitespace
run collapses to one space.`
}

// proseSeries is items as prose lists them, the last joined by conjunction:
// "a, b and c".
func proseSeries(items []string, conjunction string) string {
	if len(items) < 2 {
		return strings.Join(items, "")
	}
	return strings.Join(items[:len(items)-1], ", ") + " " + conjunction + " " + items[len(items)-1]
}
