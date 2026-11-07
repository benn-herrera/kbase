package main

import (
	"errors"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"kbase/internal/argmark"
	"kbase/internal/index"
	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

// queryAnswer is one query's answer over the loaded index and its positional
// arguments — a list or a mapping — or the items it
// refuses.
type queryAnswer func(ix *query.Index, args []string) (answer any, refusals []toolresult.Item, err error)

type queryOptions struct {
	Verb string
	// WorkDir is where the search for the repository root starts.
	WorkDir string
	Args    []string
	Answer  queryAnswer
	// List is whether the answer is a list of records returned whole, its
	// count before it.
	List   bool
	Stdout io.Writer
	Stderr io.Writer
}

func runQuery(opts queryOptions) (int, error) {
	if err := requireStreams(opts.Stdout, opts.Stderr, opts.Verb); err != nil {
		return 0, err
	}
	outcome, fields := answerQuery(opts)
	return emitResult(opts.Verb, opts.Stdout, outcome, fields)
}

func answerQuery(opts queryOptions) (string, []toolresult.Field) {
	root, refusals, err := kbRootFrom(opts.WorkDir)
	if err != nil {
		return failed(nil, err)
	}
	if refusals != nil {
		return refused(nil, refusals...)
	}
	fields := []toolresult.Field{{Key: "kb-root", Value: root}}
	src, refusals, err := openKB(root)
	if err != nil {
		return failed(fields, err)
	}
	if refusals != nil {
		return refused(fields, refusals...)
	}
	fields = withMigration(fields, src, opts.Verb, opts.Stderr)
	ix, err := query.Load(src)
	var refusal toolresult.Refusal
	if errors.As(err, &refusal) {
		return refused(fields, refusal...)
	}
	if err != nil {
		return failed(fields, err)
	}
	answer, items, err := opts.Answer(ix, opts.Args)
	switch {
	case err != nil:
		return failed(fields, err)
	case items != nil:
		return refused(fields, items...)
	case opts.List:
		return toolresult.Done, append(fields, toolresult.Field{Key: "count", Value: len(answer.([]toolresult.Record))},
			toolresult.Field{Key: query.ResultsKey, Value: answer})
	}
	return toolresult.Done, append(fields, toolresult.Field{Key: query.ResultsKey, Value: answer})
}

// answered is an answer that always succeeds.
func answered(answer any) (any, []toolresult.Item, error) { return answer, nil, nil }

// queryReads, queryHelp, queryAnswerHelp and listQueryHelp are what the
// queries share, never stated in a query's own help: the root command's help
// carries the CLI's form, and the query tool's help resource (toolFamilies)
// the MCP form, from the same text where the surfaces agree.
var queryReads = `A query reads the KB under kb-root/ beside the repository's .git and writes
nothing; a KB in an older format answers as converted, its result carrying
"migration"; every write but ` + index.UpdateFormatRemedy + ` is refused until that
converts and re-derives it; a KB whose load fails is refused, naming each
fault with its file and record. A rigor, bearing, strength or contribution
that reads *pending* is unscored, not low.`

// queryHelp is queryReads as the CLI states it.
var queryHelp = queryReads + "\n" + queryAnswerHelp(" on stdout")

// queryAnswerHelp is where a query's result puts its answer, where naming the
// channel the result arrives on as the surface reading it states it.
func queryAnswerHelp(where string) string {
	return "A query's result is one YAML document" + where + ", the answer under \"" + query.ResultsKey + "\"."
}

// listQueryHelp is how a list answer sorts and limits, each argument marked,
// so the surface reading it spells it.
var listQueryHelp = `find, edges, trace and citations-of return their complete answers, "count"
beside each; edges and citations-of keep their fixed orders. find and trace
sort: ` + argmark.Flag("sort") + ` names the key each sorts by and ` + argmark.Flag("order") + ` the direction,
ascending (the default) or descending; a pending value sorts below every
number. ` + argmark.Flag("limit") + ` N, allowed only beside an explicit ` + argmark.Flag("sort") + ` and never 0, keeps
the first N of that order, or for a negative N the last N, still in that
order.`

// sortOrderUsage is the --order flag's usage on every query that sorts.
var sortOrderUsage = "the sort's order: " + strings.Join(query.Orders, " or ")

// sortLimitUsage is the --limit flag's usage on every query that sorts.
var sortLimitUsage = "only beside an explicit " + argmark.Flag("sort") +
	" and never 0: keep the first N of its order, or the last N for a negative N"

// queryCommand is cmd answering over the KB found from the working directory
// with one mapping.
func queryCommand(cmd *cobra.Command, answer queryAnswer) *cobra.Command {
	return withQueryRun(cmd, answer, false)
}

// completeListCommand is cmd answering over the KB found from the working
// directory with a list of records returned whole, its count before it.
func completeListCommand(cmd *cobra.Command, answer queryAnswer) *cobra.Command {
	return withQueryRun(cmd, answer, true)
}

// withQueryRun is cmd running answer as a query, a list where list.
func withQueryRun(cmd *cobra.Command, answer queryAnswer, list bool) *cobra.Command {
	cmd.RunE = func(c *cobra.Command, args []string) error {
		wd, err := invocationDir(c)
		if err != nil {
			return err
		}
		code, err := runQuery(queryOptions{Verb: c.Name(), WorkDir: wd, Args: args, Answer: answer, List: list,
			Stdout: c.OutOrStdout(), Stderr: c.ErrOrStderr()})
		if err != nil {
			return err
		}
		if code != 0 {
			return exitCode(code)
		}
		return nil
	}
	return cmd
}
