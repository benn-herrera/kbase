package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"kbase/internal/config"
	"kbase/internal/mcp"
	"kbase/internal/model"
	"kbase/internal/model/modeltest"
	"kbase/internal/secret"
)

// anyStrings is a decoded YAML list of strings.
func anyStrings(v any) []string {
	list, _ := v.([]any)
	out := []string{}
	for _, e := range list {
		s, _ := e.(string)
		out = append(out, s)
	}
	return out
}

// configureRun is what one configure command invocation produced: its result
// document as parsed and as written, stderr and exit code.
type configureRun struct {
	doc    resultDoc
	stdout string
	stderr string
	code   int
}

// namedLayer is the one layer a --config-dir naming dir resolves.
func namedLayer(dir string) providerOptions {
	return providerOptions{Layers: []config.Layer{{Path: dir, Source: config.LayerSourceExplicit}}, WriteDir: dir}
}

// globalLayer is the layers an invocation naming no directory resolves, the
// global one at dir, inside the repository repo where repo is not "".
func globalLayer(dir, repo string) providerOptions {
	opts := providerOptions{WriteDir: dir, RepoRoot: repo}
	if repo != "" {
		opts.Layers = append(opts.Layers, config.Layer{Path: config.RepositoryDir(repo), Source: config.LayerSourceRepository})
	}
	opts.Layers = append(opts.Layers, config.Layer{Path: dir, Source: config.LayerSourceGlobal})
	return opts
}

// runProviders runs configure-providers with opts, every dial served by
// client.
func runProviders(t *testing.T, opts configureProvidersOptions, client *modeltest.MockClient) configureRun {
	t.Helper()
	var stdout, stderr bytes.Buffer
	opts.Stdout, opts.Stderr = &stdout, &stderr
	opts.NewClient = func(model.Endpoint) model.Client { return client.Client() }
	code, err := runConfigureProviders(context.Background(), opts)
	if err != nil {
		t.Fatalf("runConfigureProviders: %v", err)
	}
	return configureRun{doc: checkDocument(t, "configure-providers", stdout.String()), stdout: stdout.String(), stderr: stderr.String(), code: code}
}

// runModelsConfig runs configure-models with opts, every dial served by
// client.
func runModelsConfig(t *testing.T, opts configureModelsOptions, client *modeltest.MockClient) configureRun {
	t.Helper()
	var stdout, stderr bytes.Buffer
	opts.Stdout, opts.Stderr = &stdout, &stderr
	opts.NewClient = func(model.Endpoint) model.Client { return client.Client() }
	code, err := runConfigureModels(context.Background(), opts)
	if err != nil {
		t.Fatalf("runConfigureModels: %v", err)
	}
	return configureRun{doc: checkDocument(t, "configure-models", stdout.String()), stdout: stdout.String(), stderr: stderr.String(), code: code}
}

// refusal is the run's one refusal, failing the test where it has another
// outcome or count.
func (r configureRun) refusal(t *testing.T) map[string]any {
	t.Helper()
	items := r.doc.items("refusals")
	if r.doc.Outcome != "refused" || r.code != 1 || len(items) != 1 {
		t.Fatalf("outcome %s, exit %d, refusals %v: want one refusal", r.doc.Outcome, r.code, items)
	}
	return items[0]
}

// labKey is a credentials store holding one key, lab's.
func labKey() config.StoredKeys {
	return config.StoredKeys{Keys: map[string]secret.Key{"lab": secret.NewKey(testAPIKey)}}
}

