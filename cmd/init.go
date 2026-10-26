package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"kbase/internal/atomicfile"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/kbdocs"
	"kbase/internal/kbload"
	"kbase/internal/ledger"
	"kbase/internal/log"
	toolresult "kbase/internal/result"
)

// initOptions is everything the creation verb takes. WorkDir is where the
// search for the repository root starts; Title is the entry point's heading,
// the repository's directory name where it is empty.
type initOptions struct {
	WorkDir string
	Title   string
	Stdout  io.Writer
	Stderr  io.Writer
	Logger  log.Logger
}

// birthSubject is the birth commit's subject: plain, never ledger.Prefix, so
// Trail() reads no trail and build's populated-kb-root/ refusal stands over
// a created KB forever.
const birthSubject = "kb-init: knowledge base created"

// birthOwned is every path the birth commit takes, git-root relative.
var birthOwned = []string{kb.KBDir}

func runInit(opts initOptions) (int, error) {
	if err := requireStreams(opts.Stdout, opts.Stderr, "init"); err != nil {
		return 0, err
	}
	outcome, fields := initKB(opts)
	return emitResult("init", opts.Stdout, outcome, fields)
}

// initKB creates the KB: refusals before any write; then, under the write
// lock, the spine, the readiness stamps and the refresh, and the birth commit
// last. The order is load-bearing: the stamp file stands before the loader
// opens it, and the stamps before the refresh, which refuses without the
// redirect. A refusal is a result.Refusal on the error channel, the same
// shape every underlying package carries its refusals.
func initKB(opts initOptions) (string, []toolresult.Field) {
	// A line break in the title would land as further lines of the entry
	// point, a child-list line among them.
	var usage []toolresult.Item
	if strings.ContainsAny(opts.Title, "\r\n") {
		usage = append(usage, toolresult.Item{Check: checkUsage, Key: "--title",
			Detail: "the title is the entry point's one heading line and cannot hold a line break"})
	}
	repo, err := kb.RepositoryRoot(opts.WorkDir)
	if err != nil {
		return failed(nil, err)
	}
	if repo == "" {
		return refused(nil, append(usage, toolresult.Item{Check: checkWorktree, Path: opts.WorkDir, Remedy: "git init",
			Detail: fmt.Sprintf("%s is not inside a git worktree; `git init` there is the operator's to run", opts.WorkDir)})...)
	}
	kbRoot := filepath.Join(repo, kb.KBDir)
	if _, err := os.Lstat(kbRoot); err == nil {
		return refused([]toolresult.Field{{Key: "kb-root", Value: kbRoot}}, append(usage, toolresult.Item{Check: checkKBRoot, Path: kbRoot,
			Detail: fmt.Sprintf("%s already exists; creation is birth-once, and what the KB holds afterwards comes from the maintenance subcommands", kbRoot)})...)
	} else if !errors.Is(err, os.ErrNotExist) {
		return failed(nil, err)
	}
	if len(usage) > 0 {
		return refused(nil, usage...)
	}
	release, outcome, items, err := holdKB(kbRoot, "re-run kbase init")
	if err != nil {
		return failed(nil, err)
	}
	if release == nil {
		return outcome, []toolresult.Field{{Key: toolresult.RefusalsKey, Value: items}}
	}
	defer release()
	// A concurrent writer may have created the KB during the wait for the
	// lock: birth-once is decided under the lock.
	if _, err := os.Lstat(kbRoot); err == nil {
		return refused([]toolresult.Field{{Key: "kb-root", Value: kbRoot}}, toolresult.Item{Check: checkKBRoot, Path: kbRoot,
			Detail: fmt.Sprintf("%s was created while this call waited on the write lock; creation is birth-once, and what the KB holds afterwards comes from the maintenance subcommands", kbRoot)})
	} else if !errors.Is(err, os.ErrNotExist) {
		return failed(nil, err)
	}
	if err := os.Mkdir(kbRoot, 0o755); err != nil {
		return failed(nil, err)
	}
	fields := []toolresult.Field{{Key: "kb-root", Value: kbRoot}}
	written, err := skeleton(opts, repo, kbRoot)
	if err != nil {
		var refusal toolresult.Refusal
		if errors.As(err, &refusal) {
			return refused(fields, refusal...)
		}
		return failed(fields, err)
	}
	commit, err := ledger.Open(opts.WorkDir, birthOwned, opts.Logger)
	if err != nil {
		return failed(fields, err)
	}
	if _, err := commit.Commit(birthSubject, ""); err != nil {
		return failed(fields, fmt.Errorf("the skeleton stands at %s uncommitted: %w", kbRoot, err))
	}
	slices.Sort(written)
	written = slices.Compact(written)
	fields = append(fields,
		toolresult.Field{Key: "written", Value: append([]string{}, written...)},
		toolresult.Field{Key: "removed", Value: []string{}})
	return toolresult.Done, fields
}

