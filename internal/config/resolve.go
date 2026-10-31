package config

import (
	"fmt"
	"slices"
	"strings"

	"kbase/internal/result"
	"kbase/internal/secret"
)

// Role is one thing a verb asks a model for on the KB's behalf. A role names
// its own endpoint and its own model in config.toml's [models] table; no
// role's endpoint, model or provider is ever taken from another's.
type Role string

const (
	// RoleDecision is the claim graph's multichoice asks.
	RoleDecision Role = CategoryDecision
	// RoleText is the overview passage.
	RoleText Role = CategoryText
	// RoleScoring is kbase score's asks.
	RoleScoring Role = CategoryScoring
)

// Roles is every role, in the order documents report them.
var Roles = []Role{RoleDecision, RoleText, RoleScoring}

// key is the role as config.toml's [models] table spells it.
func (r Role) key() string { return tableModels + "." + string(r) }

// model is the role's value in the configuration files — a reference, or ""
// where no layer names one.
func (r Role) model(cfg ModelMap) string {
	switch r {
	case RoleDecision:
		return cfg.Decision
	case RoleText:
		return cfg.Text
	case RoleScoring:
		return cfg.Scoring
	}
	return ""
}

// BackEnd is which seam fills an ask, as a run's records name it.
type BackEnd string

const (
	// BackEndProvider asks a pool entry.
	BackEndProvider BackEnd = "provider"
)

// The rules that name an endpoint's provider, as a document reports them. A
// role's own reference names it — config — and --scoring-model names it where
// the flag is given; a catalogue verb names a pool entry by its flag, or takes
// the pool's only entry.
const (
	RuleFlag      = "flag"
	RuleConfig    = "config"
	RuleSoleEntry = "sole-entry"
)

// Reference is a model named for one endpoint: the pool entry that supplies
// the endpoint, and the model id that entry's server serves. A value is parsed
// from `<provider>/<model>`, split at the FIRST `/` — a model id may contain
// one (`openrouter/anthropic/...`), a provider name may not — and String
// writes it back, so no site concatenates a reference by hand.
type Reference struct{ Provider, Model string }

// ParseReference reads a model reference. Either part empty, or no `/` at
// all, is not a reference.
func ParseReference(s string) (Reference, error) {
	provider, model, split := strings.Cut(strings.TrimSpace(s), "/")
	provider, model = strings.TrimSpace(provider), strings.TrimSpace(model)
	if !split || provider == "" || model == "" {
		return Reference{}, fmt.Errorf("%q is not a model reference: name it <provider>/<model>, where <provider> is a table in %s", s, ProvidersFileName)
	}
	return Reference{Provider: provider, Model: model}, nil
}

// String is the reference in its one written form, `<provider>/<model>`.
func (r Reference) String() string { return r.Provider + "/" + r.Model }

// Inputs is everything a resolution decision reads — the loaded files — so
// Select and Resolve share one parameter list. WriteDir is the configuration
// directory a lacking setting belongs in. ProvidersFiles is each providers.toml
// that stood when the layers were loaded (Loaded.ProvidersFiles).
type Inputs struct {
	Cfg            Config
	Providers      Providers
	Faults         []ProviderFault
	WriteDir       string
	ProvidersFiles []string
}

// Endpoint is one role's resolved endpoint: the entry that supplies it, the
// model it asks, and what named the entry. APIKey is the key the pool entry
// resolved to: the store's, else its own file's.
//
// The source fields name the layer that supplied each field, for a report
// that states one; "" where no layer did, as under a flag that named the model
// itself. KeySource names the store, KeyKindStore, where the store supplied
// the key.
type Endpoint struct {
	Role     Role
	Provider string
	Model    string
	Rule     string
	Layer    string
	BaseURL  string
	Flavor   string
	// Protocol is the chat wire protocol the entry speaks, its default filled
	// in — `anthropic` where the entry states none. The vocabulary is this
	// package's, and the model package is handed the resolved fact rather than
	// the string.
	Protocol string
	APIKey   secret.Key

	// Temperature and Thinking are the role's sampling as the files state it,
	// unset where no layer does: nil and "", never kbase's default filled in.
	// The decision role carries neither.
	Temperature *float64
	Thinking    string

	BaseURLSource, ModelSource, KeySource string
	TemperatureSource, ThinkingSource     string
}

// Resolve is one role's endpoint, from the pool entry its reference names. It
// is pure: it reads no file, opens no connection and writes nothing; a
// refusal's remedy may name the command that fixes it.
//
// A flag names the role's model where the files name none and overrides the
// value where they name one.
func Resolve(in Inputs, role Role, flagModel string) (Endpoint, error) {
	return ResolveFlag(in, role, flagModel, keyScoringModel, false)
}

