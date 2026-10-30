package config

import (
	"path/filepath"
	"reflect"
	"testing"
)

// TestReport: a report describes the layers it read with what each holds,
// every usable pool entry and every dropped provider with the layer each came
// from, and the inference variables the process sets — with no key material and
// no value.
func TestReport(t *testing.T) {
	repoDir, globalDir := filepath.Join("repo", RepositoryDirName), filepath.Join("home", ".config", "kbase")
	repoProviders, globalProviders := ProvidersPath(repoDir), ProvidersPath(globalDir)
	layers := []Layer{{Path: repoDir, Source: LayerSourceRepository}, {Path: globalDir, Source: LayerSourceGlobal}}

	for _, tc := range []struct {
		name      string
		cfg       Config
		providers Providers
		faults    []ProviderFault
		exists    func(string) bool
		getenv    func(string) string
		want      Facts
	}{
		{
			name:   "nothing loaded",
			exists: func(string) bool { return false },
			getenv: func(string) string { return "" },
			want: Facts{
				Layers: []Layer{
					{Path: repoDir, Source: LayerSourceRepository},
					{Path: globalDir, Source: LayerSourceGlobal},
				},
			},
		},
		{
			name: "a pool with one fault of each kind",
			cfg:  Config{Provider: "reaper", Models: ModelMap{Decision: "lab/clef", Text: "lab/qwen"}},
			providers: Providers{
				"reaper": {Name: "reaper", BaseURL: "https://reaper.example/v1", APIKeyFile: "keys/reaper.txt", Layer: LayerSourceRepository,
					DecisionsFlavor: FlavorDecisions},
				"warned": {Name: "warned", BaseURL: "https://warned.example/v1", APIKeyFile: "keys/warned.txt", Layer: LayerSourceGlobal},
				"nokey":  {Name: "nokey", BaseURL: "https://nokey.example/v1", Layer: LayerSourceRepository},
			},
			faults: []ProviderFault{
				{Name: "warned", Path: globalProviders, Reason: "apiKeyFile /k/warned.txt is readable beyond its owner (mode 0644) — chmod 600 it", Warning: true},
				{Name: "broken", Path: repoProviders, Reason: "no baseUrl declared"},
			},
			exists: func(path string) bool { return path == repoProviders },
			getenv: func(name string) string {
				switch name {
				case EnvAPIBaseURL:
					return " https://env.example/v1 "
				case EnvText:
					return "qwen"
				case EnvAPIKeyFile:
					return "  "
				case EnvConfigDir:
					return "/elsewhere"
				default:
					return ""
				}
			},
			want: Facts{
				Layers: []Layer{
					{Path: repoDir, Source: LayerSourceRepository, ProvidersExists: true},
					{Path: globalDir, Source: LayerSourceGlobal},
				},
				Models: ModelMap{Decision: "lab/clef", Text: "lab/qwen"},
				Entries: []Entry{
					{Name: "broken", Layer: LayerSourceRepository, Fault: "no baseUrl declared"},
					{Name: "nokey", BaseURL: "https://nokey.example/v1", Protocol: ProviderAPIAnthropic, DecisionsFlavor: FlavorSystemOne, Key: KeyKindNone, Layer: LayerSourceRepository, Usable: true},
					{Name: "reaper", BaseURL: "https://reaper.example/v1", Protocol: ProviderAPIAnthropic, DecisionsFlavor: FlavorDecisions, Key: KeyKindFile, KeyFile: "keys/reaper.txt", Layer: LayerSourceRepository, Usable: true},
					{Name: "warned", BaseURL: "https://warned.example/v1", Protocol: ProviderAPIAnthropic, DecisionsFlavor: FlavorSystemOne, Key: KeyKindFile, KeyFile: "keys/warned.txt", Layer: LayerSourceGlobal, Usable: true,
						Fault: "apiKeyFile /k/warned.txt is readable beyond its owner (mode 0644) — chmod 600 it"},
				},
				Environment: []string{EnvAPIBaseURL, EnvText},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Report(tc.cfg, tc.providers, tc.faults, layers, tc.exists, tc.getenv); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Report = %+v\nwant %+v", got, tc.want)
			}
		})
	}
}