// TestConfigureProvidersWithNoKeyNamesTheNextSteps: an entry written with no
// key stored is written, dials nothing, and names the steps that follow: store
// the key, then list the models.
func TestConfigureProvidersWithNoKeyNamesTheNextSteps(t *testing.T) {
	dir := t.TempDir()
	client := modeltest.NewScriptedMock(nil, catalogue("m"))
	got := runProviders(t, configureProvidersOptions{providerOptions: namedLayer(dir), Name: "Lab", BaseURL: "http://lab:8000/v1"}, client)
	if got.code != 0 || got.doc.Outcome != "done" {
		t.Fatalf("exit %d, %v", got.code, got.doc.Values)
	}
	path := config.ProvidersPath(dir)
	if got.doc.Values["provider"] != "lab" || got.doc.Values["layer"] != config.LayerSourceExplicit ||
		!slices.Equal(anyStrings(got.doc.Values["written"]), []string{path}) {
		t.Errorf("result = %v, want the lowercase provider, the layer and the file written", got.doc.Values)
	}
	want := []string{config.AuthAddRemedy("lab"), "kbase models --provider lab"}
	if next := anyStrings(got.doc.Values["next"]); !slices.Equal(next, want) {
		t.Errorf("next = %q, want %q", next, want)
	}
	if _, dialled := got.doc.Values["models"]; dialled {
		t.Error("an entry with no key was dialled")
	}
	if warnings, ok := got.doc.Values["warnings"]; !ok || len(got.doc.items("warnings")) != 0 {
		t.Errorf("warnings = %v (present %t), want an empty list", warnings, ok)
	}
	pool, _, err := config.LoadProviders(path, nil)
	if err != nil || pool["lab"].BaseURL != "http://lab:8000/v1" || len(pool) != 1 {
		t.Errorf("pool = %+v, %v; want the entry written alone", pool, err)
	}

	again := runProviders(t, configureProvidersOptions{providerOptions: namedLayer(dir), Name: "lab", BaseURL: "http://lab:8000/v1"}, client)
	if again.doc.Outcome != "unchanged" || len(anyStrings(again.doc.Values["written"])) != 0 {
		t.Errorf("the same entry again = %s, written %v; want unchanged", again.doc.Outcome, again.doc.Values["written"])
	}
}

// TestConfigureProvidersDialsAStoredKey: with a key stored under the entry's
// name the written entry is dialled and its catalogue reported, sorted; the
// key appears nowhere.
func TestConfigureProvidersDialsAStoredKey(t *testing.T) {
	dir := t.TempDir()
	opts := configureProvidersOptions{providerOptions: namedLayer(dir), Name: "lab", BaseURL: "http://lab:8000/v1",
		StoredKeys: labKey()}
	got := runProviders(t, opts, modeltest.NewScriptedMock(nil, catalogue("zeta", "alpha")))
	if got.code != 0 || got.doc.Outcome != "done" {
		t.Fatalf("exit %d, %v", got.code, got.doc.Values)
	}
	if models := anyStrings(got.doc.Values["models"]); !slices.Equal(models, []string{"alpha", "zeta"}) {
		t.Errorf("models = %q, want the catalogue sorted", models)
	}
	if _, ok := got.doc.Values["next"]; ok {
		t.Errorf("result = %v, want no next where the dial answered", got.doc.Values)
	}
	if strings.Contains(got.stderr, testAPIKey) {
		t.Error("the key reached stderr")
	}
}

// TestConfigureProvidersFailedDialLeavesTheEntry: a dial that fails leaves the
// entry written, outcome done, with next naming the failure.
func TestConfigureProvidersFailedDialLeavesTheEntry(t *testing.T) {
	dir := t.TempDir()
	client := modeltest.NewScriptedMock(nil, nil)
	client.SetError(errors.New("http 503: service unavailable"))
	opts := configureProvidersOptions{providerOptions: namedLayer(dir), Name: "lab", BaseURL: "http://lab:8000/v1",
		StoredKeys: labKey()}
	got := runProviders(t, opts, client)
	if got.code != 0 || got.doc.Outcome != "done" {
		t.Fatalf("exit %d, %v", got.code, got.doc.Values)
	}
	if next := anyStrings(got.doc.Values["next"]); len(next) != 1 || !strings.Contains(next[0], "503") {
		t.Errorf("next = %q, want the failure named", next)
	}
	if pool, _, err := config.LoadProviders(config.ProvidersPath(dir), nil); err != nil || pool["lab"].BaseURL == "" {
		t.Errorf("pool = %+v, %v; want the entry written", pool, err)
	}
}

