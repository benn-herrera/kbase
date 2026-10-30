package modelclient

import (
	"testing"

	"kbase/internal/config"
	"kbase/internal/model"
	"kbase/internal/model/modeltest"
)

// closeLog counts the closes the registry's clients reported. cmd's tests keep
// their own copy: an unexported name does not cross the package line.
type closeLog struct{ n int }

// closer is a mock client that records its Close — the release the registry
// owes every client it opened.
type closer struct {
	model.Client
	log *closeLog
}

func (c closer) Close() error { c.log.n++; return nil }

// TestRegistryOpensEachEndpointOnce: a client is opened on first use and
// reused — two roles on one pool entry share it and its connection pool, and
// another entry gets its own — and CloseAll releases every one it opened, once.
func TestRegistryOpensEachEndpointOnce(t *testing.T) {
	var log closeLog
	var opened int
	reg := New(func(model.Endpoint) model.Client {
		opened++
		return closer{modeltest.NewScriptedMock(nil, nil).Client(), &log}
	}, nil, "", 0)
	lab := config.Endpoint{Provider: "lab", BaseURL: "http://lab.example/v1"}
	first, second := reg.Get(lab), reg.Get(lab)
	if opened != 1 || first != second {
		t.Errorf("the same entry opened %d client(s); want one, shared", opened)
	}
	reg.Get(config.Endpoint{Provider: "cloud", BaseURL: "http://cloud.example/v1"})
	if opened != 2 {
		t.Errorf("opened %d clients, want one per endpoint", opened)
	}
	reg.CloseAll()
	if log.n != 2 {
		t.Errorf("closed %d clients, want every one opened", log.n)
	}
	reg.CloseAll()
	if log.n != 2 {
		t.Errorf("a second close closed %d more", log.n-2)
	}
}

// TestRegistryPutsItsIdleConnsOnEveryEndpoint: the count the registry was
// built with rides every endpoint it hands its seam.
func TestRegistryPutsItsIdleConnsOnEveryEndpoint(t *testing.T) {
	var got []int
	reg := New(func(e model.Endpoint) model.Client {
		got = append(got, e.IdleConns)
		return modeltest.NewScriptedMock(nil, nil).Client()
	}, nil, "", 4)
	for _, name := range []string{"a", "b"} {
		reg.Get(config.Endpoint{Provider: name, BaseURL: "http://" + name + ".example/v1"})
	}
	if len(got) != 2 || got[0] != 4 || got[1] != 4 {
		t.Errorf("IdleConns on the endpoints = %v, want [4 4]", got)
	}
}

// TestRegistryGivesTheDecisionRoleTheOpenAIProtocol: on an entry whose api is
// anthropic the text role gets a Messages client and the decision role an
// OpenAI one, each shared by its repeat Gets.
func TestRegistryGivesTheDecisionRoleTheOpenAIProtocol(t *testing.T) {
	var got []model.Protocol
	reg := New(func(e model.Endpoint) model.Client {
		got = append(got, e.Protocol)
		return modeltest.NewScriptedMock(nil, nil).Client()
	}, nil, "", 0)
	for _, tc := range []struct {
		role config.Role
		want model.Protocol
	}{
		{config.RoleText, model.ProtocolAnthropic},
		{config.RoleDecision, model.ProtocolOpenAI},
	} {
		ep := config.Endpoint{Role: tc.role, Provider: "lab", BaseURL: "http://lab.example/v1", Protocol: config.ProviderAPIAnthropic}
		first, second := reg.Get(ep), reg.Get(ep)
		if first != second {
			t.Errorf("role %v: the second Get opened another client", tc.role)
		}
		if n := len(got); n == 0 || got[n-1] != tc.want {
			t.Errorf("role %v: seam protocol = %v, want %v", tc.role, got, tc.want)
		}
	}
	if len(got) != 2 {
		t.Errorf("%d seam calls, want 2", len(got))
	}
}

// sampledRole is a role and the sampling it states, nil and "" stating none.
type sampledRole struct {
	role        config.Role
	temperature *float64
	thinking    string
}

// statesSampling is whether s is the sampling r states, a field nil where r
// states none.
func statesSampling(s model.Sampling, r sampledRole) bool {
	temperatureOK := (s.Temperature == nil) == (r.temperature == nil) &&
		(s.Temperature == nil || *s.Temperature == *r.temperature)
	thinkingOK := (s.Thinking == nil) == (r.thinking == "") &&
		(s.Thinking == nil || s.Thinking.String() == r.thinking)
	return temperatureOK && thinkingOK
}

// TestRegistrySharesAClientOnlyBetweenEqualSampling: two roles on one `openai`
// entry share a client exactly when they send equal sampling, an unset setting
// equal to its stated default, and each client opened carries the sampling the
// role that opened it states.
func TestRegistrySharesAClientOnlyBetweenEqualSampling(t *testing.T) {
	decision := sampledRole{role: config.RoleDecision}
	for _, tc := range []struct {
		name          string
		first, second sampledRole
		wantOpened    int
	}{
		{"decision unset, then text unset", decision, sampledRole{role: config.RoleText}, 1},
		{"decision unset, then text thinking low", decision, sampledRole{role: config.RoleText, thinking: config.ThinkingLow}, 2},
		{"text 0.3, then scoring 0.5", sampledRole{config.RoleText, new(0.3), ""}, sampledRole{config.RoleScoring, new(0.5), ""}, 2},
		{"text 0.3, then scoring 0.3", sampledRole{config.RoleText, new(0.3), ""}, sampledRole{config.RoleScoring, new(0.3), ""}, 1},
		{"decision unset, then text stating the defaults", decision,
			sampledRole{config.RoleText, new(model.DefaultTemperature), config.ThinkingOff}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var opened []model.Endpoint
			reg := New(func(e model.Endpoint) model.Client {
				opened = append(opened, e)
				return modeltest.NewScriptedMock(nil, nil).Client()
			}, nil, "", 0)
			lab := config.Endpoint{Provider: "lab", BaseURL: "http://lab.example/v1", Protocol: config.ProviderAPIOpenAI}
			roles := []sampledRole{tc.first, tc.second}
			var clients []model.Client
			for _, r := range roles {
				ep := lab
				ep.Role, ep.Temperature, ep.Thinking = r.role, r.temperature, r.thinking
				clients = append(clients, reg.Get(ep))
			}
			shared := clients[0] == clients[1]
			if len(opened) != tc.wantOpened || shared != (tc.wantOpened == 1) {
				t.Fatalf("opened %d client(s), shared %t; want %d", len(opened), shared, tc.wantOpened)
			}
			for i, e := range opened {
				if !statesSampling(e.Sampling, roles[i]) {
					t.Errorf("client %d opened with sampling %+v, want what %+v states", i, e.Sampling, roles[i])
				}
			}
		})
	}
}

// TestSamplingOfMapsEveryThinkingLevel: every thinking level configuration
// accepts maps onto the model's level of the same name.
func TestSamplingOfMapsEveryThinkingLevel(t *testing.T) {
	for _, level := range config.ThinkingLevels {
		if got := samplingOf(config.Endpoint{Thinking: level}).Thinking; got == nil || got.String() != level {
			t.Errorf("samplingOf(thinking %q).Thinking = %v, want %s", level, got, level)
		}
	}
}
