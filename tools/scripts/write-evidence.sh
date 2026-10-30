#!/usr/bin/env bash
# Overwrites EVIDENCE.md under a test's output directory: which recipe produced
# the directory, its exact invocation, and one line per artifact kind actually
# present. "Results are part of the test" (CONVENTIONS.md) extends to the
# directory itself — a folder of logs that does not say what produced it is not
# evidence.
#
# One shared writer, not a copy per integration script: every one draws
# artifacts from the same small vocabulary (a log, per-paper or per-fixture
# directories, comparison records), so a fixed checklist covers them all. Each
# calls it as its LAST step, after the artifacts it inspects exist.
#
# Inputs (environment): OUT_DIR, the directory to describe; RECIPE, the recipe
# that produced it; INVOCATION, its invocation (printf %b escapes).
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || exit 1
cd "${script_dir}/../.." || exit 1

: "${OUT_DIR:?OUT_DIR is not set: the directory to describe}"
: "${RECIPE:?RECIPE is not set: the recipe that produced the directory}"
: "${INVOCATION:?INVOCATION is not set: the invocation to record}"

{
  echo "# EVIDENCE"
  echo
  echo "Producing recipe: just ${RECIPE}"
  echo
  echo "Invocation:"
  echo
  printf '%b\n' "${INVOCATION}" | sed 's/^/    /'
  echo
  echo "Artifacts present:"
  [[ -f "${OUT_DIR}/log.txt" ]]      && echo "- log.txt: full recipe log (tee'd stdout+stderr)"
  [[ -f "${OUT_DIR}/checks.tsv" ]]             && echo "- checks.tsv, docs/: per invocation its arguments, the outcome wanted and pass or fail; docs/NN-<verb>.out, .log, .exit and .keys.log (the documented-keys parse); repo/ as the run left it"
  [[ -f "${OUT_DIR}/init.tsv" ]]               && echo "- init.tsv, docs/: per invocation its arguments, the outcome wanted and pass or fail; docs/NN-<verb>.out, .log, .exit and .keys.log (the documented-keys parse), docs/init-nodot.{out,log}; repo/, repo-nodot/, gitonly/ and state/ as the runs left them"
  [[ -f "${OUT_DIR}/stage.tsv" ]]              && echo "- stage.tsv, legs/, fixtures/, plant-state/, sentinel: per check the leg, the check, pass or fail and its evidence path; legs/<leg>.<check>.check, each check's own output; legs/NN-<leg>.out, .log and .exit per run of the staging script; fixtures/ (the scratch repositories) as the legs left them; plant-state/ (leg 04's store, whose holder record no kbase writes) and sentinel (leg 06's link target)"
  [[ -f "${OUT_DIR}/config.tsv" ]]             && echo "- config.tsv, docs/: per invocation the configuration it read, the source wanted and pass or fail; docs/NN-<state>-<route>.out, .log and .exit; configs/ as staged (absent, pool, both); work/ (the directory every invocation ran in); mcp.requests.jsonl and mcp.actual.jsonl, the config tools/call and the frames the server answered"
  [[ -d "${OUT_DIR}/layered" ]]                && echo "- home/, layered/, literal/: home/ the staged home directory every invocation ran under, holding the ~ fixtures (cfg/, keys/); layered/repo the staged git repository the layered leg read, with its own .kbase/ over layered/home/.config/kbase; literal/ the configuration directory whose pool entries spell ~user/key.txt and a/~/b/key.txt — paths only a join to that directory reaches"
  [[ -f "${OUT_DIR}/comparison.tsv" ]]         && echo "- comparison.tsv, transcripts/<name>/: per MCP transcript pass or the response line that differs; requests.jsonl, expected.jsonl (placeholders substituted), actual.jsonl, mcp.log, mcp.exit; insert.*, refresh.* (.out, .log), staging/ (each staging op's values as given, .out and .log) and repo/ as the staging left them"
  [[ -f "${OUT_DIR}/legs.tsv" ]]               && echo "- legs.tsv, <leg>/: per check the leg, the check and pass or fail; <leg>/repo/ (the staged sources with their own .git, kb-root/ as the build left it), <leg>/state/ (the build's state store, reports/ among it), <leg>/<command>.out, .log and .exit per command; home/ and config/, the quarantined git and kbase configuration"
  [[ -f "${OUT_DIR}/monitor.tsv" ]]          && echo "- monitor.tsv, polls/: every status poll the monitor made while the build ran (polls/NNN.out); build.*, cancel.*, cancel-again.*, status-*.*, resume.* (.out, .log, .exit, .keys.log); repo/ and state/ as the run left them"
  [[ -f "${OUT_DIR}/test.log" ]]               && echo "- repo/ (the fixture paper, git init and one commit), requests.fifo and frames.fifo (the pipes the server read and wrote), repo/kb-root and state/ as the probe's two builds left them, mcp.log (the server's stderr) and test.log (the probe's go test output)"
  [[ -f "${OUT_DIR}/docs/04-score.out" ]]      && echo "- docs/: per step its .out, .log and .exit; repo/ (the staged KB), config/ (the three seeded files) and state/ as the run left them; comparison.yaml: the fixture, the KB, the configuration and the run's outcome"
  compgen -G "${OUT_DIR}/*/checks/kbase-status.out" > /dev/null && echo "- comparison.yaml: per paper every Direction 1 check, pass or fail, its detail and evidence path"
  compgen -G "${OUT_DIR}/*/checks/kbase-status.out" > /dev/null && echo "- <id>/: repo/ (the fixture repository kbase built), built/ (kb-root/ and the build records as kbase's build left them), state/ (progress.jsonl, records/), config/ (empty), checks/ (per command .out, .log, .exit); on the special paper also kill/, guard/ and stamp/ (each repo/ and state/), and kill/kill.txt"
  compgen -G "${OUT_DIR}/*/nodepass.json" > /dev/null && echo "- comparison.yaml: the paper's checks, its state-dir, the scratch/ census and the counts (paragraphs asked, claims minted, defaults by cause; the unmarked pairs planned and asked, their letters and outcomes, the source groups and calls; candidates by harvest, by letter and outcome, own-equation pairs dropped, edges a ring demoted); <id>/: repo/ (the fixture repository the build wrote), state/ (progress.jsonl, records/, scratch/trails/ (per trail: answers/, asks/, captures/)), counts.yaml, nodepass.json, kb-build-node-pass.yaml and unmarked-plan.json (the node-pass record and the planned pairs), checks/ (per command .out, .log, .exit); models.*: the provider's catalogue, read before the build"
  [[ -f "${OUT_DIR}/surface.yaml" ]]           && echo "- surface.yaml, run/, measure.log: the served surface's counts (tools, tools-list-chars, input-schema-chars, instructions-chars); run/ the server's run tree copied from its temporary directory (requests.jsonl, frames.jsonl, mcp.log, mcp.exit); measure.log the measuring go test's output"
  [[ -d "${OUT_DIR}/answers" ]]                && echo "- answers/: per golden case NNN.out (the answer, its kb-root path as <kb-root>), NNN.log (stderr) and NNN.exit; repo/ (the scratch copy of the 2.0.1 fixture, as the queries left it); comparison.yaml and compare.log: per case equal or its expected and actual answers, and the cases each conversion rule moved"
  true
} > "${OUT_DIR}/EVIDENCE.md"
