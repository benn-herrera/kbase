package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"kbase/internal/result"
)

// Model ids a configure run assigns, one per category.
const (
	decisionID = "clef"
	textID     = "Qwen3.8-27B"
	scoringID  = "clef-scoring"
)

// soloRef is id as a reference to the solo pool entry.
func soloRef(id string) Reference { return Reference{Provider: "solo", Model: id} }

// soloRefs is the three-category map at the solo entry.
func soloRefs(decision, text, scoring string) map[string]Reference {
	return map[string]Reference{CategoryDecision: soloRef(decision), CategoryText: soloRef(text), CategoryScoring: soloRef(scoring)}
}

// soloRequest is a configure run writing into dir over a pool of one entry,
// solo, as the merge hands it over: its layer stamped.
func soloRequest(dir string, models map[string]Reference) ConfigureRequest {
	return ConfigureRequest{Models: models, Inputs: Inputs{WriteDir: dir, Providers: Providers{
		"solo": {Name: "solo", BaseURL: "http://provider.example/v1", Layer: LayerSourceRepository}}}}
}

// listing is a CatalogueFunc serving ids for every entry; asked is each entry it
// was called for, in order.
func listing(ids ...string) (catalogue CatalogueFunc, asked *[]string) {
	var names []string
	return func(entry Provider) ([]string, error) {
		names = append(names, entry.Name)
		return ids, nil
	}, &names
}

// unlisted is a CatalogueFunc that fails the test on any call.
func unlisted(t *testing.T) CatalogueFunc {
	t.Helper()
	return func(Provider) ([]string, error) {
		t.Error("the run read a catalogue")
		return nil, errors.New("no catalogue was expected")
	}
}

// configFile is the config.toml in dir as LoadConfig reads it, and whether it
// stands.
func configFile(t *testing.T, dir string) (Config, bool) {
	t.Helper()
	path := ConfigPath(dir)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return Config{}, false
	} else if err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	return cfg, true
}

// fileText is the file at path, which must stand.
func fileText(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// TestConfigureRefusesUnassignedCategories: no category is guessed — a category
// the map does not name is refused, one item per missing category naming its
// models.<category> key, listing the selected provider's catalogue sorted as the
// choices, and nothing is written.
func TestConfigureRefusesUnassignedCategories(t *testing.T) {
	ids := []string{"text-embedding-3-large", decisionID, textID, "mistral-small-3"}
	sorted := slices.Sorted(slices.Values(ids))
	for _, tc := range []struct {
		name    string
		models  map[string]Reference
		refused []Role
	}{
		{"no map", nil, Roles},
		{"a partial map", map[string]Reference{CategoryDecision: soloRef(decisionID)}, []Role{RoleText, RoleScoring}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			catalogue, _ := listing(ids...)
			_, err := Configure(soloRequest(dir, tc.models), catalogue)
			var refusal result.Refusal
			if !errors.As(err, &refusal) || len(refusal) != len(tc.refused) {
				t.Fatalf("Configure = %v, want one refusal item per category in %v", err, tc.refused)
			}
			for i, role := range tc.refused {
				item := refusal[i]
				if item.Check != CheckDetect || item.Key != "models."+string(role) || !slices.Equal(item.Allowed, sorted) ||
					item.Remedy != "kbase configure --model-map "+string(role)+"=solo/<id>" ||
					!strings.Contains(item.Detail, "no reference assigned to the "+string(role)+" category") {
					t.Errorf("refusal %d = %+v, want models.%s offering the catalogue", i, item, role)
				}
			}
			if _, written := configFile(t, dir); written {
				t.Error("config.toml written despite unassigned categories")
			}
		})
	}
}

// TestConfigureWritesTheThreeReferences: a full map is checked against the
// selected entry's catalogue and written to config.toml, in a directory the run
// creates; the run names the entry, the rule that selected it and the layer that
// supplied it. The same run again finds the file holding its values and writes
// nothing.
func TestConfigureWritesTheThreeReferences(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "kbase")
	const pinned = "some-org/clef-flash-q4"
	req := soloRequest(dir, soloRefs(pinned, textID, scoringID))
	catalogue, _ := listing(decisionID, textID, "deepseek-v4.1")

	run, err := Configure(req, catalogue)
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	want := ModelMap{Decision: "solo/" + pinned, Text: "solo/" + textID, Scoring: "solo/" + scoringID}
	if !run.Selected || run.Provider != "solo" || run.Rule != RuleSoleEntry || run.Layer != LayerSourceRepository {
		t.Errorf("selected %v, provider %q by %q from %q; want solo by the sole-entry rule from the layer that supplies it",
			run.Selected, run.Provider, run.Rule, run.Layer)
	}
	if run.Models == nil || *run.Models != want || !slices.Equal(run.Written, []string{ConfigPath(dir)}) {
		t.Errorf("models %+v, written %v; want %+v and config.toml", run.Models, run.Written, want)
	}
	if cfg, written := configFile(t, dir); !written || cfg != (Config{Models: want}) {
		t.Errorf("config.toml = %+v (%t), want %+v", cfg, written, want)
	}

	again, err := Configure(req, catalogue)
	if err != nil || len(again.Written) != 0 {
		t.Errorf("the same run again wrote %v (%v), want nothing written", again.Written, err)
	}
}

