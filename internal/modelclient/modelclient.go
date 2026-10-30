// Package modelclient is the registry of the model clients one verb opens, and
// the one mapping from configuration's vocabulary onto the wire shapes the
// model package speaks.
package modelclient

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"kbase/internal/config"
	"kbase/internal/log"
	"kbase/internal/model"
	"kbase/internal/result"
)

// openedKey identifies an open client: the endpoint it dials (the entry's name,
// or "base:" and the base URL in environment mode), the wire protocol it speaks
// and its resolved sampling — defaults filled in, so a setting left unset and
// one stating its default key alike.
type openedKey struct {
	endpoint    string
	protocol    model.Protocol
	temperature float64
	thinking    model.Thinking
}

// NewClientFunc constructs the model client for an endpoint. It is the seam
// that lets a verb be exercised without a network — production passes
// model.NewClient, tests pass a factory returning a test double's
// model.ClientFuncs, such as a modeltest.MockClient's Client.
type NewClientFunc func(model.Endpoint) model.Client

// Registry is the registry of the model clients one verb opens: at most one per
// endpoint, wire protocol and resolved sampling, opened on first use and closed
// together before the verb returns. It is the only constructor of a model.Client in kbase, and
// it derives no deadline — each caller bounds its own call.
type Registry struct {
	newClient NewClientFunc
	// promptCacheKey is what every client the registry opens stamps on the
	// requests it sends, "" for none: a scoring run passes its run id, so
	// every ask of the run carries one key; every other verb names no run and
	// passes nothing.
	promptCacheKey string
	// idleConns is what every client the registry opens keeps idle per host.
	idleConns int
	opened    map[openedKey]model.Client
	// lg takes a close that failed. A close error is never raised: the verb
	// has usually written its result by then.
	lg log.Logger
}

// New is a registry over seam, nil taking model.NewClient — which builds the
// wire client the endpoint's protocol names — whose close failures go to lg and
// whose every client carries promptCacheKey and keeps idleConns idle
// connections to its host (0 leaves the transport's defaults).
func New(seam NewClientFunc, lg log.Logger, promptCacheKey string, idleConns int) *Registry {
	if seam == nil {
		seam = model.NewClient
	}
	if lg == nil {
		lg = log.Discard()
	}
	return &Registry{newClient: seam, promptCacheKey: promptCacheKey, idleConns: idleConns, opened: map[openedKey]model.Client{}, lg: lg}
}

// Get is the client for ep, opened on first use: roles naming one pool entry,
// speaking one wire protocol and resolving to equal sampling — an unset
// setting equal to its stated default — share a client and its connection
// pool, and an environment-mode endpoint shares by base URL, which is all it
// has. A role whose resolved sampling differs gets its own client, since a
// client asks for one sampling on every call.
// The decision role always speaks the OpenAI protocol, the only one carrying
// the decision surfaces; every other role speaks the protocol its entry's `api`
// names.
func (r *Registry) Get(ep config.Endpoint) (model.Client, error) {
	protocol := chatProtocol(ep.Protocol)
	if ep.Role == config.RoleDecision {
		protocol = model.ProtocolOpenAI
	}
	sampling := samplingOf(ep)
	resolved := sampling.Resolved()
	key := openedKey{endpoint: ep.Provider, protocol: protocol, temperature: *resolved.Temperature, thinking: *resolved.Thinking}
	if key.endpoint == "" {
		key.endpoint = "base:" + ep.BaseURL
	}
	if client, ok := r.opened[key]; ok {
		return client, nil
	}
	apiKey, err := r.Key(ep)
	if err != nil {
		return nil, err
	}
	client := r.newClient(model.Endpoint{Name: ep.Provider, BaseURL: ep.BaseURL, APIKey: apiKey,
		Protocol: protocol, Decisions: decisionsSurface(ep.Flavor), PromptCacheKey: r.promptCacheKey, IdleConns: r.idleConns,
		Sampling: sampling})
	r.opened[key] = client
	return client, nil
}

// Preflight is the key-file read alone: the refusal a file KBASE_API_KEY_FILE
// names gives where it cannot be read, and nothing opened. It is what a verb
// that reports an endpoint without dialling it can still learn.
func (r *Registry) Preflight(ep config.Endpoint) error {
	_, err := r.Key(ep)
	return err
}

