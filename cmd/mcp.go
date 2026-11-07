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
		return buildstore.ProjectRoot(flagProjectDir)
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

// mcpCommandName is the server's subcommand.
const mcpCommandName = "mcp"

func mcpCommand() *cobra.Command {
	var projectRequired bool
	cmd := &cobra.Command{
		Use:   mcpCommandName,
		Short: "serve the KB's subcommands as MCP tools over stdio",
		Long: `mcp serves kbase to an agent harness over the Model Context Protocol:
newline-delimited JSON-RPC on stdin and stdout. Each tool binds one
subcommand, but for write and query, which each take a family of subcommands
as operations. A tool takes the subcommand's arguments and returns its
result, as text and as structured content, except where its description
names a difference: build and score return once their run has started. A
call whose result has an exit code other than 0 comes back as an MCP error
(isError true). Three commands are served by no tool: auth, since a key never
travels through MCP; wait, since an MCP call cannot block, so a session polls
status instead; and mcp, the server itself. help and completion are the CLI
framework's own.

Every tool also takes project-dir, the project the call runs in: the absolute
path of its git worktree root, the directory holding .git, never kb-root/
itself. The server searches upward from its working directory for the
repository root, as every subcommand finds it; where it finds one, a call
naming no project-dir runs there, and project-dir is optional in every
schema. Where it finds none, or under --project-dir-required even inside a
repository, the server locates no project: project-dir is
required in every schema, and the instructions say how a session gets the
path. Startup never fails for want of a project. mcp takes no --project-dir.
stdout carries protocol frames and nothing else; diagnostics go to stderr and
--log-file. EOF on stdin ends the server.`,
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
	cmd.Flags().BoolVar(&projectRequired, "project-dir-required", false, "every tool call names its project with project-dir; the server locates no project of its own, even inside a repository")
	return cmd
}

// serveMCP serves the command tree under cmd's root. Where projectRequired,
// the server locates no project; otherwise it locates the repository that
// holds the invocation directory, and where there is none it serves as it
// would where projectRequired.
func serveMCP(cmd *cobra.Command, projectRequired bool) error {
	located := mcp.ProjectRequired()
	if !projectRequired {
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
