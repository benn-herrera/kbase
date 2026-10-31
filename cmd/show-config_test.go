package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"kbase/internal/auth"
	"kbase/internal/config"
	"kbase/internal/mcp"
	toolresult "kbase/internal/result"
)

// The key material the fixture configuration carries. No test lets it reach the
// document.
const fileKey = "sk-file-MUST-NOT-APPEAR"

// configFixture stages a configuration directory with both files: a pool whose
// entries cover both faults the report distinguishes — one that drops for want
// of a base URL, one supplying its key from a file any local account can read —
// and a config.toml naming the key-file entry in each role's reference.
func configFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeValues(t, dir, config.ProvidersFileName, `[broken]

[file]
baseUrl = "http://file.example/v1"
apiKeyFile = "key.txt"
`)
	keyFile := writeValues(t, dir, "key.txt", fileKey+"\n")
	if err := os.Chmod(keyFile, 0o644); err != nil {
		t.Fatal(err)
	}
	writeValues(t, dir, config.ConfigFileName, "[models]\ndecision = \"file/dec-1\"\ntext = \"file/text-1\"\n")
	return dir
}

// storedKey is the key the store fixture holds. No test lets it reach a stream.
const storedKey = "sk-stored-MUST-NOT-APPEAR"

// storeFixture points XDG_STATE_HOME at a fresh directory and stages a
// configuration over a store there: both, an entry with a stored key and an
// apiKeyFile; stored, one with a stored key alone; filed, one whose key file's
// path holds a space; and Truncated, one whose stored key file is truncated and
// whose apiKeyFile is missing. config.toml names stored, both and filed for the
// three roles. It returns the configuration directory, the path of filed's key
// file and the truncated store file.
func storeFixture(t *testing.T) (configDir, keyPath, truncated string) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	storeDir, err := authStoreDir()
	if err != nil {
		t.Fatal(err)
	}
	store := auth.NewStore(storeDir)
	for _, name := range []string{"both", "stored", "Truncated"} {
		if err := store.Add(name, storedKey); err != nil {
			t.Fatal(err)
		}
	}
	truncated = filepath.Join(storeDir, "truncated"+auth.FileSuffix)
	if err := os.WriteFile(truncated, []byte("x"), auth.FileMode); err != nil {
		t.Fatal(err)
	}

	configDir = t.TempDir()
	writeValues(t, configDir, config.ProvidersFileName, `[both]
baseUrl = "http://both.example/v1"
apiKeyFile = "both.key"

[stored]
baseUrl = "http://stored.example/v1"

[filed]
baseUrl = "http://filed.example/v1"
apiKeyFile = "my keys/filed.key"

[Truncated]
baseUrl = "http://truncated.example/v1"
apiKeyFile = "missing.key"
`)
	writeValues(t, configDir, config.ConfigFileName, "[models]\ndecision = \"stored/d\"\ntext = \"both/t\"\nscoring = \"filed/s\"\n")
	if err := os.Mkdir(filepath.Join(configDir, "my keys"), 0o700); err != nil {
		t.Fatal(err)
	}
	keyPath = filepath.Join(configDir, "my keys", "filed.key")
	if err := os.WriteFile(keyPath, []byte(fileKey+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return configDir, keyPath, truncated
}

// TestShowConfigReportsTheStore: an entry's key is reported by its kind — store
// over a stated apiKeyFile, which a warning says goes unread, with remedies
// naming the provider; file with its upgrade command, the path quoted — and a
// truncated store file is a diagnostic by its path, its remedy adding the key
// again, beside the read fault of the missing key file it fell through to,
// whose remedy stores a key too, with the outcome still done. Each role names the store as its key's source
// where the store supplied it.
func TestShowConfigReportsTheStore(t *testing.T) {
	configDir, keyPath, truncated := storeFixture(t)
	doc, stdout, _, code := runShowConfigVerb(t, t.TempDir(), "--config-dir", configDir)
	if code != 0 || doc.Outcome != toolresult.Done {
		t.Fatalf("outcome %s, exit %d; want done, 0\n%s", doc.Outcome, code, stdout)
	}
	entries := map[string]map[string]any{}
	for _, p := range doc.items("providers") {
		name, _ := p["name"].(string)
		entries[name] = p
	}
	for _, name := range []string{"both", "stored"} {
		e := entries[name]
		_, keyFile := e["key-file"]
		_, upgrade := e["upgrade"]
		if e["key"] != string(config.KeyKindStore) || keyFile || upgrade {
			t.Errorf("%s = %v, want key store with no key-file and no upgrade", name, e)
		}
	}
	if filed, want := entries["filed"], "cat '"+keyPath+"' | kbase auth add filed"; filed["key"] != string(config.KeyKindFile) ||
		filed["key-file"] != "my keys/filed.key" || filed["upgrade"] != want {
		t.Errorf("filed = %v, want key file, its key-file and upgrade %q", filed, want)
	}
	if e := entries["Truncated"]; e["usable"] != false {
		t.Errorf("Truncated = %v, want it dropped: its store file does not open and its key file is missing", e)
	}

	providersFile := config.ProvidersPath(configDir)
	diagnostics := doc.items("diagnostics")
	if len(diagnostics) != 3 {
		t.Fatalf("diagnostics = %v, want Truncated's two faults and both's unread key file", diagnostics)
	}
	if d := diagnostics[0]; d["path"] != providersFile || d["remedy"] != config.AuthAddRemedy("Truncated") {
		t.Errorf("diagnostic 0 = %v, want the missing key file's read fault against %s, its remedy storing a key for Truncated", d, providersFile)
	}
	if d := diagnostics[1]; d["path"] != truncated || d["detail"] != auth.ErrTruncated.Error() || d["remedy"] != "<command-that-prints-the-key> | kbase auth add Truncated" {
		t.Errorf("diagnostic 1 = %v, want the truncated store file, what is wrong with it, and the piped kbase auth add Truncated", d)
	}
	if d := diagnostics[2]; d["path"] != providersFile || !strings.Contains(d["detail"].(string), `provider "both"`) ||
		!strings.Contains(d["remedy"].(string), "kbase auth remove both") {
		t.Errorf("diagnostic 2 = %v, want both's unread apiKeyFile against %s, kbase auth remove both in its remedy", d, providersFile)
	}

	for role, want := range map[string]string{"decision": string(config.KeyKindStore), "text": string(config.KeyKindStore), "scoring": config.LayerSourceExplicit} {
		sources, _ := roleOf(doc, role)["sources"].(map[string]any)
		if sources["key"] != want {
			t.Errorf("%s sources.key = %v, want %s", role, sources["key"], want)
		}
	}
}

// TestStoredKeyNeverReported: show-config over a store holding keys reports
// where each came from, and neither stream carries a key.
func TestStoredKeyNeverReported(t *testing.T) {
	configDir, _, _ := storeFixture(t)
	_, stdout, stderr, _ := runShowConfigVerb(t, t.TempDir(), "--config-dir", configDir)
	for stream, text := range map[string]string{"stdout": stdout, "stderr": stderr} {
		if strings.Contains(text, storedKey) || strings.Contains(text, fileKey) {
			t.Errorf("%s (%d bytes) carries a key", stream, len(text))
		}
	}
}

// TestStoreReportIsShowConfigsAlone: a verb loading configuration warns, on
// stderr, of a store file that did not open, by its path and what is wrong with
// it — and never of the upgrade or the unread key file, which show-config's
// document alone carries.
func TestStoreReportIsShowConfigsAlone(t *testing.T) {
	configDir, _, truncated := storeFixture(t)
	t.Setenv(config.EnvConfigDir, configDir)
	opts, err := loadVerbContext(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var stderr strings.Builder
	warnFaults(&stderr, opts.Faults)
	got := stderr.String()
	if want := `provider "Truncated": ` + truncated + ": " + auth.ErrTruncated.Error(); !strings.Contains(got, want) {
		t.Errorf("stderr = %q, want the line %q", got, want)
	}
	for _, reported := range []string{"| kbase auth add", "kbase auth remove", "not read"} {
		if strings.Contains(got, reported) {
			t.Errorf("stderr = %q carries %q, which only show-config reports", got, reported)
		}
	}
	if strings.Contains(got, storedKey) || strings.Contains(got, fileKey) {
		t.Errorf("stderr (%d bytes) carries a key", len(got))
	}
}

// TestUnlistableStoreIsADiagnostic: a store directory that stands and does not
// list fails no load. Its entry falls back to the key file it states;
// show-config reports the directory as a diagnostic with what went wrong, no
// remedy and no path repeated in the detail; and a verb loading configuration
// warns of it on stderr as the credentials store's.
func TestUnlistableStoreIsADiagnostic(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a mode-000 directory still lists on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root lists a mode-000 directory")
	}
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	storeDir, err := authStoreDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(storeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(storeDir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(storeDir, 0o700); err != nil {
			t.Error(err)
		}
	})
	_, listErr := os.ReadDir(storeDir)
	var pathErr *fs.PathError
	if !errors.As(listErr, &pathErr) {
		t.Fatalf("listing the mode-000 store = %v, want a path error", listErr)
	}
	wantDetail := pathErr.Err.Error()

	configDir := t.TempDir()
	writeValues(t, configDir, config.ProvidersFileName, "[filed]\nbaseUrl = \"http://filed.example/v1\"\napiKeyFile = \"filed.key\"\n")
	if err := os.WriteFile(filepath.Join(configDir, "filed.key"), []byte(fileKey+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	doc, stdout, _, code := runShowConfigVerb(t, t.TempDir(), "--config-dir", configDir)
	if code != 0 || doc.Outcome != toolresult.Done {
		t.Fatalf("outcome %s, exit %d; want done, 0\n%s", doc.Outcome, code, stdout)
	}
	if providers := doc.items("providers"); len(providers) != 1 || providers[0]["key"] != string(config.KeyKindFile) {
		t.Errorf("providers = %v, want filed with its file key", providers)
	}
	// config.toml names no role, so each role's diagnostic follows the store's.
	diagnostics := doc.items("diagnostics")
	if len(diagnostics) == 0 {
		t.Fatal("no diagnostics, want the store directory's first")
	}
	if d := diagnostics[0]; d["path"] != storeDir || d["detail"] != wantDetail || d["remedy"] != nil {
		t.Errorf("diagnostic 0 = %v, want path %s, detail %q and no remedy", d, storeDir, wantDetail)
	}

	t.Setenv(config.EnvConfigDir, configDir)
	opts, err := loadVerbContext(t.TempDir())
	if err != nil {
		t.Fatalf("loadVerbContext = %v, want the load to go on past the store", err)
	}
	var stderr strings.Builder
	warnFaults(&stderr, opts.Faults)
	if got, want := stderr.String(), "credentials store: "+storeDir+": "+wantDetail+"\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

// runShowConfigVerb runs `kbase show-config` through the real root command from
// work and reads the document it wrote, with the verb's stderr.
func runShowConfigVerb(t *testing.T, work string, args ...string) (doc resultDoc, stdout, stderr string, code int) {
	t.Helper()
	t.Chdir(work)
	stdoutBuf, stderrBuf, err := executeRoot(t, append([]string{"show-config"}, args...)...)
	code = exitStatus(err, stdoutBuf, stderrBuf)
	if sub, _, ferr := rootCmd.Find(args); ferr == nil {
		if err := mcp.ResetFlags(sub); err != nil {
			t.Fatal(err)
		}
	}
	stdout, stderr = stdoutBuf.String(), stderrBuf.String()
	return checkDocument(t, "show-config", stdout), stdout, stderr, code
}

// reportedLayers is the report's layers, in precedence order, each as the
// mapping the document carries.
func reportedLayers(t *testing.T, doc resultDoc) []map[string]any {
	t.Helper()
	layers := doc.items("layers")
	if len(layers) == 0 {
		t.Fatalf("the document reports no layers: %v", doc.Values)
	}
	return layers
}

// TestShowConfigReportsTheConfiguration: the document names the layer the flag
// chose with what it holds, every pool entry with the layer it came from, the
// kind of key it supplies and why it faulted, the merged role values, the
// selection, one record per role with its endpoint and each field's
// source, and one diagnostic per fault and per unresolved role — and no key
// material.
func TestShowConfigReportsTheConfiguration(t *testing.T) {
	dir := configFixture(t)
	doc, stdout, _, code := runShowConfigVerb(t, t.TempDir(), "--config-dir", dir)
	if code != 0 || doc.Outcome != toolresult.Done {
		t.Fatalf("outcome %s, exit %d; want done, 0\n%s", doc.Outcome, code, stdout)
	}
	layers := reportedLayers(t, doc)
	if len(layers) != 1 {
		t.Fatalf("layers = %v, want the one directory the flag named", layers)
	}
	if want := (map[string]any{"path": dir, "source": config.LayerSourceExplicit, "exists": true,
		"providersExists": true, "configExists": true}); !reflect.DeepEqual(layers[0], want) {
		t.Errorf("layers[0] = %v, want %v", layers[0], want)
	}

	var names []string
	entries := map[string]map[string]any{}
	for _, p := range doc.items("providers") {
		name, _ := p["name"].(string)
		names = append(names, name)
		entries[name] = p
	}
	if !slices.Equal(names, []string{"broken", "file"}) {
		t.Errorf("providers %v, want every entry in name order", names)
	}
	if broken := entries["broken"]; broken["usable"] != false || broken["fault"] == nil ||
		broken["layer"] != config.LayerSourceExplicit {
		t.Errorf("the dropped entry = %v, want usable false, the reason it dropped and its layer", broken)
	} else if _, ok := broken["base-url"]; ok {
		t.Errorf("the dropped entry carries base-url: %v", broken)
	} else if _, ok := broken["key"]; ok {
		t.Errorf("the dropped entry carries key: %v", broken)
	}
	if file := entries["file"]; file["key"] != string(config.KeyKindFile) || file["key-file"] != "key.txt" ||
		file["usable"] != true || file["fault"] == nil || file["layer"] != config.LayerSourceExplicit {
		t.Errorf("the key-file entry = %v, want key file, its path, usable, the permission warning and its layer", file)
	}

	cfg, _ := doc.Values["config"].(map[string]any)
	models, _ := cfg["models"].(map[string]any)
	if models[config.CategoryDecision] != "file/dec-1" || models[config.CategoryText] != "file/text-1" ||
		models[config.CategoryScoring] != nil {
		t.Errorf("config = %v, want each role's value as written and null for the unset one", cfg)
	}
	selection, _ := doc.Values["selection"].(map[string]any)
	if selection["provider"] != "file" || selection["rule"] != config.RuleSoleEntry {
		t.Errorf("selection = %v, want the pool's only usable entry — config.toml names none", selection)
	}
	for _, want := range []struct{ role, model, provider string }{
		{"decision", "dec-1", "file"}, {"text", "text-1", "file"},
	} {
		got := roleOf(doc, want.role)
		if got == nil {
			t.Fatalf("no %s record in %v", want.role, doc.items("roles"))
		}
		if got["provider"] != want.provider || got["provider-rule"] != config.RuleConfig ||
			got["provider-layer"] != config.LayerSourceExplicit || got["model"] != want.model ||
			got["base-url"] != "http://file.example/v1" {
			t.Errorf("%s = %v, want the entry its reference names", want.role, got)
		}
		sources, _ := got["sources"].(map[string]any)
		if sources["base-url"] != config.LayerSourceExplicit || sources["model"] != config.LayerSourceExplicit ||
			sources["key"] != config.LayerSourceExplicit {
			t.Errorf("%s sources = %v, want the one layer read for every field", want.role, sources)
		}
	}
	// The unset role resolves nothing, and says what it lacks.
	scoring := roleOf(doc, "scoring")
	if scoring["detail"] == nil || scoring["provider"] != nil {
		t.Errorf("the scoring role = %v, want a detail and no provider: nothing answered it", scoring)
	}

	diagnostics := doc.items("diagnostics")
	if len(diagnostics) != 3 {
		t.Fatalf("diagnostics = %v, want one per fault and one for the unset role", diagnostics)
	}
	for _, item := range diagnostics[:2] {
		if item["check"] != config.CheckProvider || item["path"] != config.ProvidersPath(dir) || item["detail"] == "" {
			t.Errorf("diagnostic %v, want check %s, path the pool and the reason it gives", item, config.CheckProvider)
		}
	}
	if last := diagnostics[2]; last["check"] != config.CheckProvider || last["key"] != "models.scoring" || last["path"] != config.ConfigPath(dir) {
		t.Errorf("the role's diagnostic = %v, want models.scoring in the config file", last)
	}
	if strings.Contains(stdout, fileKey) {
		t.Errorf("the document (%d bytes) carries the key", len(stdout))
	}
}

// TestShowConfigReportsAnAbsentConfiguration: with no configuration directory
// at all the verb still reports — done, the one layer it read present and
// empty, nothing selected, no role resolved and one diagnostic per role — and
// it creates no directory.
func TestShowConfigReportsAnAbsentConfiguration(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "absent")
	t.Setenv(config.EnvConfigDir, dir)
	doc, stdout, _, code := runShowConfigVerb(t, t.TempDir())
	if code != 0 || doc.Outcome != toolresult.Done {
		t.Fatalf("outcome %s, exit %d; want done, 0\n%s", doc.Outcome, code, stdout)
	}
	layers := reportedLayers(t, doc)
	if want := (map[string]any{"path": dir, "source": config.LayerSourceEnvironment,
		"exists": false, "providersExists": false, "configExists": false}); len(layers) != 1 || !reflect.DeepEqual(layers[0], want) {
		t.Errorf("layers = %v, want %v", layers, want)
	}
	if providers, _ := doc.Values["providers"].([]any); len(providers) != 0 {
		t.Errorf("providers = %v, want none", doc.Values["providers"])
	}
	if selection, _ := doc.Values["selection"].(map[string]any); selection["provider"] != nil || selection["detail"] == nil {
		t.Errorf("selection = %v, want the selection's own reason and no provider", selection)
	}
	for _, role := range config.Roles {
		if got := roleOf(doc, string(role)); got == nil || got["detail"] == nil {
			t.Errorf("role %s = %v, want it listed with what it lacks", role, got)
		}
	}
	if diagnostics := doc.items("diagnostics"); len(diagnostics) != len(config.Roles) {
		t.Fatalf("diagnostics = %v, want one per role", diagnostics)
	}
	if _, err := os.Stat(dir); err == nil {
		t.Error("the verb created the configuration directory")
	}
}

// TestShowConfigNamesTheConfigFileALackBelongsTo: a role no layer sets is
// diagnosed against the config.toml of the layer this invocation writes into,
// by its path: a repository layer and a global one can each hold a config.toml,
// so the bare name would not say which.
func TestShowConfigNamesTheConfigFileALackBelongsTo(t *testing.T) {
	dir := t.TempDir()
	writeValues(t, dir, config.ProvidersFileName, "[solo]\nbaseUrl = \"http://solo.example/v1\"\n")
	doc, stdout, _, code := runShowConfigVerb(t, t.TempDir(), "--config-dir", dir)
	if code != 0 || doc.Outcome != toolresult.Done {
		t.Fatalf("outcome %s, exit %d; want done, 0\n%s", doc.Outcome, code, stdout)
	}
	diagnostics := doc.items("diagnostics")
	want := config.ConfigPath(dir)
	if len(diagnostics) != len(config.Roles) {
		t.Fatalf("diagnostics = %v, want one per role", diagnostics)
	}
	for i, role := range config.Roles {
		if diagnostics[i]["key"] != string(role) && diagnostics[i]["key"] != "models."+string(role) {
			t.Errorf("diagnostic %d = %v, want models.%s", i, diagnostics[i], role)
		}
		if diagnostics[i]["path"] != want {
			t.Errorf("diagnostic %d names %v, want %s", i, diagnostics[i]["path"], want)
		}
	}
}

// TestShowConfigReportsARolesProviderThePoolLacks: a role whose reference names
// an entry the pool does not hold is a provider item by that role's key, with
// the pool's entry names as the values that would work and no creation remedy,
// the providers.toml standing — the shape a run refuses with, reported here
// because this verb never refuses.
func TestShowConfigReportsARolesProviderThePoolLacks(t *testing.T) {
	dir := t.TempDir()
	writeValues(t, dir, config.ProvidersFileName, "[lab]\nbaseUrl = \"http://lab.example/v1\"\n")
	writeValues(t, dir, config.ConfigFileName, "[models]\ndecision = \"lab/d\"\ntext = \"lab/t\"\nscoring = \"elsewhere/clef\"\n")
	doc, stdout, _, code := runShowConfigVerb(t, t.TempDir(), "--config-dir", dir)
	if code != 0 || doc.Outcome != toolresult.Done {
		t.Fatalf("outcome %s, exit %d; want done, 0\n%s", doc.Outcome, code, stdout)
	}
	var scoring map[string]any
	for _, item := range doc.items("diagnostics") {
		if item["key"] == "models.scoring" {
			scoring = item
		}
	}
	if scoring == nil {
		t.Fatalf("no item names models.scoring: %v", doc.items("diagnostics"))
	}
	if scoring["check"] != config.CheckProvider || !slices.Equal(anyStrings(scoring["allowed"]), []string{"lab"}) ||
		!strings.Contains(scoring["detail"].(string), `provider "elsewhere"`) || scoring["remedy"] != nil {
		t.Errorf("the item = %v, want check %s, the pool's entries allowed, the provider named, and no remedy: providers.toml stands", scoring, config.CheckProvider)
	}
	if role := roleOf(doc, "scoring"); role["detail"] == nil || role["provider"] != nil {
		t.Errorf("the scoring role = %v, want a detail and no provider", role)
	}
}

// TestShowConfigRefusesAConfigurationThatDoesNotLoad: a file failing the strict
// load refuses exactly as it does for every other subcommand, and a refused
// document carries no diagnostics.
func TestShowConfigRefusesAConfigurationThatDoesNotLoad(t *testing.T) {
	dir := t.TempDir()
	writeValues(t, dir, config.ConfigFileName, "provider = \"a\"\n\n[models]\nheavy = \"some-model\"\n")
	doc, stdout, _, code := runShowConfigVerb(t, t.TempDir(), "--config-dir", dir)
	if code != toolresult.ExitCode(toolresult.Refused) || doc.Outcome != toolresult.Refused {
		t.Fatalf("outcome %s, exit %d; want refused, %d\n%s", doc.Outcome, code, toolresult.ExitCode(toolresult.Refused), stdout)
	}
	if _, ok := doc.Values["diagnostics"]; ok {
		t.Errorf("a refused document carries diagnostics:\n%s", stdout)
	}
	items := doc.items(toolresult.RefusalsKey)
	if len(items) != 1 || items[0]["check"] != toolresult.CheckUsage {
		t.Fatalf("refusals = %v, want the load's one usage item", items)
	}
	detail, _ := items[0]["detail"].(string)
	for _, key := range []string{`"provider"`, `"models.heavy"`} {
		if !strings.Contains(detail, key) {
			t.Errorf("detail = %q, want it to name %s", detail, key)
		}
	}
}

// TestShowConfigReportsTheDecisionsFlavor: every usable entry states which
// decision surface it speaks — the systemone default where the entry is
// silent, the value where it names one — and a dropped provider, which has no
// entry to describe, states none.
func TestShowConfigReportsTheDecisionsFlavor(t *testing.T) {
	dir := t.TempDir()
	writeValues(t, dir, config.ProvidersFileName, `[silent]
baseUrl = "http://silent.example/v1"

[deciding]
baseUrl = "http://deciding.example/v1"
decisionsFlavor = "decisions"

[broken]
`)
	writeValues(t, dir, config.ConfigFileName, "[models]\ndecision = \"deciding/dec-1\"\ntext = \"deciding/text-1\"\nscoring = \"silent/score-1\"\n")

	doc, stdout, _, code := runShowConfigVerb(t, t.TempDir(), "--config-dir", dir)
	if code != 0 || doc.Outcome != toolresult.Done {
		t.Fatalf("outcome %s, exit %d; want done, 0\n%s", doc.Outcome, code, stdout)
	}
	entries := map[string]map[string]any{}
	for _, p := range doc.items("providers") {
		name, _ := p["name"].(string)
		entries[name] = p
	}
	if got := entries["silent"]["decisions-flavor"]; got != config.FlavorSystemOne {
		t.Errorf("an entry silent on the flavour reports %v, want %s", got, config.FlavorSystemOne)
	}
	if got := entries["deciding"]["decisions-flavor"]; got != config.FlavorDecisions {
		t.Errorf("an entry naming decisions reports %v, want %s", got, config.FlavorDecisions)
	}
	if got, ok := entries["broken"]["decisions-flavor"]; ok {
		t.Errorf("the dropped entry states a decision surface (%v): %v", got, entries["broken"])
	}
}

// TestShowConfigReportsTheProtocol: every usable entry states the chat protocol
// it resolves to — `anthropic` where the entry is silent, `openai` where it names
// the OpenAI-compatible protocol — and a dropped provider, which has no entry
// to describe, states none.
func TestShowConfigReportsTheProtocol(t *testing.T) {
	dir := t.TempDir()
	writeValues(t, dir, config.ProvidersFileName, `[silent]
baseUrl = "http://silent.example/v1"

[compat]
baseUrl = "http://compat.example/v1"
api = "openai"

[broken]
`)
	writeValues(t, dir, config.ConfigFileName, "[models]\ndecision = \"silent/dec-1\"\ntext = \"compat/text-1\"\nscoring = \"silent/score-1\"\n")

	doc, stdout, _, code := runShowConfigVerb(t, t.TempDir(), "--config-dir", dir)
	if code != 0 || doc.Outcome != toolresult.Done {
		t.Fatalf("outcome %s, exit %d; want done, 0\n%s", doc.Outcome, code, stdout)
	}
	entries := map[string]map[string]any{}
	for _, p := range doc.items("providers") {
		name, _ := p["name"].(string)
		entries[name] = p
	}
	if got := entries["silent"]["api"]; got != config.ProviderAPIAnthropic {
		t.Errorf("an entry silent on the protocol reports %v, want %s", got, config.ProviderAPIAnthropic)
	}
	if got := entries["compat"]["api"]; got != config.ProviderAPIOpenAI {
		t.Errorf("an entry naming openai reports %v, want %s", got, config.ProviderAPIOpenAI)
	}
	if got, ok := entries["broken"]["api"]; ok {
		t.Errorf("the dropped entry states a protocol (%v): %v", got, entries["broken"])
	}
}

// TestShowConfigReportsTheSamplingAndItsLayers: a repository layer stating a
// thinking level over a global layer stating a temperature reports the merged
// values under config.sampling, each chat role's own values with the layer that
// supplied each, null for the role no layer states, and none for the decision
// role, which is not asked with sampling.
func TestShowConfigReportsTheSamplingAndItsLayers(t *testing.T) {
	home := t.TempDir()
	global := filepath.Join(home, ".config", "kbase")
	repo := t.TempDir()
	for _, dir := range []string{global, filepath.Join(repo, config.RepositoryDirName), filepath.Join(repo, ".git")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeValues(t, global, config.ProvidersFileName, "[local]\nbaseUrl = \"http://local.example/v1\"\n")
	writeValues(t, global, config.ConfigFileName,
		"[models]\ndecision = \"local/dec-1\"\ntext = \"local/text-1\"\nscoring = \"local/score-1\"\n\n[sampling.text]\ntemperature = 0.3\n")
	writeValues(t, filepath.Join(repo, config.RepositoryDirName), config.ConfigFileName, "[sampling.text]\nthinking = \"low\"\n")
	t.Setenv("HOME", home)

	doc, stdout, _, code := runShowConfigVerb(t, repo)
	if code != 0 || doc.Outcome != toolresult.Done {
		t.Fatalf("outcome %s, exit %d; want done, 0\n%s", doc.Outcome, code, stdout)
	}
	cfg, _ := doc.Values["config"].(map[string]any)
	sampling, _ := cfg["sampling"].(map[string]any)
	if want := (map[string]any{"temperature": 0.3, "thinking": "low"}); !reflect.DeepEqual(sampling["text"], want) {
		t.Errorf("config.sampling.text = %v, want %v", sampling["text"], want)
	}
	if want := (map[string]any{"temperature": nil, "thinking": nil}); !reflect.DeepEqual(sampling["scoring"], want) {
		t.Errorf("config.sampling.scoring = %v, want %v", sampling["scoring"], want)
	}

	text := roleOf(doc, "text")
	sources, _ := text["sources"].(map[string]any)
	if text["temperature"] != 0.3 || text["thinking"] != "low" ||
		sources["temperature"] != config.LayerSourceGlobal || sources["thinking"] != config.LayerSourceRepository {
		t.Errorf("text = %v, want both values and the layers global and repository", text)
	}
	scoring := roleOf(doc, "scoring")
	sources, _ = scoring["sources"].(map[string]any)
	for _, k := range []string{"temperature", "thinking"} {
		if v, ok := scoring[k]; !ok || v != nil {
			t.Errorf("scoring %s = %v (present %t), want null", k, v, ok)
		}
		if v, ok := sources[k]; !ok || v != nil {
			t.Errorf("scoring sources %s = %v (present %t), want null", k, v, ok)
		}
	}
	decision := roleOf(doc, "decision")
	sources, _ = decision["sources"].(map[string]any)
	for _, k := range []string{"temperature", "thinking"} {
		if _, ok := decision[k]; ok {
			t.Errorf("decision carries %s: %v", k, decision)
		}
		if _, ok := sources[k]; ok {
			t.Errorf("decision sources carries %s: %v", k, sources)
		}
	}
}
