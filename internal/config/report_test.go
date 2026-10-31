package config

import (
	"path/filepath"
	"reflect"
	"testing"

	"kbase/internal/result"
)

// TestReport: a report describes the layers it read with what each holds,
// every usable pool entry and every dropped provider with the layer each came
// from, the upgrade of a key read from a file, and a diagnostic per fault and
// per key file a stored key leaves unread — with no key material.
func TestReport(t *testing.T) {
	repoDir, globalDir := filepath.Join("repo", RepositoryDirName), filepath.Join("home", ".config", "kbase")
	repoProviders, globalProviders := ProvidersPath(repoDir), ProvidersPath(globalDir)
	layers := []Layer{{Path: repoDir, Source: LayerSourceRepository}, {Path: globalDir, Source: LayerSourceGlobal}}
	const (
		storeFile   = "/state/kbase/auth/lab-key.enc"
		truncated   = "too short to hold a stored key: the file is truncated"
		noUsername  = "the OS reports no username"
		storeDir    = "/state/kbase/auth"
		exposedWarn = "key file /k/warned.txt, named in providers.toml, is readable beyond its owner (mode 0644) — chmod 600 it"
		unreadKey   = "read the key file providers.toml names: open keys/gone.txt: no such file or directory"
	)

	for _, tc := range []struct {
		name      string
		cfg       Config
		providers Providers
		faults    []ProviderFault
		stored    []StoreFault
		exists    func(string) bool
		want      Facts
	}{
		{
			name:   "nothing loaded",
			exists: func(string) bool { return false },
			want: Facts{
				Layers: []Layer{
					{Path: repoDir, Source: LayerSourceRepository},
					{Path: globalDir, Source: LayerSourceGlobal},
				},
			},
		},
		{
			name: "a pool with one fault of each kind",
			cfg:  Config{Models: ModelMap{Decision: "lab/clef", Text: "lab/qwen"}},
			providers: Providers{
				"reaper": {Name: "reaper", BaseURL: "https://reaper.example/v1", APIKeyFile: "keys/reaper.txt", Layer: LayerSourceRepository,
					DecisionsFlavor: FlavorDecisions},
				"warned": {Name: "warned", BaseURL: "https://warned.example/v1", APIKeyFile: "/k/warned.txt", Layer: LayerSourceGlobal},
				"nokey":  {Name: "nokey", BaseURL: "https://nokey.example/v1", Layer: LayerSourceRepository},
			},
			faults: []ProviderFault{
				{Name: "warned", Path: globalProviders, Reason: exposedWarn, Warning: true},
				{Name: "broken", Path: repoProviders, Reason: "no baseUrl declared"},
				{Name: "gone", Path: repoProviders, Reason: unreadKey, KeyFileUnread: true},
			},
			exists: func(path string) bool { return path == repoProviders },
			want: Facts{
				Layers: []Layer{
					{Path: repoDir, Source: LayerSourceRepository, ProvidersExists: true},
					{Path: globalDir, Source: LayerSourceGlobal},
				},
				Models: ModelMap{Decision: "lab/clef", Text: "lab/qwen"},
				Entries: []Entry{
					{Name: "broken", Layer: LayerSourceRepository, Fault: "no baseUrl declared"},
					{Name: "gone", Layer: LayerSourceRepository, Fault: unreadKey},
					{Name: "nokey", BaseURL: "https://nokey.example/v1", Protocol: ProviderAPIAnthropic, DecisionsFlavor: FlavorSystemOne, Key: KeyKindNone, Layer: LayerSourceRepository, Usable: true},
					{Name: "reaper", BaseURL: "https://reaper.example/v1", Protocol: ProviderAPIAnthropic, DecisionsFlavor: FlavorDecisions, Key: KeyKindFile, KeyFile: "keys/reaper.txt",
						Upgrade: "cat " + filepath.Join(repoDir, "keys", "reaper.txt") + " | kbase auth add reaper", Layer: LayerSourceRepository, Usable: true},
					{Name: "warned", BaseURL: "https://warned.example/v1", Protocol: ProviderAPIAnthropic, DecisionsFlavor: FlavorSystemOne, Key: KeyKindFile, KeyFile: "/k/warned.txt",
						Upgrade: "cat /k/warned.txt | kbase auth add warned", Layer: LayerSourceGlobal, Usable: true, Fault: exposedWarn},
				},
				Diagnostics: []result.Item{
					{Check: CheckProvider, Path: globalProviders, Detail: exposedWarn},
					{Check: CheckProvider, Path: repoProviders, Detail: "no baseUrl declared"},
					{Check: CheckProvider, Path: repoProviders, Detail: unreadKey, Remedy: AuthAddRemedy("gone")},
				},
			},
		},
		{
			name: "a stored key over a stated apiKeyFile, and a key file whose path a shell splits",
			providers: Providers{
				"both":   {Name: "both", BaseURL: "https://both.example/v1", APIKeyFile: "both.key", KeyStored: true, Layer: LayerSourceGlobal},
				"spaced": {Name: "spaced", BaseURL: "https://spaced.example/v1", APIKeyFile: "/k/my key's.txt", Layer: LayerSourceRepository},
				"stored": {Name: "stored", BaseURL: "https://stored.example/v1", KeyStored: true, Layer: LayerSourceRepository},
			},
			exists: func(string) bool { return false },
			want: Facts{
				Layers: []Layer{
					{Path: repoDir, Source: LayerSourceRepository},
					{Path: globalDir, Source: LayerSourceGlobal},
				},
				Entries: []Entry{
					{Name: "both", BaseURL: "https://both.example/v1", Protocol: ProviderAPIAnthropic, DecisionsFlavor: FlavorSystemOne, Key: KeyKindStore, Layer: LayerSourceGlobal, Usable: true},
					{Name: "spaced", BaseURL: "https://spaced.example/v1", Protocol: ProviderAPIAnthropic, DecisionsFlavor: FlavorSystemOne, Key: KeyKindFile, KeyFile: "/k/my key's.txt",
						Upgrade: `cat '/k/my key'\''s.txt' | kbase auth add spaced`, Layer: LayerSourceRepository, Usable: true},
					{Name: "stored", BaseURL: "https://stored.example/v1", Protocol: ProviderAPIAnthropic, DecisionsFlavor: FlavorSystemOne, Key: KeyKindStore, Layer: LayerSourceRepository, Usable: true},
				},
				Diagnostics: []result.Item{{Check: CheckProvider, Path: globalProviders,
					Detail: `provider "both" has a stored key, which outranks its apiKeyFile "both.key": that file is not read`,
					Remedy: `remove the apiKeyFile line from "` + globalProviders + `", or run kbase auth remove both to use the file`}},
			},
		},
		{
			name: "stored key files that did not open",
			providers: Providers{
				"Lab":   {Name: "Lab", BaseURL: "https://lab.example/v1", Layer: LayerSourceGlobal},
				"other": {Name: "other", BaseURL: "https://other.example/v1", Layer: LayerSourceGlobal},
			},
			faults: []ProviderFault{
				{Name: "Lab", Path: storeFile, Reason: storeFile + ": " + truncated, Warning: true},
				{Name: "other", Path: storeDir, Reason: storeDir + ": " + noUsername, Warning: true},
			},
			stored: []StoreFault{
				{Name: "lab", Path: storeFile, Reason: truncated, DoesNotOpen: true},
				{Name: "other", Path: storeDir, Reason: noUsername},
			},
			exists: func(string) bool { return false },
			want: Facts{
				Layers: []Layer{
					{Path: repoDir, Source: LayerSourceRepository},
					{Path: globalDir, Source: LayerSourceGlobal},
				},
				Entries: []Entry{
					{Name: "Lab", BaseURL: "https://lab.example/v1", Protocol: ProviderAPIAnthropic, DecisionsFlavor: FlavorSystemOne, Key: KeyKindNone, Layer: LayerSourceGlobal, Usable: true,
						Fault: storeFile + ": " + truncated},
					{Name: "other", BaseURL: "https://other.example/v1", Protocol: ProviderAPIAnthropic, DecisionsFlavor: FlavorSystemOne, Key: KeyKindNone, Layer: LayerSourceGlobal, Usable: true,
						Fault: storeDir + ": " + noUsername},
				},
				Diagnostics: []result.Item{
					{Check: CheckProvider, Path: storeFile, Detail: truncated, Remedy: "<command-that-prints-the-key> | kbase auth add Lab"},
					{Check: CheckProvider, Path: storeDir, Detail: noUsername},
				},
			},
		},
		{
			name: "a store directory that does not list",
			providers: Providers{
				"lab": {Name: "lab", BaseURL: "https://lab.example/v1", Layer: LayerSourceGlobal},
			},
			faults: []ProviderFault{{Path: storeDir, Reason: storeDir + ": permission denied", Warning: true}},
			stored: []StoreFault{{Path: storeDir, Reason: "permission denied"}},
			exists: func(string) bool { return false },
			want: Facts{
				Layers: []Layer{
					{Path: repoDir, Source: LayerSourceRepository},
					{Path: globalDir, Source: LayerSourceGlobal},
				},
				Entries: []Entry{
					{Name: "lab", BaseURL: "https://lab.example/v1", Protocol: ProviderAPIAnthropic, DecisionsFlavor: FlavorSystemOne, Key: KeyKindNone, Layer: LayerSourceGlobal, Usable: true},
				},
				Diagnostics: []result.Item{{Check: CheckProvider, Path: storeDir, Detail: "permission denied"}},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Report(tc.cfg, tc.providers, tc.faults, tc.stored, layers, tc.exists); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Report = %+v\nwant %+v", got, tc.want)
			}
		})
	}
}

// TestShellWord: a path a shell would read back unchanged stands as it is, and
// any other is single-quoted so the shell reads it back as one word, a quote
// inside it included.
func TestShellWord(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"/home/u/.keys/lab-1_x.txt", "/home/u/.keys/lab-1_x.txt"},
		{"/k/my key.txt", "'/k/my key.txt'"},
		{"/k/$HOME/*.txt", "'/k/$HOME/*.txt'"},
		{"~/k.txt", "'~/k.txt'"},
		{"/k/it's", `'/k/it'\''s'`},
		{"", "''"},
	} {
		if got := shellWord(tc.in); got != tc.want {
			t.Errorf("shellWord(%q) = %s, want %s", tc.in, got, tc.want)
		}
	}
}
