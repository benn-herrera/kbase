package main

import (
	"os"
	"slices"
	"strings"
	"testing"

	"kbase/internal/config"
	"kbase/internal/model"
)

// TestRunConfigureRewritesAnOldGrammarFile is criterion 9: one dialed call
// over a file in the old grammar removes the superseded provider key, writes
// all three references, and changes every other byte of the file not at all.
func TestRunConfigureRewritesAnOldGrammarFile(t *testing.T) {
	const old = `# kept note
provider = "solo"

[models]
decision = "clef"   # the letter asks
text = "Qwen3.8-27B"

[asks]
readerConcurrency = 2
`
	const want = `# kept note

[models]
decision = "solo/clef"  # the letter asks
text = "solo/Qwen3.8-27B"
scoring = "solo/clef-scoring"

[asks]
readerConcurrency = 2
`
	dir := t.TempDir()
	path := config.ConfigPath(dir)
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}

	opts := soloOpts()
	opts.WriteDir = dir
	opts.Provider = "solo"
	opts.ModelMap = refs(decisionID, textID, scoringID)
	got := runConfig(t, opts, model.NewScriptedMock(nil, catalogue(decisionID, textID, scoringID)))
	if got.code != 0 || got.doc.Outcome != "done" {
		t.Fatalf("configure: exit %d, %v", got.code, got.doc.Values)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != want {
		t.Errorf("file =\n%s\nwant\n%s", body, want)
	}
	// The reference the file's own scoring may keep is not read here: the
	// flag named one.
	if models, _ := got.doc.Values["models"].(map[string]any); models[config.CategoryScoring] != ref(scoringID) {
		t.Errorf("models = %v, want the flag's scoring reference", models)
	}
}

// TestRunConfigureKeepsTheFilesScoring is the settled rule: a dialed run that
// leaves scoring unassigned leaves the target file's own value untouched where
// it holds one — so the same command again writes nothing — and refuses naming
// models.scoring where the file holds none.
func TestRunConfigureKeepsTheFilesScoring(t *testing.T) {
	const kept = `[models]
decision = "solo/clef"
text = "solo/Qwen3.8-27B"
scoring = "solo/clef-scoring"
`
	dir := t.TempDir()
	path := config.ConfigPath(dir)
	if err := os.WriteFile(path, []byte(kept), 0o600); err != nil {
		t.Fatal(err)
	}
	opts := soloOpts()
	opts.WriteDir = dir
	opts.ModelMap = []string{"decision=" + ref(decisionID) + ",text=" + ref(textID)}
	got := runConfig(t, opts, model.NewScriptedMock(nil, catalogue(decisionID, textID, scoringID)))
	if got.code != 0 || got.doc.Outcome != "unchanged" || len(anyStrings(got.doc.Values["written"])) != 0 {
		t.Fatalf("a run that leaves scoring standing = %s, written %v; want unchanged, nothing written",
			got.doc.Outcome, got.doc.Values["written"])
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != kept {
		t.Errorf("file =\n%s\nwant it untouched", body)
	}

	// The same run over a file with no scoring reference has nothing to keep,
	// and it is the key it names.
	bare := t.TempDir()
	barePath := config.ConfigPath(bare)
	if err := os.WriteFile(barePath, []byte("[models]\ndecision = \"solo/clef\"\ntext = \"solo/Qwen3.8-27B\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	refuse := soloOpts()
	refuse.WriteDir = bare
	refuse.ModelMap = opts.ModelMap
	got = runConfig(t, refuse, model.NewScriptedMock(nil, catalogue(decisionID, textID, scoringID)))
	items := got.doc.items("refusals")
	if got.code != 1 || got.doc.Outcome != "refused" || len(items) != 1 ||
		items[0]["key"] != "models.scoring" || items[0]["check"] != checkDetect {
		t.Fatalf("a run with nothing to keep for scoring = %s, %v; want models.scoring refused",
			got.doc.Outcome, items)
	}
	if body, err := os.ReadFile(barePath); err != nil || !strings.Contains(string(body), "decision = \"solo/clef\"") {
		t.Errorf("the refused run wrote %s (%v)", body, err)
	}
}

// TestRunConfigureDialsEachProviderItsReferencesName: references naming two
// entries are checked against each entry's own catalogue, one request each,
// and need no --provider even where the pool holds more than one.
func TestRunConfigureDialsEachProviderItsReferencesName(t *testing.T) {
	opts := configureOptions{providerOptions: providerOptions{
		WriteDir: t.TempDir(),
		Providers: config.Providers{
			"a": provider("a", "http://a.example/v1"),
			"b": provider("b", "http://b.example/v1"),
		},
	}}
	opts.ModelMap = []string{"decision=a/d,text=b/t,scoring=a/s"}
	var dialled []string
	opts.NewClient = func(e model.Endpoint) model.Client {
		dialled = append(dialled, e.BaseURL)
		return model.NewScriptedMock(nil, catalogue("d", "t", "s"))
	}
	got := runConfig(t, opts, nil)
	if got.code != 0 || got.doc.Outcome != "done" {
		t.Fatalf("configure over two referenced providers: exit %d, %v", got.code, got.doc.Values)
	}
	if want := []string{"http://a.example/v1", "http://b.example/v1"}; !slices.Equal(dialled, want) {
		t.Errorf("dialled %q, want each referenced provider once in role order", dialled)
	}
	cfg, written := got.config(t)
	want := config.ModelMap{Decision: "a/d", Text: "b/t", Scoring: "a/s"}
	if !written || cfg.Models != want || cfg.Provider != "" {
		t.Errorf("config.toml = %+v (%t), want %+v and no provider", cfg, written, want)
	}
	// No single entry was selected — the references named two — so the
	// document states none, as a null rather than by leaving the key out.
	if !slices.Contains(got.doc.Keys, "provider") || got.doc.Values["provider"] != nil {
		t.Errorf("keys %q, provider = %v: want the key present and null for a run naming two entries",
			got.doc.Keys, got.doc.Values["provider"])
	}
}
