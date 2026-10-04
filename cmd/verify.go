package main

import (
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
	findings, err := index.Verify(root)
	if err != nil {
		return failed(fields, err)
	}
	if len(findings) == 0 {
		return toolresult.Done, fields
	}
	return refused(fields, index.Items(findings)...)
}

func init() {
	rootCmd.AddCommand(maintenanceCommand("verify",
		"check the KB's freshness, links and citations",
		`verify checks the KB at kb-root/ beside the repository's .git as kb_tools'
kb-verify does: the dead-link and unknown-id gate over kb-root/, the metadata
gate (coverage, uniqueness, integrity, acyclicity, and freshness of every
derived field, .index/ and the placeholder claim-graph.svg — a drawn sheet is
left unchecked), and the citation gate. It writes nothing. Findings are
outcome refused, each one an item under refusals; one a refresh clears names
"kbase refresh" as its remedy. Its result is one YAML document on stdout.`, runVerify))
}
