package protocol

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"kbase/internal/log"
)

const (
	testURI     = "kbase://one"
	otherURI    = "kbase://two"
	unlistedURI = "kbase://absent"
)

// testResources lists two resources and reads neither.
type testResources struct{}

func (testResources) List() ([]Resource, error) {
	return []Resource{{URI: testURI, Name: "One", MimeType: "application/yaml"},
		{URI: otherURI, Name: "Two", MimeType: "text/markdown"}}, nil
}

func (testResources) Read(Resource) (string, error) { return "", nil }

// stubWatch is the subscription a test drives: Watch reports itself to the
// test, which then raises the changes the server sends, and ends when its
// context does.
type stubWatch struct{ started chan *watchCall }

type watchCall struct {
	announce func()
	ended    chan struct{}
}

func (s stubWatch) watch(ctx context.Context, announce func()) {
	c := &watchCall{announce: announce, ended: make(chan struct{})}
	s.started <- c
	<-ctx.Done()
	close(c.ended)
}

// wire drives a server over pipes, so a test can send a request and then wait
// for a frame no request owes.
type wire struct {
	requests io.WriteCloser
	frames   *bufio.Reader
	served   chan error
	id       int
}

// frame is what a test reads off the wire: a response by its id, or a
// notification by its method.
type frame struct {
	ID     json.RawMessage `json:"id"`
	Result json.RawMessage `json:"result"`
	Method string          `json:"method"`
	Params struct {
		URI string `json:"uri"`
	} `json:"params"`
	Error *struct {
		Code    int
		Message string
	} `json:"error"`
}

// send writes one request and returns its id.
func (w *wire) send(t *testing.T, method string, params any) int {
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
	return w.id
}

// read is the next frame on the wire.
func (w *wire) read(t *testing.T) frame {
	t.Helper()
	line, err := w.frames.ReadBytes('\n')
	if err != nil {
		t.Fatalf("reading a frame: %v", err)
	}
	var f frame
	if err := json.Unmarshal(line, &f); err != nil {
		t.Fatalf("frame %q: %v", line, err)
	}
	return f
}

// answer is one request's response, which the next frame must be: a frame that
// is not it — an extra notification, say — is a failure.
func (w *wire) answer(t *testing.T, method string, params any) frame {
	t.Helper()
	id := w.send(t, method, params)
	f := w.read(t)
	if f.Method != "" || string(f.ID) != jsonText(t, id) {
		t.Fatalf("%s: the next frame was %+v, want its response", method, f)
	}
	return f
}

