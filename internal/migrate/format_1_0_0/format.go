// Package format_1_0_0 is the 1.0.0 metadata format's frozen definition: its
// stamp, the register's field lines, tokens and annotations, its register file,
// and the documents it declares axioms in.
package format_1_0_0

import "regexp"

const (
	Version100 = "1.0.0"

	IndexDir = "kb-root/.index"
)

const PendingLiteral = "*pending*"

// The 1.0.0 forms the 2.0.0 reader no longer spells: a premise bullet's
// scoring target and its two annotations, the pair list an experiment
// declares, and a register field line.
var (
	EntryMarkerRE = regexp.MustCompile(`^<!--\s*id:\s*((?:clm|sup)-[a-z0-9]{6}|` + workIDPattern + `)\s*-->$`)
	FieldLineRE   = regexp.MustCompile(`^(\s*)- ([a-z][a-z-]*):(.*)$`)
	NodeKeyRE     = regexp.MustCompile(`^(\s*)(strengthens|supports):(.*)$`)
	AnalysisIDRE  = regexp.MustCompile(`^(\s*)- sup-id:\s*(sup-[a-z0-9]{6})\s*$`)

	ClaimTokenRE = regexp.MustCompile(`\bclm-[a-z0-9]{6}\b`)
	WorkTokenRE  = regexp.MustCompile(`\b` + workIDPattern)
	AxiomTokenRE = regexp.MustCompile(`\bAxiom [A-Za-z0-9][A-Za-z0-9._-]*`)
	CitationRE   = regexp.MustCompile(`\bINVARIANT-([A-Z]+[0-9]+)\b`)

	SolidityAnnRE      = regexp.MustCompile(`\(solidity\s+([^()]*)\)`)
	ApplicabilityAnnRE = regexp.MustCompile(`\(applicability\s+([^()]*)\)`)
	Tier2Marker100RE   = regexp.MustCompile(`<!--(\s*)claim-quality(\s*:)`)
	ContextRE          = regexp.MustCompile(`\[[^\[\]]*\]\s*$`)
	TrailingParenRE    = regexp.MustCompile(`\(([^()]*)\)$`)
)

// The register file's basename; a 1.0.0 register is renamed on that exact
// basename, at every volume's depth.
const Register100Name = "claim-quality.md"

// The files a 1.0.0 KB declares its axioms in.
const (
	InvariantsPath = "kb-root/invariants.md"
	AgentsPath     = "kb-root/AGENTS.md"
)

var (
	LegacyInvariantHeadingRE = regexp.MustCompile(`^### INVARIANT-([A-Z]+[0-9]+):\s*(.*)$`)
	LegacyAxiomBulletRE      = regexp.MustCompile(`^- Axiom ([0-9]+):\s*\*\*(.+?)\*\*\s*(.*)$`)
	SectionEndRE             = regexp.MustCompile(`^#{1,3}\s`)
)

// workIDPattern is an external work's id as the 1.0.0 reader took it: the
// prefix and a citation key whose ends are alphanumeric.
const workIDPattern = `work-[A-Za-z0-9](?:[A-Za-z0-9_.:+/-]*[A-Za-z0-9])?`