// ResolveFlag is Resolve with the flag that supplies a role's model spelled by
// the caller — flagModel is the flag's value and flagKey its name — so a verb
// whose flag is not --scoring-model is pointed at its own: a value that is no
// reference, and a provider the pool does not hold, name the flag they came
// from. flagKey is "" where no flag supplies a role's model — a verb that asks
// a role by name supplies none — and a role the files leave unnamed then names
// only its own key.
//
// defaulted states that the role is the caller's own default, nothing having
// named a selection, rather than one a flag named. It is consulted only where
// flagKey names a flag: the role the files leave unnamed is then refused by its
// key as the first remedy and that flag as the other, and never as though the
// flag had been passed.
func ResolveFlag(in Inputs, role Role, flagModel, flagKey string, defaulted bool) (Endpoint, error) {
	configured := role.model(in.Cfg.Models)
	configFile := ConfigPath(in.WriteDir)
	// The flag names the role's model where the files name none, and overrides
	// the file's value where they name one.
	value, rule := configured, RuleConfig
	if flagModel != "" {
		value, rule = flagModel, RuleFlag
	}
	if value == "" {
		// The key is what a role left unnamed is refused by, and the three ways
		// nothing supplied a value read differently: a role flag named the role
		// and supplies no model, so the key is the whole of what is wrong;
		// nothing named a selection, so the role is the caller's default and
		// the key is the first remedy, the flag the other; else a flag could
		// have supplied the value and named none.
		var detail string
		switch {
		case flagKey == "":
			detail = fmt.Sprintf("%s sets no %s — name one there as <provider>/<model>", configFile, role.key())
		case defaulted:
			detail = fmt.Sprintf("%s sets no %s — nothing named a selection, so the %s role is the default; name one there as <provider>/<model>, or pass %s to name a reference instead",
				configFile, role.key(), role, flagKey)
		default:
			detail = fmt.Sprintf("%s sets no %s, and %s names no model — the flag supplies a role the files do not configure",
				configFile, role.key(), flagKey)
		}
		return Endpoint{}, result.Refusal{{Check: CheckProvider, Key: role.key(), Path: configFile, Remedy: configureRemedy, Detail: detail}}
	}
	ref, err := ParseReference(value)
	if err != nil {
		if rule == RuleFlag {
			return Endpoint{}, result.Refusal{{Check: result.CheckUsage, Key: flagKey,
				Detail: fmt.Sprintf("%s names %q, and a model is named <provider>/<model> — the entry in %s that supplies the endpoint, then the model it serves",
					flagKey, value, ProvidersFileName)}}
		}
		return Endpoint{}, result.Refusal{{Check: CheckProvider, Key: role.key(), Path: configFile, Remedy: configureRemedy,
			Detail: fmt.Sprintf("%s names %q, and a model is named <provider>/<model> — the entry in %s that supplies the endpoint, then the model it serves",
				role.key(), value, ProvidersFileName)}}
	}
	entry, ok := in.Providers[ref.Provider]
	if !ok {
		// The refusal is about what named the value: the role's own key where
		// the file supplied it, the flag where the flag did, and either offers
		// the pool's entries as the names that would work.
		item := unresolvedProvider(ref.Provider, in)
		if rule == RuleFlag {
			item.Key = flagKey
			item.Detail = fmt.Sprintf("%s names a provider the pool does not hold: %s", flagKey, item.Detail)
		} else {
			item.Key = role.key()
		}
		return Endpoint{}, result.Refusal{item}
	}
	ep := Endpoint{Role: role, Provider: ref.Provider, Model: ref.Model, Rule: rule,
		Layer: entry.Layer, BaseURL: entry.BaseURL, APIKey: entry.APIKey, Flavor: entry.Flavor(),
		Protocol:      entry.Protocol(),
		BaseURLSource: entry.Layer, ModelSource: in.Cfg.ModelLayer(role)}
	switch keyKind(entry) {
	case KeyKindStore:
		ep.KeySource = string(KeyKindStore)
	case KeyKindFile:
		ep.KeySource = entry.Layer
	}
	return withSampling(in, ep)
}

// configureRemedy is the command that rewrites a config.toml still naming a
// bare model id into the reference form.
const configureRemedy = "kbase configure --provider <entry> " + modelMapSyntax

// authAddCommand is the command that stores a key for the provider name,
// replacing any stored for it; the key arrives on its stdin.
func authAddCommand(provider string) string { return "kbase auth add " + provider }

// AuthAddRemedy is how a key reaches the store for provider, as a remedy
// spells it: piped on stdin from wherever the user keeps it, never typed at a
// prompt or given as an argument.
func AuthAddRemedy(provider string) string {
	return "<command-that-prints-the-key> | " + authAddCommand(provider)
}

// AuthRemoveCommand is the command that clears the key stored for provider.
func AuthRemoveCommand(provider string) string { return "kbase auth remove " + provider }

// RefusedKeyNote ends the detail of a call failure that carries AuthAddRemedy,
// after "; ": it tells an agent reading the failure how the key reaches the
// store without entering the conversation.
const RefusedKeyNote = "`remedy` stores a key: the user runs it, or an agent runs it with a key-printing command the user names or show-config's `upgrade` gives; no one asks for the key or reads a key file into the conversation"

