package main

import (
	"errors"
	"io"
	"os"

	"github.com/spf13/cobra"

	"kbase/internal/index"
	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

// queryAnswer is one query's answer over the loaded index and its positional
// arguments — a list, or show's and stats' one mapping — or the items it
// refuses.
type queryAnswer func(ix *query.Index, args []string) (answer any, refusals []toolresult.Item, err error)

type queryOptions struct {
	Verb string
	// WorkDir is where the search for the repository root starts.
	WorkDir string
	Args    []string
	Answer  queryAnswer
	// List is whether the answer is a list, returned from Offset at most
	// Limit at a time, every one where Limit is 0.
	List          bool
	Limit, Offset int
	Stdout        io.Writer
	Stderr        io.Writer
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
	var usage []toolresult.Item
	for _, f := range []struct {
		flag  string
		value int
	}{{"--limit", opts.Limit}, {"--offset", opts.Offset}} {
		if f.value < 0 {
			usage = append(usage, toolresult.Item{Check: checkUsage, Key: f.flag, Detail: f.flag + " counts results and cannot be negative"})
		}
	}
	if usage != nil {
		return refused(fields, usage...)
	}
	ix, err := query.Load(root)
	var refusal index.Refusal
	if errors.As(err, &refusal) {
		return refused(fields, refusal.Items...)
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
	case !opts.List:
		return toolresult.Done, append(fields, toolresult.Field{Key: query.ResultsKey, Value: answer})
	}
	window, count, truncated := query.Page(answer, opts.Offset, opts.Limit)
	return toolresult.Done, append(fields, toolresult.Field{Key: "count", Value: count}, toolresult.Field{Key: "offset", Value: opts.Offset},
		toolresult.Field{Key: "truncated", Value: truncated}, toolresult.Field{Key: query.ResultsKey, Value: window})
}

// answered is an answer that always succeeds.
func answered(answer any) (any, []toolresult.Item, error) { return answer, nil, nil }

// checkUsage names a refusal of the invocation itself.
const checkUsage = "usage"

const queryHelp = `
It reads kb-root/.index/ beside the repository's .git and writes nothing. Its
result is one YAML document on stdout, the answer under "` + query.ResultsKey + `" in
kb_cmd --json's names.`

const listQueryHelp = `
A list answer is returned --limit results at a time from --offset, with
"count" the whole answer's length and "truncated" whether results lie past
the ones returned: re-issue with --offset at offset plus those returned.`

// queryCommand is cmd answering over the KB found from the working
// directory; a list query takes --limit and --offset.
func queryCommand(cmd *cobra.Command, list bool, answer queryAnswer) *cobra.Command {
	var limit, offset int
	cmd.Long += "\n" + queryHelp
	if list {
		cmd.Long += "\n" + listQueryHelp
		cmd.Flags().IntVar(&limit, "limit", query.DefaultLimit, "the most results to return; 0 returns every one")
		cmd.Flags().IntVar(&offset, "offset", 0, "how many results to skip")
	}
	cmd.RunE = func(c *cobra.Command, args []string) error {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		code, err := runQuery(queryOptions{Verb: c.Name(), WorkDir: wd, Args: args, Answer: answer, List: list,
			Limit: limit, Offset: offset, Stdout: c.OutOrStdout(), Stderr: c.ErrOrStderr()})
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
