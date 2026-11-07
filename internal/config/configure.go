package config

import (
	"fmt"
	"slices"
	"strings"

	"kbase/internal/argmark"
	"kbase/internal/result"
)

// CatalogueFunc is the model ids entry's provider lists, in the order it
// listed them. Its error is returned by ConfigureModels unchanged.
type CatalogueFunc func(entry Provider) ([]string, error)

// RoleReference is a role and a reference it holds or is given.
type RoleReference struct {
	Role      Role
	Reference Reference
}

// ConfiguredModels is what a configure-models run got as far as, whatever its
// error: the references whose model its provider's catalogue does not list, in
// role order, and — once it reached the write — the [models] table the target
// config.toml holds and the files it wrote.
type ConfiguredModels struct {
	Misses  []RoleReference
	Models  *ModelMap
	Written []string
}

// ConfigureModels writes a reference into in.WriteDir's config.toml for each
// role given holds a value for, creating the file where it is absent; the
// roles not given stand as the file has them, and so does every other byte. in
// is the stack written into: that layer and the layers beneath it. A value
// that is no reference, and a provider the pool in does not hold, are refused,
// every one at once, before anything is dialled; so is a run naming no role.
// The refusal of a provider the pool does not hold gives the configure-providers
// call that adds it, carrying layer, the `--layer` of the call being answered
// ("" for none). The provider part folds to lowercase, as the
// pool's names are, and is written so. Each provider named is listed once,
// through catalogue: a model its catalogue omits is a miss, not a failure,
// since catalogues rotate; a listing that fails is returned, nothing written.
func ConfigureModels(in Inputs, layer string, given map[Role]string, catalogue CatalogueFunc) (ConfiguredModels, error) {
	var run ConfiguredModels
	refs, err := givenReferences(in, layer, given)
	if err != nil {
		return run, err
	}

	catalogues := map[string][]string{}
	for _, role := range Roles {
		ref, ok := refs[role]
		if !ok {
			continue
		}
		if _, listed := catalogues[ref.Provider]; !listed {
			ids, err := catalogue(in.Providers[ref.Provider])
			if err != nil {
				return run, err
			}
			catalogues[ref.Provider] = ids
		}
		if !slices.Contains(catalogues[ref.Provider], ref.Model) {
			run.Misses = append(run.Misses, RoleReference{Role: role, Reference: ref})
		}
	}

	path := ConfigPath(in.WriteDir)
	cfg, err := LoadConfig(path)
	if err != nil {
		return run, err
	}
	var set ModelMap
	for role, ref := range refs {
		*role.field(&set) = ref.String()
		*role.field(&cfg.Models) = ref.String()
	}
	wrote, err := UpdateConfig(path, Config{Models: set})
	if err != nil {
		return run, err
	}
	run.Models = &cfg.Models
	if wrote {
		run.Written = append(run.Written, path)
	}
	return run, nil
}

// givenReferences is each role given a value for, as the reference it names
// at the pool entry that supplies it.
func givenReferences(in Inputs, layer string, given map[Role]string) (map[Role]Reference, error) {
	refs := map[Role]Reference{}
	var refusal result.Refusal
	for _, role := range Roles {
		value := strings.TrimSpace(given[role])
		if value == "" {
			continue
		}
		ref, err := ParseReference(value)
		if err != nil {
			refusal = append(refusal, result.Item{Check: result.CheckUsage, Key: argmark.Flag(string(role)),
				Detail: fmt.Sprintf("%s names %q, and a model reference is <provider>/<model>: a provider, then the model it serves",
					argmark.Flag(string(role)), value)})
			continue
		}
		refs[role] = ref
	}
	if len(refusal) > 0 {
		return nil, refusal
	}
	if len(refs) == 0 {
		return nil, result.Refusal{{Check: result.CheckUsage,
			Detail: fmt.Sprintf("name a reference for at least one role: %s, %s or %s",
				argmark.Flag(string(RoleDecision)), argmark.Flag(string(RoleText)), argmark.Flag(string(RoleScoring)))}}
	}
	for _, role := range Roles {
		ref, ok := refs[role]
		if !ok {
			continue
		}
		if _, found := in.Providers[ref.Provider]; !found {
			item := unresolvedProvider(ref.Provider, in)
			item.Key = argmark.Flag(string(role))
			// A faulted entry, which carries no remedy, keeps its fault's reason.
			if item.Remedy != "" {
				item.Detail = fmt.Sprintf("%s names provider `%s`, which no providers.toml this call reads holds", argmark.Flag(string(role)), ref.Provider)
				args := map[string]any{"provider": ref.Provider, "base-url": "<url>"}
				if layer != "" {
					args["layer"] = layer
				}
				item.Remedy = argmark.Call("configure-providers", args)
			}
			refusal = append(refusal, item)
		}
	}
	if len(refusal) > 0 {
		return nil, refusal
	}
	return refs, nil
}

// RolesNaming is each role, in role order, whose reference in cfg names
// provider, a lowercase name.
func RolesNaming(cfg Config, provider string) []RoleReference {
	var out []RoleReference
	for _, role := range Roles {
		if ref, err := ParseReference(role.model(cfg.Models)); err == nil && ref.Provider == provider {
			out = append(out, RoleReference{Role: role, Reference: ref})
		}
	}
	return out
}

// ProviderEntryRefusals is the refusal of a configure-providers entry: name
// must keep the provider-name rule; baseURL, api and decisionAPI, where given, must
// be an absolute http or https URL carrying no credentials and values the pool
// loads. Each fault is an item keyed by the argument that carries it, every one
// at once; none where the entry is one to write. No item names any part of
// baseURL, which may hold a key.
func ProviderEntryRefusals(name, baseURL, api, decisionAPI string) result.Refusal {
	var refusal result.Refusal
	if !providerName.MatchString(name) {
		refusal = append(refusal, result.Item{Check: result.CheckUsage, Key: argmark.Positional("provider"),
			Detail: fmt.Sprintf("%q is no provider name: %s", name, ProviderNameRule)})
	}
	switch {
	case baseURL == "":
	case baseURLCarriesCredentials(baseURL):
		refusal = append(refusal, result.Item{Check: result.CheckUsage, Key: argmark.Flag("base-url"), Remedy: AuthAddRemedy(name),
			Detail: WithRefusedKeyNote(argmark.Flag("base-url") + " carries credentials; a key is stored apart from the URL")})
	case !absoluteHTTPURL(baseURL):
		refusal = append(refusal, result.Item{Check: result.CheckUsage, Key: argmark.Flag("base-url"),
			Detail: argmark.Flag("base-url") + " is no absolute http or https URL"})
	}
	for _, c := range []struct {
		flag, value string
		allowed     []string
	}{
		{"api", api, ProviderAPIs},
		{"decision-api", decisionAPI, DecisionAPIs},
	} {
		if c.value != "" && !slices.Contains(c.allowed, c.value) {
			refusal = append(refusal, result.Item{Check: result.CheckUsage, Key: argmark.Flag(c.flag), Allowed: c.allowed,
				Detail: fmt.Sprintf("%s names %q, which is not one of %s", argmark.Flag(c.flag), c.value, strings.Join(c.allowed, ", "))})
		}
	}
	return refusal
}

// NoProvider is the refusal of a removal naming a provider the providers.toml
// at path holds no entry for. Its remedy is the call that lists the entries
// that stand.
func NoProvider(name, path string) result.Refusal {
	return result.Refusal{{Check: CheckProvider, Key: argmark.Positional("provider"), Path: path,
		Remedy: argmark.Call("show-config", nil), Detail: fmt.Sprintf("no provider `%s` in %s", name, path)}}
}
