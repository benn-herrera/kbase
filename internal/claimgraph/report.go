// Package claimgraph authors kb_tools' claim graph into a document tree:
// conformance over the Document-Tree Contract, the claim-site inventory read
// from the document graph's records and the page, the claims the author marked,
// the edge candidates and their mechanical drafts, the referenced equations,
// the off-graph endcap, the writes through the write API, and the refresh and
// verify gate. The node pass and classification put their letter asks through
// the reader they are handed; nothing here reaches a model otherwise.
package claimgraph

import (
	"fmt"
	"slices"
	"strings"

	"kbase/internal/result"
)

// Statuses of a finding: a check that held, one that did not, and a fact that
// describes state and gates nothing.
const (
	StatusPass = "PASS"
	StatusFail = "FAIL"
	StatusFact = "FACT"
)

// Finding is one thing a stage reports: its status, the check it belongs to,
// and what it found, every count present in its zero form.
type Finding struct {
	Status string
	Check  string
	Fields result.Record
}

// Record is the finding as a result-document mapping.
func (f Finding) Record() result.Record {
	return append(result.Record{{Key: "status", Value: f.Status}, {Key: "check", Value: f.Check}}, f.Fields...)
}

func field(key string, value any) result.Field { return result.Field{Key: key, Value: value} }

func recordOf(fields ...result.Field) result.Record { return fields }

func fact(check string, fields ...result.Field) Finding {
	return Finding{StatusFact, check, fields}
}

func pass(check string, fields ...result.Field) Finding {
	return Finding{StatusPass, check, fields}
}

// Report is what one stage produced and what its checks said.
type Report struct {
	Findings []Finding
}

// Failed is whether any check failed.
func (r Report) Failed() bool {
	return slices.ContainsFunc(r.Findings, func(f Finding) bool { return f.Status == StatusFail })
}

// Failures is every failed check as a result item, its detail where the
// finding states one.
func (r Report) Failures() []result.Item {
	var out []result.Item
	for _, f := range r.Findings {
		if f.Status != StatusFail {
			continue
		}
		it := result.Item{Check: f.Check, Detail: "the check failed; the stage's report holds what it found"}
		for _, ff := range f.Fields {
			if d, ok := ff.Value.(string); ok && ff.Key == "detail" {
				it.Detail = d
			}
		}
		out = append(out, it)
	}
	return out
}

// Records is every finding as a result-document mapping, in order.
func (r Report) Records() []result.Record {
	out := make([]result.Record, len(r.Findings))
	for i, f := range r.Findings {
		out[i] = f.Record()
	}
	return out
}

// stop is a stage halting on a named check: the comparison it ended on and
// what that comparison found.
type stop struct {
	check, detail string
}

func (e *stop) Error() string { return e.check + ": " + e.detail }

func stopf(check, format string, args ...any) *stop {
	return &stop{check, fmt.Sprintf(format, args...)}
}

func (e *stop) finding() Finding {
	return Finding{StatusFail, e.check, result.Record{{Key: "detail", Value: e.detail}}}
}

// counts is a zero-form count per name, names in the order given.
func counts(names []string, of map[string]int) result.Record {
	out := make(result.Record, len(names))
	for i, n := range names {
		out[i] = result.Field{Key: n, Value: of[n]}
	}
	return out
}

// tally is a count per distinct value, values sorted.
func tally(values []string) result.Record {
	of := map[string]int{}
	for _, v := range values {
		of[v]++
	}
	names := make([]string, 0, len(of))
	for n := range of {
		names = append(names, n)
	}
	slices.Sort(names)
	return counts(names, of)
}

// pair is an ordered pair of claim ids: a source and the target it names.
type pair struct{ source, target string }

func comparePairs(a, b pair) int {
	if c := strings.Compare(a.source, b.source); c != 0 {
		return c
	}
	return strings.Compare(a.target, b.target)
}

// pairs renders (source, target) pairs as "source -> target".
func pairs(ps []pair) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.source + " -> " + p.target
	}
	return out
}