// WithRefusedKeyNote is detail ended by RefusedKeyNote, joined by "; ".
func WithRefusedKeyNote(detail string) string {
	return detail + "; " + RefusedKeyNote
}

// temperatureMax is the top of the temperature range each chat protocol
// accepts; the bottom is 0 for both.
func temperatureMax(protocol string) float64 {
	switch protocol {
	case ProviderAPIAnthropic:
		return 1
	default:
		return 2
	}
}

// withSampling gives the text and scoring roles' endpoint the sampling the
// files state, with the layer of each value, and refuses what the endpoint's
// chat protocol would reject. The refusal's path is the write target's
// config.toml, the file a setting belongs in, while its detail and remedy name
// the layer that states the value, the only one removing it works in.
func withSampling(in Inputs, ep Endpoint) (Endpoint, error) {
	role := ep.Role
	if role != RoleText && role != RoleScoring {
		return ep, nil
	}
	s := in.Cfg.samplingOf(role)
	ep.Temperature, ep.Thinking = s.Temperature, s.Thinking
	ep.TemperatureSource, ep.ThinkingSource = in.Cfg.SamplingLayer(role)
	if ep.Temperature == nil {
		return ep, nil
	}

	key := "sampling." + string(role) + ".temperature"
	t, limit := *ep.Temperature, temperatureMax(ep.Protocol)
	if !(t >= 0 && t <= limit) {
		return Endpoint{}, result.Refusal{{Check: CheckProvider, Key: key, Path: ConfigPath(in.WriteDir),
			Detail: fmt.Sprintf("%s, from the %s layer, is %g, outside the 0 to %g the %q chat protocol accepts", key, ep.TemperatureSource, t, limit, ep.Protocol),
			Remedy: fmt.Sprintf("set %s within 0 to %g, or remove it from the %s layer for kbase's default 0", key, limit, ep.TemperatureSource)}}
	}
	return ep, nil
}

// Select is a catalogue verb's choice of which pool entry to dial — not a
// role: a role is never chosen this way, and config.toml names no entry. The
// --provider flag answers first; otherwise the pool's only entry does. The rule
// that answered is returned, so a caller reporting the choice does not decide
// the precedence a second time.
func Select(in Inputs, flag string) (name, rule string, err error) {
	if named := strings.TrimSpace(flag); named != "" {
		if _, ok := in.Providers[named]; !ok {
			return "", "", result.Refusal{unresolvedProvider(named, in)}
		}
		return named, RuleFlag, nil
	}
	if len(in.Providers) == 0 {
		providersFile := ProvidersPath(in.WriteDir)
		return "", "", result.Refusal{{Check: CheckProvider, Path: providersFile,
			Detail: fmt.Sprintf("no usable provider in %s — declare one there", providersFile)}}
	}
	if len(in.Providers) == 1 {
		for named := range in.Providers {
			return named, RuleSoleEntry, nil
		}
	}
	names := providerNames(in.Providers)
	return "", "", result.Refusal{{Check: CheckProvider, Key: "--provider", Allowed: names,
		Detail: fmt.Sprintf("no provider selected — pass --provider (available: %s)", strings.Join(names, ", "))}}
}

// unresolvedProvider explains a name the pool does not hold, distinguishing a
// provider that FAULTED out of the pool from one that was never declared:
// without the distinction a dropped provider reads exactly like a typo, and
// the user goes looking for the wrong mistake. A faulted name keeps the
// file's own reason. The creation remedy is given only where the file does not
// stand: configure --create never overwrites one, so there it would change
// nothing, and the detail already says what to add.
func unresolvedProvider(name string, in Inputs) result.Item {
	providersFile := ProvidersPath(in.WriteDir)
	refusal := result.Item{Check: CheckProvider, Key: "--provider", Path: providersFile, Allowed: providerNames(in.Providers),
		Detail: fmt.Sprintf("provider %q not found in %s — add a [%s] table there with its baseUrl", name, providersFile, name)}
	if !slices.Contains(in.ProvidersFiles, providersFile) {
		refusal.Remedy = "kbase configure --create"
	}
	for _, fault := range in.Faults {
		// A warning fault belongs to a provider that loaded, so it can
		// never be the reason one is missing from the pool.
		if fault.Name == name && !fault.Warning {
			refusal.Detail = fmt.Sprintf("provider %q failed to load: %s", name, fault.Reason)
			refusal.Remedy = ""
		}
	}
	return refusal
}

// providerNames returns the pool's entry names, sorted — a stable list for
// refusals the user is expected to choose from.
func providerNames(providers Providers) []string {
	names := make([]string, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// CheckProvider is the refusal class of a provider choice, which every
// resolution refusal is unless it names its own.
const CheckProvider = "provider"

// keyScoringModel is the flag that overrides a role's model, as a refusal names
// it: a bad value, or a provider the pool lacks, is the flag's fault to state.
const keyScoringModel = "--scoring-model"
