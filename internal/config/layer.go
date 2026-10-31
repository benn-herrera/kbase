package config

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"kbase/internal/secret"
)

// The four layers configuration is read from, in the one vocabulary every
// report draws on: which of them supplied a path, a pool entry or a field.
const (
	// LayerSourceExplicit is the directory --config-dir named.
	LayerSourceExplicit = "explicit"
	// LayerSourceEnvironment is the directory $KBASE_CONFIG_DIR named.
	LayerSourceEnvironment = "environment"
	// LayerSourceRepository is the repository's own configuration directory,
	// <repository>/.kbase.
	LayerSourceRepository = "repository"
	// LayerSourceGlobal is ~/.config/kbase, the one layer present in every
	// stack.
	LayerSourceGlobal = "global"
)

// RepositoryDirName is the repository's configuration directory, read over
// the global one.
const RepositoryDirName = ".kbase"

// Layer is one directory configuration is read from: the directory, which of
// the four sources named it, and — once a report has probed it — whether the
// directory and each of its files stand.
type Layer struct {
	Path   string
	Source string

	Exists          bool
	ProvidersExists bool
	ConfigExists    bool
}

// Layers returns the directories this invocation reads, in precedence order.
// A directory --config-dir or $KBASE_CONFIG_DIR names is the only one read:
// an explicit choice isolates the run from a repository's own .kbase/, which
// is what a test and a pinned deployment need. Otherwise the repository layer
// — only where repoRoot is not empty, since an invocation outside a worktree
// has none — stands over the global one.
//
// It resolves paths and nothing else: existence is the caller's to probe (see
// Report), and no directory is created or checked for.
func Layers(flag, repoRoot string) ([]Layer, error) {
	if dir, source := namedLayer(flag); dir != "" {
		return []Layer{{Path: dir, Source: source}}, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("config dir: %w", err)
	}
	var layers []Layer
	if repoRoot != "" {
		layers = append(layers, Layer{Path: repositoryDir(repoRoot), Source: LayerSourceRepository})
	}
	global := filepath.Join(home, defaultDirParent, defaultDirName)
	return append(layers, Layer{Path: global, Source: LayerSourceGlobal}), nil
}

// WriteTarget is the directory a verb writes configuration into: a directory a
// flag or $KBASE_CONFIG_DIR names, whether or not it exists, so an explicit
// choice is never second-guessed; a repository a --project flag names, whether
// or not its directory stands; otherwise the repository layer where its
// directory already stands; otherwise the global layer. It reads the layers
// this invocation resolved, so the directory written is one of the directories
// read.
//
// projectRoot is the repository root a verb's --project named, and "" where no
// verb named one. It is the caller's to resolve: the repository a --project
// writes into need not be in the read stack at all, and config resolves paths,
// never repositories.
func WriteTarget(layers []Layer, projectRoot string) string {
	if projectRoot != "" {
		return repositoryDir(projectRoot)
	}
	switch top := layers[0]; top.Source {
	case LayerSourceExplicit, LayerSourceEnvironment:
		return top.Path
	case LayerSourceRepository:
		if isDir(top.Path) {
			return top.Path
		}
	}
	return layers[len(layers)-1].Path
}

// Loaded is everything one configuration load produced: the merged pool with
// the entries that dropped out of it, and the merged config.toml choices.
// ProvidersFiles is each layer's providers.toml that stood when read, in
// precedence order.
type Loaded struct {
	Providers      Providers
	Faults         []ProviderFault
	Cfg            Config
	ProvidersFiles []string
}

// StoredKeys is the credentials store as the caller read it, every name the
// lowercase one a key file is stored under: each key that opened, by name, and
// each file that did not open — or, where the store's directory stands and
// does not list, no key and that directory's fault.
type StoredKeys struct {
	Keys   map[string]secret.Key
	Faults []StoreFault
}

// StoreFault is a stored key file that did not open: the name it is stored
// under, the file, and what went wrong — never the file's content. A fault
// with no name is the store's directory, which did not list.
// DoesNotOpen is a file that stands and is truncated or does not decrypt, so
// adding the key again, which replaces the file, is its remedy.
type StoreFault struct {
	Name, Path, Reason string
	DoesNotOpen        bool
}

