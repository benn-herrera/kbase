package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"io"
	"os"
	"strconv"
	"testing"
	"time"
)

// The build probe's paths, which the recipe supplies: it starts the server
// over two named pipes and stages the repository and state store the builds
// run in and write. Only internal/mcp/launch.go starts a process inside the
// module, and a recipe starts the server outside it, so this leg starts no
// process of its own.
var (
	probeRepo   = flag.String("probe.repo", "", "the repository the probe's builds run in")
	probeState  = flag.String("probe.state-dir", "", "the state store the probe's builds write")
	probeSend   = flag.String("probe.requests", "", "the pipe the server reads requests from")
	probeFrames = flag.String("probe.frames", "", "the pipe the server writes frames to")
)

// statusPoll is how often the probe calls status while a build runs.
const statusPoll = time.Second

// probeFrame is one frame off the wire: a response by its id, or a frame no
// request owes, by its method.
type probeFrame struct {
	ID     json.RawMessage `json:"id"`
	Result json.RawMessage `json:"result"`
	Method string          `json:"method"`
	Error  *struct {
		Code    int
		Message string
	} `json:"error"`
}

// buildProbe is the server as a client sees it: requests out of one pipe,
// frames in from another.
type buildProbe struct {
	requests io.WriteCloser
	frames   *bufio.Reader
	id       int
}

// answer is one request's response, which must be the next frame: a server
// that announces nothing writes no frame a request does not owe.
func (p *buildProbe) answer(t *testing.T, method string, params any) probeFrame {
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
	reply, err := p.frames.ReadBytes('\n')
	if err != nil {
		t.Fatalf("%s: the server stopped writing frames: %v", method, err)
	}
	var f probeFrame
	if err := json.Unmarshal(reply, &f); err != nil {
		t.Fatalf("%s: frame %q: %v", method, reply, err)
	}
	if f.Method != "" || string(f.ID) != strconv.Itoa(p.id) {
		t.Fatalf("%s: the next frame was %s, want its response", method, reply)
	}
	return f
}

// call is one tools/call's structured result, and whether the result is an
// error.
func (p *buildProbe) call(t *testing.T, name string, args map[string]any) (map[string]any, bool) {
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

// probeMargin is how long before the test's own deadline the probe stops
// waiting, so its diagnostic lands before the runner's -timeout panics.
const probeMargin = 15 * time.Second

// probeDeadline is when the probe stops waiting on its builds: probeMargin
// before the test's deadline, the zero time where the runner sets none.
func probeDeadline(t *testing.T) time.Time {
	d, ok := t.Deadline()
	if !ok {
		return time.Time{}
	}
	return d.Add(-probeMargin)
}

// followed polls status until it reports a run that is not running and that
// ran stands for, and returns that result; past deadline, unless it is the
// zero time, the test fails naming the run still going.
func (p *buildProbe) followed(t *testing.T, deadline time.Time, ran func(pid float64) bool) map[string]any {
	t.Helper()
	for {
		status, isError := p.call(t, "status", map[string]any{"state-dir": *probeState})
		if isError {
			t.Fatalf("status answered %v", status)
		}
		if pid, _ := status["pid"].(float64); ran(pid) && status["state"] != "running" {
			return status
		}
		if !deadline.IsZero() && time.Now().After(deadline) {
			t.Fatalf("the build is still running: %v", status)
		}
		time.Sleep(statusPoll)
	}
}

// TestMCPBuildFollowedByStatus: a build launched through the tool is followed
// to its end by polling status, the server announcing nothing: initialize
// declares no subscription, and every frame is a response. The build ends
// finished, every stage done; a resume over its finished trail is a new run,
// followed the same way.
func TestMCPBuildFollowedByStatus(t *testing.T) {
	if *probeFrames == "" {
		t.Skip("the recipe starts the server and names its pipes (-probe.*): just test-integration-mcp-build")
	}
	// The write end first: the server holds the pipe's read end open before the
	// frame pipe is anyone's to read.
	requests, err := os.OpenFile(*probeSend, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	frames, err := os.OpenFile(*probeFrames, os.O_RDONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	p := &buildProbe{requests: requests, frames: bufio.NewReader(frames)}
	deadline := probeDeadline(t)

	var initialized struct {
		Capabilities struct {
			Resources map[string]any `json:"resources"`
		} `json:"capabilities"`
	}
	resp := p.answer(t, "initialize", map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{},
		"clientInfo": map[string]any{"name": "build probe", "version": "1"}})
	if err := json.Unmarshal(resp.Result, &initialized); err != nil {
		t.Fatal(err)
	}
	if _, declared := initialized.Capabilities.Resources["subscribe"]; declared {
		t.Errorf("initialize declares a subscribe capability: %s", resp.Result)
	}

	start, isError := p.call(t, "build", map[string]any{"volume-root": []any{*probeRepo + "/paper.tex"}, "no-inference": true, "state-dir": *probeState})
	pid, _ := start["pid"].(float64)
	if isError || start["outcome"] != "done" || pid <= 0 {
		t.Fatalf("the build started as %v", start)
	}
	end := p.followed(t, deadline, func(ran float64) bool { return ran == pid })
	stages, _ := end["stages"].(map[string]any)
	if notDone, _ := stages["not-done"].([]any); end["state"] != "finished" || len(notDone) != 0 {
		t.Errorf("status after the build: %v, want finished with every stage done", end)
	}

	// The resume walks no stage, so it may exit before the call returns,
	// answering with its own result; either way status follows it.
	if again, isError := p.call(t, "build", map[string]any{"resume": true, "no-inference": true, "state-dir": *probeState}); isError {
		t.Fatalf("the resume answered %v", again)
	}
	if end := p.followed(t, deadline, func(ran float64) bool { return ran > 0 && ran != pid }); end["state"] != "finished" {
		t.Errorf("status after the resume: %v, want a new run, finished", end)
	}
	p.answer(t, "ping", nil)

	// EOF on its requests ends the server; the closed pipe is the last word.
	requests.Close()
}
