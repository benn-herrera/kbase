package config

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"kbase/internal/argmark"
	"kbase/internal/result"
)

// soloInputs is a pool of one entry, solo, as the merge hands it over: its name
// folded to lowercase, however the table spells it.
func soloInputs(dir string) Inputs {
	return Inputs{WriteDir: dir, Providers: Providers{
		"solo": {Name: "solo", BaseURL: "http://provider.example/v1", Layer: LayerSourceGlobal}}}
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

// fileText is the file at path, which must stand.
func fileText(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// TestConfigureModelsKeepsEveryOtherByte: the roles given are rewritten in
// place and the roles not given, every comment, every other table and key
// stand byte for byte; the provider part folds to lowercase, matching its entry
// whatever its case, and is written so.
func TestConfigureModelsKeepsEveryOtherByte(t *testing.T) {
	dir := t.TempDir()
	const src = `# my notes
[models]
decision = "Solo/old"  # keep this note
text='Solo/hand-quoted'

[asks]
concurrency = 2
`
	path := writeFile(t, dir, ConfigFileName, src)
	catalogue, asked := listing("new")
	run, err := ConfigureModels(soloInputs(dir), "", map[Role]string{RoleDecision: "SOLO/new"}, catalogue)
	if err != nil {
		t.Fatalf("ConfigureModels: %v", err)
	}
	want := strings.Replace(src, `decision = "Solo/old"  # keep this note`, `decision = "solo/new"  # keep this note`, 1)
	if got := fileText(t, path); got != want {
		t.Errorf("file =\n%s\nwant\n%s", got, want)
	}
	if !slices.Equal(run.Written, []string{path}) || run.Models == nil ||
		*run.Models != (ModelMap{Decision: "solo/new", Text: "Solo/hand-quoted"}) {
		t.Errorf("run = %+v (models %+v), want the file written and its [models] reported", run, run.Models)
	}
	if !slices.Equal(*asked, []string{"solo"}) || len(run.Misses) != 0 {
		t.Errorf("asked %v, misses %v: want one listing and no miss", *asked, run.Misses)
	}
	// What the writer writes, the resolver resolves; so does the hand-written
	// reference spelled in another case.
	in := soloInputs(dir)
	in.Cfg.Models = *run.Models
	for _, role := range []Role{RoleDecision, RoleText} {
		if ep, err := Resolve(in, role, ""); err != nil || ep.Provider != "solo" {
			t.Errorf("Resolve(%s) = %+v, %v; want solo's", role, ep, err)
		}
	}

	again, err := ConfigureModels(soloInputs(dir), "", map[Role]string{RoleDecision: "solo/new"}, catalogue)
	if err != nil || len(again.Written) != 0 {
		t.Errorf("the same run again: written %v, %v; want nothing written", again.Written, err)
	}
}

// TestConfigureModelsCreatesTheFile: an absent config.toml is created from the
// template holding the roles given, the others blank.
func TestConfigureModelsCreatesTheFile(t *testing.T) {
	dir := t.TempDir()
	catalogue, _ := listing("t")
	run, err := ConfigureModels(soloInputs(dir), "", map[Role]string{RoleText: "solo/t"}, catalogue)
	if err != nil || len(run.Written) != 1 {
		t.Fatalf("ConfigureModels: written %v, %v", run.Written, err)
	}
	cfg, err := LoadConfig(ConfigPath(dir))
	if err != nil || cfg.Models != (ModelMap{Text: "solo/t"}) {
		t.Errorf("config = %+v, %v; want the text role alone", cfg.Models, err)
	}
	if !strings.Contains(fileText(t, ConfigPath(dir)), "safe to hand-edit") {
		t.Error("the created file carries no template header")
	}
}

// TestConfigureModelsWarnsOnAMiss: a model its catalogue omits is a miss, and
// the reference is written all the same.
func TestConfigureModelsWarnsOnAMiss(t *testing.T) {
	dir := t.TempDir()
	catalogue, _ := listing("listed")
	run, err := ConfigureModels(soloInputs(dir), "", map[Role]string{RoleScoring: "solo/unlisted"}, catalogue)
	if err != nil {
		t.Fatalf("ConfigureModels: %v", err)
	}
	if len(run.Misses) != 1 || run.Misses[0].Role != RoleScoring || run.Misses[0].Reference != (Reference{"solo", "unlisted"}) {
		t.Errorf("misses = %+v, want the scoring reference", run.Misses)
	}
	if run.Models == nil || run.Models.Scoring != "solo/unlisted" {
		t.Errorf("models = %+v, want the reference written", run.Models)
	}
}

// TestConfigureModelsRefuses: a run naming no role, a value that is no
// reference and a provider with no entry are refused before any listing, each
// keyed by its flag, and nothing is written.
func TestConfigureModelsRefuses(t *testing.T) {
	for _, tc := range []struct {
		name  string
		given map[Role]string
		keys  []string
	}{
		{"no role", nil, []string{""}},
		{"no reference", map[Role]string{RoleDecision: "bare", RoleText: "solo/"}, []string{argmark.Flag("decision"), argmark.Flag("text")}},
		{"no entry", map[Role]string{RoleText: "solo/t", RoleScoring: "nosuch/m"}, []string{argmark.Flag("scoring")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			_, err := ConfigureModels(soloInputs(dir), "", tc.given, func(Provider) ([]string, error) {
				t.Error("a refused run listed a catalogue")
				return nil, nil
			})
			var refusal result.Refusal
			if !errors.As(err, &refusal) || len(refusal) != len(tc.keys) {
				t.Fatalf("err = %v, want %d items", err, len(tc.keys))
			}
			for i, key := range tc.keys {
				if refusal[i].Key != key {
					t.Errorf("item %d key = %q, want %q", i, refusal[i].Key, key)
				}
			}
			if _, err := os.Stat(ConfigPath(dir)); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("a refused run wrote config.toml (%v)", err)
			}
		})
	}
}

// TestConfigureModelsNoEntryRemedy: a provider with no entry is refused,
// against the written layer's providers.toml, with the configure-providers
// call that adds it, carrying the `--layer` the call carried.
func TestConfigureModelsNoEntryRemedy(t *testing.T) {
	for _, layer := range []string{"", LayerSourceRepository, LayerSourceGlobal} {
		t.Run(layer, func(t *testing.T) {
			dir := t.TempDir()
			_, err := ConfigureModels(soloInputs(dir), layer, map[Role]string{RoleText: "NoSuch/m"}, nil)
			var refusal result.Refusal
			if !errors.As(err, &refusal) || len(refusal) != 1 {
				t.Fatalf("err = %v, want one item", err)
			}
			args := map[string]any{"provider": "nosuch", "base-url": "<url>"}
			if layer != "" {
				args["layer"] = layer
			}
			if want := argmark.Call("configure-providers", args); refusal[0].Remedy != want || refusal[0].Path != ProvidersPath(dir) {
				t.Errorf("remedy = %q, path %q; want %q, %q", refusal[0].Remedy, refusal[0].Path, want, ProvidersPath(dir))
			}
		})
	}
}

// TestConfigureModelsFailedListing: a listing that fails is returned, and
// nothing is written.
func TestConfigureModelsFailedListing(t *testing.T) {
	dir := t.TempDir()
	unreachable := errors.New("http 503")
	_, err := ConfigureModels(soloInputs(dir), "", map[Role]string{RoleText: "solo/t"}, func(Provider) ([]string, error) {
		return nil, unreachable
	})
	if !errors.Is(err, unreachable) {
		t.Fatalf("err = %v, want the listing's", err)
	}
	if _, err := os.Stat(ConfigPath(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a failed run wrote config.toml (%v)", err)
	}
}

// TestProviderEntryRefusals: an unusable name, a base URL that is no absolute
// http(s) URL or carries credentials, and an api or decisionApi the pool
// would drop are each refused, keyed by its argument; a usable entry is not.
// No refusal of a base URL names any part of it.
func TestProviderEntryRefusals(t *testing.T) {
	for _, tc := range []struct {
		name, provider, baseURL, api, decisionAPI string
		keys                                      []string
		unnamed                                   []string
	}{
		{"usable", "lab", "http://lab:8000/v1", "openai", "decisions", nil, nil},
		{"defaults", "lab_2", "https://x.example/v1", "", "", nil, nil},
		{"a name alone", "lab", "", "", "", nil, nil},
		{"name", "-lab", "http://lab/v1", "", "", []string{argmark.Positional("provider")}, nil},
		{"base url", "lab", "lab:8000", "", "", []string{argmark.Flag("base-url")}, []string{"lab:8000"}},
		{"user and key", "lab", "https://alice:s3cret@host/v1", "", "", []string{argmark.Flag("base-url")}, []string{"alice", "s3cret", "host"}},
		{"key alone", "lab", "https://s3cret@host/v1", "", "", []string{argmark.Flag("base-url")}, []string{"s3cret", "host"}},
		{"api and decision api", "lab", "http://lab/v1", "grpc", "other", []string{argmark.Flag("api"), argmark.Flag("decision-api")}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			refusal := ProviderEntryRefusals(tc.provider, tc.baseURL, tc.api, tc.decisionAPI)
			var keys []string
			for _, item := range refusal {
				keys = append(keys, item.Key)
				for _, part := range tc.unnamed {
					if strings.Contains(item.Detail, part) || strings.Contains(item.Remedy, part) {
						t.Errorf("item %+v names %q of the base URL", item, part)
					}
				}
			}
			if !slices.Equal(keys, tc.keys) {
				t.Errorf("keys = %q, want %q", keys, tc.keys)
			}
		})
	}
}

// TestProviderEntryRefusalsCredentials: a base URL carrying credentials is a
// usage refusal whose remedy stores the key, its detail ending with the note
// on who runs that remedy and that the key stays out of the conversation.
func TestProviderEntryRefusalsCredentials(t *testing.T) {
	refusal := ProviderEntryRefusals("lab", "https://alice:s3cret@host/v1", "", "")
	if len(refusal) != 1 || refusal[0].Check != result.CheckUsage || refusal[0].Remedy != AuthAddRemedy("lab") ||
		!strings.HasSuffix(refusal[0].Detail, RefusedKeyNote) {
		t.Errorf("refusal = %+v, want one usage item whose remedy is %q and whose detail ends with the refused-key note", refusal, AuthAddRemedy("lab"))
	}
}
