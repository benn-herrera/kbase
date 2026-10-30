package main

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"kbase/internal/buildstore"
	"kbase/internal/kb"
	"kbase/internal/mcp"
	toolresult "kbase/internal/result"
)

// invocationDir is where a verb's search for the repository root starts: the
// directory an MCP tool call names, else --project-dir, which must be a git
// worktree root, else the working directory.
func invocationDir(cmd *cobra.Command) (string, error) {
	if ctx := cmd.Context(); ctx != nil {
		if dir, ok := mcp.InvocationDir(ctx); ok {
			return dir, nil
		}
	}
	if flagProjectDir != "" {
		return buildstore.ProjectRoot(flagProjectDir, "--project-dir")
	}
	return os.Getwd()
}

// absIfSet is path made absolute against the working directory, "" staying "".
func absIfSet(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	return filepath.Abs(path)
}

func mcpCommand() *cobra.Command {
	var projectRequired bool
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "serve the KB's subcommands as MCP tools over stdio",
		Long: `mcp serves kbase to an agent harness over the Model Context Protocol:
newline-delimited JSON-RPC on stdin and stdout. Every tool is one subcommand,
with the same arguments and the same result document, as text and as
structured content, but for rewind-build, which reports the build's trail and
is no subcommand; a result whose exit code is not 0 is an error result. Every
tool also takes project-dir, the project the call runs in. Without
--project-dir-required the server locates its project from the working
directory, as every subcommand finds it, and a call naming none runs there;
with it the server locates none and every call names its project. mcp takes
no --project-dir. stdout carries protocol frames and nothing else;
diagnostics go to stderr and --log-file. EOF on stdin ends the server.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("project-dir") {
				return toolresult.Refusal{{Check: toolresult.CheckUsage, Key: "--project-dir",
					Detail: "kbase mcp takes no project; each tool call names its own with project-dir",
					Remedy: "run `kbase mcp` from inside the project, or `kbase mcp --project-dir-required` where the harness has no project working directory"}}
			}
			if err := serveMCP(cmd, projectRequired); err != nil {
				processLog.logger.Error("mcp: the server stopped", "err", err)
				return exitCode(toolresult.ExitCode(toolresult.Failed))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&projectRequired, "project-dir-required", false, "every tool call names its project with project-dir; the server locates no project of its own")
	return cmd
}

// serveMCP serves the command tree under cmd's root. Where projectRequired,
// the server locates no project; otherwise it locates the repository that
// holds the invocation directory, or none.
func serveMCP(cmd *cobra.Command, projectRequired bool) error {
	var located mcp.Located
	if projectRequired {
		located = mcp.ProjectRequired()
	} else {
		dir, err := invocationDir(cmd)
		if err != nil {
			return err
		}
		root, err := kb.RepositoryRoot(dir)
		if err != nil {
			return err
		}
		if root != "" {
			located = mcp.LocatedAt(root)
		}
	}
	srv := servedServer(cmd.Root(), located, cmd.ErrOrStderr(), processLog.logger)
	srv.Instructions = mcp.ServedInstructions(located)
	return srv.Serve(cmd.InOrStdin(), cmd.OutOrStdout())
}