// TestConfigureRecordsACatalogueMiss: a mapped model its provider's catalogue
// omits is recorded, not refused — catalogues rotate, and a user pinning one the
// listing omits may know something the listing does not — and the run writes.
func TestConfigureRecordsACatalogueMiss(t *testing.T) {
	dir := t.TempDir()
	catalogue, _ := listing(textID, scoringID)
	run, err := Configure(soloRequest(dir, soloRefs("clef-preview", textID, scoringID)), catalogue)
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if want := []CatalogueMiss{{Category: CategoryDecision, Reference: soloRef("clef-preview")}}; !slices.Equal(run.Misses, want) {
		t.Errorf("misses %+v, want %+v", run.Misses, want)
	}
	if len(run.Written) == 0 {
		t.Error("a catalogue miss wrote nothing; it is configured anyway")
	}
}

// TestConfigureRewritesAnOldGrammarFile: one dialed call over a file in the old
// grammar writes all three references and changes every other byte of the file
// — a hand-written comment included — not at all.
func TestConfigureRewritesAnOldGrammarFile(t *testing.T) {
	const old = `# kept note

[models]
decision = "clef"   # the letter asks
text = "Qwen3.8-27B"
`
	const want = `# kept note

[models]
decision = "solo/clef"  # the letter asks
text = "solo/Qwen3.8-27B"
scoring = "solo/clef-scoring"
`
	dir := t.TempDir()
	if err := os.WriteFile(ConfigPath(dir), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	req := soloRequest(dir, soloRefs(decisionID, textID, scoringID))
	req.Provider = "solo"
	catalogue, _ := listing(decisionID, textID, scoringID)
	run, err := Configure(req, catalogue)
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if body := fileText(t, ConfigPath(dir)); body != want {
		t.Errorf("file =\n%s\nwant\n%s", body, want)
	}
	// The reference the file's own scoring may keep is not read here: the map
	// named one.
	if run.Models == nil || run.Models.Scoring != soloRef(scoringID).String() {
		t.Errorf("models %+v, want the map's scoring reference", run.Models)
	}
}

// TestConfigureWritesNothingWhereTheListingFails: a catalogue that cannot be
// read is the run's error, returned as the listing gave it, and the previous
// configuration outlives it — which is why the write is the last step.
func TestConfigureWritesNothingWhereTheListingFails(t *testing.T) {
	dir := t.TempDir()
	unavailable := errors.New("http 503: service unavailable")
	_, err := Configure(soloRequest(dir, soloRefs(decisionID, textID, scoringID)), func(Provider) ([]string, error) {
		return nil, unavailable
	})
	if !errors.Is(err, unavailable) {
		t.Errorf("Configure = %v, want the listing's own error", err)
	}
	if _, written := configFile(t, dir); written {
		t.Error("config.toml written despite a list failure")
	}
}

// TestConfigureOfflineWritesBothFiles: Create with no provider reads no
// catalogue and selects no entry, lands both files in a directory that did not
// exist — the pool, then the choices; the same run
// again finds both holding what it would write and writes nothing.
func TestConfigureOfflineWritesBothFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fresh")
	req := soloRequest(dir, soloRefs(decisionID, textID, scoringID))
	req.Create = true

	run, err := Configure(req, unlisted(t))
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if run.Selected || run.Provider != "" || run.Rule != "" || run.Layer != "" {
		t.Errorf("selected %v, provider %q by %q from %q; want no entry: the run dialed nothing", run.Selected, run.Provider, run.Rule, run.Layer)
	}
	want := ModelMap{Decision: soloRef(decisionID).String(), Text: soloRef(textID).String(), Scoring: soloRef(scoringID).String()}
	if run.Models == nil || *run.Models != want {
		t.Errorf("models %+v, want the three references the map named", run.Models)
	}
	if written := []string{ProvidersPath(dir), ConfigPath(dir)}; !slices.Equal(run.Written, written) {
		t.Errorf("written = %v, want %v: the pool, then the choices", run.Written, written)
	}
	pool, faults, err := LoadProviders(ProvidersPath(dir), nil)
	if err != nil || len(faults) != 0 || len(pool) != 1 {
		t.Fatalf("the created pool = %v, faults %v, err %v; want one usable entry", keysOf(pool), faults, err)
	}
	if cfg, written := configFile(t, dir); !written || cfg.Models != want {
		t.Errorf("config.toml = %+v (%t), want the references", cfg, written)
	}

	again, err := Configure(req, unlisted(t))
	if err != nil || len(again.Written) != 0 {
		t.Errorf("the same run again wrote %v (%v), want nothing written", again.Written, err)
	}
}