// TestConfigureProvidersRemove: --remove takes the entry out; a provider with
// no entry is refused, naming it, and nothing is written.
func TestConfigureProvidersRemove(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, config.ProvidersFileName, "[lab]\nbaseUrl = \"http://lab/v1\"\n\n[edge]\nbaseUrl = \"http://edge/v1\"\n")
	client := modeltest.NewScriptedMock(nil, nil)
	got := runProviders(t, configureProvidersOptions{providerOptions: namedLayer(dir), Name: "LAB", Remove: true}, client)
	if got.code != 0 || got.doc.Outcome != "done" || !slices.Equal(anyStrings(got.doc.Values["written"]), []string{path}) {
		t.Fatalf("exit %d, %v", got.code, got.doc.Values)
	}
	if body, _ := os.ReadFile(path); string(body) != "[edge]\nbaseUrl = \"http://edge/v1\"\n" {
		t.Errorf("file = %q, want the lab entry gone and edge standing", body)
	}
	if next, ok := got.doc.Values["next"]; ok {
		t.Errorf("next = %v, want none where nothing names the provider", next)
	}

	absent := runProviders(t, configureProvidersOptions{providerOptions: namedLayer(dir), Name: "lab", Remove: true}, client)
	item := absent.refusal(t)
	if item["check"] != config.CheckProvider || item["path"] != path || !strings.Contains(item["detail"].(string), "no provider `lab`") ||
		item["remedy"] != "kbase show-config" {
		t.Errorf("refusal = %v, want no provider `lab` in %s with show-config as the remedy", item, path)
	}
}

// TestConfigureProvidersRefusesArguments: an unusable name, a missing
// --base-url, --remove beside an entry field and an unknown api are refused
// before anything is written.
func TestConfigureProvidersRefusesArguments(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts configureProvidersOptions
		key  string
	}{
		{"name", configureProvidersOptions{Name: "-lab", BaseURL: "http://lab/v1"}, "<provider>"},
		{"no base url", configureProvidersOptions{Name: "lab"}, "--base-url"},
		{"remove beside a field", configureProvidersOptions{Name: "lab", Remove: true, API: "openai"}, "--api"},
		{"api", configureProvidersOptions{Name: "lab", BaseURL: "http://lab/v1", API: "grpc"}, "--api"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			tc.opts.providerOptions = namedLayer(dir)
			got := runProviders(t, tc.opts, modeltest.NewScriptedMock(nil, nil))
			if item := got.refusal(t); item["key"] != tc.key {
				t.Errorf("refusal = %v, want key %q", item, tc.key)
			}
			if _, err := os.Stat(config.ProvidersPath(dir)); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("a refused run wrote providers.toml (%v)", err)
			}
		})
	}
}

// TestConfigureLocation: no flag writes the named directory, else the global
// one, standing repository layer or not; --local writes the repository's,
// and is refused outside a worktree; --global writes the global one over the
// environment's; either beside --config-dir, and the two together, are
// refused.
func TestConfigureLocation(t *testing.T) {
	repo, global, home := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	environment := providerOptions{Layers: []config.Layer{{Path: "/elsewhere", Source: config.LayerSourceEnvironment}}, WriteDir: "/elsewhere", RepoRoot: repo}
	for _, tc := range []struct {
		name          string
		opts          providerOptions
		global, local bool
		dir, layer    string
		check         string
	}{
		{name: "named", opts: namedLayer("/somewhere"), dir: "/somewhere", layer: config.LayerSourceExplicit},
		{name: "environment", opts: environment, dir: "/elsewhere", layer: config.LayerSourceEnvironment},
		{name: "default, in a repository", opts: globalLayer(global, repo), dir: global, layer: config.LayerSourceGlobal},
		{name: "--local", opts: globalLayer(global, repo), local: true, dir: config.RepositoryDir(repo), layer: config.LayerSourceRepository},
		{name: "--local over the environment", opts: environment, local: true, dir: config.RepositoryDir(repo), layer: config.LayerSourceRepository},
		{name: "--global over the environment", opts: environment, global: true, dir: filepath.Join(home, ".config", "kbase"), layer: config.LayerSourceGlobal},
		{name: "--local outside a worktree", opts: globalLayer(global, ""), local: true, check: checkWorktree},
		{name: "--global beside --config-dir", opts: namedLayer("/somewhere"), global: true, check: "usage"},
		{name: "--local beside --config-dir", opts: namedLayer("/somewhere"), local: true, check: "usage"},
		{name: "both", opts: globalLayer(global, repo), global: true, local: true, check: "usage"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, layer, items, err := configureLocation(tc.opts, tc.global, tc.local, "/start")
			if err != nil {
				t.Fatal(err)
			}
			if tc.check != "" {
				if len(items) != 1 || items[0].Check != tc.check {
					t.Errorf("items = %+v, want one %s refusal", items, tc.check)
				}
				return
			}
			if items != nil || dir != tc.dir || layer != tc.layer {
				t.Errorf("= %q, %q, %+v; want %q, %q", dir, layer, items, tc.dir, tc.layer)
			}
		})
	}
}

