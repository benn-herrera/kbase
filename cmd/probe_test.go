package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"kbase/internal/config"
	"kbase/internal/mcp"
	"kbase/internal/model"
	toolresult "kbase/internal/result"
)

// probeWire answers each request with a short stream and keeps the body it
// read, so a test reads what the probe actually sent. The deltas are paced —
// a pause before the first, a longer one before the last — so the timings a
// document carries are measurements rather than a stub's same-instant reply.
func probeWire(t *testing.T) (*httptest.Server, func() [][]byte) {
	t.Helper()
	return probeStreamWire(t, func(_ http.ResponseWriter, flush func(string)) {
		time.Sleep(5 * time.Millisecond)
		flush("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n")
		time.Sleep(15 * time.Millisecond)
		flush("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
		flush("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":40,\"completion_tokens\":1,\"total_tokens\":41}}\n\n" +
			"data: [DONE]\n\n")
	})
}

// probeStreamWire answers each request with what answer writes — a status on
// w, or a stream each piece of which is flushed as it is written — and keeps
// the body it read.
func probeStreamWire(t *testing.T, answer func(w http.ResponseWriter, flush func(string))) (*httptest.Server, func() [][]byte) {
	t.Helper()
	var mu sync.Mutex
	var bodies [][]byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, body)
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		answer(w, func(s string) {
			_, _ = w.Write([]byte(s))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		})
	}))
	t.Cleanup(srv.Close)
	return srv, func() [][]byte {
		mu.Lock()
		defer mu.Unlock()
		return append([][]byte{}, bodies...)
	}
}

// probeRoleWire is the endpoint one role's asks reach — the decision surface for
// the decision role, whose asks are decision calls, and a chat stream for the
// two chat roles — recording each request's path so a test counts what landed
// where.
func probeRoleWire(t *testing.T, role config.Role) (*httptest.Server, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		if role == config.RoleDecision {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"model":"decision","answers":{"probe":{"type":"noul","noul":0.96}},"usage":{"input_tokens":12,"output_tokens":1}}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
			"data: [DONE]\n\n"))
	}))
	t.Cleanup(srv.Close)
	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(paths)
	}
}

// probeFixture stages a configuration directory naming one pool entry at
// baseURL and the scoring role's reference to it, plus a second entry a
// --model can name instead.
func probeFixture(t *testing.T, baseURL, otherURL string) string {
	t.Helper()
	dir := t.TempDir()
	writeValues(t, dir, config.ProvidersFileName, fmt.Sprintf("[local]\nbaseUrl = %q\napi = %q\n\n[other]\nbaseUrl = %q\napi = %q\n",
		baseURL, config.ProviderAPIOpenAI, otherURL, config.ProviderAPIOpenAI))
	writeValues(t, dir, config.ConfigFileName, "[models]\nscoring = \"local/judge\"\n")
	return dir
}

// probeRolesFixture stages one pool entry per role, each at its own base URL,
// and config.toml naming each role its own entry and model — the role's name in
// both — so a request lands on a server, and a document names a selection, that
// identify the role asked.
func probeRolesFixture(t *testing.T, urls map[config.Role]string) string {
	t.Helper()
	dir := t.TempDir()
	var providers, models strings.Builder
	for _, role := range config.Roles {
		fmt.Fprintf(&providers, "[%s]\nbaseUrl = %q\napi = %q\n\n", role, urls[role], config.ProviderAPIOpenAI)
		fmt.Fprintf(&models, "%s = \"%s/%s\"\n", role, role, role)
	}
	writeValues(t, dir, config.ProvidersFileName, providers.String())
	writeValues(t, dir, config.ConfigFileName, "[models]\n"+models.String())
	return dir
}

// runProbeVerb runs `kbase probe` through the real root command from work and
// reads the document it wrote, with the verb's streams.
func runProbeVerb(t *testing.T, work string, args ...string) (doc resultDoc, stdout, stderr string, code int) {
	t.Helper()
	t.Chdir(work)
	stdoutBuf, stderrBuf, err := executeRoot(t, append([]string{"probe"}, args...)...)
	code = exitStatus(err, stdoutBuf, stderrBuf)
	if err := mcp.ResetFlags(probeCmd); err != nil {
		t.Fatal(err)
	}
	stdout, stderr = stdoutBuf.String(), stderrBuf.String()
	return checkDocument(t, "probe", stdout), stdout, stderr, code
}

// answeredProbe reads the fields every healthy probe carries, failing on a
// field that is missing or of another kind.
func answeredProbe(t *testing.T, doc resultDoc) (firstToken, wall int, usage map[string]any) {
	t.Helper()
	for _, key := range []string{"provider", "provider-rule", "provider-layer", "selection", "model", "first-token-ms", "wall-ms", "finish-reason", "reply", "usage"} {
		if _, ok := doc.Values[key]; !ok {
			t.Fatalf("the document carries no %s: %v", key, doc.Values)
		}
	}
	firstToken, okFirst := doc.Values["first-token-ms"].(int)
	wall, okWall := doc.Values["wall-ms"].(int)
	usage, okUsage := doc.Values["usage"].(map[string]any)
	if !okFirst || !okWall || !okUsage {
		t.Fatalf("timing and usage fields: %v, want integers and a mapping", doc.Values)
	}
	return firstToken, wall, usage
}

