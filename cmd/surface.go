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
	"kbase/internal/write"
)

// readOnlyTool binds cmd as an MCP tool carrying the read-only hint.
func readOnlyTool(cmd *cobra.Command) *cobra.Command { return mcp.Bind(mcp.ReadOnly(cmd)) }

// readWriteTool binds cmd as an MCP tool that may write.
func readWriteTool(cmd *cobra.Command) *cobra.Command { return mcp.Bind(cmd) }

// cliOnly declares that no MCP tool binds cmd, for reason.
func cliOnly(cmd *cobra.Command, reason string) *cobra.Command { return mcp.Exclude(cmd, reason) }

// The family tool's name: the query tool's operations read.
const queryFamily = "query"

// queryOperation binds cmd read-only as an operation of the query tool.
func queryOperation(cmd *cobra.Command) *cobra.Command {
	return mcp.InFamily(readOnlyTool(cmd), queryFamily)
}

// writeOperation binds cmd as an op of kbase write's batch: the write tool,
// which kbase write binds, serves it, and it has no tool of its own.
func writeOperation(cmd *cobra.Command) *cobra.Command {
	return mcp.InFamily(readWriteTool(cmd), write.Batch)
}

// toolFamilies is the family tools: each one's description sentence and the
// help its operations share, in the MCP form.
func toolFamilies() []mcp.Family {
	return []mcp.Family{
		{Name: queryFamily,
			Summary: "run one read-only query over the KB: operation names it.",
			Help:    queryReads + " " + queryAnswerHelp("") + "\n\n" + listQueryHelp},
	}
}

func init() {
	// Queries: the query tool's operations.
	rootCmd.AddCommand(
		queryOperation(citationsOfCommand()),
		queryOperation(edgesCommand()),
		queryOperation(findCommand()),
		queryOperation(showCommand()),
		queryOperation(statsCommand()),
		queryOperation(traceCommand()),
	)
	// The write batch, the write tool, and the write ops it serves.
	rootCmd.AddCommand(
		readWriteTool(writeCommand()),
		writeOperation(addAnalysisCommand()),
		writeOperation(addCitationCommand()),
		writeOperation(addAxiomCommand()),
		writeOperation(addClaimCommand()),
		writeOperation(addEdgeCommand()),
		writeOperation(addExperimentCommand()),
		writeOperation(addWorkCommand()),
		writeOperation(editAnalysisCommand()),
		writeOperation(editAxiomCommand()),
		writeOperation(editClaimCommand()),
		writeOperation(editEdgeCommand()),
		writeOperation(editExperimentCommand()),
		writeOperation(editWorkCommand()),
		writeOperation(moveNodeCommand()),
		writeOperation(removeEdgeCommand()),
		writeOperation(removeNodeCommand()),
		writeOperation(restoreDemotedCommand()),
	)
	// KB maintenance.
	rootCmd.AddCommand(
		readWriteTool(exportLatexCommand()),
		readWriteTool(initCommand()),
		readWriteTool(updateFormatCommand()),
		readOnlyTool(verifyCommand()),
	)
	// Runs.
	rootCmd.AddCommand(
		readWriteTool(buildCmd),
		readWriteTool(scoreCmd),
		readOnlyTool(statusCommand()),
		readWriteTool(cancelCommand()),
		readOnlyTool(buildTrailCommand()),
		cliOnly(waitCommand(), "a tool call would hold the turn for the build's or score run's duration"),
	)
	// Configuration.
	rootCmd.AddCommand(
		readWriteTool(configureProvidersCommand()),
		readWriteTool(configureModelsCommand()),
		readOnlyTool(modelsCmd),
		readOnlyTool(showConfigCmd),
		readOnlyTool(probeCmd),
	)
	// Credentials: the store's verbs, under one parent the tool table never
	// reads below.
	authCmd := authCommand()
	authCmd.AddCommand(authAddCommand(), authRemoveCommand(), authListCommand(), authGetPipedCommand())
	rootCmd.AddCommand(cliOnly(authCmd, "a key arrives on stdin and leaves on stdout, neither of which a tool call has, and no tool accepts or returns a key"))
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
			cliOnly(c, "each bound command's help is the resource kbase://help/<command>, so a tool printing it would duplicate a resource")
		case "completion":
			cliOnly(c, "it writes a shell's completion script, which no tool caller runs")
		}
	}
}

// servedServer is the server over the command tree under root with the
// located project, or none: a write batch runs through runWriteOp, an error no verb documented is refused as exitStatus refuses it,
// build and score start detached with the server's global flags, the queries
// are the query tool's operations, and the resources are the sheets, the model
// guidance and the help of each family and bound command, in that order.
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
		ConfigWarnings: func(dir string) []toolresult.Item {
			return launchConfigWarnings(dir, stderr)
		},
		Families:  toolFamilies(),
		Resources: []mcp.ResourceFamily{mcp.SheetResources(servedKB), mcp.ModelHintsResource(), mcp.HelpResources(root)},
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
