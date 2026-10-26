// Package formats converts a KB's metadata files from an older format version
// to the current one in memory: the files as bytes in, the next form's files
// as bytes out, plus the paths the conversion made obsolete. It touches no file.
// Each format and each conversion step is a package beneath it — the live
// format, format300, among them — and the tree imports nothing of kbase outside
// itself, so maintaining a live package never changes what an old KB converts
// to.
package formats

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"kbase/internal/formats/common"
	"kbase/internal/formats/format100"
	"kbase/internal/formats/format200"
	"kbase/internal/formats/format300"
	"kbase/internal/formats/from100to200"
	"kbase/internal/formats/from200to300"
)

type (
	Files           = common.Files
	Warning         = common.Warning
	ConversionError = from100to200.ConversionError
	// AnnotationError and MalformedError are the 2.0.x load's refusals, which
	// a conversion from 2.0.x returns unchanged.
	AnnotationError = format200.AnnotationError
	MalformedError  = format200.MalformedError
)

// converter is one step from format version From to To. Convert returns the
// files in To's form — a file the step renames appears under its new path
// only — the input paths that form no longer has, and what the step dropped
// or changed that its output does not show.
type converter struct {
	From, To string
	Convert  func(Files) (Files, []string, []Warning, error)
}

var converters = []converter{
	{From: format100.Version100, To: format200.Version200, Convert: unwarned(from100to200.Convert100To200)},
	{From: format200.Version200, To: format300.Version300, Convert: from200to300.Convert200To300},
}

// unwarned is a step that reports no warnings.
func unwarned(convert func(Files) (Files, []string, error)) func(Files) (Files, []string, []Warning, error) {
	return func(files Files) (Files, []string, []Warning, error) {
		out, gone, err := convert(files)
		return out, gone, nil, err
	}
}

// Chain converts files from version from to version to through the
// registered converters in version order. One converter covers every patch of
// its minor, so from's patch is not read. The obsolete paths are the union of
// every step's, sorted; the warnings are every step's, in step order. A
// version no converter leads on from is an error naming it, unless only its
// patch differs from the form the last step produces: a patch changes the
// bytes a writer produces and not a file's form, so nothing converts to one and
// the next save stamps it (SPEC §10).
func Chain(from, to string, files Files) (Files, []string, []Warning, error) {
	return chain(converters, from, to, files)
}

func chain(steps []converter, from, to string, files Files) (Files, []string, []Warning, error) {
	target, err := ParseVersion(to)
	if err != nil {
		return nil, nil, nil, err
	}
	current, err := ParseVersion(from)
	if err != nil {
		return nil, nil, nil, err
	}
	if CompareMinor(current, target) > 0 {
		return nil, nil, nil, fmt.Errorf("migrate: %s is newer than %s, and nothing downgrades", from, to)
	}
	current[2] = 0
	obsolete := map[string]bool{}
	var warnings []Warning
	for version := current.String(); compareSemver(current, target) < 0; {
		step, ok := nextStep(steps, version, target)
		if !ok {
			// A patch above the form the last step produces converts nothing.
			if current[0] == target[0] && current[1] == target[1] {
				break
			}
			return nil, nil, nil, fmt.Errorf("migrate: no converter leads from %s toward %s", version, to)
		}
		next, gone, warned, err := step.Convert(files)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("migrate %s → %s: %w", step.From, step.To, err)
		}
		for _, p := range gone {
			obsolete[p] = true
		}
		warnings = append(warnings, warned...)
		files, version = next, step.To
		if current, err = ParseVersion(version); err != nil {
			return nil, nil, nil, err
		}
	}
	return files, slices.Sorted(maps.Keys(obsolete)), warnings, nil
}

// nextStep is the converter leading on from version without passing target.
func nextStep(steps []converter, version string, target Version) (converter, bool) {
	for _, s := range steps {
		if s.From != version {
			continue
		}
		if to, err := ParseVersion(s.To); err == nil && compareSemver(to, target) <= 0 {
			return s, true
		}
	}
	return converter{}, false
}

// Version is a format version's major, minor and patch.
type Version [3]int

// ParseVersion reads v as major.minor.patch, each part a decimal with no
// leading zero.
func ParseVersion(v string) (Version, error) {
	var out Version
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

func (v Version) String() string { return fmt.Sprintf("%d.%d.%d", v[0], v[1], v[2]) }

// CompareMinor orders two versions by major and minor alone: a patch changes
// the bytes a writer produces, never what they mean.
func CompareMinor(a, b Version) int { return slices.Compare(a[:2], b[:2]) }

func compareSemver(a, b Version) int { return slices.Compare(a[:], b[:]) }
