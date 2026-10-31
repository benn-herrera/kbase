package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kbase/internal/config"
	"kbase/internal/log"
	"kbase/internal/mcp"
	"kbase/internal/model"
	"kbase/internal/query"
	toolresult "kbase/internal/result"
)

// refusingProvider answers every request with code, its error body echoing the
// credential the request carried, as a provider's error page may. sawKey
// reports whether a request carried testAPIKey, so a test proving the key
// absent from a verb's output knows the key was sent.
func refusingProvider(t *testing.T, code int) (srv *httptest.Server, sawKey func() bool) {
	t.Helper()
	var saw atomic.Bool
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The Messages surface carries the key as x-api-key, the OpenAI one
		// as a bearer token.
		credential := r.Header.Get("Authorization") + r.Header.Get("x-api-key")
		if strings.Contains(credential, testAPIKey) {
			saw.Store(true)
		}
		w.WriteHeader(code)
		fmt.Fprintf(w, `{"error":{"message":"refused the credential %s"}}`, credential)
	}))
	t.Cleanup(srv.Close)
	return srv, saw.Load
}

// refusingConfig stages a configuration directory whose one pool entry, lab,
// is at baseURL with testAPIKey in its key file, every role on it; the store
// is on an empty scratch directory, so the key file is the key sent.
func refusingConfig(t *testing.T, baseURL string) string {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "key.txt")
	if err := os.WriteFile(keyFile, []byte(testAPIKey+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeValues(t, dir, config.ProvidersFileName, fmt.Sprintf("[lab]\nbaseUrl = %q\napiKeyFile = %q\n", baseURL, keyFile))
	writeValues(t, dir, config.ConfigFileName, "[models]\ndecision = \"lab/judge\"\ntext = \"lab/judge\"\nscoring = \"lab/judge\"\n")
	return dir
}

// refusedRun is one verb's run against a refusing provider: its document and
// both streams.
type refusedRun struct {
	doc            resultDoc
	stdout, stderr string
}

// refusedVerbs are the verbs a provider can refuse for its key, each run over
// the configuration directory dir against the provider at baseURL — the root
// command for the catalogue verbs and the probe, the verb function for score,
// whose KB and state are scratch.
var refusedVerbs = []struct {
	name string
	// entry is the pool entry the verb dials, which its remedy names.
	entry string
	run   func(t *testing.T, dir, baseURL string) refusedRun
}{
	{"probe", "lab", func(t *testing.T, dir, _ string) refusedRun {
		return refusedRoot(t, "probe", "--config-dir", dir)
	}},
	{"models", "lab", func(t *testing.T, dir, _ string) refusedRun {
		return refusedRoot(t, "models", "--config-dir", dir)
	}},
	{"configure --provider", "lab", func(t *testing.T, dir, _ string) refusedRun {
		return refusedRoot(t, "configure", "--config-dir", dir, "--provider", "lab",
			"--model-map", "decision=lab/judge,text=lab/judge,scoring=lab/judge")
	}},
	{"score", "solo", func(t *testing.T, _, baseURL string) refusedRun {
		got := scoreDrive(t, scoreRepo(t), t.TempDir(),
			scoreOptions{Pass: query.PassRigor, Backoff: []time.Duration{time.Millisecond}, providerOptions: scoreProviderAt(baseURL)},
			model.NewClient)
		return refusedRun{doc: got.doc, stdout: got.stdout, stderr: got.stderr}
	}},
}

// refusedRoot runs args through the real root command from a scratch
// directory under no repository, its flags put back afterwards.
func refusedRoot(t *testing.T, args ...string) refusedRun {
	t.Helper()
	t.Chdir(t.TempDir())
	stdout, stderr, err := executeRoot(t, args...)
	exitStatus(err, stdout, stderr)
	if sub, _, ferr := rootCmd.Find(args); ferr == nil {
		if err := mcp.ResetFlags(sub); err != nil {
			t.Fatal(err)
		}
	}
	return refusedRun{doc: checkDocument(t, args[0], stdout.String()), stdout: stdout.String(), stderr: stderr.String()}
}

// TestRefusedKeyFailureCarriesTheRemedy: a provider answering 401
// refused the call's key, a missing one or a wrong one; each verb fails as it
// does on any refusal, and its failure item carries the command that stores a
// key for the entry it dialled. A 500 is no refused key and carries none.
func TestRefusedKeyFailureCarriesTheRemedy(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusInternalServerError} {
		for _, verb := range refusedVerbs {
			t.Run(fmt.Sprintf("%s %d", verb.name, code), func(t *testing.T) {
				srv, _ := refusingProvider(t, code)
				got := verb.run(t, refusingConfig(t, srv.URL), srv.URL)
				want := ""
				if code != http.StatusInternalServerError {
					want = config.AuthAddRemedy(verb.entry)
				}
				// The messages name outcomes and remedies only: a detail could
				// carry the key were it not scrubbed.
				items := got.doc.items(toolresult.FailuresKey)
				if got.doc.Outcome != toolresult.Failed || len(items) != 1 || items[0]["remedy"] != nilIfEmpty(want) {
					t.Errorf("outcome %s, failure remedies %v, want failed with one item whose remedy is %q", got.doc.Outcome, remedies(items), want)
				}
				if len(items) == 1 && endsWithNote(items[0]) != (want != "") {
					t.Errorf("the failure's detail ends with the refused-key note: %v, want %v", endsWithNote(items[0]), want != "")
				}
			})
		}
	}
}

