package treeplan

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The name grammar. Paths are generated here and nowhere else (I-1): a model
// emits titles, this file turns them into names, and every later stage reads
// the result out of the artifact.
//
//	entry-point            entry-point.md
//	index at dir D         D{slug}/index.md      (and its children live in D{slug}/)
//	leaf at dir D          D{slug}.md
//	part k of n at dir D   D{slug}-{k}.md        title "{Title} (k/n)"
//
// The exemplar's own leaf names carry a source-section number
// (`s4-2-company-hamiltonian.md`, `sub4.2/`) and its subdirectories are named
// after LaTeX subsection numbers. kbase does not reproduce that: the number is
// a property of a numbered-sectioning source format, and stage 3 consumes the
// survey artifact, which has titles and byte offsets and no section numbers.
// What generalises is the shape — lowercase hyphenated slug, `index.md` at
// every directory level, one directory per index node — and that is what is
// implemented.
const (
	// slugSep joins slug words. It is a separator on INPUT too: a title's own
	// hyphen and a run of spaces are the same thing to a slug, and collapsing
	// both here is what keeps `A - B` from becoming `a---b`.
	slugSep = "-"

	// slugHostile is the whole character blacklist (ruled 2026-08-13). A slug
	// keeps the title's own bytes; these are the ones it cannot, each for a
	// reason that no amount of escaping downstream would fix:
	//
	//	/ \    path separators — a slug is ONE path element
	//	: * ? " < > |   illegal in a Windows filename, and this KB is delivered
	//	       on Windows; a name the filesystem refuses is not a name
	//	#      a link destination's fragment separator: `a#b.md` resolves to
	//	       document `a`, so the link is broken by the renderer, not the writer
	//	%      percent-encoding's introducer: `%41` in a name round-trips to `A`
	//	       through any URL-decoding consumer, so the name is ambiguous
	//	.      the extension separator, and the namer reserves SLUGS while the
	//	       filesystem sees `{slug}.md` and `{slug}/` — a node slugged
	//	       `index.md` would build the directory `index.md/` on top of its
	//	       own parent's `index.md` page
	//	( )    a link destination ends at the first unbalanced `)`, so the
	//	       renderer decides where the path stops. This one is not
	//	       hypothetical: partTitle stamps `(k/n)` onto every split part and
	//	       interposed batch, so parentheses reach names this package
	//	       GENERATES, where `domain-1-2` is simply the better name
	//
	// The rest of ASCII punctuation is deliberately NOT here — `!`, `_`, `+`,
	// `~`, `,`, `&`, brackets and braces are legal in a filename and inert in
	// a link destination. The blacklist is characters that break something,
	// not characters that look unusual.
	slugHostile = `/\:*?"<>|#%.()`

	// slugWordCap bounds a slug's word count. A title is already capped at
	// survey.WordCap (40 words) and forty words is not a filename; eight is
	// the length past which a path stops being readable in a terminal, and
	// the tree plan — not the filename — is what identifies a node anyway.
	slugWordCap = 8

	// slugByteCap bounds a slug's length. Eight words of a technical title
	// can still run long, and path length limits are a real deployment
	// surface. The cut lands on a word boundary, never mid-word.
	slugByteCap = 64

	// slugFallback names a node whose title slugs to nothing at all — a title
	// that is entirely whitespace or entirely blacklisted characters. It is
	// deliberately anonymous: the namer's disambiguation makes it unique, and
	// the title in the artifact is what a reader actually sees.
	slugFallback = "node"

	// indexFileName is the file an index node renders to, in its own
	// directory. It is reserved in every directory: a leaf whose title slugged
	// to "index" would otherwise render over its parent's index page.
	indexFileName = "index.md"

	// entryPointPath is the corpus root's file. It is reserved in the root
	// directory for the same reason.
	entryPointPath = "entry-point.md"

	// mdExt is the extension every delivered node carries.
	mdExt = ".md"
)

// Slug is the deterministic title→name function.
//
// The grammar is: the title's own characters are KEPT, blacklisted ones
// (slugHostile, whitespace, invisible and control runes) are separators,
// separator runs collapse to one hyphen, the ends are trimmed, and ASCII
// letters are lowercased. The result is then cut to slugWordCap words and
// slugByteCap bytes at word boundaries — and never mid-rune. An empty result
// becomes slugFallback.
//
// Verbatim rather than ASCII-only (ruled 2026-08-13). The earlier grammar
// dropped every non-ASCII byte, so `The σ₂ < 1 Condition` and `The τ₃ < 1
// Condition` were the same anonymous node, and a corpus written in Japanese
// named every one of its files `node-N`. The objection that narrowing
// answered — filesystems disagree about Unicode, so a name might not survive
// stage 9's byte-for-byte path comparison — is now answered upstream instead:
// ingest normalises every document to NFC before custody begins (see
// ingest.New), so a title has exactly one spelling by the time it reaches
// here, and one spelling is what stage 9 needs. The blacklist is what remains
// once encoding is no longer the problem: characters a path or a link cannot
// carry, listed with their reasons at slugHostile.
//
// ASCII case only. `unicode.ToLower` would be a second transform on bytes
// that arrived canonical, and case in most scripts is either absent or
// contested; the ASCII fold is the one that is uncontroversial and the one
// that matters for the English-titled majority.
func Slug(title string) string {
	var b strings.Builder
	b.Grow(len(title))
	sep := false
	for _, r := range title {
		if slugSeparator(r) {
			sep = true
			continue
		}
		if sep && b.Len() > 0 {
			b.WriteString(slugSep)
		}
		sep = false
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return capSlug(b.String())
}

// slugSeparator reports whether a rune breaks a slug word instead of joining
// it.
//
// utf8.RuneError covers a title that is not valid UTF-8 — ranging a string
// yields it for each bad byte — and a literal replacement character, which a
// filename is no worse for losing. unicode.Cf is the invisible-format class:
// zero-width joiners and, the reason it is named here, the bidi overrides
// that make a filename render as something other than what it is.
//
// unicode.Pd is dash punctuation, which is slugSep's own hyphen plus the
// typographic dashes an editor substitutes for it. An em dash between two
// words is a word break, and keeping it verbatim would spell that break
// `a-—-b`: three separators where the title had one.
func slugSeparator(r rune) bool {
	return r == utf8.RuneError ||
		unicode.IsSpace(r) || unicode.IsControl(r) ||
		unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Pd, r) ||
		strings.ContainsRune(slugHostile, r)
}

