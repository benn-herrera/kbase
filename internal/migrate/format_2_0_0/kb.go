package format_2_0_0

// Names the KB's readers key on.
const (
	KBDir = "kb-root"

	// IndexDir is the derived index's directory inside kb-root.
	IndexDir = ".index"

	AgentsFile         = "AGENTS.md"
	AgentsRedirectFile = "CLAUDE.md"
	// AxiomsFile is the declaration document axioms are stated in;
	// InvariantsFile and AGENTS.md are its legacy fallbacks.
	AxiomsFile     = "axioms.md"
	InvariantsFile = "invariants.md"
)

// ExcludeDirs are directories leaf discovery never descends into, and
// ExcludeNames files it never reads as a document.
var (
	ExcludeDirs  = map[string]bool{"session": true, IndexDir: true, "tools": true}
	ExcludeNames = map[string]bool{
		RegisterFile: true, "claim-quality-closure-roadmap.md": true, AgentsFile: true,
		AgentsRedirectFile: true, "CONVENTIONS.md": true, "README.md": true, AxiomsFile: true, InvariantsFile: true,
	}
)