// skeleton writes the spine — the plain entry point, the format stamp and
// the empty record files — opens the KB,
// stamps the readiness documents and refreshes, and returns every path it
// wrote, kb-root-relative. Refusals and failures arrive as errors, the shape
// the underlying packages carry them in; the caller decides the outcome.
func skeleton(opts initOptions, repo, kbRoot string) ([]string, error) {
	title := opts.Title
	if title == "" {
		title = filepath.Base(repo)
	}
	if err := os.MkdirAll(filepath.Join(kbRoot, kb.IndexDir), 0o755); err != nil {
		return nil, err
	}
	var written []string
	for _, f := range []struct {
		rel  string
		data []byte
	}{{kb.EntryPointFile, []byte("# " + title + "\n")}, {kb.FormatFile, kb.EncodeFormatStamp(kb.FormatVersion)},
		{kb.GraphNodesFile, kb.EncodeNodes(kb.Graph{})}, {kb.GraphEdgesFile, kb.EncodeEdges(kb.Graph{})}} {
		if err := atomicfile.Write(filepath.Join(kbRoot, f.rel), f.data, nil); err != nil {
			return written, err
		}
		written = append(written, f.rel)
	}
	src, err := kbload.Open(kbRoot)
	if err != nil {
		return written, err
	}
	stamped, _, err := kbdocs.Stamp(src, filepath.Base(repo), kbdocs.AuthoredKBPin)
	written = append(written, stamped...)
	if err != nil {
		return written, err
	}
	refreshed, _, noDot, err := index.RefreshReporting(src, opts.Logger)
	if noDot {
		sayNoDot(opts.Stderr, "init")
	}
	return append(written, refreshed...), err
}

func initCommand() *cobra.Command {
	var title string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "create a KB empty — no LaTeX, no build",
		Long: `init creates a knowledge base at kb-root/ beside the .git of the repository
it is run in — the equivalent of git init for a KB: no LaTeX, no build,
nothing distilled. It refuses outside a git worktree, remedy git init, and
refuses any repository whose kb-root/ already exists, in any state: creation
is birth-once, and what the KB holds afterwards comes from the maintenance
subcommands — insert an entry, set a field, refresh, verify, a claim at a
time. It writes entry-point.md with --title (or the repository's
directory name) as its only heading and no children and no frontmatter — a
--title holding a line break is refused — writes the format stamp
.index/format.yaml and the record files .index/nodes.yaml and .index/edges.yaml,
holding no record, stamps AGENTS.md,
CONVENTIONS.md and the CLAUDE.md redirect from their templates, and
refreshes the skeleton so the claim-graph sheet stands at birth — the
placeholder sheet where Graphviz dot is absent.
Finally it commits kb-root/ as the birth commit, "kb-init: knowledge base
created" — a plain commit, never a kb-build: stage commit, so kbase build,
which resumes from the build's stage trail, has no trail here and stays
refused against this KB. Its result is one YAML document on stdout.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := invocationDir(cmd)
			if err != nil {
				return err
			}
			code, err := runInit(initOptions{WorkDir: wd, Title: title, Stdout: cmd.OutOrStdout(), Stderr: cmd.ErrOrStderr(), Logger: processLog.logger})
			if err != nil {
				return err
			}
			if code != 0 {
				return exitCode(code)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "the KB's title, as its entry point's heading (default: the repository's directory name)")
	return cmd
}
