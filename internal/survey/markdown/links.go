package markdown

import (
	"net/url"
	"path"
	"slices"
	"strings"

	"kbase/internal/ingest"
	"kbase/internal/log"
	"kbase/internal/survey"
)

// rawLink is a link destination exactly as written, before classification.
type rawLink struct {
	target string
	image  bool
}

// resolveLinks classifies each distinct destination and sorts the result.
// Sorting by target (images after links of the same target) makes the
// artifact diffable between runs and independent of the order the walk
// happened to reach equivalent references in.
//
// An empty destination (`[text]()`) is dropped: it names nothing, so there is
// no edge to record and nothing a later stage could act on.
func resolveLinks(raws []rawLink, from string, corpus ingest.Corpus, lg log.Logger) []survey.Link {
	seen := make(map[rawLink]bool, len(raws))
	out := make([]survey.Link, 0, len(raws))
	for _, r := range raws {
		if r.target == "" || seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, classifyLink(r, from, corpus, lg))
	}
	slices.SortFunc(out, func(a, b survey.Link) int {
		if c := strings.Compare(a.Target, b.Target); c != 0 {
			return c
		}
		switch {
		case a.Image == b.Image:
			return 0
		case a.Image:
			return 1
		default:
			return -1
		}
	})
	return out
}

// classifyLink decides where one destination points.
func classifyLink(r rawLink, from string, corpus ingest.Corpus, lg log.Logger) survey.Link {
	l := survey.Link{Target: r.target, Image: r.image}
	u, err := url.Parse(r.target)
	if err != nil {
		// Not a URL at all. It cannot be resolved and it is certainly not a
		// reachable external reference, so it is recorded as the corpus's
		// problem rather than silently classified away.
		l.Kind = survey.LinkUnresolved
		return l
	}
	l.Fragment = u.Fragment
	switch {
	case u.Scheme != "" || u.Host != "":
		l.Kind = survey.LinkExternal
	case u.Path == "":
		l.Kind = survey.LinkAnchor
	default:
		// u.Path is the percent-DECODED destination, and that decode is the
		// one way a target can arrive in a spelling the corpus ids are not
		// in: custody made the source bytes NFC, but `%CC%81` is plain ASCII
		// in those bytes and becomes a combining mark only here. A target
		// written literally is already NFC and this is a no-op on it.
		if target, ok := resolveTarget(ingest.NormalizePath(u.Path), from, corpus, lg); ok {
			l.Kind, l.Path = survey.LinkInternal, target
		} else {
			l.Kind = survey.LinkUnresolved
		}
	}
	return l
}

// resolveTarget maps a link path to a corpus document id.
//
// A rooted path names the corpus root and is resolved once. A relative one is
// tried in TWO address spaces, in decreasing confidence, because a doc corpus
// is written against one of them and kbase reads it in the other:
//
//   - File space. The destination is joined against the linking file's own
//     directory — what the path means to anything reading the corpus as
//     files, and the spelling a corpus that never rendered uses.
//   - URL space. A doc-site generator serves `page.md` at `page/`, so a
//     destination in a page's own source is written relative to that
//     directory: `../properties` in `project-format.md` names the SIBLING
//     `properties`, not a file above the corpus root. The linking file
//     stripped of its extension is that directory, and joining against it is
//     the same arithmetic the site does.
//
// Both spaces run the same lookup, so extension inference and the case fold
// apply to each: neither space is a second resolver.
//
//   - Extensionless. Doc sites routinely link a sibling by name alone
//     (`[scope](scope)`) and leave the extension to the site generator. Each
//     extension the adapter reads is tried, in Extensions' order.
//   - Case-folded. The walk accepts `.MD` because casing conveys nothing
//     about a document (Ext), so resolution has to agree — otherwise every
//     link into a Windows-authored file inflates the unresolved count. A fold
//     that several documents answer to is ambiguous: it is reported as
//     unresolved and warned about, never guessed at.
//
// Escaping the corpus root disqualifies one ATTEMPT, never the destination:
// a `../` that leaves the root in file space is exactly the spelling that
// lands inside it in URL space, and refusing before the second attempt is
// what made every cross-file reference on a doc-site corpus unresolved.
//
// Every comparison here is byte equality (or the case fold) against the
// corpus's ids, with no normalization of its own. It can be, because both
// sides are already NFC: the ids because ingest normalized them, and p
// because it came out of NFC custody bytes — via classifyLink, which is where
// the one spelling that does NOT arrive that way is dealt with.
//
// The id returned is always the corpus's own byte-exact path. No target is
// invented: anything the corpus does not hold stays unresolved.
func resolveTarget(p, from string, corpus ingest.Corpus, lg log.Logger) (string, bool) {
	if strings.HasPrefix(p, "/") {
		return lookup(path.Clean(strings.TrimPrefix(p, "/")), p, from, "rooted", corpus, lg)
	}
	if id, ok := lookup(path.Join(path.Dir(from), p), p, from, "file space", corpus, lg); ok {
		return id, true
	}
	return lookup(path.Join(pageDir(from), p), p, from, "URL space", corpus, lg)
}

// pageDir is the directory a rendered doc site serves one source file from:
// the file's own path with its extension dropped. It is the base a
// destination written inside that page resolves against in URL space.
func pageDir(from string) string {
	return strings.TrimSuffix(from, path.Ext(from))
}

// lookup answers ONE resolution attempt: the exact id, then the extension the
// site generator would have added, then the case fold of either. space names
// the address space for the debug record, so a reader of the log can see which
// convention a corpus's links are written in.
func lookup(target, p, from, space string, corpus ingest.Corpus, lg log.Logger) (string, bool) {
	if target == ".." || strings.HasPrefix(target, "../") {
		return "", false // this attempt escapes the corpus root
	}
	if corpus.Has(target) {
		if space != "file space" {
			lg.Debug("survey resolved a link", "from", from, "target", p, "path", target, "space", space)
		}
		return target, true
	}

	candidates := []string{target}
	if path.Ext(target) == "" {
		// Every extension the adapter calls a document, in Extensions' own
		// order: the site generator appends one of them, and which one is the
		// corpus's business rather than this resolver's.
		for _, ext := range Extensions() {
			withExt := target + ext
			if corpus.Has(withExt) {
				lg.Debug("survey resolved an extensionless link",
					"from", from, "target", p, "path", withExt, "space", space)
				return withExt, true
			}
			candidates = append(candidates, withExt)
		}
	}
	for _, c := range candidates {
		id, ambiguous := corpus.FoldedPath(c)
		switch {
		case ambiguous:
			lg.Warn("survey link folds to several documents; leaving it unresolved",
				"from", from, "target", p, "folded", c, "space", space)
			return "", false
		case id != "":
			lg.Debug("survey resolved a link by case fold",
				"from", from, "target", p, "path", id, "space", space)
			return id, true
		}
	}
	return "", false
}
