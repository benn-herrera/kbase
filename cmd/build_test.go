package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/config"
	"kbase/internal/log"
	"kbase/internal/model"
	"kbase/internal/model/modeltest"
	"kbase/internal/modelclient"
	toolresult "kbase/internal/result"
)

// roleField is one key's value in a result document's role record, nil where
// the record carries none.
func roleField(record toolresult.Record, key string) any {
	for _, f := range record {
		if f.Key == key {
			return f.Value
		}
	}
	return nil
}

// roleOf is the role record a document states for role, or nil.
func roleOf(doc resultDoc, role string) map[string]any {
	for _, record := range doc.items("roles") {
		if record["role"] == role {
			return record
		}
	}
	return nil
}

// buildRoles is the roles record a build writes over cfgDir, whose two roles
// name two different providers, and the endpoints it dialled.
func buildRoles(t *testing.T) (doc resultDoc, dialled []model.Endpoint) {
	t.Helper()
	repo, state := buildRepo(t)
	cfgDir := t.TempDir()
	writeValues(t, cfgDir, config.ProvidersFileName, `[lab]
baseUrl = "http://lab.example/v1"

[cloud]
baseUrl = "http://cloud.example/v1"
decisionsFlavor = "decisions"
`)
	writeValues(t, cfgDir, config.ConfigFileName, "[models]\ndecision = \"lab/d3\"\ntext = \"cloud/clef\"\n")

	saved := flagConfigDir
	t.Cleanup(func() { flagConfigDir = saved })
	flagConfigDir = cfgDir
	opts, err := loadVerbContext(repo)
	if err != nil {
		t.Fatalf("loading the staged configuration: %v", err)
	}
	// A client that answers nothing: the walk fails its items, and what this
	// test reads is the document the run wrote anyway.
	newClient := func(e model.Endpoint) model.Client {
		dialled = append(dialled, e)
		return modeltest.NewScriptedMock(nil, nil).Client()
	}
	var stdout, stderr bytes.Buffer
	opts.Stderr = &stderr
	if _, err := runBuild(context.Background(), buildOptions{
		VolumeRoots: []string{"paper.tex"}, Through: "claims-discovered", StateDir: state, WorkDir: repo,
		ConfigDir: cfgDir, NewClient: newClient, Provider: buildProvider(opts, &stderr),
		Stdout: &stdout, Stderr: &stderr, Logger: log.Discard(),
	}); err != nil {
		t.Fatalf("build: %v\n%s", err, stderr.String())
	}
	return checkDocument(t, "build", stdout.String()), dialled
}

// TestBuildDocumentStatesOneRolePerEndpoint: a build's document carries `roles`
// where the endpoint's provenance stood — after state-dir — and no top-level
// provider, provider-rule or provider-layer at all: a build whose two roles name
// two different providers cannot state one. Each record is §7.1's, in role
// order, and each role's asks reach its own provider's client.
func TestBuildDocumentStatesOneRolePerEndpoint(t *testing.T) {
	doc, dialled := buildRoles(t)
	keys := slices.DeleteFunc(slices.Clone(doc.Keys), func(k string) bool { return k == "outcome" })
	if len(keys) < 3 || !slices.Equal(keys[:3], []string{"kb-root", "state-dir", "roles"}) {
		t.Errorf("keys %q, want roles after state-dir", doc.Keys)
	}
	for _, key := range []string{"provider", "provider-rule", "provider-layer"} {
		if _, ok := doc.Values[key]; ok {
			t.Errorf("a build's document carries the top-level key %s: %v", key, doc.Values[key])
		}
	}

	roles := doc.items("roles")
	if len(roles) != 2 || roles[0]["role"] != "decision" || roles[1]["role"] != "text" {
		t.Fatalf("roles = %v, want decision then text", roles)
	}
	for _, want := range []struct{ role, provider, model string }{
		{"decision", "lab", "d3"},
		{"text", "cloud", "clef"},
	} {
		got := roleOf(doc, want.role)
		if got == nil {
			t.Fatalf("no %s record in %v", want.role, roles)
		}
		for key, value := range map[string]any{"provider": want.provider,
			"provider-rule": "config", "provider-layer": "explicit", "model": want.model} {
			if got[key] != value {
				t.Errorf("%s's %s = %v, want %v", want.role, key, got[key], value)
			}
		}
	}

	// One client per provider, each dialled on the entry its own role named,
	// and no prompt-cache key on any of them: only a scoring run carries one.
	var urls []string
	for _, e := range dialled {
		urls = append(urls, e.Name+" "+e.BaseURL)
		if e.PromptCacheKey != "" {
			t.Errorf("the build's client for %s carries the prompt-cache key %q; a build sends none", e.Name, e.PromptCacheKey)
		}
	}
	slices.Sort(urls)
	if want := []string{"cloud http://cloud.example/v1", "lab http://lab.example/v1"}; !slices.Equal(urls, want) {
		t.Errorf("endpoints dialled = %q, want %q, one per role", urls, want)
	}
}