// endsWithNote reports whether item's detail ends with the refused-key note.
func endsWithNote(item map[string]any) bool {
	detail, _ := item["detail"].(string)
	return strings.HasSuffix(detail, "; "+config.RefusedKeyNote)
}

// nilIfEmpty is s as a decoded document holds it: an omitted key reads nil.
func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// remedies is each item's remedy, for a failure message that must not print
// an item's detail.
func remedies(items []map[string]any) []any {
	out := make([]any, len(items))
	for i, it := range items {
		out[i] = it["remedy"]
	}
	return out
}

// TestRefusedKeyFailureCarriesNoKey: the key a refused call carried — echoed
// back in the provider's error body — appears on neither stream of any verb
// the provider refused, its failure items included.
func TestRefusedKeyFailureCarriesNoKey(t *testing.T) {
	for _, verb := range refusedVerbs {
		t.Run(verb.name, func(t *testing.T) {
			srv, sawKey := refusingProvider(t, http.StatusUnauthorized)
			got := verb.run(t, refusingConfig(t, srv.URL), srv.URL)
			if !sawKey() {
				t.Fatal("the provider never received the key, so its absence proves nothing")
			}
			if strings.Contains(got.stdout, testAPIKey) || strings.Contains(got.stderr, testAPIKey) {
				t.Errorf("the key appears in the output: stdout %v, stderr %v",
					strings.Contains(got.stdout, testAPIKey), strings.Contains(got.stderr, testAPIKey))
			}
		})
	}
}

// TestScoreResumeKeepsTheRefusedKeyRemedy: a run whose call the provider
// refused for its key, resumed, closes on a failure item still carrying the
// remedy, its line read back from the ledger.
func TestScoreResumeKeepsTheRefusedKeyRemedy(t *testing.T) {
	srv, _ := refusingProvider(t, http.StatusUnauthorized)
	repo, state := scoreRepo(t), t.TempDir()
	opts := scoreOptions{Pass: query.PassRigor, providerOptions: scoreProviderAt(srv.URL)}
	first := scoreDrive(t, repo, state, opts, model.NewClient)
	run, _ := first.doc.Values["run"].(string)
	if first.doc.Outcome != toolresult.Failed || run == "" {
		t.Fatalf("the first run: outcome %s, run %q, want failed naming its run", first.doc.Outcome, run)
	}
	opts.Resume = run
	resumed := scoreDrive(t, repo, state, opts, model.NewClient)
	want := config.AuthAddRemedy("solo")
	if items := resumed.doc.items(toolresult.FailuresKey); resumed.doc.Outcome != toolresult.Failed || len(items) != 1 || items[0]["remedy"] != want {
		t.Errorf("the resumed run: outcome %s, failure remedies %v, want failed with one item whose remedy is %q", resumed.doc.Outcome, remedies(items), want)
	} else if !endsWithNote(items[0]) {
		t.Errorf("the resumed run's failure detail does not end with the refused-key note")
	}
}

// TestBuildRefusedKeyNamesTheRolesEntry: a build whose overview call the text
// role's entry refuses for its key fails with the remedy naming that entry —
// the one the role dials, not the decision role's — and the key sent appears
// on neither stream. The paper offers no decision ask, so the overview is the
// one call.
func TestBuildRefusedKeyNamesTheRolesEntry(t *testing.T) {
	repo, state := buildRepo(t)
	srv, sawKey := refusingProvider(t, http.StatusUnauthorized)
	cfgDir := refusingConfig(t, srv.URL)
	writeValues(t, cfgDir, config.ProvidersFileName, fmt.Sprintf("[lab]\nbaseUrl = %q\napiKeyFile = %q\n\n[cloud]\nbaseUrl = %q\n",
		srv.URL, filepath.Join(cfgDir, "key.txt"), srv.URL))
	writeValues(t, cfgDir, config.ConfigFileName, "[models]\ndecision = \"cloud/d3\"\ntext = \"lab/clef\"\n")

	saved := flagConfigDir
	t.Cleanup(func() { flagConfigDir = saved })
	flagConfigDir = cfgDir
	opts, err := loadVerbContext(repo)
	if err != nil {
		t.Fatalf("loading the staged configuration: %v", err)
	}
	var stdout, stderr bytes.Buffer
	opts.Stderr = &stderr
	if _, err := runBuild(context.Background(), buildOptions{
		VolumeRoots: []string{"paper.tex"}, StateDir: state, WorkDir: repo,
		ConfigDir: cfgDir, NewClient: model.NewClient, Provider: buildProvider(opts, &stderr),
		Stdout: &stdout, Stderr: &stderr, Logger: log.Discard(),
	}); err != nil {
		t.Fatalf("build: %v", err)
	}
	doc := checkDocument(t, "build", stdout.String())
	want := config.AuthAddRemedy("lab")
	if items := doc.items(toolresult.FailuresKey); doc.Outcome != toolresult.Failed || len(items) != 1 || items[0]["remedy"] != want {
		t.Errorf("outcome %s, failure remedies %v, want failed with one item whose remedy is %q", doc.Outcome, remedies(items), want)
	} else if !endsWithNote(items[0]) {
		t.Errorf("the build's failure detail does not end with the refused-key note")
	}
	if !sawKey() {
		t.Fatal("the provider never received the key, so its absence proves nothing")
	}
	if strings.Contains(stdout.String(), testAPIKey) || strings.Contains(stderr.String(), testAPIKey) {
		t.Errorf("the key appears in the output: stdout %v, stderr %v",
			strings.Contains(stdout.String(), testAPIKey), strings.Contains(stderr.String(), testAPIKey))
	}
}
