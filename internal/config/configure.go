package config

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"kbase/internal/result"
)

const (
	// ModelMapPairSep separates category assignments within one --model-map
	// value; modelMapAssign separates a category from its reference.
	ModelMapPairSep = ","
	modelMapAssign  = "="

	// keyModelMap is the flag a reference the pool cannot resolve belongs to.
	keyModelMap = "--model-map"

	// modelMapSyntax is the flag spelling quoted back to the user in every
	// failure a manual assignment would resolve. A value is a reference, the
	// form config.toml's [models] values take.
	modelMapSyntax = keyModelMap + " " + CategoryDecision + "=<entry>/<id>" + ModelMapPairSep +
		CategoryText + "=<entry>/<id>" + ModelMapPairSep + CategoryScoring + "=<entry>/<id>"
)

// CheckDetect names a category no --model-map assignment supplied.
const CheckDetect = "detect"

// CatalogueFunc is the model ids entry's provider lists, in the order it
// listed them. Its error is returned by Configure unchanged.
type CatalogueFunc func(entry Provider) ([]string, error)

// ConfigureRequest is one configure run: the loaded configuration, whose
// WriteDir is the layer the run writes into, the --provider flag, the
// category→reference map ParseModelMap read from --model-map, and whether the
// run may create the layer's files.
type ConfigureRequest struct {
	Inputs   Inputs
	Provider string
	Models   map[string]Reference
	Create   bool
}

// Offline is Create with no Provider: the run dials nothing, and no reference
// is checked against a catalogue.
func (req ConfigureRequest) Offline() bool {
	return req.Create && strings.TrimSpace(req.Provider) == ""
}

// CatalogueMiss is a --model-map reference whose model its provider's
// catalogue does not list.
type CatalogueMiss struct {
	Category  string
	Reference Reference
}

// Configured is what a configure run got as far as, whatever its error.
type Configured struct {
	// Selected says the dialled run settled the entry it configures:
	// Provider, Rule and Layer name it, all "" where the references alone name
	// their providers.
	Selected              bool
	Provider, Rule, Layer string

	// Misses are the references their catalogues omit, in category order.
	Misses []CatalogueMiss

	// Models is the [models] table the run wrote or tried to write, nil where
	// it did not reach the write; Written is the files it wrote, in order.
	Models  *ModelMap
	Written []string
}

// Configure assigns a reference to each model category and writes them to the
// target layer's config.toml.
//
// Every category gets a reference from the map or, for scoring alone, from
// the target file's own value: a category with neither is refused, listing
// what the selected provider offered, with nothing written — config.toml is
// what every later stage draws its models from, and nothing between the flags
// and the file picks a model on the user's behalf. Each reference is checked
// against its own provider's catalogue, read through catalogue. Create with no
// Provider is the one exception: catalogue is never called, the run writes
// what the map names over what the file holds.
func Configure(req ConfigureRequest, catalogue CatalogueFunc) (Configured, error) {
	if req.Offline() {
		return configureOffline(req)
	}
	return configureDialed(req, catalogue)
}

