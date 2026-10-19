// Package mcp serves a table of tools over the Model Context Protocol:
// newline-delimited JSON-RPC 2.0, one request answered at a time, in order.
// The table is the caller's; this package knows no tool by name.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
	"sync"

	"kbase/internal/log"
)

// ProtocolVersion is the MCP specification revision the server implements.
const ProtocolVersion = "2025-11-25"

// Tool is one callable tool. Call returns the tool's result text, the same
// result as structured content, and the exit code the result stands for; an
// error is a fault of the tool itself, not a result.
type Tool struct {
	Name        string
	Description string
	InputSchema Schema
	ReadOnly    bool
	Call        func(args map[string]any) (text string, structured map[string]any, exitCode int, err error)
}

// Schema is a tool's input: an object of Properties, the Required ones
// present, no other key allowed.
type Schema struct {
	Properties map[string]Property
	Required   []string
}

// Property is one input key: Type is its JSON Schema type, "" admitting any
// value; Items is the element schema where Type is "array".
type Property struct {
	Type        string    `json:"type,omitempty"`
	Items       *Property `json:"items,omitempty"`
	Description string    `json:"description,omitempty"`
}

// Resource is one resource the server offers.
type Resource struct {
	URI      string `json:"uri"`
	Name     string `json:"name"`
	MimeType string `json:"mimeType"`
}

// Resources is what the server offers as resources: List is every one
// present now, Read the text of one List returned.
type Resources interface {
	List() ([]Resource, error)
	Read(Resource) (string, error)
}

// Subscription is the one resource a client may subscribe to: its URI, and
// Watch, which follows that resource for the connection and calls updated once
// for each change a client should hear about. Watch runs until ctx ends; the
// URI is the only one a subscription is accepted for, so every other listed
// resource is refused as not subscribable.
type Subscription struct {
	URI   string
	Watch func(ctx context.Context, updated func())
}

func (s Schema) MarshalJSON() ([]byte, error) {
	properties := s.Properties
	if properties == nil {
		properties = map[string]Property{}
	}
	required := s.Required
	if required == nil {
		required = []string{}
	}
	return json.Marshal(struct {
		Type                 string              `json:"type"`
		Properties           map[string]Property `json:"properties"`
		Required             []string            `json:"required"`
		AdditionalProperties bool                `json:"additionalProperties"`
	}{"object", properties, required, false})
}

func (p Property) admits(v any) bool {
	switch p.Type {
	case "":
		return true
	case "string":
		_, ok := v.(string)
		return ok
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "number":
		_, ok := v.(float64)
		return ok
	case "integer":
		f, ok := v.(float64)
		return ok && f == math.Trunc(f)
	case "array":
		items, ok := v.([]any)
		if !ok {
			return false
		}
		return p.Items == nil || !slices.ContainsFunc(items, func(item any) bool { return !p.Items.admits(item) })
	}
	return false
}

// check is every way args fail s, in key order; "" where none.
func (s Schema) check(args map[string]any) string {
	var faults []string
	for _, key := range s.Required {
		if _, ok := args[key]; !ok {
			faults = append(faults, fmt.Sprintf("argument %q is required", key))
		}
	}
	keys := make([]string, 0, len(args))
	for key := range args {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		p, ok := s.Properties[key]
		switch {
		case !ok:
			allowed := make([]string, 0, len(s.Properties))
			for name := range s.Properties {
				allowed = append(allowed, name)
			}
			slices.Sort(allowed)
			faults = append(faults, fmt.Sprintf("argument %q is not one this tool takes (it takes %s)", key, strings.Join(allowed, ", ")))
		case !p.admits(args[key]):
			faults = append(faults, fmt.Sprintf("argument %q must be of type %s", key, p.Type))
		}
	}
	return strings.Join(faults, "; ")
}

// JSON-RPC error codes.
const (
	codeParse          = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternal       = -32603
	// codeResourceNotFound is MCP's own code for a resources/read naming no
	// resource.
	codeResourceNotFound = -32002
)

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

var nullID = json.RawMessage("null")

// Serve answers the requests on in with frames on out until in ends. version
// is the server's own version, reported at initialize; instructions is what
// initialize tells the session the server serves, absent where it is "";
// subscription is the resource a client may subscribe to, absent where the
// server holds none.
func Serve(in io.Reader, out io.Writer, version, instructions string, tools []Tool, resources Resources, subscription *Subscription, lg log.Logger) error {
	s := server{version: version, instructions: instructions, tools: map[string]Tool{}, resources: resources,
		subscription: subscription, lg: lg}
	for _, t := range tools {
		s.tools[t.Name] = t
		s.order = append(s.order, t.Name)
	}
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	s.enc = enc
	defer s.stopWatch()
	r := bufio.NewReader(in)
	for {
		line, err := r.ReadBytes('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("reading a request: %w", err)
		}
		if frame := bytes.TrimRight(line, "\r\n"); len(frame) > 0 {
			if resp := s.handle(frame); resp != nil {
				if werr := s.write(resp); werr != nil {
					return fmt.Errorf("writing a response: %w", werr)
				}
			}
			if after := s.pending; after != nil {
				s.pending = nil
				after()
			}
		}
		if err != nil {
			return nil
		}
	}
}

