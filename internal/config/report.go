package config

import (
	"fmt"
	"slices"
	"strings"

	"kbase/internal/result"
)

// KeyKind is how a pool entry supplies its API key.
type KeyKind string

const (
	// KeyKindStore is a key the credentials store holds under the entry's
	// name.
	KeyKindStore KeyKind = "store"
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
	// Upgrade is the command that moves a KeyKindFile key into the store,
	// values filled in; the store then outranks the file.
	Upgrade string
	Layer   string // the layer that supplied the entry
	Usable  bool
	Fault   string // the reasons the warning faults naming this provider give, or the dropping one's
}

// Facts is a configuration as loaded: the layers it was read from with what
// each holds, the pool the merge produced and what faults say about it, and
// its merged choices. It carries no key material.
type Facts struct {
	Layers   []Layer       // in precedence order, each with its existence probed
	Models   ModelMap      // the merged role values
	Sampling SamplingTable // the merged sampling values
	Entries  []Entry       // one per usable pool entry and per dropped provider, sorted by name
	// Diagnostics is one item per fault, in the order faults gives them, then
	// one per pool entry, in name order, whose stated apiKeyFile goes unread
	// because a stored key outranks it.
	Diagnostics []result.Item
}

// Report describes configuration loaded from layers. It reads no file of its
// own: file existence arrives as the seam the caller passes, so a report
// describes a configuration without this process having to be the one that
// holds it. stored is the store's faults as the caller read them — each file
// that did not open, and the directory where it did not list; a fault Load
// raised for one is reported from its parts.
func Report(cfg Config, providers Providers, faults []ProviderFault, stored []StoreFault, layers []Layer, exists func(string) bool) Facts {
	f := Facts{Models: cfg.Models, Sampling: cfg.Sampling}
	for _, layer := range layers {
		layer.Exists = exists(layer.Path)
		layer.ProvidersExists = exists(ProvidersPath(layer.Path))
		layer.ConfigExists = exists(ConfigPath(layer.Path))
		f.Layers = append(f.Layers, layer)
	}

	storeFiles := make(map[string]StoreFault, len(stored))
	for _, sf := range stored {
		storeFiles[sf.Path] = sf
	}
	warned := make(map[string][]string, len(faults))
	for _, fault := range faults {
		f.Diagnostics = append(f.Diagnostics, faultItem(fault, storeFiles))
		if fault.Warning {
			warned[fault.Name] = append(warned[fault.Name], fault.Reason)
			continue
		}
		f.Entries = append(f.Entries, Entry{Name: fault.Name, Layer: faultLayer(layers, fault.Path), Fault: fault.Reason})
	}
	for _, name := range providerNames(providers) {
		p := providers[name]
		dir := layerDir(layers, p.Layer)
		e := Entry{Name: name, BaseURL: p.BaseURL, Protocol: p.Protocol(), DecisionsFlavor: p.Flavor(), Key: keyKind(p), Layer: p.Layer, Usable: true,
			Fault: strings.Join(warned[name], "; ")}
		switch {
		case e.Key == KeyKindFile:
			e.KeyFile = p.APIKeyFile
			e.Upgrade = "cat " + shellWord(keyFilePath(p, dir)) + " | " + authAddCommand(name)
		case e.Key == KeyKindStore && p.APIKeyFile != "":
			f.Diagnostics = append(f.Diagnostics, unreadKeyFileItem(name, p.APIKeyFile, ProvidersPath(dir)))
		}
		f.Entries = append(f.Entries, e)
	}
	slices.SortStableFunc(f.Entries, func(a, b Entry) int { return strings.Compare(a.Name, b.Name) })
	return f
}

// faultItem is the diagnostic a fault reports as: the file that carries it and
// its reason — or, where storeFiles holds the store file or directory it
// names, that path and what went wrong with it, and where a key file does not
// open, adding the key again as the remedy, since adding replaces the file.
func faultItem(fault ProviderFault, storeFiles map[string]StoreFault) result.Item {
	item := result.Item{Check: CheckProvider, Path: fault.Path, Detail: fault.Reason}
	if sf, ok := storeFiles[fault.Path]; ok && fault.Warning {
		item.Detail = sf.Reason
		if sf.DoesNotOpen {
			item.Remedy = AuthAddRemedy(fault.Name)
		}
	}
	return item
}

// unreadKeyFileItem is the warning that the entry name, stated in the
// providers.toml at path, has a stored key, so the apiKeyFile it also states
// is never read.
func unreadKeyFileItem(name, keyFile, path string) result.Item {
	return result.Item{Check: CheckProvider, Path: path,
		Detail: fmt.Sprintf("provider %q has a stored key, which outranks its apiKeyFile %q: that file is not read", name, keyFile),
		Remedy: fmt.Sprintf("remove the apiKeyFile line from %q, or run kbase auth remove %s to use the file", path, name)}
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

// layerDir is the directory of the layer source names, "" where no layer has
// that source.
func layerDir(layers []Layer, source string) string {
	for _, layer := range layers {
		if layer.Source == source {
			return layer.Path
		}
	}
	return ""
}

// keyKind is the one precedence of an entry's key: the one the store holds
// under its name, whatever layer supplied the entry; else the file its
// apiKeyFile states; else none.
func keyKind(p Provider) KeyKind {
	switch {
	case p.KeyStored:
		return KeyKindStore
	case p.APIKeyFile != "":
		return KeyKindFile
	}
	return KeyKindNone
}

// shellWord is s as one word a POSIX shell reads back as s: as it stands where
// it holds no character a shell splits on or expands, else single-quoted, each
// ' inside closed, escaped and reopened.
func shellWord(s string) string {
	if s != "" && !strings.ContainsFunc(s, func(r rune) bool { return !shellPlain(r) }) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// shellPlain reports whether r stands for itself, unquoted, anywhere in a
// shell word.
func shellPlain(r rune) bool {
	switch {
	case 'a' <= r && r <= 'z', 'A' <= r && r <= 'Z', '0' <= r && r <= '9':
		return true
	}
	return strings.ContainsRune("@%+=:,./_-", r)
}