// configureDialed is the ordinary path: every reference the map names is
// checked against its own provider's catalogue and the three references are
// written; an unassigned category is offered the selected provider's
// catalogue. A refusal or a failed listing writes nothing.
func configureDialed(req ConfigureRequest, catalogue CatalogueFunc) (Configured, error) {
	var run Configured
	in := req.Inputs

	// The one value this run may leave standing: the target file's own
	// scoring reference, where the map names none. It is the file's, not the
	// merge's — a lower layer's value is not this file's to keep.
	configFile := ConfigPath(in.WriteDir)
	fileCfg, err := LoadConfig(configFile)
	if err != nil {
		return run, err
	}
	scoring, err := keepScoring(req.Models, configFile, fileCfg.Models.Scoring)
	if err != nil {
		return run, err
	}

	name, rule, err := Select(in, req.Provider)
	if err != nil {
		if req.Provider != "" || len(req.Models) == 0 {
			return run, err
		}
		// The references name their own providers, so the pool's size is no
		// reason to demand a selection no reference needs.
		name, rule = "", ""
	}
	run.Selected, run.Provider, run.Rule, run.Layer = true, name, rule, in.Providers[name].Layer

	// Every distinct provider the references name, the selected entry first:
	// one catalogue is what an unassigned category is offered, and each
	// reference's own is what its model part is checked against.
	targets := make([]string, 0, len(Roles)+1)
	addTarget := func(provider string) {
		if provider != "" && !slices.Contains(targets, provider) {
			targets = append(targets, provider)
		}
	}
	addTarget(name)
	for _, role := range Roles {
		if ref, given := req.Models[string(role)]; given {
			addTarget(ref.Provider)
		}
	}

	catalogues := make(map[string][]string, len(targets))
	for _, target := range targets {
		entry, ok := in.Providers[target]
		if !ok {
			return run, missingReferenceProvider(target, in)
		}
		ids, err := catalogue(entry)
		if err != nil {
			return run, err
		}
		catalogues[target] = ids
	}

	// A reference whose model part its provider does not list is a warning,
	// not a failure: catalogues rotate, and a user pinning a model the listing
	// omits may know something the listing does not.
	for _, role := range Roles {
		ref, given := req.Models[string(role)]
		if given && !slices.Contains(catalogues[ref.Provider], ref.Model) {
			run.Misses = append(run.Misses, CatalogueMiss{Category: string(role), Reference: ref})
		}
	}

	// A category with nothing to write is refused, one item per missing
	// category; the catalogue listing is the models to choose from.
	offered := name
	if offered == "" {
		offered = targets[0]
	}
	var unassigned result.Refusal
	for _, role := range Roles {
		if _, given := req.Models[string(role)]; given {
			continue
		}
		if role == RoleScoring && scoring != nil {
			continue
		}
		unassigned = append(unassigned, unassignedCategoryItem(role, offered, catalogues[offered]))
	}
	if unassigned != nil {
		return run, unassigned
	}

	models := ModelMap{
		Decision: req.Models[CategoryDecision].String(),
		Text:     req.Models[CategoryText].String(),
	}
	if scoring != nil {
		models.Scoring = scoring.String()
	}
	return run.write(req, Config{Models: models})
}

// keepScoring is the scoring reference a dialed run writes: the map's where
// it names one, the target file's own where the file holds one, and none where
// it holds none — which the run then refuses, naming the key. A file's bare id
// is refused rather than kept: writing it back would produce a file kbase
// score refuses.
func keepScoring(manual map[string]Reference, path, fileValue string) (*Reference, error) {
	if ref, given := manual[CategoryScoring]; given {
		return &ref, nil
	}
	if fileValue == "" {
		return nil, nil
	}
	ref, err := FileReference(fileValue)
	if err != nil {
		return nil, result.Refusal{{Check: CheckProvider, Key: RoleScoring.key(), Path: path,
			Remedy: "kbase configure " + keyModelMap + " " + CategoryScoring + "=<entry>/<id>",
			Detail: fmt.Sprintf("%s names %q, and a model is named <provider>/<model> — the entry in %s that supplies the endpoint, then the model it serves",
				RoleScoring.key(), fileValue, ProvidersFileName)}}
	}
	return &ref, nil
}

// missingReferenceProvider is a --model-map reference whose provider the pool
// does not hold: the flag is what named it, so the flag is what the refusal
// names, and the pool's entries are the names that would work. The pool's own
// refusal is reused, which keeps a faulted provider reading as faulted rather
// than as a typo.
func missingReferenceProvider(provider string, in Inputs) result.Refusal {
	_, _, err := Select(in, provider)
	var refusal result.Refusal
	if !errors.As(err, &refusal) || len(refusal) != 1 {
		return result.Refusal{{Check: CheckUsage, Key: keyModelMap,
			Detail: fmt.Sprintf("%s names provider %q, which is not in %s", keyModelMap, provider, ProvidersFileName)}}
	}
	refusal[0].Key = keyModelMap
	refusal[0].Detail = fmt.Sprintf("%s names a provider the pool does not hold: %s", keyModelMap, refusal[0].Detail)
	return refusal
}

// configureOffline writes what the map names and nothing else: no provider was
// named, so no request is made and no reference is checked against a catalogue.
// What the target layer's config.toml already holds — an untouched category, a
// comment — survives, because configure is an editor and Create is only
// permission to create the file it edits.
func configureOffline(req ConfigureRequest) (Configured, error) {
	var run Configured
	cfg, err := LoadConfig(ConfigPath(req.Inputs.WriteDir))
	if err != nil {
		return run, err
	}
	for _, role := range Roles {
		if ref, given := req.Models[string(role)]; given {
			switch role {
			case RoleDecision:
				cfg.Models.Decision = ref.String()
			case RoleText:
				cfg.Models.Text = ref.String()
			case RoleScoring:
				cfg.Models.Scoring = ref.String()
			}
		}
	}
	return run.write(req, cfg)
}

