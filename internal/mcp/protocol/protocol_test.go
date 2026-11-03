package protocol

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"testing"

	"kbase/internal/log"
)

const testURI = "kbase://one"

// testResources lists one resource and reads it empty.
type testResources struct{}

func (testResources) List() ([]Resource, error) {
	return []Resource{{URI: testURI, Name: "One", MimeType: "application/yaml"}}, nil
}

func (testResources) Read(Resource) (string, error) { return "", nil }

// wire drives a server over pipes.
type wire struct {
	requests io.WriteCloser
	frames   *bufio.Reader
	served   chan error
	id       int
}

// frame is what a test reads off the wire: a response by its id.
type frame struct {
	ID     json.RawMessage `json:"id"`
	Result json.RawMessage `json:"result"`
	Method string          `json:"method"`
	Error  *struct {
		Code    int
		Message string
	} `json:"error"`
}

// answer is one request's response, which the next frame must be.
func (w *wire) answer(t *testing.T, method string, params any) frame {
	t.Helper()
	w.id++
	request := map[string]any{"jsonrpc": "2.0", "id": w.id, "method": method}
	if params != nil {
		request["params"] = params
	}
	line, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.requests.Write(append(line, '\n')); err != nil {
		t.Fatal(err)
	}
	reply, err := w.frames.ReadBytes('\n')
	if err != nil {
		t.Fatalf("reading a frame: %v", err)
	}
	var f frame
	if err := json.Unmarshal(reply, &f); err != nil {
		t.Fatalf("frame %q: %v", reply, err)
	}
	if want, _ := json.Marshal(w.id); f.Method != "" || string(f.ID) != string(want) {
		t.Fatalf("%s: the next frame was %+v, want its response", method, f)
	}
	return f
}

// pipePair is one pipe, both ends closed when the test ends.
func pipePair(t *testing.T) (read, write *os.File) {
	t.Helper()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		read.Close()
		write.Close()
	})
	return read, write
}

// TestNoSubscription: the server declares no subscribe capability, and
// resources/subscribe and resources/unsubscribe are methods it does not
// answer, whatever URI they name.
func TestNoSubscription(t *testing.T) {
	serverIn, testOut := pipePair(t)
	testIn, serverOut := pipePair(t)
	w := &wire{requests: testOut, frames: bufio.NewReader(testIn), served: make(chan error, 1)}
	go func() {
		err := Serve(serverIn, serverOut, "1", "", nil, testResources{}, log.Discard())
		serverOut.Close()
		w.served <- err
	}()

	var initialized struct {
		Capabilities struct {
			Resources map[string]any `json:"resources"`
		} `json:"capabilities"`
	}
	resp := w.answer(t, "initialize", map[string]any{"protocolVersion": ProtocolVersion})
	if err := json.Unmarshal(resp.Result, &initialized); err != nil {
		t.Fatal(err)
	}
	if _, declared := initialized.Capabilities.Resources["subscribe"]; declared {
		t.Errorf("initialize %s, want no subscribe capability declared", resp.Result)
	}
	for _, method := range []string{"resources/subscribe", "resources/unsubscribe"} {
		f := w.answer(t, method, map[string]any{"uri": testURI})
		if f.Error == nil || f.Error.Code != codeMethodNotFound {
			t.Errorf("%s answered %+v, want error %d", method, f, codeMethodNotFound)
		}
	}
	w.requests.Close()
	if err := <-w.served; err != nil {
		t.Errorf("Serve = %v, want nil", err)
	}
}

// TestAnObjectPropertyAdmitsItsKeysAlone: an object property declaring its
// keys admits a mapping holding every required one, each of its type, and no
// other; one declaring none admits any mapping.
func TestAnObjectPropertyAdmitsItsKeysAlone(t *testing.T) {
	closed := false
	edge := Property{Type: "object", Required: []string{"source", "relation"}, AdditionalProperties: &closed,
		Properties: map[string]Property{"source": {Type: "string"}, "relation": {Type: "string"}}}
	s := Schema{Properties: map[string]Property{"edge": edge, "any": {Type: "object"}}}
	for _, c := range []struct {
		name  string
		value any
		ok    bool
	}{
		{"every key, each a string", map[string]any{"source": "a", "relation": "r"}, true},
		{"a required key missing", map[string]any{"source": "a"}, false},
		{"a key it does not declare", map[string]any{"source": "a", "relation": "r", "other": "x"}, false},
		{"a key of another type", map[string]any{"source": "a", "relation": 1.0}, false},
		{"no mapping", "a,r", false},
	} {
		if got := len(s.Faults(map[string]any{"edge": c.value})) == 0; got != c.ok {
			t.Errorf("%s: admitted %v, want %v", c.name, got, c.ok)
		}
	}
	if faults := s.Faults(map[string]any{"any": map[string]any{"x": 1.0}}); len(faults) != 0 {
		t.Errorf("an object declaring no keys refused a mapping: %v", faults)
	}
}
