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
		"check the KB's links, records, statement markers and register pages; writes nothing",
		`verify checks the KB's freshness and links at kb-root/ beside the
repository's .git: the dead-link and unknown-id gate over kb-root/, then the
metadata gate — a frontmatter block on any document but the entry point, a
claim, analysis or experiment carrying other than one statement marker, an
axiom record and axioms.md's headings that disagree, each fault of a degraded
load (`+degradedFaults+`), and each register page that does not stand as
refresh renders it. The citation grammar is checked by the build, not here. It
writes nothing. Faults are outcome refused, each one an item under refusals;
one a refresh clears names "`+index.RefreshRemedy+`" as its remedy; a KB in an
older metadata format is a fault a refresh clears by rewriting the KB in the
current one. Every demoted edge — one moved out of the scoring edges to break a
cycle, by the build or by refresh's repair — is listed under findings, which
are not faults. Its result is one YAML document on stdout.`, runVerify)
}
