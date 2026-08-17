package assemble

import (
	"fmt"
	"strings"

	"kbase/internal/treeplan"
)

// §4.7's class B: the shipped fixtures.
//
// A delivered tree holds two classes of file and they have two acceptance
// rules [MAD1: F-3]. Class A is the tree-plan nodes — one file per node, one
// node per file, all of §4's grammar. Class B is THIS list, closed and
// enumerated: it carries no up-link, is reachable by no down-link, and is
// checked against these templates rather than against §4. The exemplar's own
// README behaves exactly this way; the design imported §4.8's guarantees without
// importing the exemption, which made checks 2, 4 and 5 fail on every green
// run.
//
// The manifest is fixed. A delivered file that is neither a node nor a member
// of it is a stage-9 refusal, not a warning.
const (
	// AgentsFixture is the thin contract fixture: SPEC §3's statement, the
	// annex conventions, the definitions pointer, and a link to the
	// entry-point — which is what satisfies check 4's class-B direction.
	AgentsFixture = "AGENTS.md"
	// ReadmeFixture is the human-facing counterpart, carrying the same
	// entry-point link.
	ReadmeFixture = "README.md"
	// ClaudeFixture is what a coding-agent session rooted at the knowledge base
	// picks up on its own: where to start, the no-crawl invariant, and the two
	// navigation habits that make a tree worth having. It is a bootstrap
	// pointer and stays short — the contract's routing half lives in
	// AgentsFixture and in the entry-point, and is not restated here.
	ClaudeFixture = "CLAUDE.md"
)

// Fixture is one rendered file: its path and its bytes. Manifest and Fixtures
// use it for the delivered class-B set; GenericAgents uses it for the samples,
// which are written on demand and never delivered.
type Fixture struct {
	Path string
	Data []byte
}

// Manifest is the closed class-B set, in delivery order.
//
// It is a function rather than a slice variable so that a caller cannot hold a
// reference to a package-level slice and edit the fixture set out from under
// the verifier that checks it.
func Manifest() []string {
	return []string{AgentsFixture, ReadmeFixture, ClaudeFixture}
}

// Fixtures renders every class-B file for one tree plan.
//
// They are rendered from embedded templates plus tree-plan-derived values, and
// the two mandatory shared elements — the contract statement and the
// definitions pointer — come from the same constants §4.3's entry-point
// renders (§4.7).
func (r *Renderer) Fixtures() []Fixture {
	entry := entryPointPath(r.plan)
	return []Fixture{
		{Path: AgentsFixture, Data: r.agentsFixture(entry)},
		{Path: ReadmeFixture, Data: r.readmeFixture(entry)},
		{Path: ClaudeFixture, Data: r.claudeFixture(entry)},
	}
}

func (r *Renderer) agentsFixture(entry string) []byte {
	var b strings.Builder
	b.WriteString(generatedNotice)
	b.WriteString("\n\n# Working with this knowledge base\n\n")
	b.WriteString(contractText)
	b.WriteString("\n\nStart at [")
	b.WriteString(r.titles[entry])
	b.WriteString("](")
	b.WriteString(entry)
	b.WriteString(").\n\n")
	b.WriteString(definitionsPointer)
	b.WriteString("\n")
	if len(r.plan.Annexes) > 0 {
		b.WriteString("\n")
		b.WriteString(annexHeading)
		b.WriteString("\n\n")
		b.WriteString("These territories are not distilled into pages. Look them up directly.\n\n")
		for _, a := range r.plan.Annexes {
			fmt.Fprintf(&b, "- `%s` — %s\n", a.Prefix, a.Convention)
		}
	}
	b.WriteString("\n")
	b.WriteString(r.prov.Footer())
	b.WriteString("\n")
	return []byte(b.String())
}

func (r *Renderer) readmeFixture(entry string) []byte {
	var b strings.Builder
	b.WriteString(generatedNotice)
	b.WriteString("\n\n# ")
	b.WriteString(r.titles[entry])
	b.WriteString("\n\nThis is a kbase knowledge base: a navigable tree built mechanically from a " +
		"documentation corpus. The pages at the bottom of the tree are verbatim slices of the " +
		"source; everything above them routes.\n\nStart at [")
	b.WriteString(r.titles[entry])
	b.WriteString("](")
	b.WriteString(entry)
	b.WriteString("). Agents should read [")
	b.WriteString(AgentsFixture)
	b.WriteString("](")
	b.WriteString(AgentsFixture)
	b.WriteString(") first.\n\n")
	b.WriteString(r.prov.Footer())
	b.WriteString("\n")
	return []byte(b.String())
}

// claudeFixture is the shape validated against a session rooted at a delivered
// knowledge base: the pointer to AgentsFixture, the start link — which is also
// what satisfies check 4's class-B direction — and the three directives.
//
// They are here rather than only in AgentsFixture because a session picks this
// file up on its own and the hop to AGENTS.md is probabilistic: the invariant
// has to hold for a session that never takes it. The other two are what a
// session gets wrong before it has read anything — it greps, and it keeps
// reading after the answer.
func (r *Renderer) claudeFixture(entry string) []byte {
	var b strings.Builder
	b.WriteString(generatedNotice)
	b.WriteString("\n\n# Working with this knowledge base\n\nRead [")
	b.WriteString(AgentsFixture)
	b.WriteString("](")
	b.WriteString(AgentsFixture)
	b.WriteString(") first for instructions on how to use this knowledge base, then start at [")
	b.WriteString(r.titles[entry])
	b.WriteString("](")
	b.WriteString(entry)
	b.WriteString(").\n\n")
	b.WriteString("- " + noCrawlInvariant + "\n")
	b.WriteString("- Navigate, don't crawl: follow the tree from the entry-point instead of " +
		"grepping the file set.\n")
	b.WriteString("- Stop reading when the question is answered.\n\n")
	b.WriteString(r.prov.Footer())
	b.WriteString("\n")
	return []byte(b.String())
}

// entryPointPath is the tree plan's own entry-point node path.
func entryPointPath(plan treeplan.TreePlan) string {
	for _, n := range plan.Nodes {
		if n.Kind == treeplan.KindEntryPoint {
			return n.Path
		}
	}
	return ""
}

// EntryPointPath exposes it for the composing verb's report and for §4.8 guarantee 3.
func (r *Renderer) EntryPointPath() string { return entryPointPath(r.plan) }
