package config

import (
	"reflect"
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

// TestTemplatesSpellEveryModeledKey: a template is the only schema prose a
// user is handed, so neither may fall behind the structs it describes. Every
// key the loaders model is spelled in the template it belongs to, as a live or
// a commented `key =` line — a field added to Provider or Config without a line
// here fails before a user meets a key the file never mentions. The one
// exception is apiKeyFile: the providers.toml template names no key field,
// because a key is added with `kbase auth add`.
func TestTemplatesSpellEveryModeledKey(t *testing.T) {
	for _, tc := range []struct {
		file     string
		typ      reflect.Type
		template string
		omitted  string
	}{
		{ProvidersFileName, reflect.TypeOf(Provider{}), providersTemplate(), "apiKeyFile"},
		{ConfigFileName, reflect.TypeOf(Config{}), freshConfig(Config{}), ""},
	} {
		t.Run(tc.file, func(t *testing.T) {
			shown := templateKeys(tc.template)
			for _, key := range tomlKeys(tc.typ, "") {
				leaf := key[strings.LastIndex(key, ".")+1:]
				if leaf == tc.omitted {
					continue
				}
				if !shown[leaf] {
					t.Errorf("the %s template states no %s key (`%s =`)", tc.file, key, leaf)
				}
			}
		})
	}
}

// TestConfigTemplateCarriesTheModelGuidance: the generated config.toml carries
// the [models] guidance.
func TestConfigTemplateCarriesTheModelGuidance(t *testing.T) {
	template := freshConfig(Config{})
	if !strings.Contains(template, modelHintsComment()) {
		t.Errorf("the generated config.toml carries no [models] guidance:\n%s", template)
	}
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

	loaded, err := Load([]Layer{{Path: dir, Source: LayerSourceExplicit}}, StoredKeys{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	providers, faults, cfg := loaded.Providers, loaded.Faults, loaded.Cfg
	if len(faults) != 0 {
		t.Errorf("faults = %+v, want none: a fresh template drops nothing", faults)
	}
	if len(providers) != 1 {
		t.Fatalf("pool = %v, want the one live entry", keysOf(providers))
	}
	for name, p := range providers {
		if p.BaseURL == "" || p.Layer != LayerSourceExplicit {
			t.Errorf("entry %q: baseUrl %q, layer %q; want a usable entry from the one layer", name, p.BaseURL, p.Layer)
		}
	}
	if cfg.Models != (ModelMap{}) || cfg.Asks.Concurrency != nil || cfg.Sources != (Sources{}) {
		t.Errorf("config = %+v (%+v), want every field unset", cfg, cfg.Sources)
	}
}
