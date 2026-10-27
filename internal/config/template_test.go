package config

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

// tomlKeys returns the dotted `toml` key paths a struct type models: a
// field's own tag, or — for a field holding a struct — each of its fields'
// tags below it. A field tagged "-" is not part of the file and yields
// nothing.
func tomlKeys(t reflect.Type, prefix string) []string {
	var keys []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("toml")
		if tag == "" || tag == "-" {
			continue
		}
		key := tag
		if prefix != "" {
			key = prefix + "." + tag
		}
		if f.Type.Kind() == reflect.Struct {
			keys = append(keys, tomlKeys(f.Type, key)...)
			continue
		}
		keys = append(keys, key)
	}
	return keys
}

// templateKeys returns the leaf keys a template assigns, live or commented
// out: the left-hand side of every `key =` line, the leading # dropped.
func templateKeys(template string) map[string]bool {
	keys := make(map[string]bool)
	for _, line := range strings.Split(template, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "#"))
		key, _, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		keys[strings.TrimSpace(key)] = true
	}
	return keys
}

// modelledNotWritten is the keys config's schema models and no template
// writes: the superseded keys, which load so an old config.toml keeps working
// — the top-level provider key, which `kbase configure` removes rather than
// writes, and [asks] readerConcurrency, which asks.concurrency replaces. An
// entry fails the test once a template spells its key, so it cannot outlive its
// reason.
var modelledNotWritten = map[string]bool{"provider": true, "asks.readerConcurrency": true}

// TestTemplatesSpellEveryModeledKey: a template is the only schema prose a
// user is handed, so neither may fall behind the structs it describes. Every
// key the loaders model is spelled in the template it belongs to, as a live or
// a commented `key =` line — a field added to Provider or Config without a line
// here fails before a user meets a key the file never mentions.
func TestTemplatesSpellEveryModeledKey(t *testing.T) {
	configKeys := tomlKeys(reflect.TypeOf(Config{}), "")
	for key := range modelledNotWritten {
		if !slices.Contains(configKeys, key) {
			t.Fatalf("%s is exempt from the template but no longer modelled", key)
		}
	}
	for _, tc := range []struct {
		file     string
		typ      reflect.Type
		template string
	}{
		{ProvidersFileName, reflect.TypeOf(Provider{}), providersTemplate},
		{ConfigFileName, reflect.TypeOf(Config{}), freshConfig(Config{})},
	} {
		t.Run(tc.file, func(t *testing.T) {
			shown := templateKeys(tc.template)
			for _, key := range tomlKeys(tc.typ, "") {
				leaf := key[strings.LastIndex(key, ".")+1:]
				if modelledNotWritten[key] {
					if shown[leaf] {
						t.Errorf("the %s template states %s: drop its modelledNotWritten entry", tc.file, key)
					}
					continue
				}
				if !shown[leaf] {
					t.Errorf("the %s template states no %s key (`%s =`)", tc.file, key, leaf)
				}
			}
		})
	}
}

// TestConfigTemplateCarriesTheModelGuidance: a first-time user opens the one
// file kbase wrote and finds out there what each [models] category wants —
// the capability first, ids as examples — so the guidance reaches them without
// a lookup and without the ids reading as required.
func TestConfigTemplateCarriesTheModelGuidance(t *testing.T) {
	template := freshConfig(Config{})
	if !strings.Contains(template, modelHintsComment()) {
		t.Errorf("the generated config.toml carries no [models] guidance:\n%s", template)
	}
	prose := commentProse(modelHintsComment())
	for _, want := range []string{
		"a mid-weight reasoning model (e.g. d3, clef)",
		"a ~9B instruct model (e.g. Qwen3.5-9B)",
		"the strongest reasoning model on offer (e.g. clef, d3)",
	} {
		if !strings.Contains(prose, want) {
			t.Errorf("the guidance states no %q:\n%s", want, prose)
		}
	}
}

// commentProse is a comment block as prose: its leading "#"s and its line
// breaks folded away, so a check states what the block says and not where the
// renderer wrapped it.
func commentProse(block string) string {
	var words []string
	for _, line := range strings.Split(block, "\n") {
		words = append(words, strings.Fields(strings.TrimPrefix(strings.TrimSpace(line), "#"))...)
	}
	return strings.Join(words, " ")
}

// TestTemplatesLoadAsAFreshConfiguration: what `configure --create` writes on a
// fresh install is a configuration every loader accepts — one usable pool entry
// and no fault, and a config.toml that reads as unset in every field — so the
// first thing a user does to it is edit a value, not repair a template.
func TestTemplatesLoadAsAFreshConfiguration(t *testing.T) {
	dir := t.TempDir()
	if _, err := CreateProvidersFile(ProvidersPath(dir)); err != nil {
		t.Fatalf("CreateProvidersFile: %v", err)
	}
	if _, err := UpdateConfig(ConfigPath(dir), Config{}); err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}

	loaded, err := Load([]Layer{{Path: dir, Source: LayerSourceExplicit}})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	providers, faults, cfg := loaded.Providers, loaded.Faults, loaded.Cfg
	if len(faults) != 0 {
		t.Errorf("faults = %+v, want none: a fresh template drops nothing", faults)
	}
	if len(providers) != 1 {
		t.Fatalf("pool = %v, want the one live entry", providers)
	}
	for name, p := range providers {
		if p.BaseURL == "" || p.Layer != LayerSourceExplicit {
			t.Errorf("entry %q = %+v, want a usable entry from the one layer", name, p)
		}
	}
	if cfg.Provider != "" || cfg.Models != (ModelMap{}) || cfg.Asks.ReaderConcurrency != nil || cfg.Asks.Concurrency != nil || cfg.Sources != (Sources{}) {
		t.Errorf("config = %+v (%+v), want every field unset", cfg, cfg.Sources)
	}
}