// TestProbeReportsTheCall: a probe against a provider that answers writes the
// selection, the timings, the finish reason, the reply and the usage, and exits
// 0 — the heartbeat a healthy selection gives. With none of the four selection
// flags given, that selection is the scoring role's.
func TestProbeReportsTheCall(t *testing.T) {
	srv, _ := probeWire(t)
	dir := probeFixture(t, srv.URL, srv.URL)

	doc, stdout, _, code := runProbeVerb(t, t.TempDir(), "--config-dir", dir)
	if code != 0 || doc.Outcome != toolresult.Done {
		t.Fatalf("outcome %s, exit %d; want done, 0\n%s", doc.Outcome, code, stdout)
	}
	if doc.Values["provider"] != "local" || doc.Values["provider-rule"] != config.RuleConfig ||
		doc.Values["provider-layer"] != config.LayerSourceExplicit || doc.Values["model"] != "judge" {
		t.Errorf("provider/rule/layer %v/%v/%v, model %v; want the configured scoring reference",
			doc.Values["provider"], doc.Values["provider-rule"], doc.Values["provider-layer"], doc.Values["model"])
	}
	if doc.Values["selection"] != string(config.RoleScoring) {
		t.Errorf("selection = %v, want the scoring role a bare probe asks", doc.Values["selection"])
	}
	firstToken, wall, usage := answeredProbe(t, doc)
	if firstToken <= 0 || firstToken >= wall {
		t.Errorf("first token %dms over a call of %dms, want the first delta's moment inside it", firstToken, wall)
	}
	if doc.Values["finish-reason"] != "stop" || doc.Values["reply"] != "ok" {
		t.Errorf("finish reason %v, reply %v; want the stream's stop and its text",
			doc.Values["finish-reason"], doc.Values["reply"])
	}
	if usage["reported"] != true || usage["prompt-tokens"] != 40 || usage["completion-tokens"] != 1 ||
		usage["reasoning-tokens"] != 0 || usage["cached-tokens"] != 0 {
		t.Errorf("usage = %v, want the reported block with no reasoning or cached share", usage)
	}
}

// TestProbeAsksOneBareTurn: the probe is a diagnostic, not a chat. It sends one
// user turn — the default one-word check, or the check --prompt names — as the
// only turn, no system text, no history and no tools.
func TestProbeAsksOneBareTurn(t *testing.T) {
	for _, tc := range []struct {
		name, prompt, want string
	}{
		{"the default check", "", probePrompt},
		{"the check --prompt names", "Is this selection serving?", "Is this selection serving?"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, bodies := probeWire(t)
			dir := probeFixture(t, srv.URL, srv.URL)
			args := []string{"--config-dir", dir}
			if tc.prompt != "" {
				args = append(args, "--prompt", tc.prompt)
			}

			if _, stdout, _, code := runProbeVerb(t, t.TempDir(), args...); code != 0 {
				t.Fatalf("probe: exit %d\n%s", code, stdout)
			}
			seen := bodies()
			if len(seen) != 1 {
				t.Fatalf("%d requests, want the one turn it took", len(seen))
			}
			var body struct {
				Model    string           `json:"model"`
				Messages []map[string]any `json:"messages"`
				Tools    any              `json:"tools"`
				Stream   bool             `json:"stream"`
			}
			if err := json.Unmarshal(seen[0], &body); err != nil {
				t.Fatalf("the request is no JSON body: %v\n%s", err, seen[0])
			}
			if body.Model != "judge" {
				t.Errorf("model = %q, want the scoring role's", body.Model)
			}
			if len(body.Messages) != 2 || body.Messages[0]["role"] != "system" || body.Messages[0]["content"] != "" ||
				body.Messages[1]["role"] != "user" || body.Messages[1]["content"] != tc.want {
				t.Errorf("messages = %v, want an empty system message and %q as the one user turn",
					body.Messages, tc.want)
			}
			if body.Tools != nil || !body.Stream {
				t.Errorf("tools %v, stream %t; want no tools and a streamed call", body.Tools, body.Stream)
			}
		})
	}
}

