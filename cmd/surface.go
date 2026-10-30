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

// The family tools' names: the query tool's operations read, the write
// tool's write.
const (
	queryFamily = "query"
	writeFamily = "write"
)

// queryOperation binds cmd read-only as an operation of the query tool.
func queryOperation(cmd *cobra.Command) *cobra.Command {
	return mcp.InFamily(readOnlyTool(cmd), queryFamily)
}

// writeOperation binds cmd as an operation of the write tool, which may write.
func writeOperation(cmd *cobra.Command) *cobra.Command {
	return mcp.InFamily(readWriteTool(cmd), writeFamily)
}

// valuesArguments is how a values op takes its arguments over MCP, where
// valuesHelp's stdin and --values do not reach.
const valuesArguments = "arguments is one entry: the op's keys."

// toolFamilies is the family tools: each one's description sentence and the
// help its operations share, in the MCP form. render-citation, the query
// tool's one values operation, is why its help says how a values op takes its
// arguments.
func toolFamilies() []mcp.Family {
	return []mcp.Family{
		{Name: queryFamily,
			Summary: "Runs one read-only query over the KB, or render-citation: operation names it.",
			Help: queryReads + " " + queryAnswerHelp("") +
				"\n\n" + listQueryHelp("limit", "offset") + "\n\nFor render-citation, " + valuesArguments +
				" Its values flag does not apply."},
		{Name: writeFamily,
			Summary: "Runs one write op over the KB: operation names it.",
			Help: valuesArguments + " A call writes all of it or none; the op's values and no-refresh flags do " +
				"not apply, and a refresh always follows."},
	}
}

func init() {
	// Queries, and render-citation: the query tool's operations.
	rootCmd.AddCommand(
		queryOperation(citedByCommand()),
		queryOperation(depsCommand()),
		queryOperation(edgesCommand()),
		queryOperation(findCommand()),
		queryOperation(gatedOnCommand()),
		queryOperation(referencedByCommand()),
		queryOperation(renderCitationCommand()),
		queryOperation(showCommand()),
		queryOperation(statsCommand()),
		queryOperation(strengthCommand()),
		queryOperation(subtreeCommand()),
	)
	// Write ops: the write tool's operations.
	rootCmd.AddCommand(
		writeOperation(addPairsCommand()),
		writeOperation(addRestsOnCommand()),
		writeOperation(dropClaimEdgesCommand()),
		writeOperation(dropClaimNodeCommand()),
		writeOperation(dropInboundEdgesCommand()),
		writeOperation(insertAnalysisEntryCommand()),
		writeOperation(insertAxiomEntryCommand()),
		writeOperation(insertClaimEntryCommand()),
		writeOperation(insertExperimentEntryCommand()),
		writeOperation(insertWorkEntryCommand()),
		writeOperation(markClaimInLeafCommand()),
		writeOperation(resolveDemotedCommand()),
		writeOperation(setBearingCommand()),
		writeOperation(setRationaleCommand()),
		writeOperation(setRigorCommand()),
		writeOperation(setStatusCommand()),
	)
	// KB maintenance.
	rootCmd.AddCommand(
		readWriteTool(exportLatexCommand()),
		readWriteTool(initCommand()),
		readWriteTool(refreshCommand()),
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
			cliOnly(c, "each bound command's help is the resource kbase://help/<command>, so a tool printing it would duplicate a resource")
		case "completion":
			cliOnly(c, "it writes a shell's completion script, which no tool caller runs")
		}
	}
}

// servedServer is the server over the command tree under root with the
// located project, or none: a write op runs through runWriteOp, an error no
// verb documented is refused as exitStatus refuses it, build and score start
// detached with the server's global flags, the queries and the write ops are
// the family tools' operations, rewind-build stands beside the derived tools,
// and the resources are the sheets, the model guidance, the build trail, the
// build's state and the help of each family and bound command, in that order.
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
		Families:    toolFamilies(),
		Resources: []mcp.ResourceFamily{mcp.SheetResources(servedKB), mcp.ModelHintsResource(),
			mcp.BuildTrailResource(), mcp.BuildResource(), mcp.HelpResources(root)},
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
