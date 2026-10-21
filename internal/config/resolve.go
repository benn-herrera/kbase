package config

import (
	"fmt"
	"slices"
	"strings"
)

// Role is one thing a verb asks a model for on the KB's behalf. A role names
// its own endpoint and its own model in config.toml's [models] table, or in
// the environment by its own variable; no role's endpoint, model or provider
// is ever taken from another's.
type Role string

const (
	// RoleDecision is the claim graph's letter asks.
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

// variable is the role as the environment spells it: KBASE_<ROLE>.
func (r Role) variable() string { return "KBASE_" + strings.ToUpper(string(r)) }

// model is the role's value in the configuration files — a file-mode
// reference, or "" where no layer names one.
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

// fromEnv is the role's value in the environment — a bare model id of the
// environment's endpoint, or "".
func (r Role) fromEnv(env Overrides) string {
	switch r {
	case RoleDecision:
		return env.Decision
	case RoleText:
		return env.Text
	case RoleScoring:
		return env.Scoring
	}
	return ""
}

// Mode is which source supplies inference: the configuration files and the
// pool they draw on, or the environment's strings. There is no third, and the
// two are never merged.
type Mode string

const (
	// ModeFile is providers.toml plus config.toml.
	ModeFile Mode = "file"
	// ModeEnvironment is KBASE_API_BASE_URL plus the role variables.
	ModeEnvironment Mode = "environment"
)

// BackEnd is which seam fills an ask, as a run's records name it.
type BackEnd string

const (
	// BackEndProvider asks a pool entry, or the environment's endpoint.
	BackEndProvider BackEnd = "provider"
	// BackEndHarness asks a command kbase composes: the built-in back end
	// --harness selects.
	BackEndHarness BackEnd = "harness"
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
// the endpoint, and the model id that entry's server serves. A file-mode value
// is parsed from `<provider>/<model>`, split at the FIRST `/` — a model id may
// contain one (`openrouter/anthropic/...`), a provider name may not — and
// String writes it back, so no site concatenates a reference by hand. An
// environment-supplied model is no reference: it is a bare id of the
// environment's endpoint, and nothing parses or formats it.
type Reference struct{ Provider, Model string }

// ParseReference reads a file-mode model reference. Either part empty, or no
// `/` at all, is not a reference.
func ParseReference(s string) (Reference, error) {
	provider, model, split := strings.Cut(strings.TrimSpace(s), "/")
	provider, model = strings.TrimSpace(provider), strings.TrimSpace(model)
	if !split || provider == "" || model == "" {
		return Reference{}, fmt.Errorf("%q is not a model reference: name it <provider>/<model>, where <provider> is a table in %s", s, ProvidersFileName)
	}
	return Reference{Provider: provider, Model: model}, nil
}

// FileReference is ParseReference under the name a caller reading a file's
// value uses, so the mode a value came from is stated where it is read.
func FileReference(id string) (Reference, error) { return ParseReference(id) }

// String is the reference in its one written form, `<provider>/<model>`.
func (r Reference) String() string { return r.Provider + "/" + r.Model }

// Inputs is everything a resolution decision reads: the loaded files and the
// environment's strings, so ModeOf, Select and Resolve share one parameter
// list. WriteDir is the configuration directory a lacking setting belongs in.
type Inputs struct {
	Cfg       Config
	Providers Providers
	Faults    []ProviderFault
	WriteDir  string
	Env       Overrides
}

// Endpoint is one role's resolved endpoint: the entry that supplies it, the
// model it asks, and what named the entry. APIKeyFile is the path
// KBASE_API_KEY_FILE names, expanded and unread — the registry reads it — and
// APIKey is the key the pool entry's own file already supplied.
//
// The three source fields name the layer that supplied each field, for a
// report that states one; "" where no layer did, as under a flag that named
// the model itself.
type Endpoint struct {
	Role       Role
	Mode       Mode
	Provider   string
	Model      string
	Rule       string
	Layer      string
	BaseURL    string
	APIKeyFile string
	Flavor     string
	APIKey     string

	BaseURLSource, ModelSource, KeySource string
}

// ModeOf is which source supplies inference, decided once per invocation. It
// reads the loaded configuration and the environment's strings and nothing
// else: no file, no connection, no stderr. The sources are exclusive — an
// environment inference variable alongside a [models] value is a refusal, one
// item per side — and the environment is all or nothing: a role variable
// without KBASE_API_BASE_URL is a refusal naming the variable set and the one
// missing, and a base URL alone is the same refusal the other way round. The
// key file is not a mode: KBASE_API_KEY_FILE flips nothing.
func ModeOf(in Inputs) (Mode, error) {
	var variables []string
	for _, v := range []struct{ name, value string }{
		{EnvAPIBaseURL, in.Env.BaseURL},
		{RoleDecision.variable(), in.Env.Decision},
		{RoleText.variable(), in.Env.Text},
		{RoleScoring.variable(), in.Env.Scoring},
	} {
		if v.value != "" {
			variables = append(variables, v.name)
		}
	}
	var standing []Role
	for _, role := range Roles {
		if role.model(in.Cfg.Models) != "" {
			standing = append(standing, role)
		}
	}

	switch {
	case len(variables) == 0:
		return ModeFile, nil
	case len(standing) > 0:
		configFile := ConfigPath(in.WriteDir)
		var refusals Refusals
		for _, name := range variables {
			refusals = append(refusals, Refusal{Key: name,
				Detail: fmt.Sprintf("%s is set while %s names a model: the environment is the whole inference configuration or nothing, so unset %s, or remove the file's [%s] values",
					name, configFile, name, tableModels)})
		}
		for _, role := range standing {
			refusals = append(refusals, Refusal{Key: role.key(), Path: configFile,
				Detail: fmt.Sprintf("%s is set while the environment names inference: the environment is the whole inference configuration or nothing, so remove %s, or unset %s",
					role.key(), role.key(), role.variable())})
		}
		return "", refusals
	case in.Env.BaseURL == "":
		var refusals Refusals
		for _, name := range variables {
			refusals = append(refusals, Refusal{Key: name,
				Detail: fmt.Sprintf("%s is set, and %s is unset: the environment supplies an endpoint and a model together or neither, so set %s or unset %s",
					name, EnvAPIBaseURL, EnvAPIBaseURL, name)})
		}
		refusals = append(refusals, Refusal{Key: EnvAPIBaseURL,
			Detail: fmt.Sprintf("%s is unset, and %s names a model: the environment supplies an endpoint and a model together or neither, so set %s",
				EnvAPIBaseURL, strings.Join(variables, ", "), EnvAPIBaseURL)})
		return "", refusals
	}
	return ModeEnvironment, nil
}

// Resolve is one role's endpoint in a mode. It is pure, as ModeOf is: it reads
// no file, opens no connection, writes nothing and names no verb.
//
// The role's configured value is read first and a flag is an override of it,
// never a substitute for it: a role the source does not configure refuses even
// where flagModel names a model.
func Resolve(in Inputs, mode Mode, role Role, flagModel string) (Endpoint, error) {
	if mode == ModeEnvironment {
		return resolveEnvironment(in, role, flagModel)
	}
	return resolveFile(in, role, flagModel)
}

// configureRemedy is the command that rewrites a config.toml still naming a
// bare model id into the reference form.
const configureRemedy = "kbase configure --provider <entry> --model-map " + CategoryDecision + "=<entry>/<id>," + CategoryText + "=<entry>/<id>," + string(RoleScoring) + "=<entry>/<id>"

// resolveFile is a role's endpoint from the pool entry its reference names.
func resolveFile(in Inputs, role Role, flagModel string) (Endpoint, error) {
	configured := role.model(in.Cfg.Models)
	configFile := ConfigPath(in.WriteDir)
	if configured == "" {
		return Endpoint{}, Refusal{Key: role.key(), Path: configFile, Remedy: configureRemedy,
			Detail: fmt.Sprintf("%s sets no %s", configFile, role.key())}
	}
	// The flag overrides the configured value and never substitutes for it: a
	// role the files do not configure is refused above, whatever the flag says.
	value, rule := configured, RuleConfig
	if flagModel != "" {
		value, rule = flagModel, RuleFlag
	}
	ref, err := FileReference(value)
	if err != nil {
		if rule == RuleFlag {
			return Endpoint{}, Refusal{Check: CheckUsage, Key: keyScoringModel,
				Detail: fmt.Sprintf("%s names %q, and a model is named <provider>/<model> — the entry in %s that supplies the endpoint, then the model it serves",
					keyScoringModel, value, ProvidersFileName)}
		}
		return Endpoint{}, Refusal{Key: role.key(), Path: configFile, Remedy: configureRemedy,
			Detail: fmt.Sprintf("%s names %q, and a model is named <provider>/<model> — the entry in %s that supplies the endpoint, then the model it serves",
				role.key(), value, ProvidersFileName)}
	}
	entry, ok := in.Providers[ref.Provider]
	if !ok {
		// The refusal is about what named the value: the role's own key where
		// the file supplied it, the flag where the flag did, and either offers
		// the pool's entries as the names that would work.
		refusal := unresolvedProvider(ref.Provider, in)
		if rule == RuleFlag {
			refusal.Key = keyScoringModel
			refusal.Detail = fmt.Sprintf("%s names a provider the pool does not hold: %s", keyScoringModel, refusal.Detail)
		} else {
			refusal.Key = role.key()
		}
		return Endpoint{}, refusal
	}
	ep := Endpoint{Role: role, Mode: ModeFile, Provider: ref.Provider, Model: ref.Model, Rule: rule,
		Layer: entry.Layer, BaseURL: entry.BaseURL, APIKey: entry.APIKey, Flavor: entry.Flavor(),
		BaseURLSource: entry.Layer, ModelSource: in.Cfg.ModelLayer(role)}
	if keyKind(entry) == KeyKindFile {
		ep.KeySource = entry.Layer
	}
	// The key file is no mode: KBASE_API_KEY_FILE replaces the key of whichever
	// endpoint is in force, and the pool entry's own file is what stands where
	// it names none.
	if path := expandHome(in.Env.APIKeyFile); path != "" {
		ep.APIKeyFile, ep.KeySource = path, LayerSourceEnvironment
	}
	return ep, nil
}

// resolveEnvironment is a role's endpoint from the environment's strings.
func resolveEnvironment(in Inputs, role Role, flagModel string) (Endpoint, error) {
	configured := role.fromEnv(in.Env)
	if configured == "" {
		return Endpoint{}, Refusal{Key: role.variable(),
			Detail: fmt.Sprintf("%s is unset, and the environment mode supplies every model by its own variable", role.variable())}
	}
	ep := Endpoint{Role: role, Mode: ModeEnvironment, Model: configured, BaseURL: in.Env.BaseURL,
		Flavor: FlavorSystemOne, BaseURLSource: LayerSourceEnvironment, ModelSource: LayerSourceEnvironment}
	if flagModel != "" {
		// The flag is the whole of what named the model, and no layer did.
		ep.Model, ep.ModelSource = flagModel, ""
	}
	if path := expandHome(in.Env.APIKeyFile); path != "" {
		ep.APIKeyFile, ep.KeySource = path, LayerSourceEnvironment
	}
	return ep, nil
}

// BackEndModel is the model a harness back end is handed, as that back end
// names models: the source's value as it stands — a file reference's
// <provider>/<model>, or the environment's id — overridden by --scoring-model.
// source is the layer that supplied it, "" where the flag did, so a document
// naming the model names where it came from.
//
// The environment's id stands where it names one, else the file's reference; a
// role no source configures refuses. No entry is contacted, so the pool matters
// to nothing here: a harness back end chooses no endpoint.
func BackEndModel(in Inputs, role Role, flagModel string) (written, source string, err error) {
	written = role.fromEnv(in.Env)
	source = LayerSourceEnvironment
	if written == "" {
		if value := role.model(in.Cfg.Models); value != "" {
			if _, err := FileReference(value); err != nil {
				return "", "", Refusal{Key: role.key(), Path: ConfigPath(in.WriteDir), Remedy: configureRemedy,
					Detail: fmt.Sprintf("%s names %q, and a model is named <provider>/<model>", role.key(), value)}
			}
			written, source = value, in.Cfg.ModelLayer(role)
		}
	}
	if written == "" {
		return "", "", Refusal{Key: role.key(), Path: ConfigPath(in.WriteDir),
			Detail: fmt.Sprintf("no model is configured for the %s role: set %s, or %s", role, role.key(), role.variable())}
	}
	if flagModel != "" {
		written, source = flagModel, ""
	}
	return written, source, nil
}

// Select is a catalogue verb's choice of which pool entry to dial — not a
// role: a role is never chosen this way, and config.toml names no entry
// (its provider key is superseded and inert). The --provider flag answers
// first; otherwise the pool's only entry does. The rule that answered is
// returned, so a caller reporting the choice does not decide the precedence a
// second time.
func Select(in Inputs, flag string) (name, rule string, err error) {
	if named := strings.TrimSpace(flag); named != "" {
		if _, ok := in.Providers[named]; !ok {
			return "", "", unresolvedProvider(named, in)
		}
		return named, RuleFlag, nil
	}
	if len(in.Providers) == 0 {
		providersFile := ProvidersPath(in.WriteDir)
		return "", "", Refusal{Path: providersFile,
			Detail: fmt.Sprintf("no usable provider in %s — declare one there", providersFile)}
	}
	if len(in.Providers) == 1 {
		for named := range in.Providers {
			return named, RuleSoleEntry, nil
		}
	}
	names := providerNames(in.Providers)
	return "", "", Refusal{Key: "--provider", Allowed: names,
		Detail: fmt.Sprintf("no provider selected — pass --provider (available: %s)", strings.Join(names, ", "))}
}

// unresolvedProvider explains a name the pool does not hold, distinguishing a
// provider that FAULTED out of the pool from one that was never declared:
// without the distinction a dropped provider reads exactly like a typo, and
// the user goes looking for the wrong mistake. A faulted name keeps the
// file's own reason and gets no creation remedy — the file stands, and what is
// wrong is the entry in it.
func unresolvedProvider(name string, in Inputs) Refusal {
	providersFile := ProvidersPath(in.WriteDir)
	refusal := Refusal{Key: "--provider", Path: providersFile, Allowed: providerNames(in.Providers), Remedy: "kbase configure --create",
		Detail: fmt.Sprintf("provider %q not found in %s — a pool entry there supplies baseUrl and apiKeyFile", name, providersFile)}
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

// CheckHarness is the refusal class the built-in harness back end raises where
// it cannot run what it is given, and CheckUsage a flag value that is not the
// form the flag takes; a resolution refusal's Check is otherwise empty, which is
// the provider choice its rows are about.
const (
	CheckHarness = "harness"
	CheckUsage   = "usage"
)

// keyScoringModel is the flag that overrides a role's model, as a refusal names
// it: a bad value, or a provider the pool lacks, is the flag's fault to state.
const keyScoringModel = "--scoring-model"

// Refusal is a setting inference cannot resolve, named as the item a result
// document refuses it with: the setting's key or variable, the file it belongs
// in, the values it may take, the command that clears it, and one sentence.
type Refusal struct {
	// Check names the refusal class where it is not the provider choice.
	Check   string
	Key     string
	Path    string
	Allowed []string
	Remedy  string
	Detail  string
}

func (r Refusal) Error() string { return r.Detail }

// Named is the setting and the file the refusal names, as the build's launch
// refusal reads them.
func (r Refusal) Named() (key, path string) { return r.Key, r.Path }

// Refusals is every refusal one decision produced, where the decision has more
// than one side to state — the mixed sources, the environment missing its
// endpoint.
type Refusals []Refusal

func (rs Refusals) Error() string {
	details := make([]string, 0, len(rs))
	for _, r := range rs {
		details = append(details, r.Detail)
	}
	return strings.Join(details, "; ")
}