// write is one frame on out. The watcher and the request loop both write, so
// every frame goes through here and the mutex is what keeps them whole.
func (s *server) write(frame any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enc.Encode(frame)
}

// notify is the watcher's frame: the resource a client holds a subscription to
// has changed, and the client reads it again. It carries the URI alone, so it
// cannot carry a stale document.
func (s *server) notify() {
	params := updatedParams{URI: s.subscription.URI}
	if err := s.write(notification{JSONRPC: "2.0", Method: "notifications/resources/updated", Params: params}); err != nil {
		s.lg.Error("mcp: writing a notification failed", "uri", params.URI, "err", err)
	}
}

// startWatch runs the subscription's watcher for the connection. The watcher
// outlives the request that started it and is stopped by the unsubscribe or by
// Serve's return.
func (s *server) startWatch() {
	if s.watch != nil || s.subscription == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	w := &watch{subscription: s.subscription, cancel: cancel, done: make(chan struct{})}
	s.watch = w
	go func() {
		defer close(w.done)
		w.subscription.Watch(ctx, s.notify)
	}()
}

// stopWatch ends the watcher and waits for it, so no frame is written after the
// caller stops listening.
func (s *server) stopWatch() {
	w := s.watch
	if w == nil {
		return
	}
	s.watch = nil
	w.cancel()
	<-w.done
}

// watch is the running subscription watcher; the request loop alone reads and
// writes the server's field holding it, so it needs no lock of its own.
type watch struct {
	subscription *Subscription
	cancel       context.CancelFunc
	done         chan struct{}
}

type server struct {
	version      string
	instructions string
	tools        map[string]Tool
	order        []string
	resources    Resources
	subscription *Subscription
	lg           log.Logger
	enc          *json.Encoder
	watch        *watch
	// pending is what a request asked to happen once its response is written:
	// a subscription started, or stopped, so a frame the watcher raises at once
	// follows the answer the client is waiting for.
	pending func()
	// mu serializes the frames written on out: the request loop's responses
	// and the watcher's notifications alike.
	mu sync.Mutex
}

func failure(id json.RawMessage, code int, message string) *response {
	return &response{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: message}}
}

// handle is the response to one frame, nil where none is owed.
func (s *server) handle(frame []byte) *response {
	var raw any
	if err := json.Unmarshal(frame, &raw); err != nil {
		s.lg.Debug("mcp: unparseable frame", "err", err)
		return failure(nullID, codeParse, "the line is not JSON")
	}
	members, ok := raw.(map[string]any)
	if !ok {
		return failure(nullID, codeInvalidRequest, "a request is one JSON object; batches are not accepted")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(frame, &fields); err != nil {
		return failure(nullID, codeParse, "the line is not JSON")
	}
	id, hasID := fields["id"]
	if hasID {
		switch members["id"].(type) {
		case string, float64, nil:
		default:
			return failure(nullID, codeInvalidRequest, "id must be a string, a number or null")
		}
	} else {
		id = nullID
	}
	if members["jsonrpc"] != "2.0" {
		return s.unlessNotification(hasID, failure(id, codeInvalidRequest, `jsonrpc must be "2.0"`))
	}
	method, ok := members["method"].(string)
	if !ok {
		return s.unlessNotification(hasID, failure(id, codeInvalidRequest, "method must be a string"))
	}
	if !hasID {
		s.lg.Debug("mcp: notification", "method", method)
		return nil
	}
	s.lg.Debug("mcp: request", "method", method)
	switch method {
	case "initialize":
		return &response{JSONRPC: "2.0", ID: id, Result: initializeResult{
			ProtocolVersion: ProtocolVersion,
			Capabilities: map[string]any{
				"tools":     map[string]bool{"listChanged": false},
				"resources": map[string]bool{"subscribe": s.subscription != nil, "listChanged": false},
			},
			ServerInfo:   serverInfo{Name: "kbase", Version: s.version},
			Instructions: s.instructions,
		}}
	case "ping":
		return &response{JSONRPC: "2.0", ID: id, Result: map[string]any{}}
	case "tools/list":
		return &response{JSONRPC: "2.0", ID: id, Result: map[string]any{"tools": s.list()}}
	case "tools/call":
		return s.call(id, members["params"])
	case "resources/list":
		listed, err := s.resources.List()
		if err != nil {
			s.lg.Error("mcp: listing resources failed", "err", err)
			return failure(id, codeInternal, "listing resources: "+err.Error())
		}
		return &response{JSONRPC: "2.0", ID: id, Result: map[string]any{"resources": append([]Resource{}, listed...)}}
	case "resources/read":
		return s.read(id, members["params"])
	case "resources/subscribe":
		return s.subscriptionRequest(id, members["params"], true)
	case "resources/unsubscribe":
		return s.subscriptionRequest(id, members["params"], false)
	}
	return failure(id, codeMethodNotFound, fmt.Sprintf("method %q is not one this server answers", method))
}

// unlessNotification is resp where the frame carried an id; a frame without
// one is a notification, owed no response even when malformed.
func (s *server) unlessNotification(hasID bool, resp *response) *response {
	if !hasID {
		s.lg.Debug("mcp: malformed notification dropped", "detail", resp.Error.Message)
		return nil
	}
	return resp
}

type listedTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema Schema          `json:"inputSchema"`
	Annotations map[string]bool `json:"annotations"`
}