// soloURL is solo's base URL where no test serves it.
const soloURL = "http://provider.example/v1"

// soloModels is a configure-models input writing into the directory dir names,
// whose providers.toml it writes holding one entry, solo, at baseURL.
func soloModels(t *testing.T, dir, baseURL string, given map[config.Role]string) configureModelsOptions {
	t.Helper()
	writeConfig(t, dir, config.ProvidersFileName, "[solo]\nbaseUrl = \""+baseURL+"\"\n")
	return configureModelsOptions{providerOptions: namedLayer(dir), Models: given}
}

// TestConfigureModelsReportsTheFile: the roles given are written, the rest
// stand, and the result gives the file's three roles and what was written; a
// model missing from its catalogue is a warning in the result and on stderr,
// written all the same; the same run again is unchanged.
func TestConfigureModelsReportsTheFile(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, config.ConfigFileName, "# mine\n[models]\ntext = \"solo/kept\"\n")
	given := map[config.Role]string{config.RoleDecision: "solo/listed", config.RoleScoring: "solo/unlisted"}
	client := modeltest.NewScriptedMock(nil, catalogue("listed"))
	got := runModelsConfig(t, soloModels(t, dir, soloURL, given), client)
	if got.code != 0 || got.doc.Outcome != "done" {
		t.Fatalf("exit %d, %v", got.code, got.doc.Values)
	}
	models, _ := got.doc.Values["models"].(map[string]any)
	if models["decision"] != "solo/listed" || models["text"] != "solo/kept" || models["scoring"] != "solo/unlisted" ||
		!slices.Equal(anyStrings(got.doc.Values["written"]), []string{path}) {
		t.Errorf("result = %v, want the file's three roles and the file written", got.doc.Values)
	}
	const miss = `the scoring role's model "unlisted" is not in provider "solo"'s catalogue`
	if want := "configure-models: " + miss + "\n"; !strings.Contains(got.stderr, want) {
		t.Errorf("stderr = %q, want %q", got.stderr, want)
	}
	if warnings := got.doc.items("warnings"); len(warnings) != 1 || warnings[0]["key"] != "--scoring" || warnings[0]["detail"] != miss {
		t.Errorf("warnings = %v, want the one miss keyed --scoring", warnings)
	}
	if body, _ := os.ReadFile(path); string(body) != "# mine\n[models]\ntext = \"solo/kept\"\ndecision = \"solo/listed\"\nscoring = \"solo/unlisted\"\n" {
		t.Errorf("file = %q, want the two roles added and every other byte kept", body)
	}

	again := runModelsConfig(t, soloModels(t, dir, soloURL, given), client)
	if again.code != 0 || again.doc.Outcome != "unchanged" {
		t.Errorf("the same run again = %s, exit %d; want unchanged", again.doc.Outcome, again.code)
	}
}

