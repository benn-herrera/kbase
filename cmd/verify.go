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
	src, refusals, err := openKB(root)
	if err != nil {
		return failed(fields, err)
	}
	if refusals != nil {
		return refused(fields, refusals...)
	}
	faults, err := index.Verify(src)
	if err != nil {
		return failed(fields, err)
	}
	findings, err := index.DemotedFindings(src)
	switch {
	case err != nil && len(faults) == 0:
		return failed(fields, err)
	case err != nil:
		// The faults already name what keeps the registers from being read.
		findings = []toolresult.Item{}
	}
	fields = append(fields, toolresult.Field{Key: "findings", Value: findings})
	if len(faults) == 0 {
		return toolresult.Done, fields
	}
	return refused(fields, index.Items(faults)...)
}

func init() {
	rootCmd.AddCommand(readOnly(maintenanceCommand("verify",
		"check the KB's freshness and links",
		`verify checks the KB's freshness and links at kb-root/ beside the
repository's .git as kb_tools' kb-verify does: the dead-link and unknown-id
gate over kb-root/, then the metadata gate (coverage, uniqueness, integrity,
acyclicity, and freshness of every derived field and of .index/). The
citation grammar is checked by the build, not here. It writes nothing.
Faults are outcome refused, each one an item under refusals; one a refresh
clears names "kbase refresh" as its remedy; on a KB in an older metadata
format the index is stale, and a refresh rewrites the KB in the current one.
Every demoted edge — one the build's cycle breaking cut from depends-on — is
listed under findings, which are not faults. Its result is one YAML document
on stdout.`, runVerify)))
}
