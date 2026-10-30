package main

import (
	"bytes"
	"io"

	"github.com/spf13/cobra"

	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/mcp"
	toolresult "kbase/internal/result"
	"kbase/internal/version"
)

// readOnlyTool binds cmd as an MCP tool carrying the read-only hint.
func readOnlyTool(cmd *cobra.Command) *cobra.Command { return mcp.Bind(mcp.ReadOnly(cmd)) }

// readWriteTool binds cmd as an MCP tool that may write.
func readWriteTool(cmd *cobra.Command) *cobra.Command { return mcp.Bind(cmd) }

// cliOnly declares that no MCP tool binds cmd, for reason.
func cliOnly(cmd *cobra.Command, reason string) *cobra.Command { return mcp.Exclude(cmd, reason) }

func init() {
	// Queries.
	rootCmd.AddCommand(
		readOnlyTool(citedByCommand()),
		readOnlyTool(depsCommand()),
		readOnlyTool(edgeCensusCommand()),
		readOnlyTool(edgesCommand()),
		readOnlyTool(findCommand()),
		readOnlyTool(gatedOnCommand()),
		readOnlyTool(referencedByCommand()),
		readOnlyTool(showCommand()),
		readOnlyTool(statsCommand()),
		readOnlyTool(strengthBelowCommand()),
		readOnlyTool(subtreeCommand()),
		readOnlyTool(weakPointsCommand()),
	)
	// Write ops.
	rootCmd.AddCommand(
		readWriteTool(addPairsCommand()),
		readWriteTool(addRestsOnCommand()),
		readWriteTool(dropClaimEdgesCommand()),
		readWriteTool(dropClaimNodeCommand()),
		readWriteTool(dropInboundEdgesCommand()),
		readWriteTool(insertAnalysisEntryCommand()),
		readWriteTool(insertAxiomEntryCommand()),
		readWriteTool(insertClaimEntryCommand()),
		readWriteTool(insertExperimentEntryCommand()),
		readWriteTool(insertWorkEntryCommand()),
		readWriteTool(markClaimInLeafCommand()),
		readOnlyTool(renderCitationCommand()),
		readWriteTool(resolveDemotedCommand()),
		readWriteTool(setBearingCommand()),
		readWriteTool(setRationaleCommand()),
		readWriteTool(setRigorCommand()),
		readWriteTool(setStatusCommand()),
	)
	// KB maintenance.
	rootCmd.AddCommand(
		readWriteTool(exportLatexCommand()),
		readWriteTool(initCommand()),
		readWriteTool(refreshCommand()),
		readWriteTool(renderClaimGraphCommand()),
		readOnlyTool(verifyCommand()),
	)
	// Runs.
	rootCmd.AddCommand(
		readWriteTool(buildCmd),
		readWriteTool(scoreCmd),
		readOnlyTool(statusCommand()),
		readWriteTool(cancelCommand()),
		cliOnly(waitCommand(), "a tool call would hold the turn for the build's or score run's duration"),
	)
	// Configuration.
	rootCmd.AddCommand(
		readWriteTool(configureCmd),
		readOnlyTool(modelsCmd),
		readOnlyTool(showConfigCmd),
		cliOnly(probeCmd, "a diagnostic whose tool would let an agent poll a model at will"),
	)
	// Server.
	rootCmd.AddCommand(cliOnly(mcpCommand(), "it is the server the tools are served by"))

	// cobra adds help and completion at Execute unless the tree already holds
	// them; adding them here, once the tree holds a command, lets them declare
	// their binding as every other command does.
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultCompletionCmd()
	for _, c := range rootCmd.Commands() {
		switch c.Name() {
		case "help":
			cliOnly(c, "a tool's schema and description already carry what help prints for a terminal")
		case "completion":
			cliOnly(c, "it writes a shell's completion script, which no tool caller runs")
		}
	}
}

// servedServer is the server over the command tree under root with the
// located project, or none: a write op runs through runWriteOp, an error no
// verb documented is refused as exitStatus refuses it, build and score start
// detached with the server's global flags, rewind-build stands beside the
// derived tools, and the resources are the sheets, the model guidance, the
// build trail and the build's state, in that order.
func servedServer(root *cobra.Command, located mcp.Located, stderr io.Writer, lg log.Logger) mcp.Server {
	return mcp.Server{
		Version: version.Current,
		Root:    root,
		Located: located,
		Stderr:  stderr,
		Logger:  lg,
		Globals: mcp.Globals{ConfigDir: flagConfigDir, LogLevel: flagLogLevel, LogFile: flagLogFile},
		WriteOp: func(dir, op string, values []byte, out io.Writer) error {
			_, err := runWriteOp(writeOpOptions{Op: op, WorkDir: dir, Stdin: bytes.NewReader(values),
				ArgumentEntry: true, Stdout: out, Stderr: stderr, Logger: lg})
			return err
		},
		RenderError: func(err error, out, stderr io.Writer) { exitStatus(err, out, stderr) },
		Launches:    map[*cobra.Command]mcp.Launch{buildCmd: mcp.BuildLaunch(), scoreCmd: mcp.ScoreLaunch()},
		Listed:      []mcp.ProjectTool{mcp.RewindBuildTool()},
		Resources: []mcp.ResourceFamily{mcp.SheetResources(servedKB), mcp.ModelHintsResource(),
			mcp.BuildTrailResource(), mcp.BuildResource()},
	}
}

// servedKB is the KB found from dir for the sheet resources: none where no KB
// is found yet, and the loader's refusal where the KB found will not open.
func servedKB(dir string) (*kb.Source, error) {
	root, items, err := kbRootFrom(dir)
	if err != nil || items != nil {
		return nil, err
	}
	src, items, err := openKB(root)
	if items != nil {
		return nil, toolresult.Refusal(items)
	}
	return src, err
}
