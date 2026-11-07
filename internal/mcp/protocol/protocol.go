// Package protocol serves a table of tools over the Model Context Protocol:
// newline-delimited JSON-RPC 2.0, one request answered at a time, in order.
// The table is the caller's; this package knows no tool by name.
package protocol

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"slices"
	"strings"

	"kbase/internal/log"
)

// ProtocolVersion is the MCP specification revision the server implements.
const ProtocolVersion = "2025-11-25"

// Tool is one callable tool. Call returns the tool's result text, the same
// result as structured content, and whether the result is an error; an
// error is a fault of the tool itself, not a result.
type Tool struct {
	Name        string
	Description string
	InputSchema Schema
	ReadOnly    bool
	Call        func(args map[string]any) (text string, structured map[string]any, isError bool, err error)
}

// Schema is a tool's input: an object of Properties, the Required ones
// present, no other key allowed.
type Schema struct {
	Properties map[string]Property
	Required   []string
}

// Property is one input key: Type is its JSON Schema type, "" admitting any
// value; Items is the element schema where Type is "array"; Enum, where set,
// is every value a "string" property admits. An "object" property declaring
// Properties admits a mapping of those keys alone, each admitted by its own
// property and the Required ones present; AdditionalProperties false states
// that closure in the schema the client reads.
type Property struct {
	Type                 string              `json:"type,omitempty"`
	Items                *Property           `json:"items,omitempty"`
	Enum                 []string            `json:"enum,omitempty"`
	Properties           map[string]Property `json:"properties,omitempty"`
	Required             []string            `json:"required,omitempty"`
	AdditionalProperties *bool               `json:"additionalProperties,omitempty"`
	Description          string              `json:"description,omitempty"`
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
		s, ok := v.(string)
		return ok && (p.Enum == nil || slices.Contains(p.Enum, s))
	case "object":
		m, ok := v.(map[string]any)
		if !ok || p.Properties == nil {
			return ok
		}
		return p.objectFaults("", m) == nil
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

// Fault is one argument failing a schema, Key its dotted path where it lies
// inside an object argument (edge.relation): Missing where the object
// requires it and lacks it, Unknown where the object has no property of its
// name — Want then being that object — and otherwise a value Want does not
// admit. Allowed is every key of the object Key lies in, sorted; nil where it
// declares none.
type Fault struct {
	Key              string
	Missing, Unknown bool
	Want             Property
	Allowed          []string
}

// Detail is f in words, naming its argument by its path.
func (f Fault) Detail() string {
	switch {
	case f.Missing:
		return fmt.Sprintf("argument %q is required", f.Key)
	case f.Unknown:
		allowed := "none"
		if len(f.Allowed) > 0 {
			allowed = strings.Join(f.Allowed, ", ")
		}
		return fmt.Sprintf("argument %q is unknown (allowed: %s)", f.Key, allowed)
	case f.Want.Enum != nil:
		return fmt.Sprintf("argument %q must be one of %s", f.Key, strings.Join(f.Want.Enum, ", "))
	}
	return fmt.Sprintf("argument %q must be of type %s", f.Key, f.Want.Type)
}

// Faults is every way args fail s: each required key missing, in s's order,
// then each argument s has no property for or does not admit, in key order;
// an object argument declaring properties is descended into, its faults
// reported the same way at their dotted paths.
func (s Schema) Faults(args map[string]any) []Fault {
	return Property{Type: "object", Properties: s.Properties, Required: s.Required}.objectFaults("", args)
}

// objectFaults is Faults over m, an object p declares the properties of,
// each key's path prefixed by path.
func (p Property) objectFaults(path string, m map[string]any) []Fault {
	at := func(key string) string {
		if path == "" {
			return key
		}
		return path + "." + key
	}
	allowed := slices.Sorted(maps.Keys(p.Properties))
	var faults []Fault
	for _, key := range p.Required {
		if _, ok := m[key]; !ok {
			faults = append(faults, Fault{Key: at(key), Missing: true, Want: p, Allowed: allowed})
		}
	}
	for _, key := range slices.Sorted(maps.Keys(m)) {
		inner, ok := p.Properties[key]
		object, isMap := m[key].(map[string]any)
		switch {
		case !ok:
			faults = append(faults, Fault{Key: at(key), Unknown: true, Want: p, Allowed: allowed})
		case inner.Type == "object" && inner.Properties != nil && isMap:
			faults = append(faults, inner.objectFaults(at(key), object)...)
		case !inner.admits(m[key]):
			faults = append(faults, Fault{Key: at(key), Want: inner, Allowed: allowed})
		}
	}
	return faults
}

// check is every way args fail s, in Faults' order; "" where none.
func (s Schema) check(args map[string]any) string {
	var faults []string
	for _, f := range s.Faults(args) {
		faults = append(faults, f.Detail())
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
// initialize tells the session the server serves, absent where it is "". No
// resource is subscribable: the server declares no subscribe capability.
func Serve(in io.Reader, out io.Writer, version, instructions string, tools []Tool, resources Resources, lg log.Logger) error {
	s := server{version: version, instructions: instructions, tools: map[string]Tool{}, resources: resources, lg: lg}
	for _, t := range tools {
		s.tools[t.Name] = t
		s.order = append(s.order, t.Name)
	}
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	r := bufio.NewReader(in)
	for {
		line, err := r.ReadBytes('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("reading a request: %w", err)
		}
		if frame := bytes.TrimRight(line, "\r\n"); len(frame) > 0 {
			if resp := s.handle(frame); resp != nil {
				if werr := enc.Encode(resp); werr != nil {
					return fmt.Errorf("writing a response: %w", werr)
				}
			}
		}
		if err != nil {
			return nil
		}
	}
}

type server struct {
	version      string
	instructions string
	tools        map[string]Tool
	order        []string
	resources    Resources
	lg           log.Logger
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
				"resources": map[string]bool{"listChanged": false},
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
	text, structured, isError, err := tool.Call(args)
	if err != nil {
		s.lg.Error("mcp: tool failed", "tool", name, "err", err)
		return failure(id, codeInternal, name+": "+err.Error())
	}
	return &response{JSONRPC: "2.0", ID: id, Result: callResult{
		Content:           []textContent{{Type: "text", Text: text}},
		StructuredContent: structured,
		IsError:           isError,
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
