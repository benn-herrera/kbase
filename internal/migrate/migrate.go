// Package migrate converts a KB's metadata files from an older format version
// to the current one in memory: the files as bytes in, the next form's files
// as bytes out, plus the paths the conversion made obsolete. It touches no file
// and imports nothing of kbase; every superseded form's parser lives here.
package migrate

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// Files is a KB's metadata files by repository-relative slash path: kb-root/…
// and the build records beside it.
type Files map[string][]byte

// converter is one step from format version From to To. Convert returns the
// files in To's form — a file the step renames appears under its new path
// only — and the input paths that form no longer has.
type converter struct {
	From, To string
	Convert  func(Files) (Files, []string, error)
}

var converters = []converter{{From: version090, To: version100, Convert: convert090To100}}

// Chain converts files from version from to version to through the
// registered converters in version order. The obsolete paths are the union of
// every step's, sorted. A version no converter leads on from is an error
// naming it.
func Chain(from, to string, files Files) (Files, []string, error) {
	return chain(converters, from, to, files)
}

func chain(steps []converter, from, to string, files Files) (Files, []string, error) {
	target, err := parseSemver(to)
	if err != nil {
		return nil, nil, err
	}
	current, err := parseSemver(from)
	if err != nil {
		return nil, nil, err
	}
	if compareSemver(current, target) > 0 {
		return nil, nil, fmt.Errorf("migrate: %s is newer than %s, and nothing downgrades", from, to)
	}
	obsolete := map[string]bool{}
	for version := from; compareSemver(current, target) < 0; {
		step, ok := nextStep(steps, version, target)
		if !ok {
			return nil, nil, fmt.Errorf("migrate: no converter leads from %s toward %s", version, to)
		}
		next, gone, err := step.Convert(files)
		if err != nil {
			return nil, nil, fmt.Errorf("migrate %s → %s: %w", step.From, step.To, err)
		}
		for _, p := range gone {
			obsolete[p] = true
		}
		files, version = next, step.To
		if current, err = parseSemver(version); err != nil {
			return nil, nil, err
		}
	}
	return files, slices.Sorted(maps.Keys(obsolete)), nil
}

// nextStep is the converter leading on from version without passing target.
func nextStep(steps []converter, version string, target [3]int) (converter, bool) {
	for _, s := range steps {
		if s.From != version {
			continue
		}
		if to, err := parseSemver(s.To); err == nil && compareSemver(to, target) <= 0 {
			return s, true
		}
	}
	return converter{}, false
}

func parseSemver(v string) ([3]int, error) {
	var out [3]int
	parts := strings.Split(v, ".")
	if len(parts) != len(out) {
		return out, fmt.Errorf("migrate: %q is not a major.minor.patch version", v)
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || strconv.Itoa(n) != p {
			return out, fmt.Errorf("migrate: %q is not a major.minor.patch version", v)
		}
		out[i] = n
	}
	return out, nil
}

func compareSemver(a, b [3]int) int { return slices.Compare(a[:], b[:]) }