// TestProbeSendsAndReportsTheRoleSampling: a chat probe sends its role's
// sampling — the scoring role's where --model names the selection — and its
// document reports the temperature and thinking that went on the wire,
// whether the call was answered or declined. An
// answer cut off at the completion cap before any text is an answer: done, its
// finish reason and empty reply reported, on the one request it took.
func TestProbeSendsAndReportsTheRoleSampling(t *testing.T) {
	answered := func(_ http.ResponseWriter, flush func(string)) {
		flush("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
			"data: [DONE]\n\n")
	}
	cutOff := func(_ http.ResponseWriter, flush func(string)) {
		flush("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"length\"}]}\n\n" +
			"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":40,\"completion_tokens\":8,\"total_tokens\":48}}\n\n" +
			"data: [DONE]\n\n")
	}
	declined := func(w http.ResponseWriter, _ func(string)) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"unknown model"}}`))
	}
	for _, tc := range []struct {
		name     string
		flags    []string
		sampling string
		answer   func(w http.ResponseWriter, flush func(string))
		// temperature is the configured value the document must report, nil
		// where none is configured and only agreement with the wire is checked.
		temperature      any
		thinking, effort string
		outcome          string
		// finishReason and reply are nil where the document carries no answer.
		finishReason, reply any
	}{
		{"--text sends the text role's", []string{"--text"},
			"[sampling.text]\ntemperature = 0.3\nthinking = \"low\"\n", answered,
			0.3, "low", "low", toolresult.Done, "stop", "ok"},
		{"--model sends the scoring role's", []string{"--model", "local/other"},
			"[sampling.text]\nthinking = \"high\"\n\n[sampling.scoring]\nthinking = \"medium\"\n", answered,
			nil, "medium", "medium", toolresult.Done, "stop", "ok"},
		{"an answer cut off before any text is done", []string{"--text"}, "", cutOff,
			nil, "off", "", toolresult.Done, "length", ""},
		{"a declined call reports what it sent", []string{"--text"},
			"[sampling.text]\ntemperature = 0.3\nthinking = \"low\"\n", declined,
			0.3, "low", "low", toolresult.Failed, nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, bodies := probeStreamWire(t, tc.answer)
			dir := t.TempDir()
			writeValues(t, dir, config.ProvidersFileName, fmt.Sprintf("[local]\napi = \"openai\"\nbaseUrl = %q\n", srv.URL))
			writeValues(t, dir, config.ConfigFileName,
				"[models]\ntext = \"local/writer\"\nscoring = \"local/judge\"\n\n"+tc.sampling)

			doc, stdout, _, code := runProbeVerb(t, t.TempDir(), append([]string{"--config-dir", dir}, tc.flags...)...)
			if doc.Outcome != tc.outcome || (code == 0) != (tc.outcome == toolresult.Done) {
				t.Fatalf("outcome %s, exit %d; want %s\n%s", doc.Outcome, code, tc.outcome, stdout)
			}
			seen := bodies()
			if len(seen) != 1 {
				t.Fatalf("%d requests, want the one call", len(seen))
			}
			var body struct {
				Temperature     *float64 `json:"temperature"`
				ReasoningEffort string   `json:"reasoning_effort"`
			}
			if err := json.Unmarshal(seen[0], &body); err != nil {
				t.Fatalf("the request is no JSON body: %v\n%s", err, seen[0])
			}
			wire := "absent"
			if body.Temperature != nil {
				wire = fmt.Sprint(*body.Temperature)
			}
			if fmt.Sprint(doc.Values["temperature"]) != wire {
				t.Errorf("document temperature %v, wire %s; want the value sent", doc.Values["temperature"], wire)
			}
			if tc.temperature != nil && doc.Values["temperature"] != tc.temperature {
				t.Errorf("temperature = %v, want the configured %v", doc.Values["temperature"], tc.temperature)
			}
			if doc.Values["thinking"] != tc.thinking || body.ReasoningEffort != tc.effort {
				t.Errorf("document thinking %v, wire reasoning_effort %q; want %s and %q",
					doc.Values["thinking"], body.ReasoningEffort, tc.thinking, tc.effort)
			}
			if doc.Values["finish-reason"] != tc.finishReason || doc.Values["reply"] != tc.reply {
				t.Errorf("finish reason %v, reply %#v; want %v and %#v",
					doc.Values["finish-reason"], doc.Values["reply"], tc.finishReason, tc.reply)
			}
		})
	}
}

// TestProbeNamesAProviderRefusal: an endpoint that answers with a status that is
// not success fails the probe in the document's own vocabulary — the call's
// outcome and text, the status and the provider's words — and exits on that
// outcome (§7). The one attempt is not re-issued: a 400 will be as wrong again.
func TestProbeNamesAProviderRefusal(t *testing.T) {
	srv, requests := statusProvider(t, http.StatusBadRequest)
	dir := probeFixture(t, srv.URL, srv.URL)

	doc, stdout, _, code := runProbeVerb(t, t.TempDir(), "--config-dir", dir)
	if doc.Outcome != toolresult.Failed || code != toolresult.ExitCode(toolresult.Failed) {
		t.Fatalf("outcome %s, exit %d; want failed, %d\n%s", doc.Outcome, code, toolresult.ExitCode(toolresult.Failed), stdout)
	}
	items := doc.items(toolresult.FailuresKey)
	if len(items) != 1 {
		t.Fatalf("failures = %v, want the one item", items)
	}
	detail, _ := items[0]["detail"].(string)
	for _, want := range []string{"provider-refused", "http 400", "unknown model"} {
		if !strings.Contains(detail, want) {
			t.Errorf("failure = %q, want it to carry %q", detail, want)
		}
	}
	if doc.Values["provider"] != "local" || doc.Values["model"] != "judge" {
		t.Errorf("the document names %v/%v, want the selection it dialled",
			doc.Values["provider"], doc.Values["model"])
	}
	if n := requests(); n != 1 {
		t.Errorf("%d requests, want the one attempt: a refusal is not re-issued", n)
	}
}

// TestProbeModelOverridesTheRole: --model names the reference to probe instead
// of the configured scoring role, resolved against the same pool — so the call
// reaches the entry and model the flag named.
func TestProbeModelOverridesTheRole(t *testing.T) {
	role, roleBodies := probeWire(t)
	other, otherBodies := probeWire(t)
	dir := probeFixture(t, role.URL, other.URL)

	doc, stdout, _, code := runProbeVerb(t, t.TempDir(), "--config-dir", dir, "--model", "other/alt")
	if code != 0 {
		t.Fatalf("probe --model: exit %d\n%s", code, stdout)
	}
	if doc.Values["provider"] != "other" || doc.Values["provider-rule"] != config.RuleFlag || doc.Values["model"] != "alt" {
		t.Errorf("provider/rule model = %v/%v %v, want the flag's reference",
			doc.Values["provider"], doc.Values["provider-rule"], doc.Values["model"])
	}
	// The document names the reference the flag gave as the selection that
	// answered, so a reader does not have to read it out of the model id.
	if doc.Values["selection"] != "other/alt" {
		t.Errorf("selection = %v, want the reference --model named", doc.Values["selection"])
	}
	if len(otherBodies()) != 1 || len(roleBodies()) != 0 {
		t.Errorf("%d requests to the flag's entry, %d to the role's; want the flag's alone",
			len(otherBodies()), len(roleBodies()))
	}
}

// TestProbeEachRoleFlagAsksItsOwnRole: --decision, --text and --scoring each
// probe the selection the configuration resolves for that role, and supply no
// model of their own. A fixture naming three different entries proves which one
// is dialled; the document names the role as the selection that answered.
func TestProbeEachRoleFlagAsksItsOwnRole(t *testing.T) {
	for _, tc := range []struct {
		flag string
		role config.Role
	}{
		{"--decision", config.RoleDecision},
		{"--text", config.RoleText},
		{"--scoring", config.RoleScoring},
	} {
		t.Run(tc.flag, func(t *testing.T) {
			urls := map[config.Role]string{}
			asked := map[config.Role]func() []string{}
			for _, role := range config.Roles {
				srv, seen := probeRoleWire(t, role)
				urls[role], asked[role] = srv.URL, seen
			}
			dir := probeRolesFixture(t, urls)

			doc, stdout, _, code := runProbeVerb(t, t.TempDir(), "--config-dir", dir, tc.flag)
			if code != 0 || doc.Outcome != toolresult.Done {
				t.Fatalf("%s: outcome %s, exit %d\n%s", tc.flag, doc.Outcome, code, stdout)
			}
			if doc.Values["selection"] != string(tc.role) || doc.Values["provider"] != string(tc.role) ||
				doc.Values["model"] != string(tc.role) {
				t.Errorf("%s asked %v on %v/%v, want the %s role's own selection",
					tc.flag, doc.Values["selection"], doc.Values["provider"], doc.Values["model"], tc.role)
			}
			for _, role := range config.Roles {
				want := 0
				if role == tc.role {
					want = 1
				}
				if n := len(asked[role]()); n != want {
					t.Errorf("%d requests to the %s role's entry, want %d", n, role, want)
				}
			}
		})
	}
}

// TestProbeRefusesTwoSelections: a probe asks one selection — one of --model,
// --decision, --text and --scoring. Two role flags, or a role flag with
// --model, is a usage refusal naming all four, and nothing is dialled, the
// roles' own configuration notwithstanding.
func TestProbeRefusesTwoSelections(t *testing.T) {
	srv, bodies := probeWire(t)
	dir := probeFixture(t, srv.URL, srv.URL)
	selectionFlags := []string{"--model", "--decision", "--text", "--scoring"}

	for _, tc := range []struct {
		name  string
		flags []string
	}{
		{"two role flags", []string{"--decision", "--text"}},
		{"a role flag and a reference", []string{"--text", "--model", "other/alt"}},
	} {
		doc, stdout, _, code := runProbeVerb(t, t.TempDir(), append([]string{"--config-dir", dir}, tc.flags...)...)
		if doc.Outcome != toolresult.Refused || code != toolresult.ExitCode(toolresult.Refused) {
			t.Fatalf("%s: outcome %s, exit %d; want refused, %d\n%s", tc.name, doc.Outcome, code, toolresult.ExitCode(toolresult.Refused), stdout)
		}
		items := doc.items(toolresult.RefusalsKey)
		if len(items) != 1 || items[0]["check"] != toolresult.CheckUsage {
			t.Fatalf("%s: refusals = %v, want the one usage item", tc.name, items)
		}
		if !slices.Equal(anyStrings(items[0]["allowed"]), selectionFlags) {
			t.Errorf("%s: allowed = %v, want the four that may be chosen once", tc.name, items[0]["allowed"])
		}
		detail, _ := items[0]["detail"].(string)
		for _, flag := range selectionFlags {
			if !strings.Contains(detail, flag) {
				t.Errorf("%s: refusal = %q, want it to name %s", tc.name, detail, flag)
			}
		}
	}
	if n := len(bodies()); n != 0 {
		t.Errorf("%d requests, want none: more than one selection is refused before anything dials", n)
	}
}

// TestProbeRefusesAModelItCannotResolve: a model the configuration cannot
// supply — a --model that is no reference, a provider the pool does not hold,
// or a role the files leave unnamed — is a refusal naming the key or the flag
// and what is missing, before any client is built. A role flag names a role,
// not a model, so the refusal names the role's own key and never the flag that
// could have supplied one; a probe naming no selection at all is refused by the
// scoring role's key first, the role being the default rather than a flag's,
// and names --model only as the other way to ask.
func TestProbeRefusesAModelItCannotResolve(t *testing.T) {
	srv, bodies := probeWire(t)
	dir := probeFixture(t, srv.URL, srv.URL)
	unconfigured := t.TempDir()
	writeValues(t, unconfigured, config.ProvidersFileName, fmt.Sprintf("[local]\nbaseUrl = %q\n", srv.URL))

	for _, tc := range []struct {
		name, dir, model, roleFlag, wantKey string
		// want is what the refusal must say, and absent what it must not: no
		// case reads as though a flag it never carried had been passed.
		want   []string
		absent string
	}{
		{"a bare id is no reference", dir, "judge", "", "--model",
			[]string{"a model is named <provider>/<model>"}, ""},
		{"a provider the pool does not hold", dir, "absent/judge", "", "--model",
			[]string{"a provider the pool does not hold"}, ""},
		{"nothing named a selection", unconfigured, "", "", "models.scoring",
			[]string{"sets no models.scoring", "nothing named a selection",
				"the scoring role is the default", "--model"}, "names no model"},
		{"a role flag the files leave unnamed", unconfigured, "", "--text", "models.text",
			[]string{"sets no models.text"}, "--model"},
		{"the scoring role named by its own flag", unconfigured, "", "--scoring", "models.scoring",
			[]string{"sets no models.scoring"}, "--model"},
	} {
		args := []string{"--config-dir", tc.dir}
		if tc.model != "" {
			args = append(args, "--model", tc.model)
		}
		if tc.roleFlag != "" {
			args = append(args, tc.roleFlag)
		}
		doc, stdout, _, code := runProbeVerb(t, t.TempDir(), args...)
		if doc.Outcome != toolresult.Refused || code != toolresult.ExitCode(toolresult.Refused) {
			t.Fatalf("%s: outcome %s, exit %d; want refused, %d\n%s", tc.name, doc.Outcome, code, toolresult.ExitCode(toolresult.Refused), stdout)
		}
		items := doc.items(toolresult.RefusalsKey)
		if len(items) != 1 || items[0]["key"] != tc.wantKey {
			t.Fatalf("%s: refusals = %v, want one naming %s", tc.name, items, tc.wantKey)
		}
		detail, _ := items[0]["detail"].(string)
		for _, want := range tc.want {
			if !strings.Contains(detail, want) {
				t.Errorf("%s: refusal = %q, want it to say %q", tc.name, detail, want)
			}
		}
		if tc.absent != "" && strings.Contains(detail, tc.absent) {
			t.Errorf("%s: refusal = %q, want no %q", tc.name, detail, tc.absent)
		}
	}
	if n := len(bodies()); n != 0 {
		t.Errorf("%d requests, want none: a model that does not resolve is never dialled", n)
	}
}

// anthropicProbeRequest is what one request carried, as the Messages fixture
// records it.
type anthropicProbeRequest struct {
	path, apiKey, version string
	body                  []byte
}

// anthropicProbeWire answers each request as a Messages event stream — usage on
// message_start, one text delta, the stop reason and message_stop — and records
// what the request carried. It is the Messages counterpart of probeWire.
func anthropicProbeWire(t *testing.T) (*httptest.Server, func() []anthropicProbeRequest) {
	t.Helper()
	var mu sync.Mutex
	var seen []anthropicProbeRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen = append(seen, anthropicProbeRequest{
			path:    r.URL.Path,
			apiKey:  r.Header.Get("x-api-key"),
			version: r.Header.Get("anthropic-version"),
			body:    body,
		})
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`event: message_start
data: {"type":"message_start","message":{"usage":{"input_tokens":30,"cache_read_input_tokens":4}}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"ok"}}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}

