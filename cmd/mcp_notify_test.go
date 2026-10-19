package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"kbase/internal/build"
)

// The build-end probe's paths, which the recipe supplies: it starts the server
// over two named pipes and stages the repository and state store the builds
// run in and write. The exec monopoly (I1) leaves starting the server to
// cmd/mcp.go inside the module and to a recipe outside it, so this leg starts
// no process of its own.
var (
	notifyRepo   = flag.String("notify.repo", "", "the repository the probe's builds run in")
	notifyState  = flag.String("notify.state-dir", "", "the state store the probe's builds write")
	notifySend   = flag.String("notify.requests", "", "the pipe the server reads requests from")
	notifyFrames = flag.String("notify.frames", "", "the pipe the server writes frames to")
)

// TestBuildWatchAnnouncesEachRunOnce: the watcher announces a run's end once,
// keyed on the run's identity so a resumed build is a new run. A run already
// over at the subscription's first reading is announced there, but only for a
// run this server launched, or for any run where it launched none: until the
// child takes the store's lock the run before it reads interrupted, which is
// no new build's end — and which that guard consumes, so no later reading of it
// announces either. The states are internal/build's vocabulary.
func TestBuildWatchAnnouncesEachRunOnce(t *testing.T) {
	running := build.State{State: "running", PID: 7, Started: "10:00"}
	finished := build.State{State: "finished", PID: 7, Started: "10:00"}
	resumed := build.State{State: "finished", PID: 9, Started: "10:30"}
	interrupted := build.State{State: "interrupted", PID: 5, Started: "09:00"}
	none := build.State{State: "none"}
	for _, tc := range []struct {
		name     string
		launched int
		reads    []build.State
		want     []bool
	}{
		{"a run ending while the subscription is held", 7,
			[]build.State{running, running, finished, finished},
			[]bool{false, false, true, false}},
		{"a resumed build is a new run, announced once", 7,
			[]build.State{finished, running, resumed, resumed},
			[]bool{true, false, true, false}},
		{"a store no build wrote", 7,
			[]build.State{none, none},
			[]bool{false, false}},
		{"a run over before the subscription, this server's", 7,
			[]build.State{finished},
			[]bool{true}},
		{"a run over before the subscription, one this server did not launch", 7,
			[]build.State{interrupted, running, finished},
			[]bool{false, false, true}},
		{"a run this server did not launch, read twice, stays silent", 7,
			[]build.State{interrupted, interrupted},
			[]bool{false, false}},
		{"a run over before the subscription, this server launched none", 0,
			[]build.State{interrupted},
			[]bool{true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := &buildWatch{atStart: true}
			for i, state := range tc.reads {
				if got := w.announce(state, tc.launched); got != tc.want[i] {
					t.Errorf("reading %d (%s, pid %d) announced %v, want %v", i+1, state.State, state.PID, got, tc.want[i])
				}
			}
		})
	}
}

