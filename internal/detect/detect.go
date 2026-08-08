// Package detect classifies provider model ids into the gemma-4 tiers the
// pipeline names (ARCHITECTURE.md §10). It is pure: ids in, buckets out —
// no network, no config, no I/O — so the matching rules are testable as a
// table and the configure verb is the only place a decision becomes a file.
//
// The matcher is deliberately precision-first. Provider catalogues name the
// same weights half a dozen ways (`google/gemma-4-31b-it`, `gemma4:31b-a4b`),
// so recognition must tolerate that variance; but a model admitted to the
// wrong tier runs a whole pipeline stage on the wrong weights and says
// nothing about it, which is exactly the silent-platform failure SPEC.md §1
// forbids. Anything the rules do not positively recognize is reported as
// unrecognized so the caller can fail loud and list.
package detect

import "strings"

// Family and tier markers, as they appear after normalization. These are
// the whole grammar; there is no other place an id is inspected.
const (
	// markerFamily plus markerVersion is the split spelling of the family
	// (`gemma-4`, `gemma_4`, `gemma.4`); markerFamilyFused is the same
	// marker written as one token (`gemma4`, `gemma4:31b`).
	markerFamily      = "gemma"
	markerVersion     = "4"
	markerFamilyFused = markerFamily + markerVersion

	// markerMoE is the 26B-A4B mixture-of-experts marker — the active-
	// parameter count, which no dense variant carries.
	markerMoE = "a4b"

	// markerMoESize is the MoE's total parameter count. On its own it
	// names the architecture only by inference: a gemma-4 26B that is not
	// the A4B MoE is a model Google is extremely unlikely to ship, so the
	// bet is taken (user ruling, 2026-08-08) and revisited only if one
	// appears.
	markerMoESize = "26b"

	// markerDenseSize is the dense 31B parameter count.
	markerDenseSize = "31b"
)

// separators are the characters providers use between an id's parts. All
// are equivalent: normalization splits on any of them, so `gemma-4-31b`,
// `gemma_4_31b` and `gemma4:31b` reduce to the same token sequence.
const separators = "/:_-. "

// Result is one classification pass over a provider's catalogue. Every id
// handed to Classify appears in exactly one bucket, and each bucket keeps
// the order the ids arrived in.
type Result struct {
	// Heavy and Light hold the gemma-4 candidates for each tier. A tier
	// is resolvable only when its bucket holds exactly one id: zero is a
	// no-match, more than one is an ambiguity, and both are failures the
	// caller reports rather than guesses past.
	Heavy []string
	Light []string

	// UnclassifiedFamily holds ids recognized as gemma-4 but carrying no
	// tier marker — a size the tier table does not cover, or a non-chat
	// variant such as an embedding model. They are kept apart from Others
	// because they are the likeliest thing a user meant when a tier came
	// up empty, and a failure listing that buries them among unrelated
	// models is a listing nobody reads.
	UnclassifiedFamily []string

	// Others holds every id that is not gemma-4 family. It exists for the
	// fail-loud listing: "no gemma-4 family detected; found these".
	Others []string
}

// Classify sorts provider model ids into the gemma-4 tier buckets.
//
// An id is a family member when, after normalization, it contains the
// family marker: `gemma` immediately followed by `4`, either fused
// (`gemma4`) or as the next token (`gemma-4`, `gemma_4`, `gemma.4`). The
// version token must be exactly `4`, so `gemma-3-27b`, `gemma34b` and a
// hypothetical `gemma-4b` (a 4-billion-parameter model, not the family)
// are all non-members.
//
// Within the family, an `a4b` token means the MoE tier (Light) and wins
// over any size token — `gemma4:31b-a4b` names the MoE by its total and
// active parameter counts at once, and only `a4b` distinguishes the two
// architectures. A bare `26b` is read as the same MoE by its total count
// (see markerMoESize). A `31b` token with neither MoE marker means the
// dense tier (Heavy). A family member with no marker at all lands in
// UnclassifiedFamily: the tier table (ARCHITECTURE.md §10) covers two
// variants, and inferring a tier for a third from its size alone is the
// guess this package exists to refuse.
func Classify(ids []string) Result {
	var res Result
	for _, id := range ids {
		tokens := tokenize(id)
		switch {
		case !isFamily(tokens):
			res.Others = append(res.Others, id)
		case containsToken(tokens, markerMoE), containsToken(tokens, markerMoESize):
			res.Light = append(res.Light, id)
		case containsToken(tokens, markerDenseSize):
			res.Heavy = append(res.Heavy, id)
		default:
			res.UnclassifiedFamily = append(res.UnclassifiedFamily, id)
		}
	}
	return res
}

// tokenize normalizes one model id into lower-case tokens: the provider
// prefix (everything up to the last `/`) is dropped, and the remainder is
// split on every separator.
//
// Dropping the prefix is what keeps an organization named after a model
// family from being read as one — `gemma-labs/llama-3-8b` is a llama.
func tokenize(id string) []string {
	name := id
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	return strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return strings.ContainsRune(separators, r)
	})
}

// isFamily reports whether normalized tokens carry the gemma-4 family
// marker in either of its two spellings.
func isFamily(tokens []string) bool {
	for i, tok := range tokens {
		if tok == markerFamilyFused {
			return true
		}
		if tok == markerFamily && i+1 < len(tokens) && tokens[i+1] == markerVersion {
			return true
		}
	}
	return false
}

// containsToken reports whether want appears as a whole token. Whole-token
// matching is what separates the `a4b` marker from an id that merely
// contains those letters.
func containsToken(tokens []string, want string) bool {
	for _, tok := range tokens {
		if tok == want {
			return true
		}
	}
	return false
}
