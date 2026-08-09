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
	// (`gemma-4`, `gemma_4`); markerFamilyFused is the same marker written
	// as one token (`gemma4`, `gemma4:31b`); markerFamilyDotted is the
	// dotted spelling (`gemma.4`), matched whole because the family marker
	// is read WITHOUT splitting on `.` — see familySeparators.
	markerFamily       = "gemma"
	markerVersion      = "4"
	markerFamilyFused  = markerFamily + markerVersion
	markerFamilyDotted = markerFamily + "." + markerVersion

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
// are equivalent when reading TIER markers: normalization splits on any of
// them, so `gemma-4-31b`, `gemma_4_31b` and `gemma4:31b` reduce to the same
// token sequence.
const separators = "/:_-. "

// familySeparators is separators without `.`, and is what the FAMILY marker
// is read with. A point release is written with a dot, so keeping `4.1`
// whole is what makes `gemma-4.1-31b` a non-member: split on the dot it
// would read as `gemma` `4` and inherit this family's tier assignment,
// which is a successor family's weights running an evaluated prompt set
// nobody ran them against. The dotted spelling of the family itself is
// still recognized — see isFamily.
const familySeparators = "/:_- "

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
// version must be exactly `4`, so `gemma-3-27b`, `gemma34b`, a
// hypothetical `gemma-4b` (a 4-billion-parameter model, not the family)
// and a `gemma-4.1-*` successor are all non-members.
//
// What this cannot see: a third-party finetune whose id is token-identical
// to a first-party one — `someorg/gemma-4-31b-abliterated` differs from
// the real thing only in a suffix no rule can enumerate, and the org
// prefix is dropped precisely so that an org name never decides a family.
// Such ids classify as first-party, and that is accepted: the alternative
// is an allowlist of suffixes that goes stale the week it is written.
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
		tokens := tokenize(id, separators)
		switch {
		case !isFamily(tokenize(id, familySeparators)):
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
// split on every character in seps — separators for tier markers,
// familySeparators for the family marker.
//
// Dropping the prefix is what keeps an organization named after a model
// family from being read as one — `gemma-labs/llama-3-8b` is a llama.
func tokenize(id, seps string) []string {
	name := id
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	return strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return strings.ContainsRune(seps, r)
	})
}

// isFamily reports whether tokens — split on familySeparators, so a dotted
// version arrives whole — carry the gemma-4 family marker.
//
// Three spellings count and no others: fused (`gemma4`), dotted
// (`gemma.4`), and split across two tokens (`gemma` `4`). A version token
// that is anything but exactly `4` fails all three, which is what puts
// `gemma-4.1-31b` and `gemma4.1-31b` outside the family rather than inside
// it wearing its tier.
func isFamily(tokens []string) bool {
	for i, tok := range tokens {
		switch {
		case tok == markerFamilyFused, tok == markerFamilyDotted:
			return true
		case tok == markerFamily && i+1 < len(tokens) && tokens[i+1] == markerVersion:
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
