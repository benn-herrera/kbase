package kb

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
)

// Kinds of dead link: a target inside the repository, one that escapes it, and
// a fragment naming no anchor of the document it reaches.
const (
	BrokenIntra    = "broken intra"
	BrokenInter    = "broken inter"
	BrokenFragment = "broken fragment"
)

// DeadLink is one link whose target names no file, or no anchor of the
// document it names.
type DeadLink struct {
	// File is the document holding the link, relative to the scanned root.
	File   string
	Line   int
	Kind   string
	Target string
	// Resolved is the absolute path the target names.
	Resolved string
}

func (d DeadLink) String() string {
	return fmt.Sprintf("%s:%d %s %q", d.File, d.Line, d.Kind, d.Target)
}

// schemeRE is a destination naming no file: a URL, or a scheme-relative one.
var schemeRE = regexp.MustCompile(`(?i)^(?:[a-z][a-z0-9+.\-]*:)?//|^(?:https?|mailto):`)

// DeadLinks is every link in every Markdown file of the KB whose target is
// not on disk, spelled as written: a filesystem that folds case does not
// rescue a link a case-sensitive one would not. External URLs, home-directory
// paths, bare anchors and .tex targets are not file links here. Links inside
// code and maths spans and fenced blocks are not links. kb-root is its own
// repository: a target escaping it is broken inter.
func DeadLinks(src *Source) ([]DeadLink, error) {
	return DeadLinksIn(src, src.Root(), nil)
}

// DeadLinksIn is DeadLinks over the KB's files, each target judged against
// repo, a directory at or above kb-root: broken intra where it lies under
// repo, broken inter only where it escapes it, and spelled as written below
// repo; a migrated source's target inside kb-root is judged through the
// source (targetExists). pages, where it is not nil, is every register page
// by kb-root-relative slash path and the slugs of the entries derived for it,
// none for a page no longer called for, and turns the fragment check on: a
// link carrying a fragment into a Markdown document under kb-root, a bare
// #fragment into its own document included, is broken fragment where the
// fragment is none of a page's slugs, or, on any other document, none of its
// anchors (documentAnchors). Where pages is nil no fragment is checked: the
// document-graph gate reads pandoc's output while a wrapped cross-reference
// still links a bare LaTeX label, which names no heading.
func DeadLinksIn(src *Source, repo string, pages map[string][]string) ([]DeadLink, error) {
	root := src.Root()
	repo, err := filepath.Abs(repo)
	if err != nil {
		return nil, err
	}
	files, err := MarkdownFiles(src)
	if err != nil {
		return nil, err
	}
	listings := map[string]map[string]bool{}
	anchors := map[string]map[string]bool{}
	var dead []DeadLink
	for _, file := range files {
		text, err := src.ReadFile(file)
		if err != nil {
			return nil, err
		}
		rel, err := filepath.Rel(root, file)
		if err != nil {
			return nil, err
		}
		for n, line := range Lines(StripCode(string(text))) {
			for _, raw := range rawTargets(line) {
				if schemeRE.MatchString(raw) || strings.HasPrefix(raw, "~") {
					continue
				}
				target := StripTarget(raw)
				resolved := filepath.FromSlash(target)
				switch {
				case target == "" && pages != nil && strings.HasPrefix(unwrapTarget(raw), "#"):
					resolved = file
				case target == "" || strings.HasSuffix(target, ".tex"):
					continue
				case !filepath.IsAbs(resolved):
					resolved = filepath.Join(filepath.Dir(file), resolved)
				}
				found, err := targetExists(src, resolved, repo, pages, listings)
				if err != nil {
					return nil, err
				}
				broken := false
				if found && pages != nil {
					if broken, err = brokenFragment(src, raw, resolved, pages, anchors); err != nil {
						return nil, err
					}
				}
				kind := BrokenIntra
				switch {
				case found && !broken:
					continue
				case found:
					kind = BrokenFragment
				case !Within(repo, resolved):
					kind = BrokenInter
				}
				dead = append(dead, DeadLink{File: filepath.ToSlash(rel), Line: n + 1, Kind: kind, Target: raw, Resolved: resolved})
			}
		}
	}
	return dead, nil
}

