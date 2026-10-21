package config

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// writeHarnesses writes a harnesses.toml fixture and returns its path.
func writeHarnesses(t *testing.T, dir, body string) string {
	t.Helper()
	return writeFile(t, dir, HarnessesFileName, body)
}

// TestLoadHarnesses: a profile is one top-level table, keyed by its name, an
// open set; its command is the argv verbatim, an empty element included; a
// custom name is a profile like any other; and a file that does not stand holds
// no profile.
func TestLoadHarnesses(t *testing.T) {
	dir := t.TempDir()
	path := writeHarnesses(t, dir, `
[claude]
command = ["claude", "-p", "--tools", "", "--strict-mcp-config", "--safe-mode"]

[my-ask]
command = ["/opt/ask", "--prompt", "{prompt}"]
`)
	profiles, err := LoadHarnesses(path)
	if err != nil {
		t.Fatalf("LoadHarnesses: %v", err)
	}
	if got := ProfileNames(profiles); !slices.Equal(got, []string{"claude", "my-ask"}) {
		t.Fatalf("profiles = %q, want the two names sorted", got)
	}
	claude := profiles["claude"]
	want := []string{"claude", "-p", "--tools", "", "--strict-mcp-config", "--safe-mode"}
	if !slices.Equal(claude.Command, want) {
		t.Errorf("claude's command = %q, want %q", claude.Command, want)
	}
	if claude.Layer != "" {
		t.Errorf("a loader-set layer = %q, want the merge's to fill it", claude.Layer)
	}
	if got := profiles["my-ask"].Command; !slices.Equal(got, []string{"/opt/ask", "--prompt", "{prompt}"}) {
		t.Errorf("my-ask's command = %q", got)
	}

	empty, err := LoadHarnesses(HarnessesPath(t.TempDir()))
	if err != nil || len(empty) != 0 {
		t.Errorf("a missing file = %v, %v; want no profiles and no error", empty, err)
	}
}

// TestLoadHarnessesRefuses: a field the schema does not model, a profile
// naming no command, and malformed TOML each refuse the whole load, naming the
// file.
func TestLoadHarnessesRefuses(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
	}{
		{"an unknown field", "[claude]\ncommnd = [\"claude\"]\n", `"claude.commnd"`},
		{"an unknown field beside a good one", "[a]\ncommand = [\"x\"]\n[b]\ncommand = [\"y\"]\ntools = true\n", `"b.tools"`},
		{"no command", "[claude]\n", `"claude" profile names no command`},
		{"an empty command", "[claude]\ncommand = []\n", `"claude" profile names no command`},
		{"the nested form", "[harness.claude]\ncommand = [\"claude\"]\n", `"harness.claude.command"`},
		{"malformed toml", "[claude\ncommand = [\"claude\"]\n", "parse"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeHarnesses(t, t.TempDir(), tc.body)
			_, err := LoadHarnesses(path)
			if err == nil {
				t.Fatalf("LoadHarnesses(%s) = nil, want a refusal", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), path) {
				t.Errorf("the refusal = %q, want it to name %s and the file", err, tc.want)
			}
		})
	}
}