// TestConfigureModelsListFailure: a catalogue that cannot be listed fails the
// run with the provider's own reason, nothing written.
func TestConfigureModelsListFailure(t *testing.T) {
	dir := t.TempDir()
	client := modeltest.NewScriptedMock(nil, nil)
	client.SetError(errors.New("http 503: service unavailable"))
	got := runModelsConfig(t, soloModels(t, dir, soloURL, map[config.Role]string{config.RoleText: "solo/t"}), client)
	if failures := got.doc.items("failures"); got.code != 3 || len(failures) != 1 || !strings.Contains(failures[0]["detail"].(string), "503") {
		t.Fatalf("exit %d, failures %v; want it failed", got.code, failures)
	}
	if _, err := os.Stat(config.ConfigPath(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a failed run wrote config.toml (%v)", err)
	}
}

// TestConfigureModelsWithNoKeyCarriesTheRemedy: an entry written before its
// key is stored, dialled by configure-models and refused for the missing key,
// fails carrying the command that stores one and the refused-key note.
func TestConfigureModelsWithNoKeyCarriesTheRemedy(t *testing.T) {
	srv, _ := refusingProvider(t, http.StatusUnauthorized)
	dir := t.TempDir()
	opts := soloModels(t, dir, srv.URL, map[config.Role]string{config.RoleText: "solo/t"})
	var stdout, stderr bytes.Buffer
	opts.Stdout, opts.Stderr, opts.NewClient = &stdout, &stderr, model.NewClient
	code, err := runConfigureModels(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	got := checkDocument(t, "configure-models", stdout.String())
	items := got.items("failures")
	if code != 3 || len(items) != 1 || items[0]["remedy"] != config.AuthAddRemedy("solo") || !endsWithNote(items[0]) {
		t.Errorf("exit %d, failures %v; want one failure carrying the auth add remedy and the refused-key note", code, remedies(items))
	}
	if _, err := os.Stat(config.ConfigPath(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a failed run wrote config.toml (%v)", err)
	}
}

// TestConfigureModelsLocalOutsideAWorktree: --local outside a git worktree is
// refused naming the directory the search started from, nothing dialled.
func TestConfigureModelsLocalOutsideAWorktree(t *testing.T) {
	opts := soloModels(t, t.TempDir(), soloURL, map[config.Role]string{config.RoleText: "solo/t"})
	opts.providerOptions = globalLayer(t.TempDir(), "")
	opts.Local, opts.WorkDir = true, "/start"
	client := modeltest.NewScriptedMock(nil, catalogue("t"))
	got := runModelsConfig(t, opts, client)
	if item := got.refusal(t); item["check"] != checkWorktree || item["path"] != "/start" || item["remedy"] != "git init" {
		t.Errorf("refusal = %v, want the worktree refusal naming /start", item)
	}
}

// TestConfigureModelsChecksTheWrittenStack: configure-models checks a provider
// against the layer it writes into and the layers beneath it, not the stack
// the invocation read. A provider only the repository layer holds is refused
// under --global, against the global providers.toml with the remedy carrying
// --global, nothing dialled or written; under --local it is accepted.
func TestConfigureModelsChecksTheWrittenStack(t *testing.T) {
	home, repo := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	global := filepath.Join(home, ".config", "kbase")
	repoDir := config.RepositoryDir(repo)
	if err := os.MkdirAll(repoDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeConfig(t, repoDir, config.ProvidersFileName, "[lab]\nbaseUrl = \"http://lab/v1\"\n")
	read := func() providerOptions {
		t.Helper()
		layers := globalLayer(global, repo)
		opts, err := loadConfigContext(layers.Layers)
		if err != nil {
			t.Fatal(err)
		}
		opts.RepoRoot, opts.WriteDir = repo, global
		return opts
	}
	given := map[config.Role]string{config.RoleDecision: "lab/d"}

	undialled := modeltest.NewScriptedMock(nil, nil)
	undialled.SetError(errors.New("dialled"))
	got := runModelsConfig(t, configureModelsOptions{providerOptions: read(), Models: given, Global: true}, undialled)
	item := got.refusal(t)
	remedy, _ := item["remedy"].(string)
	if item["check"] != config.CheckProvider || item["key"] != "--decision" || item["path"] != config.ProvidersPath(global) ||
		!strings.HasPrefix(remedy, "`kbase configure-providers lab ") || !strings.Contains(remedy, "--global") {
		t.Errorf("refusal = %v, want provider lab refused against %s with a remedy carrying --global", item, config.ProvidersPath(global))
	}
	if _, err := os.Stat(config.ConfigPath(global)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a refused run wrote the global config.toml (%v)", err)
	}

	got = runModelsConfig(t, configureModelsOptions{providerOptions: read(), Models: given, Local: true}, modeltest.NewScriptedMock(nil, catalogue("d")))
	if got.code != 0 || got.doc.Outcome != "done" || !slices.Equal(anyStrings(got.doc.Values["written"]), []string{config.ConfigPath(repoDir)}) {
		t.Errorf("--local: exit %d, %v; want the repository's config.toml written", got.code, got.doc.Values)
	}
}

// TestConfigureModelsReadsNoLayerOutsideItsStack: a repository providers.toml
// that fails the strict load does not stop configure-models --global, which
// neither writes nor checks against that layer; the call answers from the
// global stack alone.
func TestConfigureModelsReadsNoLayerOutsideItsStack(t *testing.T) {
	home, repo := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(config.EnvConfigDir, "")
	for _, dir := range []string{filepath.Join(repo, ".git"), config.RepositoryDir(repo)} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	writeConfig(t, config.RepositoryDir(repo), config.ProvidersFileName, "[[[ not toml\n")
	t.Chdir(repo)
	args := []string{"configure-models", "--global", "--decision", "nosuch/d"}
	stdout, stderr, err := executeRoot(t, args...)
	t.Cleanup(func() {
		if sub, _, ferr := rootCmd.Find(args); ferr == nil {
			if err := mcp.ResetFlags(sub); err != nil {
				t.Error(err)
			}
		}
	})
	code := exitStatus(err, stdout, stderr)
	doc := checkDocument(t, "configure-models", stdout.String())
	items := doc.items("refusals")
	global := filepath.Join(home, ".config", "kbase")
	if code != 1 || len(items) != 1 || items[0]["check"] != config.CheckProvider || items[0]["path"] != config.ProvidersPath(global) {
		t.Errorf("exit %d, %s, refusals %v; want the provider refusal against %s", code, doc.Outcome, items, config.ProvidersPath(global))
	}
}

// TestConfigureProvidersReadsNoLayerOutsideItsStack: a repository
// providers.toml that fails the strict load does not stop
// configure-providers --global, which neither writes nor reads that layer.
func TestConfigureProvidersReadsNoLayerOutsideItsStack(t *testing.T) {
	home, repo := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(config.EnvConfigDir, "")
	for _, dir := range []string{filepath.Join(repo, ".git"), config.RepositoryDir(repo)} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	writeConfig(t, config.RepositoryDir(repo), config.ProvidersFileName, "[[[ not toml\n")
	t.Chdir(repo)
	args := []string{"configure-providers", "lab", "--global", "--base-url", "http://lab/v1"}
	stdout, stderr, err := executeRoot(t, args...)
	t.Cleanup(func() {
		if sub, _, ferr := rootCmd.Find(args); ferr == nil {
			if err := mcp.ResetFlags(sub); err != nil {
				t.Error(err)
			}
		}
	})
	code := exitStatus(err, stdout, stderr)
	doc := checkDocument(t, "configure-providers", stdout.String())
	want := config.ProvidersPath(filepath.Join(home, ".config", "kbase"))
	if code != 0 || doc.Outcome != "done" || !slices.Equal(anyStrings(doc.Values["written"]), []string{want}) {
		t.Errorf("exit %d, %s, %v; want %s written", code, doc.Outcome, doc.Values, want)
	}
}

// TestConfigureProvidersNamesAFaultedEntry: a written entry that does not load
// — here its apiKeyFile cannot be read — gives next naming the fault's reason
// and show-config, not the steps that store a key.
func TestConfigureProvidersNamesAFaultedEntry(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, config.ProvidersFileName, "[lab]\nbaseUrl = \"http://lab/v1\"\napiKeyFile = \"missing.key\"\n")
	got := runProviders(t, configureProvidersOptions{providerOptions: namedLayer(dir), Name: "lab", BaseURL: "http://lab:8000/v1"},
		modeltest.NewScriptedMock(nil, catalogue("m")))
	if got.code != 0 || got.doc.Outcome != "done" {
		t.Fatalf("exit %d, %v", got.code, got.doc.Values)
	}
	next := anyStrings(got.doc.Values["next"])
	if len(next) != 1 || !strings.Contains(next[0], "missing.key") || !strings.Contains(next[0], "kbase show-config") ||
		strings.Contains(next[0], "auth add") {
		t.Errorf("next = %q, want one entry naming the unread key file and show-config", next)
	}
	if warnings := got.doc.items("warnings"); len(warnings) != 1 || warnings[0]["path"] != config.ProvidersPath(dir) {
		t.Errorf("warnings = %v, want the one fault against providers.toml", warnings)
	}
	if n := strings.Count(got.stderr, "missing.key"); n != 1 {
		t.Errorf("stderr = %q names the fault %d times, want once", got.stderr, n)
	}
}

// TestConfigureProvidersWarnsOfAFault: a fault in the configuration written
// into — here another entry with no baseUrl — is an item of warnings against
// its file and a line on stderr, and the removal goes ahead.
func TestConfigureProvidersWarnsOfAFault(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, config.ProvidersFileName, "[lab]\nbaseUrl = \"http://lab/v1\"\n\n[broken]\napi = \"openai\"\n")
	got := runProviders(t, configureProvidersOptions{providerOptions: namedLayer(dir), Name: "lab", Remove: true},
		modeltest.NewScriptedMock(nil, nil))
	if got.code != 0 || got.doc.Outcome != "done" {
		t.Fatalf("exit %d, %v", got.code, got.doc.Values)
	}
	const fault = `provider "broken" unavailable: no baseUrl declared`
	if warnings := got.doc.items("warnings"); len(warnings) != 1 || warnings[0]["path"] != path || warnings[0]["detail"] != fault {
		t.Errorf("warnings = %v, want %q against %s", warnings, fault, path)
	}
	if !strings.Contains(got.stderr, fault) {
		t.Errorf("stderr = %q, want %q", got.stderr, fault)
	}
}

// TestConfigureProvidersWarningCarriesNoKey: a key file readable beyond its
// owner is a warning naming the file, the provider is dialled all the same,
// and the key it holds reaches neither the result nor stderr.
func TestConfigureProvidersWarningCarriesNoKey(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no permission bits to expose a key file")
	}
	dir := t.TempDir()
	key := writeConfig(t, dir, "lab.key", testAPIKey)
	if err := os.Chmod(key, 0o644); err != nil {
		t.Fatal(err)
	}
	path := writeConfig(t, dir, config.ProvidersFileName, "[lab]\nbaseUrl = \"http://lab/v1\"\napiKeyFile = \"lab.key\"\n")
	got := runProviders(t, configureProvidersOptions{providerOptions: namedLayer(dir), Name: "lab", BaseURL: "http://lab:8000/v1"},
		modeltest.NewScriptedMock(nil, catalogue("m")))
	if got.code != 0 || !slices.Equal(anyStrings(got.doc.Values["models"]), []string{"m"}) {
		t.Fatalf("exit %d, %v; want the provider dialled", got.code, got.doc.Values)
	}
	warnings := got.doc.items("warnings")
	if len(warnings) != 1 || warnings[0]["path"] != path || !strings.Contains(fmt.Sprint(warnings[0]["detail"]), key) {
		t.Errorf("warnings = %v, want one naming %s", warnings, key)
	}
	if strings.Contains(got.stdout+got.stderr, testAPIKey) {
		t.Error("the key reached the result or stderr")
	}
}

// TestConfigureProvidersRemovalNamesWhatRefersToIt: a removal that leaves a
// role's reference and a stored key naming the provider gives next naming the
// role with the configure-models call that replaces it, then auth remove.
func TestConfigureProvidersRemovalNamesWhatRefersToIt(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, config.ProvidersFileName, "[lab]\nbaseUrl = \"http://lab/v1\"\n")
	writeConfig(t, dir, config.ConfigFileName, "[models]\ndecision = \"lab/d\"\ntext = \"other/t\"\n")
	got := runProviders(t, configureProvidersOptions{providerOptions: namedLayer(dir), Name: "lab", Remove: true,
		StoredKeys: labKey()}, modeltest.NewScriptedMock(nil, nil))
	if got.code != 0 || got.doc.Outcome != "done" {
		t.Fatalf("exit %d, %v", got.code, got.doc.Values)
	}
	next := anyStrings(got.doc.Values["next"])
	if len(next) != 2 || !strings.Contains(next[0], "decision") || !strings.Contains(next[0], "lab/d") ||
		!strings.Contains(next[0], "kbase configure-models --decision") || !strings.Contains(next[1], "kbase auth remove lab") {
		t.Errorf("next = %q, want the decision role with its replacing call, then kbase auth remove lab", next)
	}
	if strings.Contains(got.stderr+strings.Join(next, ""), testAPIKey) {
		t.Error("the key reached the result or stderr")
	}
}

// TestConfigureProvidersRemovalNamesAKeyThatDoesNotOpen: a key file stored
// under the removed provider's name that does not open is still named, with
// the call that clears it.
func TestConfigureProvidersRemovalNamesAKeyThatDoesNotOpen(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, config.ProvidersFileName, "[lab]\nbaseUrl = \"http://lab/v1\"\n")
	store := config.StoredKeys{Faults: []config.StoreFault{{Name: "lab", Path: "/store/lab-key.enc", Reason: "truncated", DoesNotOpen: true}}}
	got := runProviders(t, configureProvidersOptions{providerOptions: namedLayer(dir), Name: "lab", Remove: true, StoredKeys: store},
		modeltest.NewScriptedMock(nil, nil))
	next := anyStrings(got.doc.Values["next"])
	if got.code != 0 || len(next) != 1 || !strings.Contains(next[0], "does not open") || !strings.Contains(next[0], "kbase auth remove lab") {
		t.Errorf("exit %d, next = %q; want one entry naming the key that does not open and kbase auth remove lab", got.code, next)
	}
}

// writeConfig writes a configuration file fixture into dir and returns its
// path.
func writeConfig(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