// brokenFragment is whether the link raw, its file resolved, carries a
// fragment naming no anchor of a Markdown document under kb-root: on a page of
// pages none of its slugs, never the page standing; on any other document none
// of documentAnchors, cached in anchors by path. An empty fragment names the
// document's top. A target outside kb-root, or not a Markdown file, has no
// anchors kbase reads.
func brokenFragment(src *Source, raw, resolved string, pages map[string][]string, anchors map[string]map[string]bool) (bool, error) {
	_, fragment, ok := strings.Cut(unwrapTarget(raw), "#")
	if !ok || fragment == "" || !Within(src.Root(), resolved) || !strings.HasSuffix(resolved, ".md") {
		return false, nil
	}
	rel, err := filepath.Rel(src.Root(), resolved)
	if err != nil {
		return false, err
	}
	if decoded, err := url.PathUnescape(fragment); err == nil {
		fragment = decoded
	}
	if slugs, isPage := pages[filepath.ToSlash(rel)]; isPage {
		return !slices.Contains(slugs, fragment), nil
	}
	if !src.IsFile(resolved) {
		return false, nil
	}
	live, cached := anchors[resolved]
	if !cached {
		text, err := src.ReadText(resolved)
		if err != nil {
			return false, err
		}
		live = documentAnchors(text)
		anchors[resolved] = live
	}
	return !live[fragment], nil
}

// elementIDRE is the id attribute of an HTML element, as a rendered
// reference-list entry's <div id="ref-<key>"> carries one.
var elementIDRE = regexp.MustCompile(`<[A-Za-z][^<>]*?\sid="([^"]+)"`)

// documentAnchors is every anchor a link's fragment can name in text: each
// heading's (HeadingAnchors) and each HTML element's id outside code.
func documentAnchors(text string) map[string]bool {
	out := map[string]bool{}
	for _, a := range HeadingAnchors(text) {
		out[a] = true
	}
	for _, m := range elementIDRE.FindAllStringSubmatch(StripCode(text), -1) {
		out[m[1]] = true
	}
	return out
}

// rawTargets are the destinations a line declares, inline and by definition.
func rawTargets(line string) []string {
	var out []string
	for _, re := range []*regexp.Regexp{LinkRE, RefDefRE} {
		for _, m := range re.FindAllStringSubmatch(line, -1) {
			out = append(out, m[1])
		}
	}
	return out
}

// targetExists is whether the link target p names a file the KB reads. Inside
// kb-root of a migrated source, whose converted files stand only in memory
// until the save, that is a file the source holds or a register page of
// pages, which the save renders, and never a path the migration made
// obsolete; anywhere else it is a file on disk, spelled as written.
func targetExists(src *Source, p, repo string, pages map[string][]string, listings map[string]map[string]bool) (bool, error) {
	if src.Migrated() && Within(src.Root(), p) {
		if rel, err := filepath.Rel(src.Root(), p); err == nil {
			if _, isPage := pages[filepath.ToSlash(rel)]; isPage {
				return true, nil
			}
		}
		if rel, ok := src.RepoRel(p); ok {
			if _, held := src.files[rel]; held {
				return true, nil
			}
			if src.isObsolete(rel) {
				return false, nil
			}
		}
	}
	return existsAsSpelled(p, repo, listings)
}

// existsAsSpelled is whether p exists with every segment below root spelled
// as the directory listing spells it. listings caches directory listings for
// one scan.
func existsAsSpelled(p, root string, listings map[string]map[string]bool) (bool, error) {
	if _, err := os.Stat(p); errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) || errors.Is(err, syscall.ELOOP) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if !Within(root, p) {
		return true, nil
	}
	rel, err := filepath.Rel(root, p)
	if err != nil || rel == "." {
		return err == nil, err
	}
	current := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		names, ok := listings[current]
		if !ok {
			entries, err := os.ReadDir(current)
			if err != nil {
				return false, nil
			}
			names = map[string]bool{}
			for _, e := range entries {
				names[e.Name()] = true
			}
			listings[current] = names
		}
		if !names[part] {
			return false, nil
		}
		current = filepath.Join(current, part)
	}
	return true, nil
}

// Within is whether path p lies at or under directory dir.
func Within(dir, p string) bool {
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
