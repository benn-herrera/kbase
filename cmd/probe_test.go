package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"kbase/internal/config"
	toolresult "kbase/internal/result"
)

// probeWire answers each request with a short stream and keeps the body it
// read, so a test reads what the probe actually sent. The deltas are paced —
// a pause before the first, a longer one before the last — so the timings a
// document carries are measurements rather than a stub's same-instant reply.
func probeWire(t *testing.T) (*httptest.Server, func() [][]byte) {
	t.Helper()
	var mu sync.Mutex
	var bodies [][]byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, body)
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		flush := func(s string) {
			_, _ = w.Write([]byte(s))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		time.Sleep(5 * time.Millisecond)
		flush("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n")
		time.Sleep(15 * time.Millisecond)
		flush("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
		flush("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":40,\"completion_tokens\":1,\"total_tokens\":41}}\n\n" +
			"data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)
	return srv, func() [][]byte {
		mu.Lock()
		defer mu.Unlock()
		return append([][]byte{}, bodies...)
	}
}

// probeFixture stages a configuration directory naming one pool entry at
// baseURL and the scoring role's reference to it, plus a second entry a
// --model can name instead.
func probeFixture(t *testing.T, baseURL, otherURL string) string {
	t.Helper()
	dir := t.TempDir()
	writeValues(t, dir, config.ProvidersFileName, fmt.Sprintf("[local]\nbaseUrl = %q\n\n[other]\nbaseUrl = %q\n", baseURL, otherURL))
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
		fmt.Fprintf(&providers, "[%s]\nbaseUrl = %q\n\n", role, urls[role])
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
	if err := resetFlags(probeCmd); err != nil {
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
	unsetInferenceEnv(t)
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

// TestProbeAsksOneBareTurn: the probe is a diagnostic, not a chat. It sends the
// default one-word check as the one user turn, no system text, no history and
// no tools.
func TestProbeAsksOneBareTurn(t *testing.T) {
	unsetInferenceEnv(t)
	srv, bodies := probeWire(t)
	dir := probeFixture(t, srv.URL, srv.URL)

	if _, stdout, _, code := runProbeVerb(t, t.TempDir(), "--config-dir", dir); code != 0 {
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
		body.Messages[1]["role"] != "user" || body.Messages[1]["content"] != probePrompt {
		t.Errorf("messages = %v, want an empty system message and the default check as the one user turn", body.Messages)
	}
	if body.Tools != nil || !body.Stream {
		t.Errorf("tools %v, stream %t; want no tools and a streamed call", body.Tools, body.Stream)
	}
}

// TestProbeNamesAProviderRefusal: an endpoint that answers with a status that is
// not success fails the probe in the document's own vocabulary — the call's
// outcome and text, the status and the provider's words — and exits on that
// outcome (§7). The one attempt is not re-issued: a 400 will be as wrong again.
func TestProbeNamesAProviderRefusal(t *testing.T) {
	unsetInferenceEnv(t)
	srv, requests := statusProvider(t, http.StatusBadRequest)
	dir := probeFixture(t, srv.URL, srv.URL)

	doc, stdout, _, code := runProbeVerb(t, t.TempDir(), "--config-dir", dir)
	if doc.Outcome != toolresult.Failed || code != exitCodes[toolresult.Failed] {
		t.Fatalf("outcome %s, exit %d; want failed, %d\n%s", doc.Outcome, code, exitCodes[toolresult.Failed], stdout)
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
	unsetInferenceEnv(t)
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
	unsetInferenceEnv(t)
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
			bodies := map[config.Role]func() [][]byte{}
			for _, role := range config.Roles {
				srv, seen := probeWire(t)
				urls[role], bodies[role] = srv.URL, seen
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
				if n := len(bodies[role]()); n != want {
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
	unsetInferenceEnv(t)
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
		if doc.Outcome != toolresult.Refused || code != exitCodes[toolresult.Refused] {
			t.Fatalf("%s: outcome %s, exit %d; want refused, %d\n%s", tc.name, doc.Outcome, code, exitCodes[toolresult.Refused], stdout)
		}
		items := doc.items(toolresult.RefusalsKey)
		if len(items) != 1 || items[0]["check"] != config.CheckUsage {
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
// or a role the files leave unnamed and no flag too — is a refusal naming the
// key or the flag and what is missing, before any client is built. A role flag
// names a role, not a model, so the refusal names the role's own key and never
// the flag that could have supplied one.
func TestProbeRefusesAModelItCannotResolve(t *testing.T) {
	unsetInferenceEnv(t)
	srv, bodies := probeWire(t)
	dir := probeFixture(t, srv.URL, srv.URL)
	unconfigured := t.TempDir()
	writeValues(t, unconfigured, config.ProvidersFileName, fmt.Sprintf("[local]\nbaseUrl = %q\n", srv.URL))

	for _, tc := range []struct {
		name, dir, model, roleFlag, wantKey, want string
	}{
		{"a bare id is no reference", dir, "judge", "", "--model", "a model is named <provider>/<model>"},
		{"a provider the pool does not hold", dir, "absent/judge", "", "--model", "a provider the pool does not hold"},
		{"no scoring role and no flag", unconfigured, "", "", "models.scoring", "--model names no model"},
		{"a role flag the files leave unnamed", unconfigured, "", "--text", "models.text", "sets no models.text"},
	} {
		args := []string{"--config-dir", tc.dir}
		if tc.model != "" {
			args = append(args, "--model", tc.model)
		}
		if tc.roleFlag != "" {
			args = append(args, tc.roleFlag)
		}
		doc, stdout, _, code := runProbeVerb(t, t.TempDir(), args...)
		if doc.Outcome != toolresult.Refused || code != exitCodes[toolresult.Refused] {
			t.Fatalf("%s: outcome %s, exit %d; want refused, %d\n%s", tc.name, doc.Outcome, code, exitCodes[toolresult.Refused], stdout)
		}
		items := doc.items(toolresult.RefusalsKey)
		if len(items) != 1 || items[0]["key"] != tc.wantKey {
			t.Fatalf("%s: refusals = %v, want one naming %s", tc.name, items, tc.wantKey)
		}
		detail, _ := items[0]["detail"].(string)
		if !strings.Contains(detail, tc.want) {
			t.Errorf("%s: refusal = %q, want it to name what is missing (%q)", tc.name, detail, tc.want)
		}
		if tc.roleFlag != "" && strings.Contains(detail, "--model") {
			t.Errorf("%s: refusal = %q, want no flag named: a role flag supplies no model", tc.name, detail)
		}
	}
	if n := len(bodies()); n != 0 {
		t.Errorf("%d requests, want none: a model that does not resolve is never dialled", n)
	}
}

// TestProbeReadsNoKBAndWritesNoState: the probe is a diagnostic of a model
// selection. It runs where no worktree stands, over a KB whose format every
// reading verb refuses, and leaves the state store untouched — and it declares
// no flag that could point it at either.
func TestProbeReadsNoKBAndWritesNoState(t *testing.T) {
	unsetInferenceEnv(t)
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
