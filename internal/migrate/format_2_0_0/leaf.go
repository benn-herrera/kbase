package format_2_0_0

import (
	"fmt"
	"strings"
)

// MalformedError is authored metadata no reader can take: refresh and verify
// stop on it.
type MalformedError struct{ Msg string }

func (e MalformedError) Error() string { return e.Msg }

func malformed(format string, args ...any) error {
	return MalformedError{Msg: fmt.Sprintf(format, args...)}
}

// The frontmatter keys a leaf's node declaration carries besides its id and
// status: the node's local value, and the pair lists an analysis or experiment
// authors on the source.
const (
	LiftsKey   = "lifts"
	RefutesKey = "refutes"
)

// pairTargetKind is a scoring pair target's node kind, read off an id
// pairTargetIDRE matched.
func pairTargetKind(id string) string {
	if strings.HasPrefix(id, "ana-") {
		return NodeKindAnalysis
	}
	return NodeKindClaim
}