func (s *server) list() []listedTool {
	listed := make([]listedTool, 0, len(s.order))
	for _, name := range s.order {
		t := s.tools[name]
		listed = append(listed, listedTool{Name: t.Name, Description: t.Description, InputSchema: t.InputSchema,
			Annotations: map[string]bool{"readOnlyHint": t.ReadOnly}})
	}
	return listed
}

func (s *server) call(id json.RawMessage, params any) *response {
	p, ok := params.(map[string]any)
	if !ok {
		return failure(id, codeInvalidParams, "tools/call takes an object of params")
	}
	name, ok := p["name"].(string)
	if !ok {
		return failure(id, codeInvalidParams, "params.name must be a tool's name")
	}
	tool, ok := s.tools[name]
	if !ok {
		return failure(id, codeInvalidParams, fmt.Sprintf("no tool is named %q", name))
	}
	args := map[string]any{}
	if a, present := p["arguments"]; present {
		if args, ok = a.(map[string]any); !ok {
			return failure(id, codeInvalidParams, "params.arguments must be an object")
		}
	}
	if fault := tool.InputSchema.check(args); fault != "" {
		return failure(id, codeInvalidParams, name+": "+fault)
	}
	text, structured, exitCode, err := tool.Call(args)
	if err != nil {
		s.lg.Error("mcp: tool failed", "tool", name, "err", err)
		return failure(id, codeInternal, name+": "+err.Error())
	}
	return &response{JSONRPC: "2.0", ID: id, Result: callResult{
		Content:           []textContent{{Type: "text", Text: text}},
		StructuredContent: structured,
		IsError:           exitCode != 0,
	}}
}

func (s *server) read(id json.RawMessage, params any) *response {
	p, _ := params.(map[string]any)
	uri, ok := p["uri"].(string)
	if !ok {
		return failure(id, codeInvalidParams, "params.uri must be a resource's URI")
	}
	listed, err := s.resources.List()
	if err != nil {
		s.lg.Error("mcp: listing resources failed", "err", err)
		return failure(id, codeInternal, "listing resources: "+err.Error())
	}
	i := slices.IndexFunc(listed, func(r Resource) bool { return r.URI == uri })
	if i < 0 {
		return failure(id, codeResourceNotFound, fmt.Sprintf("Resource not found: no resource has the URI %q", uri))
	}
	text, err := s.resources.Read(listed[i])
	if err != nil {
		s.lg.Error("mcp: reading a resource failed", "uri", uri, "err", err)
		return failure(id, codeInternal, "reading "+uri+": "+err.Error())
	}
	return &response{JSONRPC: "2.0", ID: id, Result: map[string]any{"contents": []resourceText{{listed[i].URI, listed[i].MimeType, text}}}}
}

type resourceText struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
}

// subscriptionRequest takes or drops the connection's subscription to the one
// resource the server lets a client subscribe to. A URI it lists but does not
// let a client subscribe to is invalid params, and a URI it does not list is
// resource not found; taking a subscription already held and dropping one not
// held change nothing.
func (s *server) subscriptionRequest(id json.RawMessage, params any, hold bool) *response {
	p, _ := params.(map[string]any)
	uri, ok := p["uri"].(string)
	if !ok {
		return failure(id, codeInvalidParams, "params.uri must be a resource's URI")
	}
	if s.subscription == nil || uri != s.subscription.URI {
		listed, err := s.resources.List()
		if err != nil {
			s.lg.Error("mcp: listing resources failed", "err", err)
			return failure(id, codeInternal, "listing resources: "+err.Error())
		}
		if !slices.ContainsFunc(listed, func(r Resource) bool { return r.URI == uri }) {
			return failure(id, codeResourceNotFound, fmt.Sprintf("Resource not found: no resource has the URI %q", uri))
		}
		return failure(id, codeInvalidParams, fmt.Sprintf("%q is not a resource a client may subscribe to", uri))
	}
	held := s.watch != nil
	switch {
	case hold && !held:
		s.pending = s.startWatch
	case !hold && held:
		s.pending = s.stopWatch
	}
	return &response{JSONRPC: "2.0", ID: id, Result: map[string]any{}}
}

// notification is a frame the server sends a client that owed it no request.
type notification struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  updatedParams `json:"params"`
}

// updatedParams is a resources/updated notification's params: the URI to read
// again, and nothing else. The document is read, never carried, so the frame
// cannot go stale.
type updatedParams struct {
	URI string `json:"uri"`
}

type initializeResult struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ServerInfo      serverInfo     `json:"serverInfo"`
	Instructions    string         `json:"instructions,omitempty"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type callResult struct {
	Content           []textContent  `json:"content"`
	StructuredContent map[string]any `json:"structuredContent"`
	IsError           bool           `json:"isError"`
}
