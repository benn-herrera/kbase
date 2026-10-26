// Package kb is the kb-root model: the names KB readers key on, the paths
// leaf discovery skips, the Markdown link primitives with the dead-link gate
// built on them, and the metadata layer's readers and schema.
package kb

import (
	"errors"
	"os"
	"path/filepath"

	"kbase/internal/formats/format300"
)

// Names the KB's readers key on.
const (
	KBDir          = "kb-root"
	EntryPointFile = "entry-point.md"
	IndexFile      = "index.md"
	UplinkMarker   = "↑"
	// OwnProseTitle heads the leaf a container's own prose is lifted into.
	OwnProseTitle = "Overview"

	// IndexDir is the index's directory inside kb-root.
	IndexDir = format300.IndexDir
	// ClaimGraphFile is the claim-graph sheet, inside kb-root and each
	// volume; ClaimGraphDigestFile is the volume digest, inside kb-root.
	ClaimGraphFile       = "claim-graph.svg"
	ClaimGraphDigestFile = "claim-graph-digest.svg"

	AgentsFile         = "AGENTS.md"
	AgentsRedirectFile = "CLAUDE.md"
	AgentsRedirect     = "@" + AgentsFile
	ConventionsFile    = "CONVENTIONS.md"
	// AxiomsFile is the declaration document axioms are stated in;
	// InvariantsFile is the superseded name a converted KB may still carry,
	// which leaf discovery skips.
	AxiomsFile     = "axioms.md"
	InvariantsFile = "invariants.md"
)

// CheckSymlink is the refusal class of a KB document that is a symbolic link:
// a read meets it where the document stands, a write where it would replace it.
const CheckSymlink = "symlink"

// CheckLock is the refusal class of an operation that finds a lock held.
const CheckLock = "lock"

// CheckKBRoot is the refusal class of a KB that is not where a command looks
// for it, or has no entry point.
const CheckKBRoot = "kb-root"

// CheckUnknownID is the refusal class of an id no node record carries.
const CheckUnknownID = "unknown-id"

// RepositoryRoot is the nearest directory at or above dir holding .git — the
// directory kb-root/ sits in — or "" where there is none.
func RepositoryRoot(dir string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}

// ResolvePath is p with every symlink of its longest existing prefix resolved
// and the rest kept as written, as Python's non-strict resolve has it: p
// resolved where it resolves, else as given.
func ResolvePath(p string) string {
	p = filepath.Clean(p)
	var rest []string
	for cur := p; ; {
		if resolved, err := filepath.EvalSymlinks(cur); err == nil {
			return filepath.Join(append([]string{resolved}, rest...)...)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return p
		}
		rest = append([]string{filepath.Base(cur)}, rest...)
		cur = parent
	}
}

// ExcludeDirs are directories leaf discovery never descends into, and
// ExcludeNames files it never reads as a document.
var (
	ExcludeDirs  = map[string]bool{"session": true, IndexDir: true, "tools": true}
	ExcludeNames = map[string]bool{
		RegisterFile: true, "claim-quality-closure-roadmap.md": true, AgentsFile: true,
		AgentsRedirectFile: true, ConventionsFile: true, "README.md": true, AxiomsFile: true, InvariantsFile: true,
	}
)

// StampedFiles are the kb-root documents kbase stamps from its templates, in
// the order it stamps them: kbase owns and generates them, and a migration
// save writes them whole.
var StampedFiles = []string{AgentsFile, ConventionsFile}
