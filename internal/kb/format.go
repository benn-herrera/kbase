package kb

import (
	"errors"
	"fmt"
)

const (
	// FormatVersion is the one metadata format version kbase reads and writes.
	FormatVersion = "1.0.0"
	// UnstampedFormatVersion is the version of a KB whose entry point carries
	// no stamp.
	UnstampedFormatVersion = "0.9.0"
	// FormatKey is the stamp's key in the entry point's frontmatter.
	FormatKey = "kb-format"
)

// ErrNoEntryPoint is the loader's error for a kb-root with no entry point.
var ErrNoEntryPoint = errors.New("kb-root has no " + EntryPointFile)

// FormatStamp is the kb-format stamp in an entry point's YAML frontmatter,
// UnstampedFormatVersion where that frontmatter carries none. ok is false
// where the entry point opens with no YAML frontmatter: its form is a
// superseded one, whose stamp only the migration reads. A stamp that is not
// one non-empty string is an error.
func FormatStamp(entryPoint string) (stamp string, ok bool, err error) {
	if FindFrontmatter(entryPoint) == nil {
		return "", false, nil
	}
	fm, err := ParseFrontmatter(entryPoint)
	if err != nil {
		return "", true, err
	}
	v, stamped := fm[FormatKey]
	switch {
	case !stamped:
		return UnstampedFormatVersion, true, nil
	case v.IsList || v.IsBool || v.Str == "":
		return "", true, fmt.Errorf("%s in %s is not a version", FormatKey, EntryPointFile)
	}
	return v.Str, true, nil
}
