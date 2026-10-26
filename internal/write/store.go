package write

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"kbase/internal/kb"
)

// status is what a batch of edits did.
type status int

const (
	statusWritten status = iota
	statusUnchanged
	statusRetry
)

// reasonContended is why a write that found its file changed under it did not
// happen.
const reasonContended = "contended"

// storeError is a path or a file the store will not read or write.
type storeError struct {
	subject, detail string
}

func (e *storeError) Error() string { return e.subject + ": " + e.detail }

// interruptedError is a replace that failed after earlier targets of the
// batch were committed.
type interruptedError struct {
	written []string
	failed  string
	cause   error
}

func (e *interruptedError) Error() string {
	landed := "nothing"
	if len(e.written) > 0 {
		landed = strings.Join(e.written, ", ")
	}
	return fmt.Sprintf("%s could not be replaced (%v); this batch is interrupted, not refused — already committed: %s", e.failed, e.cause, landed)
}

type outcome struct {
	status                  status
	written                 []string
	reason, subject, detail string
}

func lineTerminator(line string) string {
	body := kb.SplitLines(line)
	if len(body) == 0 {
		return line
	}
	return line[len(body[0]):]
}

// prevailingTerminator is the line break a line spliced in at start carries:
// the displaced line's, else the nearest one above, else \n.
func prevailingTerminator(parts []string, start int) string {
	if start < len(parts) {
		if t := lineTerminator(parts[start]); t != "" {
			return t
		}
	}
	for i := min(start, len(parts)) - 1; i >= 0; i-- {
		if t := lineTerminator(parts[i]); t != "" {
			return t
		}
	}
	return "\n"
}

// spliceLines replaces the line range [start, end) with lines, each emitted
// with the terminator of the line it displaces; every other line keeps its
// bytes, terminator included.
func spliceLines(document string, start, end int, lines []string) string {
	parts := kb.SplitLinesKeepEnds(document)
	if start < 0 || start > end || end > len(parts) {
		panic(fmt.Sprintf("write: line range [%d, %d) is outside a document of %d lines", start, end, len(parts)))
	}
	eol := prevailingTerminator(parts, start)
	head := slices.Clone(parts[:start])
	tail := parts[end:]
	emitted := make([]string, len(lines))
	for i, l := range lines {
		emitted[i] = l + eol
	}
	if len(head) > 0 && lineTerminator(head[len(head)-1]) == "" {
		head[len(head)-1] += eol
	}
	if len(tail) == 0 && len(emitted) > 0 && len(parts) > 0 && lineTerminator(parts[len(parts)-1]) == "" {
		last := emitted[len(emitted)-1]
		emitted[len(emitted)-1] = last[:len(last)-len(eol)]
	}
	return strings.Join(head, "") + strings.Join(emitted, "") + strings.Join(tail, "")
}

// resolveTarget resolves a kb-root-relative path and refuses one that is a
// symbolic link or does not name a file inside kb-root.
func resolveTarget(root, rel string) (string, error) {
	if strings.ContainsRune(rel, 0) {
		return "", &storeError{subject: strings.ReplaceAll(rel, "\x00", `\x00`), detail: "contains a NUL byte, which no path can hold"}
	}
	joined := filepath.Join(root, filepath.FromSlash(rel))
	if info, err := os.Lstat(joined); err == nil && info.Mode()&fs.ModeSymlink != 0 {
		return "", &kb.SymlinkError{Path: joined}
	}
	target := kb.ResolvePath(joined)
	if target == root {
		return "", &storeError{subject: rel, detail: fmt.Sprintf("resolves to the kb root %s itself, and a target must be a file inside it", root)}
	}
	if !kb.Within(root, target) {
		return "", &storeError{subject: rel, detail: fmt.Sprintf("resolves to %s which is outside %s", target, root)}
	}
	return target, nil
}

// readExact is a file's exact text as the KB reads it, refused when it is
// not UTF-8.
func readExact(src *kb.Source, path, subject string) (string, error) {
	data, err := src.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(data) {
		return "", &storeError{subject: subject, detail: "is not valid UTF-8"}
	}
	return string(data), nil
}

// onDisk is the live file's bytes, nil where it does not exist.
func onDisk(src *kb.Source, path string) (*string, error) {
	data, err := src.DiskFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s := string(data)
	return &s, nil
}

// stillMatches is whether the live file still holds what it held when it
// was read; a nil disk means it must still be absent.
func stillMatches(src *kb.Source, path string, disk *string) bool {
	now, err := onDisk(src, path)
	if err != nil {
		return false
	}
	return (now == nil) == (disk == nil) && (now == nil || *now == *disk)
}

func scoreEqual(a, b *float64) bool { return (a == nil) == (b == nil) && (a == nil || *a == *b) }
