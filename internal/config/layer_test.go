package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// homeDir is the directory the global layer resolves under, with HOME pointed
// at a temp directory first so no test reads the operator's own.
func homeDir(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home directory in this environment: %v", err)
	}
	return home
}

// globalPath is the global layer's directory under home.
func globalPath(home string) string { return filepath.Join(home, defaultDirParent, defaultDirName) }

// TestLayers: a directory a flag or the environment names is the only layer
// read — an explicit choice isolates the run from a repository's own .kbase —
// and otherwise the repository layer stands over the global one, or, outside a
// worktree, the global one alone. A blank or whitespace-only setting counts as
// unset.
func TestLayers(t *testing.T) {
	home := homeDir(t)
	global := Layer{Path: globalPath(home), Source: LayerSourceGlobal}
	flagDir, envDir := t.TempDir(), t.TempDir()
	repo := t.TempDir()
	for _, tc := range []struct {
		name, flag, env, repoRoot string
		want                      []Layer
	}{
		{"the flag alone", flagDir, envDir, repo, []Layer{{Path: flagDir, Source: LayerSourceExplicit}}},
		{"the environment when the flag is blank", "   ", envDir, repo, []Layer{{Path: envDir, Source: LayerSourceEnvironment}}},
		{"the repository over the global", "", "", repo, []Layer{
			{Path: filepath.Join(repo, RepositoryDirName), Source: LayerSourceRepository}, global,
		}},
		{"the global alone outside a worktree", "", "   ", "", []Layer{global}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(EnvConfigDir, tc.env)
			got, err := Layers(tc.flag, tc.repoRoot)
			if err != nil {
				t.Fatalf("Layers: %v", err)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("Layers(%q, %q) = %+v, want %+v", tc.flag, tc.repoRoot, got, tc.want)
			}
		})
	}
}

