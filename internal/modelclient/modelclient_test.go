package modelclient

import (
	"errors"
	"testing"

	"kbase/internal/config"
	"kbase/internal/model"
	"kbase/internal/model/modeltest"
	"kbase/internal/result"
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
// an environment endpoint keys by its base URL — and CloseAll releases every
// one it opened, once.
func TestRegistryOpensEachEndpointOnce(t *testing.T) {
	var log closeLog
	var opened int
	reg := New(func(model.Endpoint) model.Client {
		opened++
		return closer{modeltest.NewScriptedMock(nil, nil).Client(), &log}
	}, nil, "", 0)
	lab := config.Endpoint{Mode: config.ModeFile, Provider: "lab", BaseURL: "http://lab.example/v1"}
	first, err := reg.Get(lab)
	if err != nil {
		t.Fatal(err)
	}
	second, err := reg.Get(lab)
	if err != nil {
		t.Fatal(err)
	}
	if opened != 1 || first != second {
		t.Errorf("the same entry opened %d client(s); want one, shared", opened)
	}
	if _, err := reg.Get(config.Endpoint{Mode: config.ModeEnvironment, BaseURL: "http://env.example/v1"}); err != nil {
		t.Fatal(err)
	}
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
		if _, err := reg.Get(config.Endpoint{Provider: name, BaseURL: "http://" + name + ".example/v1"}); err != nil {
			t.Fatal(err)
		}
	}
	if len(got) != 2 || got[0] != 4 || got[1] != 4 {
		t.Errorf("IdleConns on the endpoints = %v, want [4 4]", got)
	}
}

// TestRegistryPreflightReadsTheKeyFile: the key file is the registry's read,
// and the refusal it cannot read names the variable and the path. Nothing is
// opened.
func TestRegistryPreflightReadsTheKeyFile(t *testing.T) {
	var opened int
	reg := New(func(model.Endpoint) model.Client {
		opened++
		return modeltest.NewScriptedMock(nil, nil).Client()
	}, nil, "", 0)
	_, err := reg.Get(config.Endpoint{Provider: "lab", BaseURL: "http://lab.example/v1", APIKeyFile: "/absent/key.txt"})
	var refusal result.Refusal
	if !errors.As(err, &refusal) || len(refusal) != 1 || refusal[0].Key != config.EnvAPIKeyFile || refusal[0].Path != "/absent/key.txt" ||
		refusal[0].Check != config.CheckProvider {
		t.Fatalf("err = %v, want the key file's refusal", err)
	}
	if opened != 0 {
		t.Error("a refused key file opened a client")
	}
	if err := reg.Preflight(config.Endpoint{Provider: "lab", APIKeyFile: "/absent/key.txt"}); err == nil {
		t.Error("the preflight read no key file")
	}
	if opened != 0 {
		t.Error("the preflight opened a client")
	}
}
