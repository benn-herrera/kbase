package config

import (
	"slices"
	"strings"
)

// KeyKind is how a pool entry supplies its API key.
type KeyKind string

const (
	// KeyKindFile is a key read from the file the entry's apiKeyFile names.
	KeyKindFile KeyKind = "file"
	// KeyKindNone is an endpoint that needs no key.
	KeyKindNone KeyKind = "none"
)

// Entry is one provider a report describes: a pool entry that loaded, or a
// provider a dropping fault removed from the pool. A dropped provider carries
// its name, Usable false, the layer whose file dropped it and the fault's
// reason, and nothing else — the pool holds no entry to describe.
type Entry struct {
	Name    string
	BaseURL string
	// Protocol is the chat wire protocol the entry speaks, its default filled
	// in: a report states what the entry resolves to, not whether it said so.
	Protocol string
	// DecisionsFlavor is the decision surface the entry speaks, its default
	// filled in: a report states what the entry resolves to, not whether it
	// said so.
	DecisionsFlavor string
	Key             KeyKind
	KeyFile         string // the apiKeyFile the entry states, when Key is KeyKindFile
	Layer           string // the layer that supplied the entry
	Usable          bool
	Fault           string // the reason a fault gives this provider, when one names it
}

// Facts is a configuration as loaded: the layers it was read from with what
// each holds, the pool the merge produced and what faults say about it, and
// its merged choices. It carries no key material.
type Facts struct {
	Layers   []Layer       // in precedence order, each with its existence probed
	Models   ModelMap      // the merged role values
	Sampling SamplingTable // the merged sampling values
	Entries  []Entry       // one per usable pool entry and per dropped provider, sorted by name
}

// Report describes configuration loaded from layers. It reads no file of its
// own: file existence arrives as the seam the caller passes, so a report
// describes a configuration without this process having to be the one that
// holds it.
func Report(cfg Config, providers Providers, faults []ProviderFault, layers []Layer, exists func(string) bool) Facts {
	f := Facts{Models: cfg.Models, Sampling: cfg.Sampling}
	for _, layer := range layers {
		layer.Exists = exists(layer.Path)
		layer.ProvidersExists = exists(ProvidersPath(layer.Path))
		layer.ConfigExists = exists(ConfigPath(layer.Path))
		f.Layers = append(f.Layers, layer)
	}

	warned := make(map[string]string, len(faults))
	for _, fault := range faults {
		if fault.Warning {
			warned[fault.Name] = fault.Reason
			continue
		}
		f.Entries = append(f.Entries, Entry{Name: fault.Name, Layer: faultLayer(layers, fault.Path), Fault: fault.Reason})
	}
	for name, p := range providers {
		e := Entry{Name: name, BaseURL: p.BaseURL, Protocol: p.Protocol(), DecisionsFlavor: p.Flavor(), Key: keyKind(p), Layer: p.Layer, Usable: true, Fault: warned[name]}
		if e.Key == KeyKindFile {
			e.KeyFile = p.APIKeyFile
		}
		f.Entries = append(f.Entries, e)
	}
	slices.SortFunc(f.Entries, func(a, b Entry) int { return strings.Compare(a.Name, b.Name) })
	return f
}

// faultLayer names the layer whose providers.toml carries the fault at path,
// and "" where no layer's file does.
func faultLayer(layers []Layer, path string) string {
	for _, layer := range layers {
		if ProvidersPath(layer.Path) == path {
			return layer.Source
		}
	}
	return ""
}

// keyKind names how p supplies its key: the file its apiKeyFile states, or
// none.
func keyKind(p Provider) KeyKind {
	if p.APIKeyFile != "" {
		return KeyKindFile
	}
	return KeyKindNone
}
