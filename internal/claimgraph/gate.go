package claimgraph

import (
	"slices"

	"kbase/internal/index"
	"kbase/internal/log"
	"kbase/internal/result"
)

// gate is kb_claimgraph stage G: refresh, then verify. Green or stop, on what
// each found; there is no fix loop.
func gate(kbRoot string, lg log.Logger) []Finding {
	written, err := index.Refresh(kbRoot, lg)
	if err != nil {
		return []Finding{{StatusFail, "refresh", recordOf(field("detail", err.Error()))}}
	}
	slices.Sort(written)
	findings := []Finding{pass("refresh", field("written", slices.Compact(written)))}
	found, err := index.Verify(kbRoot)
	if err != nil {
		return append(findings, Finding{StatusFail, "verify", recordOf(field("detail", err.Error()))})
	}
	if len(found) > 0 {
		return append(findings, Finding{StatusFail, "verify", recordOf(field("findings", index.Items(found)))})
	}
	return append(findings, pass("verify", field("findings", []result.Item{})))
}
