// Package ingest is the deterministic head of the pipeline
// (ARCHITECTURE.md §4, stage 1): it walks a documentation corpus and takes
// custody of the source bytes every later stage refers to.
//
// Custody is the whole point. The bytes a Unit carries are the canonical
// source: the survey records byte offsets into them, the dissector slices
// them by verified offsets (§5), and nothing in between rewrites, re-encodes,
// or normalizes them. A stage that wants a transformed view derives an
// overlay; it never replaces the custody bytes, because the moment two
// versions of a document exist the offset discipline stops meaning anything.
//
// Everything here is offline and deterministic: the same tree produces the
// same Corpus, hash included.
package ingest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// MarkdownExt is the only extension this adapter ingests. Matching is
// case-insensitive (`.MD` off a Windows-authored tree is the same document);
// `.markdown` and friends are deliberately not accepted — a corpus that mixes
// extensions should say so, and a silent near-miss is worse than a loud
// absence.
//
// It is exported because link resolution needs the same constant: a corpus
// link written without an extension names a document only if this is the
// extension that document carries.
const MarkdownExt = ".md"

// Unit is one source document under custody: its corpus-wide identity, its
// exact bytes, and their digest.
type Unit struct {
	// Path is the slash-separated path relative to the corpus root. It is
	// the corpus-wide id — link resolution, the survey artifact, and the
	// taxonomy all name documents this way, so the id is stable across the
	// operating systems a corpus is checked out on.
	Path string

	// Bytes is the file exactly as it was read. Callers read it and index
	// into it; nobody writes to it.
	Bytes []byte

	// SHA256 is the hex digest of Bytes, filled in by New.
	SHA256 string
}

// Corpus is an ingested document set: every unit, in path order, plus the
// identity of the set as a whole.
type Corpus struct {
	// Root is the directory the corpus was read from. It is deliberately
	// absent from every derived artifact — an absolute path is machine
	// state, and putting it in an artifact makes the same corpus produce
	// different bytes on two checkouts.
	Root string

	// Units are the source documents, sorted by Path.
	Units []Unit

	// ContentHash is the corpus-level source identity used by the
	// provenance stamp (ARCHITECTURE.md §3) when the corpus is not a git
	// checkout. See New for how it is derived.
	ContentHash string

	// index maps Path to a position in Units, so link resolution is a map
	// lookup rather than a scan per link. Unexported and never marshalled:
	// map iteration order would be a determinism hazard in an artifact.
	index map[string]int
}

// Has reports whether the corpus contains a document at the given
// slash-separated relative path.
func (c Corpus) Has(path string) bool {
	_, ok := c.index[path]
	return ok
}

// Unit returns the unit at the given slash-separated relative path.
func (c Corpus) Unit(path string) (Unit, bool) {
	i, ok := c.index[path]
	if !ok {
		return Unit{}, false
	}
	return c.Units[i], true
}

// New assembles a Corpus from units that already have Path and Bytes set: it
// sorts them by path, digests each one, and derives the corpus content hash.
// It is the only constructor, so hashing has exactly one implementation, and
// it is the seam tests build in-memory corpora through.
//
// ContentHash is the sha256 of the "<path>\x00<per-file hex digest>\n" lines
// in path order. Deriving it from the per-file digests rather than from the
// concatenated bytes means it changes when a file is renamed or removed, not
// only when content changes — a rename reorganizes a knowledge base even
// though no byte of any document moved.
func New(root string, units []Unit) (Corpus, error) {
	sorted := slices.Clone(units)
	slices.SortFunc(sorted, func(a, b Unit) int { return strings.Compare(a.Path, b.Path) })

	index := make(map[string]int, len(sorted))
	corpusDigest := sha256.New()
	for i := range sorted {
		u := &sorted[i]
		if u.Path == "" {
			return Corpus{}, fmt.Errorf("ingest: unit %d has no path", i)
		}
		if _, dup := index[u.Path]; dup {
			return Corpus{}, fmt.Errorf("ingest: duplicate source path %q", u.Path)
		}
		index[u.Path] = i

		sum := sha256.Sum256(u.Bytes)
		u.SHA256 = hex.EncodeToString(sum[:])
		fmt.Fprintf(corpusDigest, "%s\x00%s\n", u.Path, u.SHA256)
	}

	return Corpus{
		Root:        root,
		Units:       sorted,
		ContentHash: hex.EncodeToString(corpusDigest.Sum(nil)),
		index:       index,
	}, nil
}

// WalkMarkdown ingests every Markdown document under root.
//
// It reads the whole corpus into memory. That is the right trade for a batch
// appliance whose largest target is tens of megabytes of prose, and it is
// what lets the rest of the pipeline treat source as a stable byte array
// rather than re-reading (and re-deciding what a file contained) per stage.
//
// Policies, all loud rather than quiet:
//
//   - Dot-prefixed entries are skipped, directories and files alike: `.git`
//     and `.github` are tooling, not documentation.
//   - A file symlink is followed — a corpus that assembles itself from
//     elsewhere is legitimate. A symlinked DIRECTORY is refused, because
//     following it invites a cycle and a corpus that never finishes walking,
//     and because the same document reachable at two paths breaks the
//     path-is-identity rule the whole pipeline rests on.
//   - An unreadable file fails the ingest. A knowledge base silently missing
//     a chapter is the failure mode this rule exists to prevent.
//   - A corpus with no Markdown at all fails: at this point in the pipeline
//     it is a mistyped path, not an empty job.
func WalkMarkdown(root string) (Corpus, error) {
	info, err := os.Stat(root)
	if err != nil {
		return Corpus{}, fmt.Errorf("ingest: corpus root: %w", err)
	}
	if !info.IsDir() {
		return Corpus{}, fmt.Errorf("ingest: corpus root %s is not a directory", root)
	}

	var units []Unit
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("ingest: walk %s: %w", p, err)
		}
		name := d.Name()
		if p != root && strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}

		rel, err := relPath(root, p)
		if err != nil {
			return err
		}

		regular := d.Type().IsRegular()
		if d.Type()&fs.ModeSymlink != 0 {
			target, err := os.Stat(p)
			if err != nil {
				return fmt.Errorf("ingest: %s: %w", rel, err)
			}
			if target.IsDir() {
				return fmt.Errorf("ingest: %s is a symlink to a directory; "+
					"symlinked directories are refused (link cycles, and one document at two paths)", rel)
			}
			regular = target.Mode().IsRegular()
		}
		if !isMarkdown(name) {
			return nil
		}
		if !regular {
			return fmt.Errorf("ingest: %s is not a regular file", rel)
		}

		b, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("ingest: read %s: %w", rel, err)
		}
		units = append(units, Unit{Path: rel, Bytes: b})
		return nil
	})
	if err != nil {
		return Corpus{}, err
	}
	if len(units) == 0 {
		return Corpus{}, fmt.Errorf("ingest: no %s files under %s", MarkdownExt, root)
	}
	return New(root, units)
}

// relPath renders p as the corpus-wide id: relative to root, slash-separated
// whatever the host separator is.
func relPath(root, p string) (string, error) {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return "", fmt.Errorf("ingest: relative path for %s: %w", p, err)
	}
	return filepath.ToSlash(rel), nil
}

// isMarkdown reports whether a file name carries the Markdown extension,
// case-insensitively.
func isMarkdown(name string) bool {
	return strings.EqualFold(filepath.Ext(name), MarkdownExt)
}