// probeFrame is one frame off the wire: a response by its id, or a notification
// by its method.
type probeFrame struct {
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

// buildProbe is the server as a client sees it: requests out of one pipe,
// frames in from another, read by a goroutine so a frame no request owes can be
// waited for.
type buildProbe struct {
	requests io.WriteCloser
	frames   chan probeFrame
	id       int
	// notifications is every notification a request's response passed on the
	// way; a run's end announced twice shows up here.
	notifications []probeFrame
}

func newBuildProbe(t *testing.T, requests io.WriteCloser, frames io.ReadCloser) *buildProbe {
	t.Helper()
	p := &buildProbe{requests: requests, frames: make(chan probeFrame)}
	go func() {
		defer close(p.frames)
		r := bufio.NewReader(frames)
		for {
			line, err := r.ReadBytes('\n')
			var f probeFrame
			if len(line) > 0 && json.Unmarshal(line, &f) == nil {
				p.frames <- f
			}
			if err != nil {
				return
			}
		}
	}()
	return p
}

// send writes one request and returns its id.
func (p *buildProbe) send(t *testing.T, method string, params any) int {
	t.Helper()
	p.id++
	request := map[string]any{"jsonrpc": "2.0", "id": p.id, "method": method}
	if params != nil {
		request["params"] = params
	}
	line, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.requests.Write(append(line, '\n')); err != nil {
		t.Fatal(err)
	}
	return p.id
}

// next is the next frame the server writes.
func (p *buildProbe) next(t *testing.T) probeFrame {
	t.Helper()
	select {
	case f, ok := <-p.frames:
		if !ok {
			t.Fatal("the server stopped writing frames")
		}
		return f
	case <-time.After(time.Minute):
		t.Fatal("no frame arrived within a minute")
	}
	return probeFrame{}
}

// answer is one request's response. A notification written before it is not a
// failure — a build can end before the call that started it returns — but it is
// counted, so an announced run is announced once.
func (p *buildProbe) answer(t *testing.T, method string, params any) probeFrame {
	t.Helper()
	id := p.send(t, method, params)
	for {
		f := p.next(t)
		if f.Method == "" {
			if string(f.ID) != strconv.Itoa(id) {
				t.Fatalf("%s: the response to %d was not the next response (%+v)", method, id, f)
			}
			return f
		}
		p.notifications = append(p.notifications, f)
	}
}

// awaited waits for the notification a run's end raises — the next one, in
// order — and requires it to carry the build resource and nothing else.
func (p *buildProbe) awaited(t *testing.T, want int) {
	t.Helper()
	for len(p.notifications) < want {
		f := p.next(t)
		if f.Method == "" {
			t.Fatalf("a response (%+v) arrived while waiting for notification %d", f, want)
		}
		p.notifications = append(p.notifications, f)
	}
	if len(p.notifications) != want {
		t.Fatalf("%d notifications, want %d", len(p.notifications), want)
	}
	if f := p.notifications[want-1]; f.Method != "notifications/resources/updated" || f.Params.URI != buildURI || f.ID != nil {
		t.Fatalf("notification %d is %+v, want resources/updated for %s", want, f, buildURI)
	}
}

// read is the build resource's document.
func (p *buildProbe) read(t *testing.T) string {
	t.Helper()
	var read struct {
		Contents []struct {
			URI, MimeType, Text string
		} `json:"contents"`
	}
	resp := p.answer(t, "resources/read", map[string]any{"uri": buildURI})
	if resp.Error != nil || json.Unmarshal(resp.Result, &read) != nil || len(read.Contents) != 1 ||
		read.Contents[0].URI != buildURI || read.Contents[0].MimeType != "application/yaml" {
		t.Fatalf("reading %s answered %+v", buildURI, resp)
	}
	return read.Contents[0].Text
}

// TestMCPBuildEndNotification: a build launched through the tool ends with an
// event the session can act on and no request owed. The test starts a build
// with no inference, subscribes to the build resource, sends nothing and reads
// frames until the notification arrives — one, for that URI — then reads a
// finished document; a second build under the same subscription announces once
// more; and a subscription dropped and taken again announces the run already
// over, because it is the one this server launched.
func TestMCPBuildEndNotification(t *testing.T) {
	if *notifyFrames == "" {
		t.Skip("the recipe starts the server and names its pipes (-notify.*): just test-integration-mcp-build")
	}
	buildArgs := func() map[string]any {
		return map[string]any{"volume-root": []any{*notifyRepo + "/paper.tex"}, "no-inference": true, "state-dir": *notifyState}
	}
	// The write end first: the server holds the pipe's read end open before the
	// frame pipe is anyone's to read.
	requests, err := os.OpenFile(*notifySend, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	frames, err := os.OpenFile(*notifyFrames, os.O_RDONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	p := newBuildProbe(t, requests, frames)

	p.answer(t, "initialize", map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{},
		"clientInfo": map[string]any{"name": "build-end probe", "version": "1"}})
	// call is one tools/call's result: the structured document, and whether the
	// result is an error.
	call := func(name string, args map[string]any) (map[string]any, bool) {
		t.Helper()
		var result struct {
			Structured map[string]any `json:"structuredContent"`
			IsError    bool           `json:"isError"`
		}
		resp := p.answer(t, "tools/call", map[string]any{"name": name, "arguments": args})
		if resp.Error != nil || json.Unmarshal(resp.Result, &result) != nil {
			t.Fatalf("%s answered %+v", name, resp)
		}
		return result.Structured, result.IsError
	}
	start, isError := call("build", buildArgs())
	if pid, _ := start["pid"].(float64); isError || start["outcome"] != "done" || pid <= 0 {
		t.Fatalf("the build started as %v", start)
	}
	p.answer(t, "resources/subscribe", map[string]any{"uri": buildURI})

	p.awaited(t, 1)
	if doc := p.read(t); !strings.Contains(doc, `state: "finished"`) {
		t.Errorf("the build's document after the notification:\n%s", doc)
	}
	if len(p.notifications) != 1 {
		t.Errorf("%d notifications after the read, want exactly one for the run", len(p.notifications))
	}

	// The same command again: a resumed run — a new pid, a new start — which
	// announces once more.
	if again, isError := call("build", buildArgs()); isError {
		t.Fatalf("the second build answered %v", again)
	}
	p.awaited(t, 2)
	if doc := p.read(t); !strings.Contains(doc, `state: "finished"`) {
		t.Errorf("the build's document after the second notification:\n%s", doc)
	}
	if len(p.notifications) != 2 {
		t.Errorf("%d notifications after the second read, want exactly two", len(p.notifications))
	}

	p.answer(t, "resources/unsubscribe", map[string]any{"uri": buildURI})
	p.answer(t, "resources/subscribe", map[string]any{"uri": buildURI})
	p.awaited(t, 3)
	p.read(t)
	if len(p.notifications) != 3 {
		t.Errorf("%d notifications, want the run already over announced once at the subscription", len(p.notifications))
	}
	p.answer(t, "ping", nil)

	// EOF on its requests ends the server; the closed pipe is the last word.
	requests.Close()
}
