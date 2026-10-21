package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/config"
	"kbase/internal/model"
	"kbase/internal/query"
)

// scoreWithoutScoring is a pool and configuration whose scoring role is unset:
// the two other roles are configured, so a run that fell back to either would
// have something to fall back to.
func scoreWithoutScoring() providerOptions {
	return providerOptions{
		Providers: config.Providers{"solo": provider("solo", "http://solo.example/v1")},
		Config:    config.Config{Models: config.ModelMap{Decision: "solo/decision-model", Text: "solo/text-model"}},
	}
}

// TestScoreRefusesWithoutAScoringRole: a run whose mode names no scoring model
// refuses naming that key before anything is asked, in file mode and in
// environment mode; --scoring-model overrides a configured role and never
// substitutes for one; and a dry run lists the set, resolves no role and
// refuses nothing for one.
func TestScoreRefusesWithoutAScoringRole(t *testing.T) {
	repo, state := scoreRepo(t), t.TempDir()
	fileMode := scoreOptions{Pass: query.PassRigor, providerOptions: scoreWithoutScoring()}
	got := scoreIn(t, repo, state, fileMode, nil)
	items := got.doc.items("refusals")
	if got.doc.Outcome != "refused" || len(items) != 1 || items[0]["check"] != checkProvider || items[0]["key"] != "models.scoring" {
		t.Fatalf("a run with no scoring role = %s, %v; want models.scoring refused\n%s", got.doc.Outcome, items, got.stdout)
	}
	if _, asked := got.doc.Values["run"]; asked {
		t.Errorf("the refusal opened a run:\n%s", got.stdout)
	}
	// Setup refuses before the KB is opened and before the run's directory
	// stands: nothing of this invocation is written.
	if _, err := os.Stat(filepath.Join(state, "score")); !os.IsNotExist(err) {
		t.Errorf("the refusal left a score directory: %v", err)
	}

	// The flag names a model, and the role is still unset: the flag is an
	// override, never the role itself.
	fileMode.ScoringModel = "solo/judge"
	got = scoreIn(t, repo, t.TempDir(), fileMode, nil)
	if items := got.doc.items("refusals"); got.doc.Outcome != "refused" || len(items) != 1 || items[0]["key"] != "models.scoring" {
		t.Errorf("--scoring-model substituted for an unset role: %s, %v\n%s", got.doc.Outcome, items, got.stdout)
	}

	dry := scoreIn(t, repo, t.TempDir(), scoreOptions{Pass: query.PassRigor, DryRun: true,
		providerOptions: scoreWithoutScoring()}, nil)
	if dry.doc.Outcome != "done" {
		t.Errorf("a dry run without the role = %s; want done\n%s", dry.doc.Outcome, dry.stdout)
	}
	for _, key := range []string{"roles", "refusals"} {
		if _, ok := dry.doc.Values[key]; ok {
			t.Errorf("a dry run carries %s:\n%s", key, dry.stdout)
		}
	}

	// The environment's spelling of the same absence.
	unsetInferenceEnv(t)
	t.Setenv(config.EnvAPIBaseURL, "http://env.example/v1")
	t.Setenv(config.EnvText, "env-text")
	envMode := scoreOptions{Pass: query.PassRigor, providerOptions: providerOptions{Providers: config.Providers{}}}
	got = scoreIn(t, repo, t.TempDir(), envMode, nil)
	items = got.doc.items("refusals")
	if got.doc.Outcome != "refused" || len(items) != 1 || items[0]["key"] != config.EnvScoring {
		t.Errorf("a run with no scoring variable = %s, %v; want %s refused\n%s",
			got.doc.Outcome, items, config.EnvScoring, got.stdout)
	}
}

// TestScoreConfiguredByEnvironmentAlone: the environment's endpoint and its
// scoring variable configure a run with no configuration directory at all: one
// client on that endpoint, a roles record naming the environment back end and
// the variable's model, and a ledger whose lines name it. The text variable is
// set beside it and appears nowhere.
func TestScoreConfiguredByEnvironmentAlone(t *testing.T) {
	unsetInferenceEnv(t)
	t.Setenv(config.EnvAPIBaseURL, "http://env.example/v1")
	t.Setenv(config.EnvDecision, "env-decision")
	t.Setenv(config.EnvText, "env-text")
	t.Setenv(config.EnvScoring, "env-scoring")

	repo, state := scoreRepo(t), t.TempDir()
	got := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor,
		providerOptions: providerOptions{Providers: config.Providers{}}}, func() model.Client { return scoring("none") })
	if got.doc.Outcome != "done" || got.dialled != 1 {
		t.Fatalf("the environment-mode run: %s, %d dials\n%s", got.doc.Outcome, got.dialled, got.stdout)
	}
	role := roleOf(got.doc, "scoring")
	if role == nil || role["back-end"] != "environment" || role["model"] != "env-scoring" {
		t.Fatalf("roles = %v, want the scoring role on the environment's endpoint\n%s", got.doc.Values["roles"], got.stdout)
	}
	for _, key := range []string{"provider", "provider-rule", "provider-layer"} {
		if _, ok := role[key]; ok {
			t.Errorf("an environment endpoint names %s: %v", key, role)
		}
	}
	if strings.Contains(got.stdout, "env-text") {
		t.Errorf("the run names the text model:\n%s", got.stdout)
	}
	dir, _ := got.doc.Values["ledger"].(string)
	ledger, err := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if err != nil {
		t.Fatalf("read the ledger: %v", err)
	}
	if !strings.Contains(string(ledger), `"model":"env-scoring"`) || strings.Contains(string(ledger), "env-text") {
		t.Errorf("the ledger's lines do not name the scoring variable alone:\n%s", ledger)
	}

	// The flag under the environment's endpoint is a bare id taken verbatim: a
	// value carrying a slash is not split against a provider the environment
	// does not name.
	flagged := scoreIn(t, repo, t.TempDir(), scoreOptions{Pass: query.PassRigor, ScoringModel: "org/model",
		providerOptions: providerOptions{Providers: config.Providers{}}}, func() model.Client { return scoring("none") })
	if flagged.doc.Outcome != "done" {
		t.Fatalf("the flagged environment run: %s\n%s", flagged.doc.Outcome, flagged.stdout)
	}
	role = roleOf(flagged.doc, "scoring")
	if role == nil || role["back-end"] != "environment" || role["model"] != "org/model" {
		t.Errorf("roles = %v, want the flag verbatim on the environment's endpoint\n%s", flagged.doc.Values["roles"], flagged.stdout)
	}
}