// TestConfigureOfflineKeepsWhatStandsThere: Create is permission to create, not
// to replace. A config.toml holding an unmentioned category and a hand-written
// comment keeps both, and a standing providers.toml is left exactly as it was.
func TestConfigureOfflineKeepsWhatStandsThere(t *testing.T) {
	dir := t.TempDir()
	const note = "# this box reaches the lab"
	pool := "[solo]\nbaseUrl = \"http://kept.example/v1\"\n"
	if err := os.WriteFile(ConfigPath(dir), []byte(note+"\n\n[models]\ndecision = \"kept\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ProvidersPath(dir), []byte(pool), 0o600); err != nil {
		t.Fatal(err)
	}
	req := soloRequest(dir, map[string]Reference{CategoryText: soloRef(textID)})
	req.Create = true
	run, err := Configure(req, unlisted(t))
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if cfg, _ := configFile(t, dir); cfg.Models.Decision != "kept" || cfg.Models.Text != soloRef(textID).String() {
		t.Errorf("config.toml = %+v, want the standing decision kept and text set", cfg)
	}
	if body := fileText(t, ConfigPath(dir)); !strings.Contains(body, note) {
		t.Errorf("config.toml, want the hand-written comment kept:\n%s", body)
	}
	if kept := fileText(t, ProvidersPath(dir)); kept != pool {
		t.Errorf("providers.toml = %q, want it untouched", kept)
	}
	if !slices.Equal(run.Written, []string{ConfigPath(dir)}) {
		t.Errorf("written = %v, want config.toml alone: providers.toml already stood", run.Written)
	}
}

// TestConfigureRefusesAProviderThePoolLacks: a dialed run is refused before it
// writes, so a provider the pool does not hold is refused against the target
// layer's own providers.toml — the file the entry belongs in — with the creation
// remedy, and neither file is created.
func TestConfigureRefusesAProviderThePoolLacks(t *testing.T) {
	dir := t.TempDir()
	req := soloRequest(dir, soloRefs(decisionID, textID, scoringID))
	req.Create, req.Provider = true, "typo"
	catalogue, _ := listing(decisionID, textID, scoringID)
	_, err := Configure(req, catalogue)
	var refusal result.Refusal
	if !errors.As(err, &refusal) || len(refusal) != 1 {
		t.Fatalf("Configure = %v, want one refusal", err)
	}
	if item := refusal[0]; item.Check != CheckProvider || item.Key != "--provider" || item.Path != ProvidersPath(dir) ||
		item.Remedy != "kbase configure --create" {
		t.Errorf("refusal = %+v, want --provider refused against %s with the creation remedy", item, ProvidersPath(dir))
	}
	for _, path := range []string{ProvidersPath(dir), ConfigPath(dir)} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s written despite the refusal", path)
		}
	}
}

