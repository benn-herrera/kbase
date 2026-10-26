package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"kbase/internal/kb"
	"kbase/internal/mcp"
	toolresult "kbase/internal/result"
)

// invocationDir is where a verb's search for the repository root starts: the
// directory an MCP tool call names, else the working directory.
func invocationDir(cmd *cobra.Command) (string, error) {
	if ctx := cmd.Context(); ctx != nil {
		if dir, ok := mcp.InvocationDir(ctx); ok {
			return dir, nil
		}
	}
	return os.Getwd()
}

func mcpCommand() *cobra.Command {
	var kbRoot string
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "serve the KB's subcommands as MCP tools over stdio",
		Long: `mcp serves the KB at --kb-root to an agent harness over the Model Context
Protocol: newline-delimited JSON-RPC on stdin and stdout. Every tool is one
subcommand, with the same arguments and the same result document, as text and
as structured content, but for rewind-build, which reports the build's trail
and is no subcommand; a result whose exit code is not 0 is an error result.
Without --kb-root the KB is found from the working directory, as every
subcommand finds it. stdout carries protocol frames and nothing else;
diagnostics go to stderr and --log-file. EOF on stdin ends the server.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := invocationDir(cmd)
			if err != nil {
				return err
			}
			if kbRoot != "" {
				root, refusal, err := servedKBRoot(kbRoot)
				if err != nil {
					return err
				}
				if refusal != nil {
					outcome, fields := refused(nil, *refusal)
					code, err := emitResult("mcp", cmd.OutOrStdout(), outcome, fields)
					if err != nil {
						return err
					}
					return exitCode(code)
				}
				dir = root
			}
			if err := serveMCP(cmd, dir); err != nil {
				processLog.logger.Error("mcp: the server stopped", "err", err)
				return exitCode(toolresult.ExitCode(toolresult.Failed))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&kbRoot, "kb-root", "", "the KB to serve: its kb-root/ directory, or the repository root that holds it or will (default: the one found from the working directory)")
	return cmd
}

// servedKBRoot is the directory the tools find their KB from, given the path
// --kb-root names: a repository root, whether or not its kb-root/ exists yet,
// or that kb-root/ itself. Any other path is a usage refusal.
func servedKBRoot(path string) (string, *toolresult.Item, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", nil, err
	}
	if info, err := os.Stat(abs); err == nil && info.IsDir() {
		repo, err := kb.RepositoryRoot(abs)
		if err != nil {
			return "", nil, err
		}
		if repo != "" && (abs == repo || abs == filepath.Join(repo, kb.KBDir)) {
			return abs, nil, nil
		}
	}
	return "", &toolresult.Item{Check: checkUsage, Path: abs, Key: "--kb-root",
		Detail: fmt.Sprintf("--kb-root %s is neither a repository root nor the %s/ directory beside its .git", abs, kb.KBDir)}, nil
}

func serveMCP(cmd *cobra.Command, dir string) error {
	srv := servedServer(cmd.Root(), dir, cmd.ErrOrStderr(), processLog.logger)
	instructions, err := mcp.ServedInstructions(dir)
	if err != nil {
		return err
	}
	srv.Instructions = instructions
	return srv.Serve(cmd.InOrStdin(), cmd.OutOrStdout())
}
