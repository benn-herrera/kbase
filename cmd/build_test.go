package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"kbase/internal/config"
	"kbase/internal/log"
	"kbase/internal/model"
	"kbase/internal/model/modeltest"
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
	for _, name := range []string{config.EnvAPIBaseURL, config.EnvDecision, config.EnvText, config.EnvScoring, config.EnvAPIKeyFile} {
		t.Setenv(name, "")
	}
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
		return modeltest.NewScriptedMock(nil, nil)
	}
	var stdout, stderr bytes.Buffer
	opts.Stderr = &stderr
	if _, err := runBuild(context.Background(), buildOptions{
		VolumeRoots: []string{"paper.tex"}, Through: "claims-discovered", StateDir: state, WorkDir: repo,
		ConfigDir: cfgDir, NewClient: newClient, Provider: buildProvider(opts, config.EnvOverrides(os.Getenv), &stderr),
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
		for key, value := range map[string]any{"back-end": "provider", "provider": want.provider,
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
		_, err := buildProvider(providers, config.Overrides{}, &stderr)(newClients(nil, nil, ""))
		var refusal config.Refusal
		if !errors.As(err, &refusal) || refusal.Key != "models."+string(role) || !strings.Contains(refusal.Detail, `provider "absent"`) {
			t.Errorf("%s's absent provider = %v, want a refusal keyed models.%s naming %q", role, err, role, "absent")
		}
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
		NoInference: true, ConfigDir: cfgDir, Provider: buildProvider(opts, config.EnvOverrides(os.Getenv), &stderr),
		Stdout: &stdout, Stderr: &stderr, Logger: log.Discard(),
	}); err != nil {
		t.Fatal(err)
	}
	doc := checkDocument(t, "build", stdout.String())
	if _, ok := doc.Values["roles"]; ok {
		t.Errorf("a build spending no inference carries roles: %v", doc.Values["roles"])
	}
}
