package assemble

import (
	"strings"

	"kbase/internal/version"
)

// The generic agent-definition samples: what `kbase write-agents` writes to a
// directory the user names.
//
// They are NOT delivered files. They are not in the manifest, no gate reads
// them, and kbase never rewrites one — the verb refuses an existing file
// rather than overwriting it — so they carry no corpus hash and no build date.
// A sample is a function of the app version and nothing else.
//
// They live in this package because they render `contractText`, which §4.3's
// entry-point and §8.1's AGENTS.md fixture also render: one string, N render
// sites, no second place for the contract statement to drift.
const (
	docentSample     = "docent.md"
	maintainerSample = "maintainer.md"

	// adaptationSampleName is a NOTE about the definitions beside it, not one
	// of them. Under a name like `adaptation.md`, sitting next to
	// `docent.md` and `maintainer.md`, it reads as a third agent definition —
	// a claim about a file that has no such content.
	adaptationSampleName = "README-ADAPTATION.md"

	// sampleStamp is the only provenance a sample carries: the app version,
	// which is what implies the definition set. A corpus hash or a build date
	// would be a claim about a build that never happened.
	sampleStamp = "kbase " + version.Current

	// sampleNotice is the samples' counterpart of generatedNotice. It cannot
	// be that constant: generatedNotice says edits are overwritten on the next
	// build, and for a sample that is false in both halves — there is no next
	// build, and the verb refuses rather than overwrite.
	sampleNotice = "<!-- written by " + sampleStamp + "; this copy is yours to adapt — kbase never rewrites it -->"

	// sampleFooter is the delivered pages' provenance receipt with the
	// per-build fields removed.
	sampleFooter = "<!-- " + sampleStamp + " -->"

	// stubNotice marks a definition whose real content is a later burst's. It
	// is a marked stub rather than a plausible-looking definition, because a
	// definition that reads like the finished one is how a stub ships.
	stubNotice = "**This is a stub.** A finished definition ships in a later kbase version; " +
		"until then this file records what will be in it."
)

// GenericAgents renders the three samples, in write order.
func GenericAgents() []Fixture {
	return []Fixture{
		{Path: docentSample, Data: definitionSample("Docent",
			"navigates a kbase knowledge base: reads the entry-point, follows down-links to the leaf that answers, and answers from leaf text.")},
		{Path: maintainerSample, Data: definitionSample("Maintainer",
			"extends a kbase knowledge base in place: adds leaves, keeps up-links and down-links true, and re-runs the verify gates.")},
		{Path: adaptationSampleName, Data: adaptationSample()},
	}
}

func definitionSample(name, role string) []byte {
	var b strings.Builder
	b.WriteString(sampleNotice)
	b.WriteString("\n\n# ")
	b.WriteString(name)
	b.WriteString("\n\n")
	b.WriteString(stubNotice)
	b.WriteString("\n\nRole: an agent that ")
	b.WriteString(role)
	b.WriteString("\n\n")
	b.WriteString(contractText)
	b.WriteString("\n\n")
	b.WriteString(sampleFooter)
	b.WriteString("\n")
	return []byte(b.String())
}

func adaptationSample() []byte {
	var b strings.Builder
	b.WriteString(sampleNotice)
	b.WriteString("\n\n# Adapting these definitions\n\n")
	b.WriteString("These are samples, not the definitions kbase runs on. The ones it operates with " +
		"are embedded in the binary and fixed by its version, so editing a file here changes " +
		"nothing about how kbase builds a knowledge base.\n\n" +
		"Adapt them for the model you bring. A stronger model than the one a knowledge base was " +
		"built with will do better with a definition written for its own capabilities — having it " +
		"rewrite these is the recommended first step, and kbase will not take the edit back.\n\n")
	b.WriteString(sampleFooter)
	b.WriteString("\n")
	return []byte(b.String())
}