event: message_stop
data: {"type":"message_stop"}

`))
	}))
	t.Cleanup(srv.Close)
	return srv, func() []anthropicProbeRequest {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(seen)
	}
}

// TestProbeReachesAMessagesProvider: a pool entry stating `api = "anthropic"`
// reaches the ask. The probe's one call lands on `POST <base>/messages` with
// that protocol's headers and body, and the reply, stop reason and usage come
// back in the document — so the whole route, from providers.toml to the wire,
// is exercised.
func TestProbeReachesAMessagesProvider(t *testing.T) {
	srv, requests := anthropicProbeWire(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "claude.key"), []byte(testAPIKey+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeValues(t, dir, config.ProvidersFileName, fmt.Sprintf(`[claude]
baseUrl = %q
api = "anthropic"
apiKeyFile = "claude.key"
`, srv.URL))
	writeValues(t, dir, config.ConfigFileName, "[models]\nscoring = \"claude/claude-x\"\n")

	doc, stdout, stderr, code := runProbeVerb(t, t.TempDir(), "--config-dir", dir)
	if code != 0 || doc.Outcome != toolresult.Done {
		t.Fatalf("outcome %s, exit %d; want done, 0\n%s%s", doc.Outcome, code, stdout, stderr)
	}
	if doc.Values["provider"] != "claude" || doc.Values["model"] != "claude-x" {
		t.Errorf("provider/model = %v/%v, want the Messages entry it resolved",
			doc.Values["provider"], doc.Values["model"])
	}
	if doc.Values["reply"] != "ok" || doc.Values["finish-reason"] != "end_turn" {
		t.Errorf("reply %v, finish reason %v; want the event stream's text and stop reason",
			doc.Values["reply"], doc.Values["finish-reason"])
	}
	usage, _ := doc.Values["usage"].(map[string]any)
	if usage["reported"] != true || usage["prompt-tokens"] != 30 || usage["completion-tokens"] != 1 ||
		usage["cached-tokens"] != 4 {
		t.Errorf("usage = %v, want the message_start and message_delta figures", usage)
	}

	seen := requests()
	if len(seen) != 1 {
		t.Fatalf("%d requests, want the one turn it took", len(seen))
	}
	if seen[0].path != "/messages" {
		t.Errorf("path = %q, want /messages", seen[0].path)
	}
	if seen[0].apiKey != testAPIKey || seen[0].version != "2023-06-01" {
		t.Errorf("auth headers: x-api-key %q, anthropic-version %q; want the entry's key and the protocol version",
			seen[0].apiKey, seen[0].version)
	}
	var body struct {
		Model     string `json:"model"`
		MaxTokens int    `json:"max_tokens"`
		Stream    bool   `json:"stream"`
		Messages  []struct{ Role, Content string }
	}
	if err := json.Unmarshal(seen[0].body, &body); err != nil {
		t.Fatalf("the request is no JSON body: %v\n%s", err, seen[0].body)
	}
	if body.Model != "claude-x" || !body.Stream || body.MaxTokens < 256 ||
		len(body.Messages) != 1 || body.Messages[0].Role != "user" || body.Messages[0].Content != probePrompt {
		t.Errorf("body = %+v, want the probe's one streamed user turn under a completion cap", body)
	}
	if strings.Contains(string(seen[0].body), "prompt_cache_key") {
		t.Errorf("the request carries a prompt-cache key, which the protocol has no field for: %s", seen[0].body)
	}
}

// probeDecisionWire answers the decision surface at path — the SystemOne
// object or the Decisions array, by which path it is — and answers any other
// path the way a provider refuses a decision model on a chat call. It records
// every path it served and the body it read there, so a probe that took the
// chat request is visible as a 400 and as the wrong path, and a test reads the
// ask the probe sent.
func probeDecisionWire(t *testing.T, path string) (*httptest.Server, func() ([]string, [][]byte)) {
	t.Helper()
	var mu sync.Mutex
	var paths []string
	var bodies [][]byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		paths = append(paths, r.URL.Path)
		bodies = append(bodies, body)
		mu.Unlock()
		if r.URL.Path != path {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"this model is not an LLM / chat model"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if path == "/decisions" {
			_, _ = w.Write([]byte(`{"model":"d","answers":[{"type":"predicate","name":"probe","probability":0.96}],"usage":{"input_tokens":12,"output_tokens":1}}`))
			return
		}
		_, _ = w.Write([]byte(`{"model":"d","answers":{"probe":{"type":"noul","noul":0.96}},"usage":{"input_tokens":12,"output_tokens":1}}`))
	}))
	t.Cleanup(srv.Close)
	return srv, func() ([]string, [][]byte) {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(paths), append([][]byte{}, bodies...)
	}
}

// probeDecisionConfig stages a configuration directory naming one pool entry at
// baseURL for the decision role, the entry carrying keys where the test gives
// some.
func probeDecisionConfig(t *testing.T, baseURL, keys string) string {
	t.Helper()
	dir := t.TempDir()
	writeValues(t, dir, config.ProvidersFileName, fmt.Sprintf("[local]\nbaseUrl = %q\n%s", baseURL, keys))
	writeValues(t, dir, config.ConfigFileName, "[models]\ndecision = \"local/d\"\n")
	return dir
}

// probeAskedQuestion reads the one question a decision request carried, in the
// shape the surface's wire format gives it: the SystemOne object keyed by the
// question's key, the Decisions array whose entry names it. It returns the
// model dialled and the question's key, type and instructions — the fields a
// reader of either surface needs — and fails on a request that asks any number
// of questions but one.
func probeAskedQuestion(t *testing.T, body []byte) (modelID, key, typ, instructions string) {
	t.Helper()
	var req struct {
		Model     string          `json:"model"`
		Questions json.RawMessage `json:"questions"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("the decision request is no JSON body: %v\n%s", err, body)
	}
	questions := strings.TrimSpace(string(req.Questions))
	if questions == "" {
		t.Fatalf("the request carries no questions: %s", body)
	}
	if questions[0] == '{' {
		var asked map[string]struct {
			Type         string `json:"type"`
			Instructions string `json:"instructions"`
		}
		if err := json.Unmarshal(req.Questions, &asked); err != nil {
			t.Fatalf("the request's questions are no keyed object: %v\n%s", err, questions)
		}
		if len(asked) != 1 {
			t.Fatalf("%d questions, want the probe's one\n%s", len(asked), questions)
		}
		for k, q := range asked {
			return req.Model, k, q.Type, q.Instructions
		}
	}
	var asked []struct {
		Type         string `json:"type"`
		Name         string `json:"name"`
		Instructions string `json:"instructions"`
	}
	if err := json.Unmarshal(req.Questions, &asked); err != nil {
		t.Fatalf("the request's questions are neither a keyed object nor an array: %v\n%s", err, questions)
	}
	if len(asked) != 1 {
		t.Fatalf("%d questions, want the probe's one\n%s", len(asked), questions)
	}
	return req.Model, asked[0].Name, asked[0].Type, asked[0].Instructions
}

