package kb

import (
	"errors"
	"fmt"
)

const (
	// FormatVersion is the one metadata format version kbase reads and writes.
	// The patch above the scoring vocabulary is the ladder's last rung's words:
	// the bytes a writer produces changed, their meaning did not.
	FormatVersion = "2.0.1"
	// FormatKey is the stamp's key in the entry point's frontmatter.
	FormatKey = "kb-format"
)

// ErrNoEntryPoint is the loader's error for a kb-root with no entry point.
var ErrNoEntryPoint = errors.New("kb-root has no " + EntryPointFile)

// FormatStamp is the kb-format stamp in an entry point's YAML frontmatter. An
// entry point carrying no stamp, and a stamp that is not one non-empty string,
// is an error.
func FormatStamp(entryPoint string) (string, error) {
	fm, err := ParseFrontmatter(entryPoint)
	if err != nil {
		return "", err
	}
	v, stamped := fm[FormatKey]
	switch {
	case !stamped:
		return "", fmt.Errorf("%s carries no %s", EntryPointFile, FormatKey)
	case v.IsList || v.IsBool || v.Str == "":
		return "", fmt.Errorf("%s in %s is not a version", FormatKey, EntryPointFile)
	}
	return v.Str, nil
}