// capSlug cuts a slug to the word and byte caps, always at a word boundary.
func capSlug(s string) string {
	if s == "" {
		return slugFallback
	}
	words := strings.Split(s, slugSep)
	if len(words) > slugWordCap {
		words = words[:slugWordCap]
	}
	out := ""
	for i, w := range words {
		next := w
		if i > 0 {
			next = out + slugSep + w
		}
		if len(next) > slugByteCap {
			break
		}
		out = next
	}
	if out == "" {
		// One word longer than the whole byte cap: cut it, since a truncated
		// slug is still a unique-able name and the title is unharmed. A CJK
		// title is one word by construction — it has no spaces — so this is
		// the ordinary path there, not the exotic one.
		out = truncateRunes(words[0], slugByteCap)
	}
	return out
}

// truncateRunes cuts s to at most n bytes without splitting a rune. Half a
// rune is a byte sequence no filesystem, terminal or comparison agrees about,
// and a slug is a filename.
func truncateRunes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// namer allocates the names inside ONE directory and guarantees they are
// unique there.
//
// Uniqueness is guaranteed rather than checked, which is why a duplicate path
// surviving to the composed check is a DefectError and not a rejection (§3.4).
// The namespace is the slug, not the rendered file name: an index child called
// `sync` creates the directory `sync/` and a leaf sibling called `sync` creates
// `sync.md`, which do not collide as paths but read as the same node to
// anybody navigating.
//
// Uniqueness is byte-based, and byte-based is enough now that slugs carry
// Unicode: two siblings whose names differ only in normalization form cannot
// arise, because ingest normalised every document to NFC before any title was
// read out of it (ingest.New). There is one spelling to be unique over.
type namer struct {
	taken map[string]bool
}

func newNamer() *namer {
	return &namer{taken: map[string]bool{
		strings.TrimSuffix(indexFileName, mdExt):  true,
		strings.TrimSuffix(entryPointPath, mdExt): true,
	}}
}

// claim returns the free slug this node gets, and reserves it.
//
// parts is the size of the split family the name has to cover: for parts > 1
// the returned base is free AND every `{base}-{k}` is free, so the family's
// members cannot collide with a sibling either.
//
// origin is the slug of a container this node came out of — dissolution's
// group, interposition's index — or empty. On a collision it is tried as a prefix before the
// ordinal fallback — that is the design's stated disambiguation, and it
// produces a name a reader can trace back to where the node came from. The
// ordinal fallback is what covers the rest, including two nodes with the same
// title from the same container.
func (n *namer) claim(base, origin string, parts int) string {
	if origin != "" {
		if got, ok := n.tryClaim(base, parts); ok {
			return got
		}
		if got, ok := n.tryClaim(origin+slugSep+base, parts); ok {
			return got
		}
	}
	for i := 1; ; i++ {
		cand := base
		if i > 1 {
			cand = fmt.Sprintf("%s%s%d", base, slugSep, i)
		}
		if got, ok := n.tryClaim(cand, parts); ok {
			return got
		}
	}
}

// tryClaim reserves cand and its part names if every one of them is free.
func (n *namer) tryClaim(cand string, parts int) (string, bool) {
	if n.taken[cand] {
		return "", false
	}
	for k := 1; k <= parts && parts > 1; k++ {
		if n.taken[partName(cand, k)] {
			return "", false
		}
	}
	n.taken[cand] = true
	for k := 1; k <= parts && parts > 1; k++ {
		n.taken[partName(cand, k)] = true
	}
	return cand, true
}

// partName is `{slug}-{k}`, the k-th member of a split family.
//
// The name is a function of the group's title and the part's ORDINAL, and of
// nothing else. That independence is not provisional (O-8): stage 4 moves the
// boundaries inside a group, and a name derived from where a boundary landed
// would falsify the tree plan the moment refinement ran.
func partName(base string, k int) string {
	return fmt.Sprintf("%s%s%d", base, slugSep, k)
}

// partTitle is `{Title} (k/n)`, the rendered H1 of one part of a split span.
// Provisional in format, position-independent by rule, exactly as the name is.
func partTitle(title string, k, n int) string {
	return fmt.Sprintf("%s (%d/%d)", title, k, n)
}

// joinDir puts a name in a directory. dir is "" for the corpus root and
// otherwise ends in "/".
func joinDir(dir, name string) string { return dir + name }
