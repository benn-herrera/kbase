package main

import (
	"errors"

	"github.com/spf13/cobra"

	"kbase/internal/index"
	toolresult "kbase/internal/result"
)

func runVerify(opts maintenanceOptions) (int, error) {
	if err := requireStreams(opts.Stdout, opts.Stderr, "verify"); err != nil {
		return 0, err
	}
	outcome, fields := verify(opts)
	return emitResult("verify", opts.Stdout, outcome, fields)
}

func verify(opts maintenanceOptions) (string, []toolresult.Field) {
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
	fields = withMigration(fields, src, "verify", opts.Stderr)
	faults, err := index.Verify(src)
	var refusal toolresult.Refusal
	if errors.As(err, &refusal) {
		return refused(fields, refusal...)
	}
	if err != nil {
		return failed(fields, err)
	}
	findings, err := index.DemotedFindings(src)
	switch {
	case err != nil && len(faults) == 0:
		return failed(fields, err)
	case err != nil:
		// The faults already name what keeps the records from being read.
		findings = []toolresult.Item{}
	}
	fields = append(fields, toolresult.Field{Key: "findings", Value: findings})
	if len(faults) == 0 {
		return toolresult.Done, fields
	}
	return refused(fields, index.Items(faults)...)
}

func verifyCommand() *cobra.Command {
	return maintenanceCommand("verify",
		"check the KB's links, records, statement markers and summary pages; writes nothing",
		`verify checks the KB at kb-root/ in the git worktree root and writes
nothing. Each fault is an item under "refusals", outcome refused:
- any link under kb-root/, citations included, whose target names no file, or
  whose fragment names no heading or element id of the document it reaches,
  or no entry of a summary page (check: dead-link);
- an id in a citation's link title, or a node id in prose, that no node
  carries (check: unknown-id);
- a node id standing bare in prose, outside code, markers and citation link
  titles (check: bare-id; remedy: add-citation with the id as "snippet");
- a frontmatter block on any document (check: frontmatter);
- a KB in an older format (check: index-freshness; remedy:
  `+index.UpdateFormatRemedy+`);
- a claim, analysis or experiment carrying no statement marker or more than
  one (check: statement-marker);
- an axiom record without exactly one heading in axioms.md carrying its
  marker, a marked heading whose marker names no axiom record, or one whose
  text is not its record's label and title (check: axiom-heading);
- a summary page that does not stand as the derivation renders it (check:
  register-freshness; remedy: `+index.EmptyBatchRemedy+`, or
  `+index.UpdateFormatRemedy+` over a KB in an older format).
Any other fault is put right by a prose edit or git work, as its detail
guides. `+loadErrorRefusedHelp+` Every demoted record — an edge the build
moved out of the scoring edges to break a cycle — is listed under "findings"
(check: demoted), which are not faults. Its result: "findings", then
"refusals" where refused.`, runVerify)
}
