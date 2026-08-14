package assemble

import (
	"fmt"
	"strings"

	"kbase/internal/treeplan"
)

// §8.1's class B: the shipped fixtures.
//
// A delivered tree holds two classes of file and they have two acceptance
// rules [MAD1: F-3]. Class A is the tree-plan nodes — one file per node, one
// node per file, all of §4's grammar. Class B is THIS list, closed and
// enumerated: it carries no up-link, is reachable by no down-link, and is
// checked against these templates rather than against §4. The exemplar's own
// README behaves exactly this way; the design imported §9's checks without
// importing the exemption, which made checks 2, 4 and 5 fail on every green
// run.
//
// The manifest is fixed. A delivered file that is neither a node nor a member
// of it is a stage-9 refusal, not a warning.
const (
	// AgentsFixture is the thin contract fixture: SPEC §3's statement, the
	// annex conventions, the `.agents/` pointer, and a link to the
	// entry-point — which is what satisfies check 4's class-B direction.
	AgentsFixture = "AGENTS.md"
	// ReadmeFixture is the human-facing counterpart, carrying the same
	// entry-point link.
	ReadmeFixture = "README.md"

	// agentsDir holds the definitions that ship with every KB. Its members are
	// exempt from file-level reachability: the directory-level pointer the
	// entry-point and AGENTS.md both carry discharges it, and it discharges it
	// for nothing else (§9 check 4).
	agentsDir = ".agents/"

	docentFixture     = agentsDir + "docent.md"
	maintainerFixture = agentsDir + "maintainer.md"
	adaptationFixture = agentsDir + "adaptation.md"
	routingEvalArt    = agentsDir + "routing-eval.json"

	// stubNotice marks a fixture whose real content is a later burst's. It is
	// a marked stub rather than a plausible-looking definition, because a
	// definition that reads like the finished one is how a stub ships.
	stubNotice = "**This is a stub.** kbase writes the finished definition here when the " +
		"definition burst lands; until then this file records what will be in it."
)

// Fixture is one class-B file: its delivered path and its bytes.
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
	return []string{AgentsFixture, ReadmeFixture, docentFixture, maintainerFixture, adaptationFixture, routingEvalArt}
}

// InAgentsDir reports whether a delivered path is inside the shipped
// definitions directory — check 4's one exemption, asked in one place.
func InAgentsDir(path string) bool { return strings.HasPrefix(path, agentsDir) }

// Fixtures renders every class-B file for one tree plan.
//
// They are rendered from embedded templates plus tree-plan-derived values, and
// the two mandatory shared elements — the contract statement and the
// `.agents/` pointer — come from the same constants §4.3's entry-point renders
// (§8.1).
func (r *Renderer) Fixtures() []Fixture {
	entry := entryPointPath(r.plan)
	return []Fixture{
		{Path: AgentsFixture, Data: r.agentsFixture(entry)},
		{Path: ReadmeFixture, Data: r.readmeFixture(entry)},
		{Path: docentFixture, Data: r.definitionFixture("Docent",
			"navigates this knowledge base: reads the entry-point, follows down-links to the leaf that answers, and answers from leaf text.")},
		{Path: maintainerFixture, Data: r.definitionFixture("Maintainer",
			"extends this knowledge base in place: adds leaves, keeps up-links and down-links true, and re-runs the verify gates.")},
		{Path: adaptationFixture, Data: r.adaptationFixture()},
		{Path: routingEvalArt, Data: r.routingEvalFixture()},
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
	b.WriteString(agentsPointer)
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

func (r *Renderer) definitionFixture(name, role string) []byte {
	var b strings.Builder
	b.WriteString(generatedNotice)
	b.WriteString("\n\n# ")
	b.WriteString(name)
	b.WriteString("\n\n")
	b.WriteString(stubNotice)
	b.WriteString("\n\nRole: an agent that ")
	b.WriteString(role)
	b.WriteString("\n\n")
	b.WriteString(contractText)
	b.WriteString("\n\n")
	b.WriteString(r.prov.Footer())
	b.WriteString("\n")
	return []byte(b.String())
}

func (r *Renderer) adaptationFixture() []byte {
	var b strings.Builder
	b.WriteString(generatedNotice)
	b.WriteString("\n\n# Adapting these definitions\n\n")
	b.WriteString("These are copies for adaptation. kbase does not read them: the definitions it " +
		"operates with are embedded in the binary and fixed by its version, so editing a file here " +
		"changes nothing about how this knowledge base was built.\n\n" +
		"Adapt them for the model you bring. A stronger model than the one that wrote this base " +
		"will do better with a definition written for its own capabilities — having it rewrite " +
		"these is the recommended first step.\n\n")
	b.WriteString(r.prov.Footer())
	b.WriteString("\n")
	return []byte(b.String())
}

// routingEvalFixture is the routing-eval question set: SPEC §5's Q→leaf pairs,
// empty until stage 7.4 writes them.
//
// It is JSON rather than Markdown, so the generated-notice comment has to be a
// field. The empty question list is honest — a fixture carrying invented
// questions would be an eval that passes because it asks nothing.
func (r *Renderer) routingEvalFixture() []byte {
	var b strings.Builder
	b.WriteString("{\n")
	b.WriteString(`  "schema": "kbase.routing-eval/1",` + "\n")
	fmt.Fprintf(&b, "  %q: %q,\n", "note",
		"generated by kbase; the question set is empty until the routing-eval stage lands")
	fmt.Fprintf(&b, "  %q: %q,\n", "corpus", r.prov.CorpusHash)
	fmt.Fprintf(&b, "  %q: %q,\n", "built", r.prov.BuildDate)
	b.WriteString(`  "questions": []` + "\n")
	b.WriteString("}\n")
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

// EntryPointPath exposes it for the composing verb's report and for §9 check 3.
func (r *Renderer) EntryPointPath() string { return entryPointPath(r.plan) }
