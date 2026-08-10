// Package ingest is the deterministic head of the pipeline
// (ARCHITECTURE.md §4, stage 1): it walks a documentation corpus and takes
// custody of the source bytes every later stage refers to.
//
// It holds no format knowledge: which extensions are documents is a
// parameter (Walk), supplied by the caller's format adapter. Custody is
// byte-level and format-blind, so the same walk serves a Markdown corpus and
// a LaTeX one.
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

	"kbase/internal/log"
)

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
//
// The directory the corpus was read from is deliberately not carried. An
// absolute path is machine state — putting it in a derived artifact makes the
// same corpus produce different bytes on two checkouts — and nothing
// downstream needs it, because every id here is already relative.
type Corpus struct {
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

	// folded maps a case-folded path to every unit that folds to it. It
	// exists because the walk matches extensions case-insensitively — it
	// accepts `.MD` — while a link to that document is routinely written
	// `.md`; without the fold, accepting the file and resolving links to it
	// disagree. It is a lookup aid only — the id itself stays byte-exact.
	folded map[string][]int
}

// Has reports whether the corpus contains a document at the given
// slash-separated relative path.
func (c Corpus) Has(path string) bool {
	_, ok := c.index[path]
	return ok
}

// FoldedPath resolves a slash-separated path case-insensitively to the
// byte-exact id of the one document that folds to it.
//
// ambiguous is true when SEVERAL documents fold to the same key: a corpus
// holding both `Setup.md` and `setup.MD` cannot say which one a link to
// `setup.md` meant, and a guess would hand a later stage a silently wrong
// edge. Callers treat that as unresolved and say so out loud.
func (c Corpus) FoldedPath(path string) (id string, ambiguous bool) {
	switch is := c.folded[foldPath(path)]; len(is) {
	case 0:
		return "", false
	case 1:
		return c.Units[is[0]].Path, false
	default:
		return "", true
	}
}

// foldPath is the case-fold key: lowercasing, applied to the whole path,
// extension included. The variance this exists for is a Windows-authored
// `.MD` or a hand-typed `Setup.md`, not a Unicode special-casing rule.
func foldPath(path string) string { return strings.ToLower(path) }

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
func New(units []Unit) (Corpus, error) {
	sorted := slices.Clone(units)
	slices.SortFunc(sorted, func(a, b Unit) int { return strings.Compare(a.Path, b.Path) })

	index := make(map[string]int, len(sorted))
	folded := make(map[string][]int, len(sorted))
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
		// Every unit is recorded under its fold key, collisions included:
		// dropping one would make the corpus quietly answer for a document
		// it holds two of. See FoldedPath.
		key := foldPath(u.Path)
		folded[key] = append(folded[key], i)

		sum := sha256.Sum256(u.Bytes)
		u.SHA256 = hex.EncodeToString(sum[:])
		fmt.Fprintf(corpusDigest, "%s\x00%s\n", u.Path, u.SHA256)
	}

	return Corpus{
		Units:       sorted,
		ContentHash: hex.EncodeToString(corpusDigest.Sum(nil)),
		index:       index,
		folded:      folded,
	}, nil
}

// maxCorpusBytes bounds what one ingest reads into memory. The largest
// intended target is tens of megabytes of prose, so the cap is provisional
// headroom rather than a tuned figure — its job is to turn a mistyped root
// pointed at a media tree or a whole home directory into the loud refusal
// this package prefers everywhere else, instead of an OOM kill with nothing
// to read afterwards. There is deliberately no override flag: nothing has
// asked for one, and a limit with an escape hatch is a limit nobody reads.
const maxCorpusBytes = 256 << 20 // 256 MB

// Walk ingests every document under root whose name carries one of exts.
//
// exts is the format adapter's document-extension set (markdown.Extensions()
// today). It is a parameter rather than a constant because "which files are
// documents" is the one piece of format knowledge a walk needs, and this
// package is not where format knowledge lives. Matching is case-insensitive:
// `.MD` off a Windows-authored tree is the same document.
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
//     elsewhere is legitimate. A symlinked DIRECTORY is refused: filepath
//     .WalkDir does not follow one, so refusing is the only alternative to
//     silently omitting a directory that may hold documents — and the same
//     document reachable at two paths would break the path-is-identity rule
//     the whole pipeline rests on.
//   - An unreadable file fails the ingest when it could be a document — a
//     knowledge base silently missing a chapter is the failure mode this
//     rule exists to prevent. A broken symlink NOT named like a document
//     (`logo.png -> gone`) cannot hide a chapter, so it is a skip.
//   - A corpus with no matching document at all fails: at this point in the
//     pipeline it is a mistyped path, not an empty job. So does one over
//     maxCorpusBytes.
//
// Every skip is a debug record on lg, because "why is that file not in my
// knowledge base" is the question the policies above generate.
func Walk(root string, exts []string, lg log.Logger) (Corpus, error) {
	if len(exts) == 0 {
		return Corpus{}, fmt.Errorf("ingest: no document extensions given; " +
			"the caller's format adapter must name at least one")
	}
	info, err := os.Stat(root)
	if err != nil {
		return Corpus{}, fmt.Errorf("ingest: corpus root: %w", err)
	}
	if !info.IsDir() {
		return Corpus{}, fmt.Errorf("ingest: corpus root %s is not a directory", root)
	}

	var units []Unit
	var total int64
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("ingest: walk %s: %w", p, err)
		}
		if p == root {
			return nil
		}
		rel, err := relPath(root, p)
		if err != nil {
			return err
		}

		name := d.Name()
		if strings.HasPrefix(name, ".") {
			lg.Debug("ingest skipping dot-prefixed entry", "path", rel, "dir", d.IsDir())
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}

		document := hasExt(name, exts)
		regular := d.Type().IsRegular()
		if d.Type()&fs.ModeSymlink != 0 {
			target, err := os.Stat(p)
			switch {
			case err != nil && !document:
				lg.Debug("ingest skipping broken non-document symlink", "path", rel, "reason", err)
				return nil
			case err != nil:
				return fmt.Errorf("ingest: %s: %w", rel, err)
			case target.IsDir():
				return fmt.Errorf("ingest: %s is a symlink to a directory; symlinked directories "+
					"are refused (a linked directory may hold documents, and one document at two paths)", rel)
			}
			regular = target.Mode().IsRegular()
		}
		if !document {
			lg.Debug("ingest skipping non-document file", "path", rel)
			return nil
		}
		if !regular {
			return fmt.Errorf("ingest: %s is not a regular file", rel)
		}

		b, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("ingest: read %s: %w", rel, err)
		}
		// Checked after the read rather than before: the bound that matters
		// is the whole corpus, and one document of overshoot is cheaper than
		// stat-ing every file to pre-compute a total that the read would then
		// produce anyway.
		total += int64(len(b))
		if total > maxCorpusBytes {
			return fmt.Errorf("ingest: corpus under %s exceeds the %d-byte in-memory limit "+
				"(reached at %s); there is no override — a tree this large is a mistyped root "+
				"far more often than a document set", root, maxCorpusBytes, rel)
		}
		units = append(units, Unit{Path: rel, Bytes: b})
		return nil
	})
	if err != nil {
		return Corpus{}, err
	}
	if len(units) == 0 {
		return Corpus{}, fmt.Errorf("ingest: no %s files under %s", strings.Join(exts, ", "), root)
	}
	return New(units)
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

// hasExt reports whether a file name carries one of the document extensions,
// case-insensitively.
func hasExt(name string, exts []string) bool {
	got := filepath.Ext(name)
	return slices.ContainsFunc(exts, func(e string) bool { return strings.EqualFold(got, e) })
}