// Load reads both files of every layer — providers.toml and config.toml —
// highest precedence first, and merges them into one configuration: the pool is
// the union, a repository entry replacing a same-named global one whole, and
// config.toml's fields overlay one by one. Each pooled provider and model id
// names the layer it came from; each fault names the providers.toml that
// carries it. Each entry's key is stored's under its name, else its apiKeyFile's
// (LoadProviders).
//
// A stored key file that did not open is a warning fault naming the entry that
// carries its name — in the pool or dropped from it — as that entry spells it,
// and the file as its path; the entry's key then comes from its apiKeyFile, else
// none. A file whose name no entry carries is read by nothing and not reported.
// A fault with no name — the store's directory, which did not list — names no
// entry and is a warning fault of its own, Name empty and the directory as its
// path, since every entry's stored key then goes unread.
//
// A fault naming an entry a higher layer already supplies is dropped: the
// pool's entry is the higher one, so the lower entry is a value nobody reads.
// A higher layer's own fault stands — the entry it names never entered the
// pool — and a lower entry of that name is then the one in use.
//
// A layer that fails the strict load refuses the whole load. The error is
// returned as the loader wrote it, naming the file, and no lower layer is
// read: a broken repository file that silently fell through to the global one
// would leave the user's edit doing nothing with nothing said. An entry whose
// name differs only by case from one a higher layer states refuses the load
// the same way, naming both spellings and the file stating each: an override
// names its entry exactly.
func Load(layers []Layer, stored StoredKeys) (Loaded, error) {
	var loaded Loaded
	pool := Providers{}
	stated := map[string]statedName{}
	for _, layer := range layers {
		path := ProvidersPath(layer.Path)
		data, stands, err := readProvidersFile(path)
		if err != nil {
			return Loaded{}, err
		}
		if stands {
			loaded.ProvidersFiles = append(loaded.ProvidersFiles, path)
		}
		providers, layerFaults, err := parseProviders(path, data, stored.Keys)
		if err != nil {
			return Loaded{}, err
		}
		if err := rejectCaseOverrides(stated, path, providers, layerFaults); err != nil {
			return Loaded{}, err
		}
		layerCfg, err := LoadConfig(ConfigPath(layer.Path))
		if err != nil {
			return Loaded{}, err
		}
		// Filtered against the pool as the higher layers left it: this layer's
		// own warnings name entries it is about to contribute.
		for _, fault := range layerFaults {
			if _, superseded := pool[fault.Name]; superseded {
				continue
			}
			loaded.Faults = append(loaded.Faults, fault)
		}
		takeProviders(pool, providers, layer.Source)
		takeConfig(&loaded.Cfg, layerCfg, layer.Source)
	}
	loaded.Faults = append(loaded.Faults, storeFaults(stored.Faults, pool, loaded.Faults)...)
	loaded.Providers = pool
	return loaded, nil
}

// storeFaults is a warning fault for each stored key file in faults whose name
// an entry carries — in pool, or dropped from it by one of dropped — named as
// the entry spells it, and for each nameless fault, the store's directory's.
func storeFaults(faults []StoreFault, pool Providers, dropped []ProviderFault) []ProviderFault {
	spelled := make(map[string]string, len(pool)+len(dropped))
	for name := range pool {
		spelled[strings.ToLower(name)] = name
	}
	for _, f := range dropped {
		spelled[strings.ToLower(f.Name)] = f.Name
	}
	var out []ProviderFault
	for _, sf := range faults {
		fault := ProviderFault{Path: sf.Path, Reason: sf.Reason, Warning: true, DoesNotOpen: sf.DoesNotOpen}
		if sf.Name == "" {
			fault.Source = SourceStoreDir
		} else if name, ok := spelled[sf.Name]; ok {
			fault.Name, fault.Source = name, SourceStoreFile
		} else {
			continue
		}
		out = append(out, fault)
	}
	return out
}

