package kb

import (
	"errors"
	"fmt"

	"kbase/internal/formats/format300"
)

const (
	// FormatVersion is the one metadata format version kbase reads and writes:
	// the KB's nodes and edges are records in two files.
	FormatVersion = format300.Version300
	// FormatKey is the stamp's one key, in the stamp file and in the frontmatter
	// of a KB a migration has not stamped there yet.
	FormatKey = format300.FormatKey
	// FormatFile is the metadata-format stamp file, kb-root-relative.
	FormatFile = format300.FormatFile
)

// EncodeFormatStamp is the stamp file's content holding version.
func EncodeFormatStamp(version string) []byte { return format300.EncodeFormatStamp(version) }

// DecodeFormatStamp is the version the stamp file's bytes hold; anything that
// is not the one key with a non-empty string value is an error.
func DecodeFormatStamp(data []byte) (string, error) { return format300.DecodeFormatStamp(data) }

// ErrNoEntryPoint is the loader's error for a kb-root with no entry point.
var ErrNoEntryPoint = errors.New("kb-root has no " + EntryPointFile)

// FormatStamp is the kb-format stamp in an entry point's YAML frontmatter:
// the standing place of a KB a migration has not yet stamped in FormatFile,
// read forever. An entry point carrying no stamp, and a stamp that is not one
// non-empty string, is an error.
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
