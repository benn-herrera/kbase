package treeplan

import (
	"errors"
	"fmt"

	"kbase/internal/dissect"
)

// Rejection is a verifier failure a MODEL could be responsible for: a
// partition that dropped a candidate, a group count past the fan-out cap, an
// index asked for at the depth cap, a span too large to be one leaf.
//
// The stage-3 seam retries once with Note attached and then fails the unit —
// taxonomy is a no-fallback seam, so there is no mechanical fallback to fall
// back to (I-7, §3.4). The interface is declared here rather than at that seam
// because the classification is knowledge this package has and the runner does
// not.
//
// Every failure this package returns is either a Rejection or a DefectError,
// and callers tell them apart with errors.As.
type Rejection interface {
	error

	// Note is the model-facing rendering: the mechanical fact alone, no path
	// and no byte offset. The runner caps the retry note it becomes at
	// twelve words (pipeline.retryNoteWords), so a sentence of prose
	// here is a sentence the model never sees the end of — and a path in it
	// would be a node name the model is forbidden to emit sitting in the very
	// next prompt (I-2).
	Note() string
}

// RejectionError is the general rejection: a structural post-condition a
// legal answer could plausibly fail.
//
// It has TWO renderings and the difference is the point. Error is
// operator-facing and names the subject — a node path, a title, a group id.
// Note is model-facing and names nothing: Reason carries the mechanical fact
// and no identifier the model could act on.
type RejectionError struct {
	// Subject locates the failure for an operator: a path, a title, a group
	// id. It never reaches a prompt.
	Subject string
	// Reason is the mechanical fact, twelve words or fewer, carrying no path.
	Reason string
}

func (e RejectionError) Error() string {
	if e.Subject == "" {
		return "treeplan: " + e.Reason
	}
	return fmt.Sprintf("treeplan: %s: %s", e.Subject, e.Reason)
}

// Note is the model-facing rendering: the mechanical fact alone.
func (e RejectionError) Note() string { return e.Reason }

// StarvedRejection is §2.4 step 4: dissect.Split could not cut a span under
// the leaf budget, so the material cannot be placed where the model placed it.
//
// It is a rejection and not a defect: the remedy is at the stage that decides
// how big a unit is (dissect.StarvedRejection says so in its own words), and here
// that stage is the model — the usual correction is to descend one level. A
// second failure fails the unit loudly, because taxonomy has no fallback.
//
// It wraps the dissector's own error, so a caller that wants the span and the
// walk's diagnosis reaches it with errors.As. The Note deliberately carries
// the token count and no offset: §2.4 specifies the count as the corrective
// note's content, and an offset is a number in the shape of an answer.
type StarvedRejection struct {
	// Subject is the node whose span was refused, operator-facing.
	Subject string
	// File is the corpus file the span is in.
	File string
	// Starved is the dissector's refusal, with the span, its token count and
	// the budget it was measured against.
	Starved dissect.StarvedRejection
}

func (e StarvedRejection) Error() string {
	return fmt.Sprintf("treeplan: %s: %s in %s", e.Subject, e.Starved.Error(), e.File)
}

// Note names the size and the remedy, and nothing that locates bytes.
func (e StarvedRejection) Note() string {
	return fmt.Sprintf("%d tokens is too large for one leaf; place it deeper", e.Starved.Tokens)
}

// Unwrap exposes the dissector's refusal to errors.As/errors.Is.
func (e StarvedRejection) Unwrap() error { return e.Starved }

// DefectError is a failure no legal answer could produce: a duplicate node
// path when the namer guarantees uniqueness, a group named by the wrong number
// of leaves, a candidate list longer than the cap the mechanical pre-batching
// is supposed to enforce.
//
// It indicts the derivation that produced both the question and the check, so
// retrying re-asks something that was never asked wrong. The stage-3 seam maps
// it to pipeline.ErrVerifierDefect, which aborts the worker (§3.4). That
// mapping is one line at the seam rather than an import here: the pipeline's
// error vocabulary is the runner's, and this package's job is to know which
// class a failure is in.
type DefectError struct {
	// Subject locates the failure.
	Subject string
	// Reason is the mechanical fact. It is operator-facing only, so it may be
	// as specific as it likes.
	Reason string
}

func (e DefectError) Error() string {
	if e.Subject == "" {
		return "treeplan: " + e.Reason
	}
	return fmt.Sprintf("treeplan: %s: %s", e.Subject, e.Reason)
}

// AsRejection reports the Rejection in err, if err is one.
//
// It exists so the seam has one spelling of the question it asks about every
// error this package returns — "is this the model's to fix?" — rather than an
// errors.As per concrete type, which is a list that grows silently wrong every
// time a rejection type is added here.
func AsRejection(err error) (Rejection, bool) {
	var r Rejection
	if errors.As(err, &r) {
		return r, true
	}
	return nil, false
}
