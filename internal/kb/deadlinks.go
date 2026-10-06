package kb

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

// Kinds of dead link: a target inside the repository, or one that escapes it.
const (
	BrokenIntra = "broken intra"
	BrokenInter = "broken inter"
)

// DeadLink is one link whose target names no file.
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
	return DeadLinksIn(src, src.Root())
}

// DeadLinksIn is DeadLinks over the KB's files, each target judged against
// repo, a directory at or above kb-root: broken intra where it lies under
// repo, broken inter only where it escapes it, and spelled as written below
// repo.
func DeadLinksIn(src *Source, repo string) ([]DeadLink, error) {
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
				if target == "" || strings.HasSuffix(target, ".tex") {
					continue
				}
				resolved := filepath.FromSlash(target)
				if !filepath.IsAbs(resolved) {
					resolved = filepath.Join(filepath.Dir(file), resolved)
				}
				found, err := existsAsSpelled(resolved, repo, listings)
				if err != nil {
					return nil, err
				}
				if found {
					continue
				}
				kind := BrokenIntra
				if !Within(repo, resolved) {
					kind = BrokenInter
				}
				dead = append(dead, DeadLink{File: filepath.ToSlash(rel), Line: n + 1, Kind: kind, Target: raw, Resolved: resolved})
			}
		}
	}
	return dead, nil
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