func jsonText(t *testing.T, id int) string {
	t.Helper()
	b, err := json.Marshal(id)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// startWire serves testURI over pipes, and returns the wire and the
// subscription's stub.
func startWire(t *testing.T) (*wire, stubWatch) {
	t.Helper()
	serverIn, testOut := pipePair(t)
	testIn, serverOut := pipePair(t)
	sub := stubWatch{started: make(chan *watchCall, 4)}
	w := &wire{requests: testOut, frames: bufio.NewReader(testIn), served: make(chan error, 1)}
	go func() {
		err := Serve(serverIn, serverOut, "1", "", nil, testResources{}, &Subscription{URI: testURI, Watch: sub.watch}, log.Discard())
		serverOut.Close()
		w.served <- err
	}()
	return w, sub
}

// pipePair is one pipe, both ends closed when the test ends. An operating
// system pipe rather than io.Pipe, so the frame a watcher writes does not wait
// for the test to read it — a client reading its output is no different.
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

// watchReceived is the watcher a subscription started, which the server runs
// in a goroutine of its own.
func watchReceived(t *testing.T, sub stubWatch) *watchCall {
	t.Helper()
	select {
	case c := <-sub.started:
		return c
	case <-time.After(10 * time.Second):
		t.Fatal("the subscription was taken and no watcher started")
		return nil
	}
}

// endedWithin waits for the watcher to stop, within a bound only a failure
// reaches.
func (c *watchCall) endedWithin(t *testing.T) {
	t.Helper()
	select {
	case <-c.ended:
	case <-time.After(10 * time.Second):
		t.Fatal("the watcher is still running after the subscription dropped")
	}
}

// TestSubscriptions: the server holds a subscription to the one resource it
// lets a client subscribe to, and answers the rest with the code that says
// why. What the watcher raises follows the response that took the
// subscription, so the client holds it before the frame arrives; the frames
// carry the URI alone.
func TestSubscriptions(t *testing.T) {
	w, sub := startWire(t)

	var initialized struct {
		Capabilities struct {
			Resources struct {
				Subscribe bool `json:"subscribe"`
			} `json:"resources"`
		} `json:"capabilities"`
	}
	resp := w.answer(t, "initialize", map[string]any{"protocolVersion": ProtocolVersion})
	if err := json.Unmarshal(resp.Result, &initialized); err != nil {
		t.Fatal(err)
	}
	if !initialized.Capabilities.Resources.Subscribe {
		t.Fatal("initialize does not advertise subscription")
	}

	if f := w.answer(t, "resources/subscribe", map[string]any{"uri": testURI}); string(f.Result) != "{}" {
		t.Errorf("a subscription answered %s, want {}", f.Result)
	}
	first := watchReceived(t, sub)
	first.announce()
	f := w.read(t)
	if f.Method != "notifications/resources/updated" || f.Params.URI != testURI || f.ID != nil {
		t.Errorf("the watcher raised %+v, want a resources/updated notification for %s", f, testURI)
	}

	// A second subscribe to the subscription already held changes nothing: the
	// watcher that was running still carries the frames.
	if f := w.answer(t, "resources/subscribe", map[string]any{"uri": testURI}); string(f.Result) != "{}" {
		t.Errorf("a second subscription answered %s, want {}", f.Result)
	}
	first.announce()
	if f := w.read(t); f.Method != "notifications/resources/updated" || f.Params.URI != testURI {
		t.Errorf("the second subscribe raised %+v, want the held watcher's notification", f)
	}

	// Unsubscribing stops it, and a subscription dropped again changes nothing.
	if f := w.answer(t, "resources/unsubscribe", map[string]any{"uri": testURI}); string(f.Result) != "{}" {
		t.Errorf("an unsubscribe answered %s, want {}", f.Result)
	}
	first.endedWithin(t)
	if f := w.answer(t, "resources/unsubscribe", map[string]any{"uri": testURI}); string(f.Result) != "{}" {
		t.Errorf("an unsubscribe of a subscription not held answered %s, want {}", f.Result)
	}
	w.answer(t, "ping", nil)

	// The subscription is the connection's to take again.
	w.answer(t, "resources/subscribe", map[string]any{"uri": testURI})
	second := watchReceived(t, sub)

	for _, tc := range []struct {
		name   string
		params map[string]any
		code   int
	}{
		{"a listed resource that is not subscribable", map[string]any{"uri": otherURI}, codeInvalidParams},
		{"a URI no resource has", map[string]any{"uri": unlistedURI}, codeResourceNotFound},
		{"no uri", map[string]any{}, codeInvalidParams},
	} {
		f := w.answer(t, "resources/subscribe", tc.params)
		if f.Error == nil || f.Error.Code != tc.code {
			t.Errorf("%s answered %+v, want error %d", tc.name, f, tc.code)
		}
	}

	// EOF ends the server, and the watcher with it.
	w.requests.Close()
	if err := <-w.served; err != nil {
		t.Errorf("Serve = %v, want nil", err)
	}
	second.endedWithin(t)
}

// TestNoSubscription: a server given none advertises none, and refuses every
// URI, listed or not.
func TestNoSubscription(t *testing.T) {
	serverIn, testOut := pipePair(t)
	testIn, serverOut := pipePair(t)
	w := &wire{requests: testOut, frames: bufio.NewReader(testIn), served: make(chan error, 1)}
	go func() {
		err := Serve(serverIn, serverOut, "1", "", nil, testResources{}, nil, log.Discard())
		serverOut.Close()
		w.served <- err
	}()

	resp := w.answer(t, "initialize", map[string]any{"protocolVersion": ProtocolVersion})
	if !strings.Contains(string(resp.Result), `"subscribe":false`) {
		t.Errorf("initialize %s, want no subscription advertised", resp.Result)
	}
	for _, tc := range []struct {
		method string
		uri    string
		code   int
	}{
		{"resources/subscribe", testURI, codeInvalidParams},
		{"resources/subscribe", unlistedURI, codeResourceNotFound},
		{"resources/unsubscribe", testURI, codeInvalidParams},
	} {
		f := w.answer(t, tc.method, map[string]any{"uri": tc.uri})
		if f.Error == nil || f.Error.Code != tc.code {
			t.Errorf("%s %s answered %+v, want error %d", tc.method, tc.uri, f, tc.code)
		}
	}
	w.requests.Close()
	if err := <-w.served; err != nil {
		t.Errorf("Serve = %v, want nil", err)
	}
}
