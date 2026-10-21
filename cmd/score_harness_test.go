package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/config"
	"kbase/internal/query"
)

// harnessOpts is a score run on the harness back end: a profile the loaded
// layers supply, and the file-mode configuration the scoring role resolves from
// — a harness consults no mode, but the role's model is still configured there.
func harnessOpts(profile config.Profile, harness string) scoreOptions {
	opts := scoreOptions{Pass: query.PassRigor, Harness: harness}
	opts.providerOptions = scoreProvider()
	opts.Harnesses = config.Harnesses{profile.Name: profile}
	return opts
}

// shProfile is a profile over the host's shell, so a command's behaviour is one
// script and needs no fixture: the reply, the exit, and what the run's working
// directory holds.
func shProfile(name, script string, argv ...string) config.Profile {
	return config.Profile{Name: name, Command: append([]string{"sh", "-c", script, "sh"}, argv...), Layer: config.LayerSourceExplicit}
}

// TestScoreHarnessAsksOneProcessPerItem: a harness run asks one item per
// process in the run's own working directory, empties that directory before
// each item, and lands the value the command's stdout carries.
func TestScoreHarnessAsksOneProcessPerItem(t *testing.T) {
	repo, state := scoreFixture(t), t.TempDir()
	// A command that fails where the previous item's file is still visible: the
	// run only completes four items if each one's directory was emptied.
	script := `if [ -e seen ]; then printf 'the previous item is visible\n' >&2; exit 4; fi
touch seen; printf 'none\n'`
	got := scoreIn(t, repo, state, harnessOpts(shProfile("mine", script), "mine"), nil)
	if got.doc.Outcome != "done" {
		t.Fatalf("the harness run: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	if got.dialled != 0 {
		t.Errorf("a harness run dialled a provider %d time(s)", got.dialled)
	}
	counts, _ := got.doc.Values["counts"].(map[string]any)
	if counts["total"] != 4 || counts["done"] != 4 || counts["failed"] != 0 {
		t.Errorf("counts %v, want the run's four items done by their own processes\n%s", counts, got.stdout)
	}
	dir, _ := got.doc.Values["ledger"].(string)
	work := filepath.Join(dir, harnessDirName)
	if _, err := os.Stat(work); err != nil {
		t.Errorf("the run left no working directory for its commands: %v", err)
	}
	// The command's own file survives the last item alone: it is the run's one
	// directory, emptied per item rather than grown per item.
	if _, err := os.Stat(filepath.Join(work, "seen")); err != nil {
		t.Errorf("the last item left no artifact: %v", err)
	}
}

// TestScoreHarnessRecordsWhatAnswered: the record names the back end, the
// profile and its layer beside the model the command was asked for, and the
// run's header names the argv with {model} substituted and {prompt} the token.
func TestScoreHarnessRecordsWhatAnswered(t *testing.T) {
	repo, state := scoreFixture(t), t.TempDir()
	profile := config.Profile{Name: "modelled", Layer: config.LayerSourceGlobal,
		Command: []string{"sh", "-c", "printf 'none\\n'", "sh", "--model", "{model}", "{prompt}"}}
	opts := harnessOpts(profile, "modelled")
	opts.ScoringModel = "org/judge"
	got := scoreIn(t, repo, state, opts, nil)
	if got.doc.Outcome != "done" {
		t.Fatalf("the harness run: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	role := roleOf(got.doc, "scoring")
	if role == nil || role["back-end"] != "harness" || role["harness"] != "modelled" ||
		role["harness-layer"] != config.LayerSourceGlobal || role["model"] != "org/judge" {
		t.Fatalf("roles = %v, want the harness back end, the profile, its layer and the flag's id verbatim\n%s",
			got.doc.Values["roles"], got.stdout)
	}
	for _, key := range []string{"provider", "provider-rule", "provider-layer"} {
		if _, ok := role[key]; ok {
			t.Errorf("a harness role names %s: %v", key, role)
		}
	}

	dir, _ := got.doc.Values["ledger"].(string)
	header, err := os.ReadFile(filepath.Join(dir, "run.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"backend: harness", "harness: modelled", "harness_layer: global", "model: org/judge",
		"- sh", "- --model", "- org/judge", "- '{prompt}'",
	} {
		if !strings.Contains(string(header), want) {
			t.Errorf("run.yaml does not carry %q:\n%s", want, header)
		}
	}
	ledger, err := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"backend":"harness"`, `"harness":"modelled"`, `"harness_layer":"global"`, `"model":"org/judge"`} {
		if !strings.Contains(string(ledger), want) {
			t.Errorf("the ledger's lines do not carry %s:\n%s", want, ledger)
		}
	}
}

// TestScoreHarnessModelIsTheRolesOrNull: a profile naming {model} is asked the
// scoring role's model part when no flag is given; one naming none asks its own
// model, which kbase cannot name, so the document's model is null — and a
// --scoring-model the profile cannot carry is refused rather than dropped.
func TestScoreHarnessModelIsTheRolesOrNull(t *testing.T) {
	repo := scoreFixture(t)
	modelled := config.Profile{Name: "modelled", Layer: config.LayerSourceExplicit,
		Command: []string{"sh", "-c", "printf 'none\\n'", "sh", "--model", "{model}"}}
	got := scoreIn(t, repo, t.TempDir(), harnessOpts(modelled, "modelled"), nil)
	if role := roleOf(got.doc, "scoring"); role == nil || role["model"] != "scoring-model" {
		t.Errorf("roles = %v, want the role's model part\n%s", got.doc.Values["roles"], got.stdout)
	}

	plain := config.Profile{Name: "plain", Layer: config.LayerSourceExplicit,
		Command: []string{"sh", "-c", "printf 'none\\n'", "sh"}}
	got = scoreIn(t, repo, t.TempDir(), harnessOpts(plain, "plain"), nil)
	if got.doc.Outcome != "done" {
		t.Fatalf("the plain-profile run: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	if role := roleOf(got.doc, "scoring"); role == nil || role["model"] != nil {
		t.Errorf("roles = %v, want a null model: the profile asks its own\n%s", got.doc.Values["roles"], got.stdout)
	}
	dir, _ := got.doc.Values["ledger"].(string)
	ledger, err := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(ledger), `"model"`) {
		t.Errorf("the ledger names a model the profile never carried:\n%s", ledger)
	}

	refused := harnessOpts(plain, "plain")
	refused.ScoringModel = "org/judge"
	got = scoreIn(t, repo, t.TempDir(), refused, nil)
	items := got.doc.items("refusals")
	if got.doc.Outcome != "refused" || len(items) != 1 || items[0]["check"] != config.CheckHarness || items[0]["key"] != "--scoring-model" {
		t.Errorf("a flag no profile can carry = %s, %v; want a harness refusal naming the flag\n%s",
			got.doc.Outcome, items, got.stdout)
	}
	// The role must still be configured even where the flag names a model: a
	// flag overrides a role, never substitutes for one.
	unset := harnessOpts(plain, "plain")
	unset.ScoringModel = "org/judge"
	unset.Config.Models.Scoring = ""
	got = scoreIn(t, repo, t.TempDir(), unset, nil)
	if items := got.doc.items("refusals"); got.doc.Outcome != "refused" || len(items) != 1 || items[0]["key"] != "models.scoring" {
		t.Errorf("an unconfigured role under a harness = %s, %v; want models.scoring refused\n%s",
			got.doc.Outcome, items, got.stdout)
	}
}

// TestScoreHarnessRefusals: a name no loaded layer supplies, and a command that
// cannot start, are each a refusal before the KB is opened — with the loaded
// names as the allowed set, and nothing written.
func TestScoreHarnessRefusals(t *testing.T) {
	repo, state := scoreRepo(t), t.TempDir()
	profiles := config.Harnesses{"mine": shProfile("mine", "printf 'none\\n'")}
	opts := scoreOptions{Pass: query.PassRigor, Harness: "nosuch"}
	opts.providerOptions = scoreProvider()
	opts.Harnesses = profiles
	got := scoreIn(t, repo, state, opts, nil)
	items := got.doc.items("refusals")
	if got.doc.Outcome != "refused" || len(items) != 1 || items[0]["check"] != config.CheckHarness ||
		items[0]["key"] != "--harness" || !slices.Equal(anyStrings(items[0]["allowed"]), []string{"mine"}) {
		t.Fatalf("--harness nosuch = %s, %v; want a harness refusal listing the loaded profile\n%s",
			got.doc.Outcome, items, got.stdout)
	}
	if _, opened := got.doc.Values["run"]; opened {
		t.Errorf("the refusal opened a run:\n%s", got.stdout)
	}
	if _, err := os.Stat(filepath.Join(state, "score")); !os.IsNotExist(err) {
		t.Errorf("the refusal left a score directory: %v", err)
	}

	absent := config.Profile{Name: "absent", Layer: config.LayerSourceExplicit,
		Command: []string{"kbase-no-such-harness-program", "--ask"}}
	got = scoreIn(t, repo, state, harnessOpts(absent, "absent"), nil)
	items = got.doc.items("refusals")
	if got.doc.Outcome != "refused" || len(items) != 1 || items[0]["check"] != config.CheckHarness ||
		items[0]["key"] != "--harness" || !strings.Contains(items[0]["detail"].(string), "cannot be started") {
		t.Errorf("a command that cannot start = %s, %v; want a harness refusal naming --harness\n%s",
			got.doc.Outcome, items, got.stdout)
	}
}

// TestScoreHarnessFailureIsTheItem: a command exiting non-zero fails its item
// and the run goes on — the next item is asked, the line carries the exit code
// and the stderr, and the run closes on failures.
func TestScoreHarnessFailureIsTheItem(t *testing.T) {
	repo, state := scoreRepo(t), t.TempDir()
	profile := shProfile("fails", "printf 'the stub refuses\\n' >&2; exit 3")
	got := scoreIn(t, repo, state, harnessOpts(profile, "fails"), nil)
	if got.doc.Outcome != "failed" {
		t.Fatalf("the failing run: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	counts, _ := got.doc.Values["counts"].(map[string]any)
	if counts["failed"] != 1 || counts["done"] != 0 {
		t.Errorf("counts %v, want the item failed and counted\n%s", counts, got.stdout)
	}
	items := got.doc.items("failures")
	if len(items) != 1 || items[0]["check"] != checkError {
		t.Fatalf("failures = %v, want one item\n%s", items, got.stdout)
	}
	detail, _ := items[0]["detail"].(string)
	for _, want := range []string{"exit status 3", "the stub refuses"} {
		if !strings.Contains(detail, want) {
			t.Errorf("the failure's detail = %q, want it to carry %q", detail, want)
		}
	}
	dir, _ := got.doc.Values["ledger"].(string)
	ledger, err := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ledger), `"status":"failed"`) || !strings.Contains(string(ledger), "the stub refuses") {
		t.Errorf("the line does not record the failure with its stderr:\n%s", ledger)
	}
}

// TestScoreHarnessFailureReasonIsASummary: a command's stderr is arbitrary bytes
// a repository chose, so what reaches the document's detail — and the log line
// beside it — is bounded and stripped of control characters, while the ledger's
// line keeps it whole. A terminal obeying a child's escape and a document
// carrying 64 KiB of it are both what the one channel prevents.
func TestScoreHarnessFailureReasonIsASummary(t *testing.T) {
	repo, state := scoreRepo(t), t.TempDir()
	script := `printf 'the stub refuses\n\033[2J\033]0;owned\007' >&2; printf 'x%.0s' $(seq 1 2000) >&2; exit 3`
	got := scoreIn(t, repo, state, harnessOpts(shProfile("noisy", script), "noisy"), nil)
	if got.doc.Outcome != "failed" {
		t.Fatalf("the failing run: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	items := got.doc.items("failures")
	if len(items) != 1 {
		t.Fatalf("failures = %v, want one item\n%s", items, got.stdout)
	}
	detail, _ := items[0]["detail"].(string)
	for _, banned := range []string{"\x1b", "\n", "\r", "\x07"} {
		if strings.Contains(detail, banned) {
			t.Errorf("the failure's detail carries %q: %q", banned, detail)
		}
	}
	if !strings.Contains(detail, "the stub refuses") || !strings.Contains(detail, "exit status 3") {
		t.Errorf("the failure's detail = %q, want the exit code and the stderr's own words", detail)
	}
	if len(detail) > 600 {
		t.Errorf("the failure's detail is %d bytes, want a bounded summary: %q", len(detail), detail)
	}
	// The one place a reply lands whole is the ledger's line.
	dir, _ := got.doc.Values["ledger"].(string)
	raw, _ := lineRecord(t, filepath.Join(dir, "ledger.jsonl"))["reply"].(string)
	if !strings.Contains(raw, "\x1b[2J") || !strings.Contains(raw, strings.Repeat("x", 2000)) {
		t.Errorf("the ledger's reply = %q, want the stderr whole", raw)
	}
}

// TestScoreHarnessIgnoresTheMode: the mode is a fact about the provider back
// end, so a base URL and role variables set beside --harness are neither a
// mixed mode nor a refusal.
func TestScoreHarnessIgnoresTheMode(t *testing.T) {
	unsetInferenceEnv(t)
	t.Setenv(config.EnvAPIBaseURL, "http://env.example/v1")
	t.Setenv(config.EnvDecision, "env-decision")
	t.Setenv(config.EnvText, "env-text")
	t.Setenv(config.EnvScoring, "env-scoring")
	repo := scoreFixture(t)
	// The file names a scoring reference as well, which is what the mixed-mode
	// refusal would otherwise be about; the profile names {model}, so the
	// model the harness is asked for says which of the two sources was read.
	modelled := shProfile("mine", "printf 'none\\n'", "--model", "{model}")
	got := scoreIn(t, repo, t.TempDir(), harnessOpts(modelled, "mine"), nil)
	if got.doc.Outcome != "done" {
		t.Fatalf("the harness run with the environment set: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	// The environment names the model where it names one, and the file is the
	// fallback the harness reads — the same precedence a mode would have.
	if role := roleOf(got.doc, "scoring"); role == nil || role["model"] != "env-scoring" {
		t.Errorf("roles = %v, want the environment's id under the harness\n%s", got.doc.Values["roles"], got.stdout)
	}
	if got.stderr != "" {
		t.Errorf("stderr = %q, want nothing: no mode refusal and no superseded line", got.stderr)
	}
}

// TestScoreHarnessRepositoryProfileIsAnnounced: a profile the repository's own
// harnesses.toml supplied writes one line naming the file and the layer; one the
// global layer or a named directory supplied says nothing.
func TestScoreHarnessRepositoryProfileIsAnnounced(t *testing.T) {
	repo := scoreRepo(t)
	layer := t.TempDir()
	profile := shProfile("mine", "printf 'none\\n'")
	profile.Layer = config.LayerSourceRepository
	opts := harnessOpts(profile, "mine")
	opts.Layers = []config.Layer{{Path: layer, Source: config.LayerSourceRepository}}
	got := scoreIn(t, repo, t.TempDir(), opts, nil)
	if got.doc.Outcome != "done" {
		t.Fatalf("the repository-profile run: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	wantLine := config.HarnessesPath(layer)
	if want := wantLine + ` supplies the "mine" harness profile (layer: repository) — kbase runs the command it names` + "\n"; got.stderr != want {
		t.Errorf("stderr = %q, want the add-only line unchanged:\n%s", got.stderr, want)
	}
	if role := roleOf(got.doc, "scoring"); role == nil || role["harness-layer"] != config.LayerSourceRepository {
		t.Errorf("roles = %v, want harness-layer repository\n%s", got.doc.Values["roles"], got.stdout)
	}

	global := profile
	global.Layer = config.LayerSourceGlobal
	opts = harnessOpts(global, "mine")
	opts.Layers = []config.Layer{{Path: layer, Source: config.LayerSourceGlobal}}
	if got := scoreIn(t, repo, t.TempDir(), opts, nil); got.stderr != "" {
		t.Errorf("stderr = %q, want nothing: the user put the profile there", got.stderr)
	}
}

// TestScoreHarnessRepositoryProfileNamesTheLayerItDisplaced: a repository
// harnesses.toml whose [claude] replaces a same-named global [claude] says so on
// the one line, naming the layer it displaced — a user's hardened profile stepped
// on reads differently from a profile merely added. The global command answers
// nonsense, so the run standing done proves the repository's entry is the one
// that ran, and the global layer keeps its silence.
func TestScoreHarnessRepositoryProfileNamesTheLayerItDisplaced(t *testing.T) {
	repo := scoreRepo(t)
	globalDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(globalDir, config.HarnessesFileName),
		[]byte("[claude]\ncommand = [\"sh\", \"-c\", \"echo displaced\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeRepo(t, repo, map[string]string{
		config.RepositoryDirName + "/" + config.HarnessesFileName: "[claude]\ncommand = [\"sh\", \"-c\", \"echo none\"]\n"})
	layers := []config.Layer{
		{Path: filepath.Join(repo, config.RepositoryDirName), Source: config.LayerSourceRepository},
		{Path: globalDir, Source: config.LayerSourceGlobal},
	}
	loaded, err := config.Load(layers)
	if err != nil {
		t.Fatalf("the replacing stack did not load: %v", err)
	}
	opts := harnessOpts(loaded.Harnesses["claude"], "claude")
	opts.Layers = layers
	got := scoreIn(t, repo, t.TempDir(), opts, nil)
	if got.doc.Outcome != "done" {
		t.Fatalf("the replacing run: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	want := config.HarnessesPath(layers[0].Path) +
		` supplies the "claude" harness profile (layer: repository, replacing the global layer's) — kbase runs the command it names` + "\n"
	if got.stderr != want {
		t.Errorf("stderr = %q, want the one line naming the displaced layer:\n%s", got.stderr, want)
	}
}

// TestScoreDryRunTouchesNoProfile: a dry run asks nothing, so --harness changes
// nothing — no profile is looked up, no command probed, no line written, and the
// document carries no roles record.
func TestScoreDryRunTouchesNoProfile(t *testing.T) {
	repo, state := scoreRepo(t), t.TempDir()
	opts := scoreOptions{Pass: query.PassRigor, DryRun: true, Harness: "no-such-profile"}
	opts.providerOptions = scoreWithoutScoring()
	got := scoreIn(t, repo, state, opts, nil)
	if got.doc.Outcome != "done" {
		t.Fatalf("the dry run: outcome %s\n%s", got.doc.Outcome, got.stdout)
	}
	for _, key := range []string{"roles", "resume"} {
		if v, ok := got.doc.Values[key]; ok {
			t.Errorf("a dry run carries %s: %v\n%s", key, v, got.stdout)
		}
	}
	dir, _ := got.doc.Values["ledger"].(string)
	if _, err := os.Stat(filepath.Join(dir, harnessDirName)); err == nil {
		t.Error("a dry run left a working directory for a command it never ran")
	}
}

// TestScoreResumeCarriesTheBackEnd: the command that continues a run carries the
// harness and the flag it carried, and an environment-mode run carries the
// mode's settings as assignments before the program.
func TestScoreResumeCarriesTheBackEnd(t *testing.T) {
	opts := scoreOptions{Pass: query.PassRigor, ScoringModel: "org/judge", Harness: "claude"}
	got := scoreResume(opts, "run-1", "/state", "", config.Overrides{})
	want := "kbase score --pass rigor --resume run-1 --state-dir /state --scoring-model org/judge --harness claude"
	if got != want {
		t.Errorf("resume = %q, want %q", got, want)
	}
	env := config.Overrides{BaseURL: "http://env.example/v1", Decision: "d", Text: "t", Scoring: "s", APIKeyFile: "/keys/k.txt"}
	got = scoreResume(scoreOptions{Pass: query.PassRigor}, "run-2", "/state", config.ModeEnvironment, env)
	want = "KBASE_API_BASE_URL=http://env.example/v1 KBASE_DECISION=d KBASE_TEXT=t KBASE_SCORING=s " +
		"KBASE_API_KEY_FILE=/keys/k.txt kbase score --pass rigor --resume run-2 --state-dir /state"
	if got != want {
		t.Errorf("resume = %q, want %q", got, want)
	}
	// A --config-dir the invocation was given is carried on: the resumed run
	// reads that directory and no other, so it resolves the profile and the
	// reference the first run did under the same run id.
	got = scoreResume(scoreOptions{Pass: query.PassRigor, ConfigDir: "/cfg dir"}, "run-3", "/state", config.ModeFile, config.Overrides{})
	want = "kbase score --pass rigor --resume run-3 --state-dir /state --config-dir '/cfg dir'"
	if got != want {
		t.Errorf("resume = %q, want %q", got, want)
	}
}