// TestBuildDialsOneEntryBothRolesNameOnce: two roles referencing one OpenAI
// pool entry are one endpoint, dialled once with the key its apiKeyFile holds; each role's
// record names that entry; and a key bound for a non-loopback http:// endpoint
// is one line on the verb's stderr, naming no key.
func TestBuildDialsOneEntryBothRolesNameOnce(t *testing.T) {
	cfgDir := t.TempDir()
	writeValues(t, cfgDir, "key.txt", "secret-key\n")
	if err := os.Chmod(filepath.Join(cfgDir, "key.txt"), 0o600); err != nil {
		t.Fatal(err)
	}
	// The entry speaks OpenAI chat, the decision surface's protocol, so both
	// roles reach one endpoint; an entry speaking Messages is two.
	writeValues(t, cfgDir, config.ProvidersFileName, "[lab]\nbaseUrl = \"http://reaper.local:4000/v1\"\napiKeyFile = \"key.txt\"\napi = \"openai\"\n")
	writeValues(t, cfgDir, config.ConfigFileName, "[models]\ndecision = \"lab/Some-Decision\"\ntext = \"lab/Some-Text\"\n")
	saved := flagConfigDir
	t.Cleanup(func() { flagConfigDir = saved })
	flagConfigDir = cfgDir

	providers, err := loadVerbContext(t.TempDir())
	if err != nil {
		t.Fatalf("loading the staged configuration: %v", err)
	}
	var stderr bytes.Buffer
	var dialled []model.Endpoint
	providers.NewClient = func(e model.Endpoint) model.Client {
		dialled = append(dialled, e)
		return modeltest.NewScriptedMock(nil, nil).Client()
	}
	p, err := buildProvider(providers, &stderr)(modelclient.New(providers.NewClient, nil, "", 0))
	if err != nil {
		t.Fatalf("provider = %v", err)
	}
	if p.Multichoice != "Some-Decision" || p.Overview != "Some-Text" {
		t.Errorf("provider = %+v, want each role's own model", p)
	}
	if len(dialled) != 1 || dialled[0].BaseURL != "http://reaper.local:4000/v1" || dialled[0].APIKey != "secret-key" {
		t.Errorf("endpoints dialled = %+v, want the one entry, once, with its key file read", dialled)
	}
	if len(p.Roles) != 2 {
		t.Fatalf("roles = %v, want one per role the build asks", p.Roles)
	}
	for i, id := range []string{"Some-Decision", "Some-Text"} {
		for key, want := range map[string]any{"provider": "lab", "provider-rule": config.RuleConfig, "model": id} {
			if got := roleField(p.Roles[i], key); got != want {
				t.Errorf("roles[%d] %s = %v, want %v", i, key, got, want)
			}
		}
	}
	if lines := strings.Split(strings.TrimSpace(stderr.String()), "\n"); len(lines) != 1 || !strings.Contains(lines[0], "cleartext") || strings.Contains(stderr.String(), "secret-key") {
		t.Errorf("stderr = %q, want one cleartext line naming no key", stderr.String())
	}
}