// TestLoadProfilesLayerWhole: profiles layer as the pool does — the
// repository's file over the global one, a same-named profile replaced whole,
// and each profile naming the layer whose file supplied it. A malformed
// harnesses.toml in a layer refuses the whole load.
func TestLoadProfilesLayerWhole(t *testing.T) {
	globalDir, repoDir := t.TempDir(), t.TempDir()
	writeFile(t, globalDir, HarnessesFileName, "[shared]\ncommand = [\"global\", \"{prompt}\"]\n\n[inherited]\ncommand = [\"inherited\"]\n")
	writeFile(t, repoDir, HarnessesFileName, "[shared]\ncommand = [\"repo\"]\n\n[local]\ncommand = [\"local\"]\n")

	loaded := mustLoad(t, []Layer{
		{Path: repoDir, Source: LayerSourceRepository},
		{Path: globalDir, Source: LayerSourceGlobal},
	})
	if got := ProfileNames(loaded.Harnesses); !slices.Equal(got, []string{"inherited", "local", "shared"}) {
		t.Fatalf("profiles = %q, want the union of both layers", got)
	}
	shared := loaded.Harnesses["shared"]
	if !slices.Equal(shared.Command, []string{"repo"}) {
		t.Errorf("shared = %q, want the repository's command whole — not the global {prompt} element beside it", shared.Command)
	}
	for name, want := range map[string]string{"shared": LayerSourceRepository, "local": LayerSourceRepository, "inherited": LayerSourceGlobal} {
		if got := loaded.Harnesses[name].Layer; got != want {
			t.Errorf("%s.Layer = %q, want %q", name, got, want)
		}
	}
	// A replaced entry names the layer it displaced; an added one names none.
	for name, want := range map[string]string{"shared": LayerSourceGlobal, "local": "", "inherited": ""} {
		if got := loaded.Harnesses[name].ReplacedLayer; got != want {
			t.Errorf("%s.ReplacedLayer = %q, want %q", name, got, want)
		}
	}

	writeFile(t, repoDir, HarnessesFileName, "[local]\ncommnd = [\"local\"]\n")
	if _, err := Load([]Layer{
		{Path: repoDir, Source: LayerSourceRepository},
		{Path: globalDir, Source: LayerSourceGlobal},
	}); err == nil {
		t.Error("a malformed repository harnesses.toml loaded; want the whole load refused")
	}
}

// TestCreateHarnessesFile: the template lands where nothing stands — one live
// profile, `claude`, taking its prompt on stdin with its tools removed, and
// `opencode` commented with the limitation of its command stated — and a file
// that stands is never rewritten, so a seed a user deleted stays deleted.
func TestCreateHarnessesFile(t *testing.T) {
	path := HarnessesPath(t.TempDir())
	wrote, err := CreateHarnessesFile(path)
	if err != nil || !wrote {
		t.Fatalf("CreateHarnessesFile = %t, %v; want the template written", wrote, err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, want := range []string{
		`[claude]`,
		`command = ["claude", "-p", "--tools", "", "--strict-mcp-config", "--safe-mode"]`,
		`# command = ["opencode", "run", "--format", "default", "{prompt}"]`,
		"has no flag that removes tools or",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the template does not carry %q:\n%s", want, text)
		}
	}
	profiles, err := LoadHarnesses(path)
	if err != nil {
		t.Fatalf("the template does not load: %v", err)
	}
	if got := ProfileNames(profiles); !slices.Equal(got, []string{"claude"}) {
		t.Errorf("the template loads %q, want claude alone: opencode ships commented", got)
	}
	if got := profiles["claude"].Command; !slices.Equal(got, []string{"claude", "-p", "--tools", "", "--strict-mcp-config", "--safe-mode"}) {
		t.Errorf("the live profile's command = %q", got)
	}

	// A file that stands is the user's: nothing is added to it, and the seed
	// it no longer carries is not restored.
	mine := "[only-mine]\ncommand = [\"mine\"]\n"
	if err := os.WriteFile(path, []byte(mine), 0o600); err != nil {
		t.Fatal(err)
	}
	wrote, err = CreateHarnessesFile(path)
	if err != nil || wrote {
		t.Errorf("CreateHarnessesFile over an existing file = %t, %v; want nothing written", wrote, err)
	}
	if b, err := os.ReadFile(path); err != nil || string(b) != mine {
		t.Errorf("the existing file = %q, %v; want it untouched", b, err)
	}
}

// TestProfileCarriesModel: the token is what decides whether kbase can name the
// model a profile asks — the one case a document's model is null though the
// role is configured.
func TestProfileCarriesModel(t *testing.T) {
	for _, tc := range []struct {
		name    string
		command []string
		want    bool
	}{
		{"the model as its own element", []string{"ask", "--model", "{model}", "{prompt}"}, true},
		{"the model inside an element", []string{"ask", "--model={model}"}, true},
		{"the prompt alone", []string{"ask", "{prompt}"}, false},
		{"the seed", []string{"claude", "-p", "--tools", "", "--safe-mode"}, false},
	} {
		if got := (Profile{Name: "p", Command: tc.command}).CarriesModel(); got != tc.want {
			t.Errorf("%s: CarriesModel = %t, want %t", tc.name, got, tc.want)
		}
	}
}