// TestScoreScoringModelOverridesTheRole: in file mode the flag is a reference;
// it names the provider and model the run asks, and the roles record says the
// flag is what named it.
func TestScoreScoringModelOverridesTheRole(t *testing.T) {
	repo, state := scoreFixture(t), t.TempDir()
	// The role names one entry, the flag names another: the flag's entry is the
	// endpoint asked, which is what makes it an override and not a label.
	opts := scoreOptions{Pass: query.PassRigor, ScoringModel: "other/judge"}
	opts.providerOptions = providerOptions{
		Providers: config.Providers{
			"solo":  provider("solo", "http://solo.example/v1"),
			"other": provider("other", "http://other.example/v1"),
		},
		Config: config.Config{Models: config.ModelMap{Text: "solo/text-model", Scoring: "solo/scoring-model"}},
	}
	got := scoreIn(t, repo, state, opts, func() model.Client { return scoring("none", "none", "none", "none") })
	if got.doc.Outcome != "done" || got.dialled != 1 {
		t.Fatalf("the flagged run: %s, %d dials\n%s", got.doc.Outcome, got.dialled, got.stdout)
	}
	role := roleOf(got.doc, "scoring")
	if role == nil || role["provider"] != "other" || role["provider-rule"] != config.RuleFlag || role["model"] != "judge" {
		t.Errorf("roles = %v, want other/judge under the flag's rule\n%s", got.doc.Values["roles"], got.stdout)
	}
	if len(got.endpoints) != 1 || got.endpoints[0].BaseURL != "http://other.example/v1" {
		t.Errorf("endpoints = %+v, want the flag's entry", got.endpoints)
	}
	dir, _ := got.doc.Values["ledger"].(string)
	ledger, err := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if err != nil {
		t.Fatalf("read the ledger: %v", err)
	}
	if !strings.Contains(string(ledger), `"model":"judge"`) {
		t.Errorf("the ledger does not name the flag's model:\n%s", ledger)
	}
	// The resume command carries the flag, so the continued run asks the same
	// model.
	if resumed, _ := got.doc.Values["resume"].(string); resumed != "" {
		t.Errorf("a finished run carries resume: %q", resumed)
	}
	partial := scoreIn(t, repo, t.TempDir(), scoreOptions{Pass: query.PassRigor, Limit: 1, ScoringModel: "solo/judge"},
		func() model.Client { return scoring("none") })
	resumed, _ := partial.doc.Values["resume"].(string)
	if !strings.Contains(resumed, "--scoring-model solo/judge") {
		t.Errorf("resume = %q, want it to carry --scoring-model", resumed)
	}
}

// TestScoreScoringModelNamesAMissingProvider: the flag naming a provider the
// pool does not hold refuses before the first item, names the flag, and lists
// the pool's entries as the names that would work.
func TestScoreScoringModelNamesAMissingProvider(t *testing.T) {
	repo := scoreRepo(t)
	got := scoreIn(t, repo, t.TempDir(), scoreOptions{Pass: query.PassRigor, ScoringModel: "elsewhere/judge"}, nil)
	items := got.doc.items("refusals")
	if got.doc.Outcome != "refused" || len(items) != 1 || items[0]["check"] != checkProvider ||
		items[0]["key"] != "--scoring-model" || !slices.Equal(anyStrings(items[0]["allowed"]), []string{"solo"}) {
		t.Fatalf("the missing provider = %s, %v; want --scoring-model refused with the pool listed\n%s",
			got.doc.Outcome, items, got.stdout)
	}
	if _, asked := got.doc.Values["run"]; asked {
		t.Errorf("the refusal opened a run:\n%s", got.stdout)
	}
}

// TestScoreHelpNamesTheScoringFlag: the flags are documented where a user looks
// for them — the scoring model, and the back end that asks instead of an
// endpoint — and neither the retired --provider nor the internal override is.
func TestScoreHelpNamesTheScoringFlag(t *testing.T) {
	help := scoreCmd.UsageString()
	for _, flag := range []string{"--scoring-model", "--harness"} {
		if !strings.Contains(help, flag) {
			t.Errorf("score --help names no %s:\n%s", flag, help)
		}
	}
	if strings.Contains(help, "--provider") {
		t.Errorf("score --help names --provider:\n%s", help)
	}
	if strings.Contains(help, backEndCommandEnv) {
		t.Errorf("score --help names the internal back end override:\n%s", help)
	}
}
