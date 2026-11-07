// Package format20 is the 2.0.0 metadata format's frozen definition, every
// 2.0 patch included: its stamp, the premise bullet's annotations, its register
// file, and the document it declares axioms in.
package format20

import "regexp"

const Version = "2.0.0"

var (
	BearingAnnRE  = regexp.MustCompile(`\(bearing\s+[^()]*\)`)
	StrengthAnnRE = regexp.MustCompile(`\(strength\s+[^()]*\)`)
)

// The one document the 2.0.0 form declares its axioms in.
const AxiomsPath = "kb-root/axioms.md"