// TestProbeDecisionAsksTheDecisionSurface: --decision asks a decision model on
// the surface its entry declares, not on the chat path — one request, on
// /systemone or on /decisions as the entry's decisionsFlavor names, asking the
// probe's one question: the model dialled, the key the answer comes back under,
// the type that surface names a noul question, and the check as its
// instructions — the probe's own predicate by default, or the one --prompt
// names. A document whose reply is the answer. The surface blocks, so no first
// token arrived and no finish reason came back; the row states those as zero and
// null. A probe that took the chat path would be answered 400 and fail.
func TestProbeDecisionAsksTheDecisionSurface(t *testing.T) {
	for _, tc := range []struct {
		name, keys, path, prompt, wantType, wantInstructions string
	}{
		{"the SystemOne surface, an entry silent on the flavour", "", "/systemone", "", "noul", probeDecisionPrompt},
		{"the Decisions surface", "decisionsFlavor = \"decisions\"\n", "/decisions", "", "predicate", probeDecisionPrompt},
		{"the SystemOne surface, --prompt naming the predicate", "", "/systemone", "Is this selection answering?", "noul", "Is this selection answering?"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, served := probeDecisionWire(t, tc.path)
			dir := probeDecisionConfig(t, srv.URL, tc.keys)
			args := []string{"--config-dir", dir, "--decision"}
			if tc.prompt != "" {
				args = append(args, "--prompt", tc.prompt)
			}

			doc, stdout, _, code := runProbeVerb(t, t.TempDir(), args...)
			if code != 0 || doc.Outcome != toolresult.Done {
				t.Fatalf("outcome %s, exit %d; want done, 0\n%s", doc.Outcome, code, stdout)
			}
			if doc.Values["selection"] != string(config.RoleDecision) || doc.Values["model"] != "d" {
				t.Errorf("selection/model = %v/%v, want the decision role's selection",
					doc.Values["selection"], doc.Values["model"])
			}
			if got := doc.Values["first-token-ms"]; got != 0 {
				t.Errorf("first-token-ms = %v, want 0: the decision surface streams nothing", got)
			}
			if v, ok := doc.Values["finish-reason"]; !ok || v != nil {
				t.Errorf("finish-reason = %v (present %t), want null: the surface reports none", v, ok)
			}
			if doc.Values["reply"] != "0.96" {
				t.Errorf("reply = %v, want the predicate's probability", doc.Values["reply"])
			}
			if usage, _ := doc.Values["usage"].(map[string]any); usage["reported"] != true ||
				usage["prompt-tokens"] != 12 || usage["completion-tokens"] != 1 {
				t.Errorf("usage = %v, want the answer's own accounting", doc.Values["usage"])
			}
			seen, sent := served()
			if !slices.Equal(seen, []string{tc.path}) {
				t.Fatalf("requests %v, want the one decision call on %s", seen, tc.path)
			}
			if len(sent) != 1 {
				t.Fatalf("%d request bodies, want the one ask", len(sent))
			}
			modelID, key, typ, instructions := probeAskedQuestion(t, sent[0])
			if modelID != "d" || key != probeDecisionQuestion || typ != tc.wantType ||
				instructions != tc.wantInstructions {
				t.Errorf("the ask = model %q, question %q of type %q with instructions %q; want model d, question %s of type %s with instructions %q",
					modelID, key, typ, instructions, probeDecisionQuestion, tc.wantType, tc.wantInstructions)
			}
		})
	}
}