// TestBuildRefusesTheRoleWhoseProviderThePoolLacks: a role whose reference
// names an entry the pool does not hold refuses by that role's key and that
// provider's name — never as "the provider" — before any stage is walked.
func TestBuildRefusesTheRoleWhoseProviderThePoolLacks(t *testing.T) {
	for _, role := range []config.Role{config.RoleDecision, config.RoleText} {
		models := config.ModelMap{Decision: "lab/d3", Text: "lab/clef"}
		if role == config.RoleDecision {
			models.Decision = "absent/d3"
		} else {
			models.Text = "absent/clef"
		}
		var stderr bytes.Buffer
		providers := providerOptions{
			Providers: config.Providers{"lab": provider("lab", "http://lab.example/v1")},
			Config:    config.Config{Models: models},
			Stderr:    &stderr,
		}
		_, err := buildProvider(providers, &stderr)(modelclient.New(nil, nil, "", 0))
		var refusal toolresult.Refusal
		if !errors.As(err, &refusal) || len(refusal) != 1 || refusal[0].Key != "models."+string(role) ||
			refusal[0].Check != config.CheckProvider || !strings.Contains(refusal[0].Detail, `provider "absent"`) {
			t.Errorf("%s's absent provider = %v, want a refusal keyed models.%s naming %q", role, err, role, "absent")
		}
	}
}

// TestBuildResumeNamesTheConfigDirAbsolute: a relative --config-dir is the
// directory where the flag was typed, so a build refused after recording a
// boundary hands out a resume naming it absolute.
func TestBuildResumeNamesTheConfigDirAbsolute(t *testing.T) {
	repo, state := buildRepoWith(t, "\\documentclass{article}\n\\title{A Paper}\n\\begin{document}\n\\maketitle\n"+
		"\\section{Results}\n\\input{missing.tex}\nSome prose.\n\\end{document}\n")
	cfgDir := filepath.Join(repo, "cfg")
	if err := os.Mkdir(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	out, _ := kbase(t, repo, "build", "paper.tex", "--config-dir", "cfg", "--no-inference", "--state-dir", state)
	doc := checkDocument(t, "build", out)
	resume, _ := doc.Values["resume"].(string)
	if doc.Outcome != toolresult.Refused || !strings.HasPrefix(resume, "kbase build --resume ") {
		t.Fatalf("build = %s, resume %q; want refused inside a stage with a resume\n%s", doc.Outcome, resume, out)
	}
	_, named, found := strings.Cut(resume, "--config-dir ")
	named, _, _ = strings.Cut(named, " ")
	typed, err := os.Stat(cfgDir)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.Stat(named); !found || !filepath.IsAbs(named) || err != nil || !os.SameFile(got, typed) {
		t.Errorf("resume = %q names --config-dir %q, want %s absolute", resume, named, cfgDir)
	}
}

// TestBuildDocumentOmitsRolesWithoutInference: a build spending no inference
// resolved no endpoint, so it names no role at all.
func TestBuildDocumentOmitsRolesWithoutInference(t *testing.T) {
	repo, state := buildRepo(t)
	cfgDir := t.TempDir()
	saved := flagConfigDir
	t.Cleanup(func() { flagConfigDir = saved })
	flagConfigDir = cfgDir
	opts, err := loadVerbContext(repo)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	opts.Stderr = &stderr
	if _, err := runBuild(context.Background(), buildOptions{
		VolumeRoots: []string{"paper.tex"}, Through: "spine-seed", StateDir: state, WorkDir: repo,
		NoInference: true, ConfigDir: cfgDir, Provider: buildProvider(opts, &stderr),
		Stdout: &stdout, Stderr: &stderr, Logger: log.Discard(),
	}); err != nil {
		t.Fatal(err)
	}
	doc := checkDocument(t, "build", stdout.String())
	if _, ok := doc.Values["roles"]; ok {
		t.Errorf("a build spending no inference carries roles: %v", doc.Values["roles"])
	}
}