// rejectCaseOverrides refuses a layer's providers.toml, at path, where one of
// its entries — pooled or dropped by a fault — differs only by case from an
// entry a higher layer stated, as recorded in stated; it then records this
// layer's entries there. An override names its entry exactly.
func rejectCaseOverrides(stated map[string]statedName, path string, providers Providers, faults []ProviderFault) error {
	names := slices.Collect(maps.Keys(providers))
	for _, f := range faults {
		names = append(names, f.Name)
	}
	slices.Sort(names)
	var refusals []string
	for _, c := range caseCollisions(stated, path, names) {
		refusals = append(refusals, fmt.Sprintf("%q in %s and %q in %s differ only by case: to override, spell %q in %s as %q; otherwise rename it",
			c.first.name, c.first.path, c.second.name, c.second.path, c.first.name, c.first.path, c.second.name))
	}
	if len(refusals) > 0 {
		return fmt.Errorf("providers: %s", strings.Join(refusals, "; "))
	}
	return nil
}

// takeProviders adds over's entries to pool, each naming the layer it came
// from. An entry a higher layer already supplied stands as it is: a same-named
// entry is replaced whole, never field by field, so a repository entry cannot
// borrow the global entry's apiKeyFile for an endpoint it chose. A stored key
// is the name's, whatever layer supplies the entry.
func takeProviders(pool, over Providers, source string) {
	for name, p := range over {
		if _, ok := pool[name]; ok {
			continue
		}
		p.Layer = source
		pool[name] = p
	}
}

// takeConfig overlays over's stated values onto cfg and records the layer that
// supplied each. A field the higher layer leaves out keeps the lower layer's
// value, and the lower layer's name.
func takeConfig(cfg *Config, over Config, source string) {
	takeStated(&cfg.Models.Decision, &cfg.Sources.Decision, over.Models.Decision, source)
	takeStated(&cfg.Models.Text, &cfg.Sources.Text, over.Models.Text, source)
	takeStated(&cfg.Models.Scoring, &cfg.Sources.Scoring, over.Models.Scoring, source)
	takeStated(&cfg.Sampling.Text.Temperature, &cfg.Sources.TextTemperature, over.Sampling.Text.Temperature, source)
	takeStated(&cfg.Sampling.Text.Thinking, &cfg.Sources.TextThinking, over.Sampling.Text.Thinking, source)
	takeStated(&cfg.Sampling.Scoring.Temperature, &cfg.Sources.ScoringTemperature, over.Sampling.Scoring.Temperature, source)
	takeStated(&cfg.Sampling.Scoring.Thinking, &cfg.Sources.ScoringThinking, over.Sampling.Scoring.Thinking, source)
	if cfg.Asks.Concurrency == nil {
		cfg.Asks.Concurrency = over.Asks.Concurrency
	}
}

// takeStated sets *value and *layer from over and source where *value is still
// T's zero value and over is not: the first layer to state a field keeps it.
func takeStated[T comparable](value *T, layer *string, over T, source string) {
	var zero T
	if *value == zero && over != zero {
		*value, *layer = over, source
	}
}

// namedLayer returns the directory a flag or the environment names — a leading
// ~ expanded from the home directory, as every configured path is — and which
// of the two named it; both empty where neither does. A blank flag, and a
// blank or whitespace-only EnvConfigDir, count as unset — an empty
// KBASE_CONFIG_DIR exported by a shell profile must not resolve every config
// path to the process working directory.
func namedLayer(flag string) (dir, source string) {
	if dir := strings.TrimSpace(flag); dir != "" {
		return expandHome(dir), LayerSourceExplicit
	}
	if dir := strings.TrimSpace(os.Getenv(EnvConfigDir)); dir != "" {
		return expandHome(dir), LayerSourceEnvironment
	}
	return "", ""
}

// repositoryDir is the repository root's configuration directory.
func repositoryDir(repoRoot string) string { return filepath.Join(repoRoot, RepositoryDirName) }

// isDir reports whether path names a directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