// TestWriteTarget: an explicit layer is written whether or not it exists — a
// flag is not second-guessed; a repository --project named is written whether
// or not its directory stands, over the directory the environment named;
// otherwise the repository layer where its directory already stands; otherwise
// the global layer.
func TestWriteTarget(t *testing.T) {
	repo := t.TempDir()
	standing := filepath.Join(repo, RepositoryDirName)
	if err := os.MkdirAll(standing, 0o755); err != nil {
		t.Fatal(err)
	}
	absent := filepath.Join(t.TempDir(), RepositoryDirName)
	project := t.TempDir()
	for _, tc := range []struct {
		name        string
		layers      []Layer
		projectRoot string
		want        string
	}{
		{"an explicit layer, absent", []Layer{{Path: "/somewhere", Source: LayerSourceExplicit}}, "", "/somewhere"},
		{"an environment layer, absent", []Layer{{Path: "/elsewhere", Source: LayerSourceEnvironment}}, "", "/elsewhere"},
		{
			name:   "the repository layer, standing",
			layers: []Layer{{Path: standing, Source: LayerSourceRepository}, {Path: "/home/.config/kbase", Source: LayerSourceGlobal}},
			want:   standing,
		},
		{
			name:   "the repository layer, absent",
			layers: []Layer{{Path: absent, Source: LayerSourceRepository}, {Path: "/home/.config/kbase", Source: LayerSourceGlobal}},
			want:   "/home/.config/kbase",
		},
		{"no repository layer", []Layer{{Path: "/home/.config/kbase", Source: LayerSourceGlobal}}, "", "/home/.config/kbase"},
		{
			name:        "a repository --project names, absent",
			layers:      []Layer{{Path: absent, Source: LayerSourceRepository}, {Path: "/home/.config/kbase", Source: LayerSourceGlobal}},
			projectRoot: project,
			want:        filepath.Join(project, RepositoryDirName),
		},
		{
			name:        "a repository --project names, over the environment",
			layers:      []Layer{{Path: "/elsewhere", Source: LayerSourceEnvironment}},
			projectRoot: project,
			want:        filepath.Join(project, RepositoryDirName),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := WriteTarget(tc.layers, tc.projectRoot); got != tc.want {
				t.Errorf("WriteTarget = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestLoadMerges: the repository layer's pool entry replaces a same-named
// global one whole — the global entry's key file is not inherited — while
// config.toml's fields overlay one by one, each model id and each entry
// naming the layer that supplied it.
func TestLoadMerges(t *testing.T) {
	globalDir, repoDir := t.TempDir(), t.TempDir()
	writeFile(t, globalDir, "shared.key", fakeKey)
	writeProviders(t, globalDir, `
[shared]
baseUrl = "http://shared.example/v1"
apiKeyFile = "shared.key"

[inherited]
baseUrl = "http://inherited.example/v1"
`)
	writeFile(t, globalDir, ConfigFileName, `
[models]
decision = "global-decision"
text = "global-text"

[asks]
concurrency = 3
`)
	writeProviders(t, repoDir, `
[shared]
baseUrl = "http://shared.example/v2"

[local]
baseUrl = "http://local.example/v1"
`)
	writeFile(t, repoDir, ConfigFileName, "[models]\ndecision = \"repo-decision\"\n")

	layers := []Layer{
		{Path: repoDir, Source: LayerSourceRepository},
		{Path: globalDir, Source: LayerSourceGlobal},
	}
	loaded, err := Load(layers)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	providers, faults, cfg := loaded.Providers, loaded.Faults, loaded.Cfg
	if len(faults) != 0 {
		t.Fatalf("a healthy stack reported faults: %+v", faults)
	}
	if names := keysOf(providers); len(names) != 3 {
		t.Errorf("pool = %v, want the union of both layers", names)
	}
	shared := providers["shared"]
	if shared.BaseURL != "http://shared.example/v2" || shared.APIKeyFile != "" || shared.APIKey != "" {
		t.Errorf("shared = %+v, want the repository entry whole: its URL, and no key inherited from the global entry",
			Provider{BaseURL: shared.BaseURL, APIKeyFile: shared.APIKeyFile})
	}
	for name, wantLayer := range map[string]string{
		"shared": LayerSourceRepository, "local": LayerSourceRepository, "inherited": LayerSourceGlobal,
	} {
		if got := providers[name].Layer; got != wantLayer {
			t.Errorf("%s.Layer = %q, want %q", name, got, wantLayer)
		}
	}
	want := Config{Models: ModelMap{Decision: "repo-decision", Text: "global-text"},
		Sources: Sources{Decision: LayerSourceRepository, Text: LayerSourceGlobal}}
	if cfg.Models != want.Models || cfg.Sources != want.Sources {
		t.Errorf("config = %+v (%+v), want %+v", cfg, cfg.Sources, want)
	}
	if n := cfg.Asks.Concurrency; n == nil || *n != 3 {
		t.Errorf("concurrency = %v, want the global layer's 3 inherited", n)
	}
}

// TestLoadOneLayer: a directory a flag or the environment named is the whole
// stack, and every entry and model id names that one layer's source.
func TestLoadOneLayer(t *testing.T) {
	dir := t.TempDir()
	writeProviders(t, dir, "[solo]\nbaseUrl = \"http://solo.example/v1\"\n")
	writeFile(t, dir, ConfigFileName, "[models]\ndecision = \"d\"\ntext = \"t\"\n")

	loaded, err := Load([]Layer{{Path: dir, Source: LayerSourceExplicit}})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	providers, cfg := loaded.Providers, loaded.Cfg
	if got := providers["solo"].Layer; got != LayerSourceExplicit {
		t.Errorf("solo.Layer = %q, want %q", got, LayerSourceExplicit)
	}
	if cfg.Sources != (Sources{Decision: LayerSourceExplicit, Text: LayerSourceExplicit}) {
		t.Errorf("config sources = %+v, want %q for both categories", cfg.Sources, LayerSourceExplicit)
	}
}

// TestLoadFaultNamesItsFile: a fault names the providers.toml that carries it,
// so a repository entry that dropped is not reported against the global file.
func TestLoadFaultNamesItsFile(t *testing.T) {
	globalDir, repoDir := t.TempDir(), t.TempDir()
	writeProviders(t, globalDir, "[ok]\nbaseUrl = \"http://ok.example/v1\"\n")
	repoPath := writeProviders(t, repoDir, "[broken]\n")

	faults := mustLoad(t, []Layer{
		{Path: repoDir, Source: LayerSourceRepository},
		{Path: globalDir, Source: LayerSourceGlobal},
	}).Faults
	if len(faults) != 1 || faults[0].Name != "broken" || faults[0].Path != repoPath {
		t.Fatalf("faults = %+v, want the dropped repository entry naming %s", faults, repoPath)
	}
}

// mustLoad is Load for a test that has already checked it succeeds.
func mustLoad(t *testing.T, layers []Layer) Loaded {
	t.Helper()
	loaded, err := Load(layers)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return loaded
}

// TestLoadDropsSupersededFault: a fault naming an entry a higher layer already
// supplies is not reported — the pool's entry is the higher one, so a
// diagnostic about a value nobody reads is not a fact about this run. A fault
// from the layer that supplies the entry stands: it never entered the pool, so
// the lower layer's entry of that name is the one in use.
func TestLoadDropsSupersededFault(t *testing.T) {
	const valid = `
[local]
baseUrl = "http://local.example/v1"
apiKeyFile = "local.key"
`
	const broken = `
[local]
baseUrl = "http://local.example/v1"
apiKeyFile = "not-here.key"
`
	for _, tc := range []struct {
		name        string
		repoBroken  bool // the repository entry is the one that drops
		wantLayer   string
		wantFaults  int
		wantDropped int // dropped-provider records the report carries
	}{
		{"the repository supersedes a broken global entry", false, LayerSourceRepository, 0, 0},
		{"a broken repository entry never supersedes the global one", true, LayerSourceGlobal, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repoDir, globalDir := t.TempDir(), t.TempDir()
			writeFile(t, repoDir, "local.key", fakeKey)
			writeFile(t, globalDir, "local.key", fakeKey)
			repoBody, globalBody := valid, broken
			if tc.repoBroken {
				repoBody, globalBody = broken, valid
			}
			repoPath := writeProviders(t, repoDir, repoBody)
			globalPath := writeProviders(t, globalDir, globalBody)
			brokenPath := globalPath
			if tc.repoBroken {
				brokenPath = repoPath
			}

			layers := []Layer{
				{Path: repoDir, Source: LayerSourceRepository},
				{Path: globalDir, Source: LayerSourceGlobal},
			}
			loaded, err := Load(layers)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			providers, faults, cfg := loaded.Providers, loaded.Faults, loaded.Cfg
			if len(providers) != 1 || providers["local"].Layer != tc.wantLayer {
				t.Fatalf("pool = %+v, want the one local entry from %s", providers, tc.wantLayer)
			}
			if len(faults) != tc.wantFaults {
				t.Fatalf("faults = %+v, want %d", faults, tc.wantFaults)
			}
			if tc.wantFaults > 0 && (faults[0].Name != "local" || faults[0].Path != brokenPath) {
				t.Errorf("fault = %+v, want the dropped repository entry naming %s", faults[0], brokenPath)
			}

			facts := Report(cfg, providers, faults, layers, func(string) bool { return false })
			usable, dropped := 0, 0
			for _, e := range facts.Entries {
				switch {
				case e.Name != "local":
					t.Errorf("report entry %+v, want only local", e)
				case e.Usable && e.Layer == tc.wantLayer:
					usable++
				case !e.Usable:
					dropped++
				default:
					t.Errorf("usable entry from %q, want %q", e.Layer, tc.wantLayer)
				}
			}
			if usable != 1 || dropped != tc.wantDropped {
				t.Errorf("report carries %d usable and %d dropped local records, want 1 and %d",
					usable, dropped, tc.wantDropped)
			}
		})
	}
}

// TestLoadRefusesTheStack: a layer that fails the strict load refuses the whole
// load by the highest layer that fails, and returns no partial configuration —
// a broken repository file never falls through to the global one.
func TestLoadRefusesTheStack(t *testing.T) {
	globalDir, repoDir := t.TempDir(), t.TempDir()
	globalConfig := writeFile(t, globalDir, ConfigFileName, "[models]\n")
	repoConfig := writeFile(t, repoDir, ConfigFileName, "provdier = \"b\"\n")

	loaded, err := Load([]Layer{
		{Path: repoDir, Source: LayerSourceRepository},
		{Path: globalDir, Source: LayerSourceGlobal},
	})
	if err == nil || !strings.Contains(err.Error(), repoConfig) || strings.Contains(err.Error(), globalConfig) {
		t.Fatalf("Load = %v, want the repository file refused and the global one unnamed", err)
	}
	if loaded.Providers != nil || loaded.Faults != nil || loaded.Cfg != (Config{}) {
		t.Errorf("a refused load returned a partial configuration: %+v", loaded)
	}
}

// TestLoadMergesSampling: each sampling field overlays on its own, so the
// repository layer's thinking overrides the global one while the global
// temperature is kept, and every value names the layer that supplied it.
func TestLoadMergesSampling(t *testing.T) {
	globalDir, repoDir := t.TempDir(), t.TempDir()
	writeFile(t, globalDir, ConfigFileName, "[sampling.text]\ntemperature = 0.7\nthinking = \"low\"\n\n[sampling.scoring]\nthinking = \"high\"\n")
	writeFile(t, repoDir, ConfigFileName, "[sampling.text]\nthinking = \"medium\"\n\n[sampling.scoring]\ntemperature = 0.2\n")

	cfg := mustLoad(t, []Layer{
		{Path: repoDir, Source: LayerSourceRepository},
		{Path: globalDir, Source: LayerSourceGlobal},
	}).Cfg
	if tt := cfg.Sampling.Text.Temperature; tt == nil || *tt != 0.7 || cfg.Sampling.Text.Thinking != "medium" {
		t.Errorf("text sampling = %+v, want the global 0.7 under the repository's medium", cfg.Sampling.Text)
	}
	if st := cfg.Sampling.Scoring.Temperature; st == nil || *st != 0.2 || cfg.Sampling.Scoring.Thinking != "high" {
		t.Errorf("scoring sampling = %+v, want the repository 0.2 over the global high", cfg.Sampling.Scoring)
	}
	for _, tc := range []struct {
		role                  Role
		temperature, thinking string
	}{
		{RoleText, LayerSourceGlobal, LayerSourceRepository},
		{RoleScoring, LayerSourceRepository, LayerSourceGlobal},
		{RoleDecision, "", ""},
	} {
		if temperature, thinking := cfg.SamplingLayer(tc.role); temperature != tc.temperature || thinking != tc.thinking {
			t.Errorf("SamplingLayer(%s) = %q, %q; want %q, %q", tc.role, temperature, thinking, tc.temperature, tc.thinking)
		}
	}
}