// TestProbeDecisionOnAnAnthropicEntryAsksTheDecisionSurface: --decision on an
// entry whose api is anthropic still asks the OpenAI-shaped decision surface on
// the entry's base URL.
func TestProbeDecisionOnAnAnthropicEntryAsksTheDecisionSurface(t *testing.T) {
	srv, served := probeDecisionWire(t, "/systemone")
	dir := probeDecisionConfig(t, srv.URL, "api = \"anthropic\"\n")

	doc, stdout, _, code := runProbeVerb(t, t.TempDir(), "--config-dir", dir, "--decision")
	if code != 0 || doc.Outcome != toolresult.Done {
		t.Fatalf("outcome %s, exit %d; want done, 0\n%s", doc.Outcome, code, stdout)
	}
	if seen, _ := served(); !slices.Equal(seen, []string{"/systemone"}) {
		t.Errorf("requests %v, want the one decision call on /systemone", seen)
	}
}

// TestProbeDecisionReplyStatesTheAnswer: the reply is the answer's own — a
// predicate's probability, the letter a choice picked, and the type alone where
// the answer decided nothing, which is what a refusal carries.
func TestProbeDecisionReplyStatesTheAnswer(t *testing.T) {
	for _, tc := range []struct {
		name   string
		answer model.DecisionAnswer
		want   string
	}{
		{"a predicate's probability", model.DecisionAnswer{Key: probeDecisionQuestion, Type: "noul", Noul: 0.96}, "0.96"},
		{"a chosen letter", model.DecisionAnswer{Key: probeDecisionQuestion, Type: "choice", Choice: "b"}, "b"},
		{"an answer carrying only its type", model.DecisionAnswer{Key: probeDecisionQuestion, Type: "refusal"}, "refusal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := probeDecisionReply(tc.answer); got != tc.want {
				t.Errorf("reply = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestProbeReadsNoKBAndWritesNoState: the probe is a diagnostic of a model
// selection. It runs where no worktree stands, over a KB whose format every
// reading verb refuses, and leaves the state store untouched — and it declares
// no flag that could point it at either.
func TestProbeReadsNoKBAndWritesNoState(t *testing.T) {
	srv, _ := probeWire(t)
	dir := probeFixture(t, srv.URL, srv.URL)
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	work := t.TempDir()
	writeRepo(t, work, map[string]string{
		".git/HEAD":              "ref: refs/heads/main\n",
		"kb-root/entry-point.md": "---\nkind: entry-point\nkb-format: \"99.0.0\"\n---\n\n# KB\n",
	})

	doc, stdout, _, code := runProbeVerb(t, work, "--config-dir", dir)
	if code != 0 || doc.Outcome != toolresult.Done {
		t.Fatalf("outcome %s, exit %d; want done where a KB-refusing KB stands\n%s", doc.Outcome, code, stdout)
	}
	if _, ok := doc.Values["kb-root"]; ok {
		t.Errorf("the document carries kb-root: %v", doc.Values["kb-root"])
	}
	if _, ok := doc.Values["state-dir"]; ok {
		t.Errorf("the document carries state-dir: %v", doc.Values["state-dir"])
	}
	if entries, err := os.ReadDir(stateHome); err != nil || len(entries) != 0 {
		t.Errorf("the state store %s holds %v (%v), want nothing written", stateHome, entries, err)
	}
	for _, flag := range []string{"kb-root", "state-dir"} {
		if probeCmd.Flags().Lookup(flag) != nil {
			t.Errorf("probe takes --%s; a probe reads no KB and writes no state", flag)
		}
	}
}