// Key is the key ep is dialled with: the file KBASE_API_KEY_FILE names where
// it names one, and otherwise the key the pool entry's own file supplied.
func (r *Registry) Key(ep config.Endpoint) (string, error) {
	if ep.APIKeyFile == "" {
		return ep.APIKey, nil
	}
	b, err := os.ReadFile(ep.APIKeyFile)
	if err != nil {
		return "", result.Refusal{{Check: config.CheckProvider, Key: config.EnvAPIKeyFile, Path: ep.APIKeyFile,
			Detail: fmt.Sprintf("%s names %s, which cannot be read: %v", config.EnvAPIKeyFile, ep.APIKeyFile, err)}}
	}
	return strings.TrimSpace(string(b)), nil
}

// CloseAll closes every client the registry opened, whatever the verb's
// outcome. A client that is no io.Closer — a test's mock — has nothing to
// release.
func (r *Registry) CloseAll() {
	for key, client := range r.opened {
		closer, ok := client.(io.Closer)
		if !ok {
			continue
		}
		if err := closer.Close(); err != nil {
			r.lg.Warn("a model client was not closed cleanly", "endpoint", key.endpoint, "error", err)
		}
	}
	r.opened = map[openedKey]model.Client{}
}

// Catalogue is the model ids entry's provider lists, in the order it listed
// them: callers that show them to a human sort first; callers that classify
// them do not care. A key file that cannot be read is the refusal Get gives.
func (r *Registry) Catalogue(ctx context.Context, entry config.Provider) ([]string, error) {
	client, err := r.Get(entryEndpoint(entry))
	if err != nil {
		return nil, err
	}
	return catalogueIDs(ctx, client)
}

// entryEndpoint is a pool entry as the registry takes it, for a verb that dials
// the entry it selected to read that entry's catalogue: no role, no model, and
// nothing the registry does not read.
func entryEndpoint(entry config.Provider) config.Endpoint {
	return config.Endpoint{Provider: entry.Name, BaseURL: entry.BaseURL, APIKey: entry.APIKey,
		Protocol: entry.Protocol(), Flavor: entry.Flavor()}
}

// catalogueIDs fetches the provider's catalogue and returns the model ids in
// the order the provider listed them.
func catalogueIDs(ctx context.Context, client model.Client) ([]string, error) {
	infos, err := client.ListModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	ids := make([]string, 0, len(infos))
	for _, mi := range infos {
		ids = append(ids, mi.ID)
	}
	return ids, nil
}

// decisionsSurface is whether the decision surface a configured flavour names is
// the OpenAI Decisions shape. The flavour vocabulary is configuration's — it
// refuses a value it does not know at load — and the wire shapes are model's,
// so the one mapping between the two lives here.
func decisionsSurface(flavor string) bool { return flavor == config.FlavorDecisions }

// chatProtocol is the chat wire protocol a configured `api` names, in the model
// package's vocabulary. As decisionsSurface is for the decision surfaces: the
// configured value is configuration's — it refuses one it does not know at load
// — and the wire shapes are model's, so the one mapping between the two lives
// here.
func chatProtocol(api string) model.Protocol {
	if api == config.ProviderAPIAnthropic {
		return model.ProtocolAnthropic
	}
	return model.ProtocolOpenAI
}

// samplingOf is the sampling ep's role states, in the model package's
// vocabulary, a field left nil where the role states none. As chatProtocol is
// for `api`: the thinking level is configuration's — it refuses one it does
// not know at load — and model.Thinking is model's, so the one mapping between
// the two lives here.
func samplingOf(ep config.Endpoint) model.Sampling {
	s := model.Sampling{Temperature: ep.Temperature}
	switch ep.Thinking {
	case config.ThinkingOff:
		s.Thinking = new(model.ThinkingOff)
	case config.ThinkingLow:
		s.Thinking = new(model.ThinkingLow)
	case config.ThinkingMedium:
		s.Thinking = new(model.ThinkingMedium)
	case config.ThinkingHigh:
		s.Thinking = new(model.ThinkingHigh)
	}
	return s
}