// write writes cfg into the target layer, creating the layer's files where the
// run may — providers.toml, then config.toml — and records what it wrote.
//
// Nothing is overwritten: the providers.toml template lands only where no file
// stands, and config.toml is edited line by line, so a directory a user has
// already filled in keeps everything this call was not asked to change.
func (run Configured) write(req ConfigureRequest, cfg Config) (Configured, error) {
	run.Models = &cfg.Models
	dir := req.Inputs.WriteDir
	if req.Create {
		providersFile := ProvidersPath(dir)
		created, err := CreateProvidersFile(providersFile)
		if err != nil {
			return run, err
		}
		if created {
			run.Written = append(run.Written, providersFile)
		}
	}
	configFile := ConfigPath(dir)
	wrote, err := UpdateConfig(configFile, cfg)
	if err != nil {
		return run, err
	}
	if wrote {
		run.Written = append(run.Written, configFile)
	}
	return run, nil
}

// ParseModelMap turns the raw --model-map values into a category→reference
// map. Values accumulate across repeats of the flag and across comma-separated
// entries within one value, so `--model-map decision=a/d --model-map text=a/t`
// and `--model-map decision=a/d,text=a/t` are the same request. A value is a
// reference — the form config.toml's [models] values take — so no run writes a
// bare id. A partial map is syntactically legal here; Configure refuses a
// category it does not name, so every accepted dialled run assigns what the
// file does not already hold.
func ParseModelMap(values []string) (map[string]Reference, error) {
	categories := categoryNames()
	out := make(map[string]Reference, len(categories))
	for _, value := range values {
		for _, entry := range strings.Split(value, ModelMapPairSep) {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			rawCategory, rawID, split := strings.Cut(entry, modelMapAssign)
			category := strings.ToLower(strings.TrimSpace(rawCategory))
			id := strings.TrimSpace(rawID)
			if !split || category == "" || id == "" {
				return nil, modelMapError(fmt.Sprintf("malformed --model-map entry %q — expected %s", entry, modelMapSyntax))
			}
			if !slices.Contains(categories, category) {
				err := modelMapError(fmt.Sprintf("unknown category %q in --model-map (categories: %s)", category, strings.Join(categories, ", ")))
				err[0].Allowed = categories
				return nil, err
			}
			ref, err := FileReference(id)
			if err != nil {
				return nil, modelMapError(fmt.Sprintf("--model-map %s names %q, and a value is a reference: %s", category, id, modelMapSyntax))
			}
			if prev, dup := out[category]; dup && prev != ref {
				return nil, modelMapError(fmt.Sprintf("--model-map assigns the %s category twice (%q and %q)", category, prev.String(), ref.String()))
			}
			out[category] = ref
		}
	}
	return out, nil
}

// categoryNames is Roles as the [models] table spells each role.
func categoryNames() []string {
	out := make([]string, len(Roles))
	for i, role := range Roles {
		out[i] = string(role)
	}
	return out
}

// modelMapError is a --model-map value refused as usage.
func modelMapError(detail string) result.Refusal {
	return result.Refusal{{Check: CheckUsage, Key: keyModelMap, Detail: detail}}
}

// unassignedCategoryItem refuses a category with no reference to write. The
// provider's catalogue is listed as the models to choose from — informational
// only, nothing in it is matched against the category.
func unassignedCategoryItem(role Role, provider string, ids []string) result.Item {
	detail := fmt.Sprintf("no reference assigned to the %s category at provider %q; assign one explicitly, the provider offers: %s",
		role, provider, strings.Join(sortedCopy(ids), ", "))
	if len(ids) == 0 {
		detail = fmt.Sprintf("no reference assigned to the %s category at provider %q; assign one explicitly, it offered no models at all", role, provider)
	}
	return result.Item{Check: CheckDetect, Key: role.key(), Allowed: sortedCopy(ids),
		Remedy: "kbase configure " + keyModelMap + " " + string(role) + "=" + provider + "/<id>", Detail: detail}
}

func sortedCopy(values []string) []string {
	sorted := append([]string{}, values...)
	slices.Sort(sorted)
	return sorted
}