// TestConfigureKeepsTheFilesScoring: a dialed run that leaves scoring unassigned
// leaves the target file's own value untouched where it holds one — so the run
// writes nothing — and is refused naming models.scoring where the file holds
// none, writing nothing.
func TestConfigureKeepsTheFilesScoring(t *testing.T) {
	models := map[string]Reference{CategoryDecision: soloRef(decisionID), CategoryText: soloRef(textID)}
	catalogue, _ := listing(decisionID, textID, scoringID)

	const kept = "[models]\ndecision = \"solo/clef\"\ntext = \"solo/Qwen3.8-27B\"\nscoring = \"solo/clef-scoring\"\n"
	dir := t.TempDir()
	if err := os.WriteFile(ConfigPath(dir), []byte(kept), 0o600); err != nil {
		t.Fatal(err)
	}
	run, err := Configure(soloRequest(dir, models), catalogue)
	if err != nil || len(run.Written) != 0 {
		t.Fatalf("a run that leaves scoring standing wrote %v (%v), want nothing written", run.Written, err)
	}
	if body := fileText(t, ConfigPath(dir)); body != kept {
		t.Errorf("file =\n%s\nwant it untouched", body)
	}

	// The same run over a file with no scoring reference has nothing to keep,
	// and it is the key it names.
	const bare = "[models]\ndecision = \"solo/clef\"\ntext = \"solo/Qwen3.8-27B\"\n"
	bareDir := t.TempDir()
	if err := os.WriteFile(ConfigPath(bareDir), []byte(bare), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Configure(soloRequest(bareDir, models), catalogue)
	var refusal result.Refusal
	if !errors.As(err, &refusal) || len(refusal) != 1 || refusal[0].Key != "models.scoring" || refusal[0].Check != CheckDetect {
		t.Fatalf("a run with nothing to keep for scoring = %v, want models.scoring refused", err)
	}
	if body := fileText(t, ConfigPath(bareDir)); body != bare {
		t.Errorf("the refused run wrote:\n%s", body)
	}
}

// TestConfigureChecksEachReferenceAgainstItsOwnProvider: references naming two
// entries are checked against each entry's own catalogue, one listing each in
// role order, and need no selected provider where the pool holds more than one:
// the run settles on no entry and writes no provider key.
func TestConfigureChecksEachReferenceAgainstItsOwnProvider(t *testing.T) {
	dir := t.TempDir()
	req := ConfigureRequest{Inputs: Inputs{WriteDir: dir, Providers: Providers{
		"a": {Name: "a", BaseURL: "http://a.example/v1", Layer: LayerSourceRepository},
		"b": {Name: "b", BaseURL: "http://b.example/v1", Layer: LayerSourceRepository},
	}}, Models: map[string]Reference{
		CategoryDecision: {Provider: "a", Model: "d"}, CategoryText: {Provider: "b", Model: "t"}, CategoryScoring: {Provider: "a", Model: "s"},
	}}
	catalogue, asked := listing("d", "t", "s")
	run, err := Configure(req, catalogue)
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if want := []string{"a", "b"}; !slices.Equal(*asked, want) {
		t.Errorf("listed %q, want each referenced provider once in role order", *asked)
	}
	if !run.Selected || run.Provider != "" {
		t.Errorf("selected %v, provider %q; want the run settled on no one entry", run.Selected, run.Provider)
	}
	want := ModelMap{Decision: "a/d", Text: "b/t", Scoring: "a/s"}
	if cfg, written := configFile(t, dir); !written || cfg.Models != want {
		t.Errorf("config.toml = %+v (%t), want %+v", cfg, written, want)
	}
}

// TestParseModelMap covers the flag grammar, including the forms that must
// be rejected rather than half-understood.
func TestParseModelMap(t *testing.T) {
	for _, tc := range []struct {
		name    string
		values  []string
		want    map[string]Reference
		wantErr string
	}{{
		name:   "empty",
		values: nil,
		want:   map[string]Reference{},
	}, {
		name:   "comma-separated pair",
		values: []string{"decision=a/d,text=b/t"},
		want:   map[string]Reference{CategoryDecision: {Provider: "a", Model: "d"}, CategoryText: {Provider: "b", Model: "t"}},
	}, {
		name:   "repeated flag accumulates",
		values: []string{"decision=a/d", "text=a/t"},
		want:   map[string]Reference{CategoryDecision: {Provider: "a", Model: "d"}, CategoryText: {Provider: "a", Model: "t"}},
	}, {
		name:   "surrounding space and category case are tolerated",
		values: []string{" DECISION = a/d , text=a/t "},
		want:   map[string]Reference{CategoryDecision: {Provider: "a", Model: "d"}, CategoryText: {Provider: "a", Model: "t"}},
	}, {
		name:   "a model id may carry a slash of its own",
		values: []string{"scoring=openrouter/anthropic/x"},
		want:   map[string]Reference{CategoryScoring: {Provider: "openrouter", Model: "anthropic/x"}},
	}, {
		name:   "repeating one category with the same reference is not a conflict",
		values: []string{"decision=a/d", "decision=a/d"},
		want:   map[string]Reference{CategoryDecision: {Provider: "a", Model: "d"}},
	}, {
		name:    "no assignment",
		values:  []string{"decision"},
		wantErr: "malformed --model-map entry",
	}, {
		name:    "missing value",
		values:  []string{"decision="},
		wantErr: "malformed --model-map entry",
	}, {
		name:    "a bare id is not a reference",
		values:  []string{"decision=clef"},
		wantErr: "a value is a reference",
	}, {
		name:    "unknown category",
		values:  []string{"medium=a/m"},
		wantErr: `unknown category "medium"`,
	}, {
		name:    "conflicting assignments to one category",
		values:  []string{"decision=a/d,decision=a/e"},
		wantErr: "assigns the decision category twice",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseModelMap(tc.values)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected an error containing %q, got %v", tc.wantErr, got)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error: got %v, want substring %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseModelMap: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("map = %v, want %v", got, tc.want)
			}
		})
	}
}
