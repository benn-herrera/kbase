package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"kbase/internal/auth"
	"kbase/internal/config"
	"kbase/internal/log"
	toolresult "kbase/internal/result"
)

// authSentinelKey is a key no test output may carry.
const authSentinelKey = "sk-auth-sentinel-0d6f3c2a9b"

// runAuth runs verb over a store in dir, as the CLI runs it, and returns its
// stdout.
func runAuth(t *testing.T, name string, verb func(authOptions) (string, []toolresult.Field), opts authOptions) string {
	t.Helper()
	var stdout bytes.Buffer
	opts.Stdout = &stdout
	if opts.Logger == nil {
		opts.Logger = log.Discard()
	}
	if opts.Stdin == nil {
		opts.Stdin = strings.NewReader("")
	}
	if _, err := runAuthVerb(name, verb, opts); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return stdout.String()
}

// storeFiles is every file under dir, none where dir does not stand.
func storeFiles(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return files
}

// TestAuthAddRefusesBeforeWriting: each refusal of add fires in its order —
// the name rule, the configured check, a terminal, empty input — and leaves
// the store empty; the first two fire before stdin is read.
func TestAuthAddRefusesBeforeWriting(t *testing.T) {
	for _, tc := range []struct {
		name       string
		provider   string
		configured []string
		stdin      string
		terminal   bool
		key        any
		remedy     string
		unread     bool
	}{
		{"an unusable name", "a/b", []string{"a/b"}, authSentinelKey, true, "a/b", authPipedForm(authProviderArg), true},
		{"an unconfigured name", "ghost", []string{"p"}, authSentinelKey, true, "ghost", "kbase show-config", true},
		{"a terminal", "p", []string{"p"}, authSentinelKey, true, nil, authPipedForm("p"), true},
		{"empty input", "p", []string{"p"}, "", false, nil, authPipedForm("p"), false},
		{"whitespace alone", "p", []string{"p"}, " \n\t \r\n", false, nil, authPipedForm("p"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "auth")
			stdin := strings.NewReader(tc.stdin)
			out := runAuth(t, "auth add", authAdd, authOptions{Provider: tc.provider, Dir: dir, Configured: tc.configured,
				Stdin: stdin, StdinIsTerminal: tc.terminal})
			d := checkDocument(t, "auth add", out)
			items := d.items(toolresult.RefusalsKey)
			if d.Outcome != toolresult.Refused || len(items) != 1 {
				t.Fatalf("outcome %s with %d refusals, want one refusal", d.Outcome, len(items))
			}
			if items[0]["key"] != tc.key || items[0]["remedy"] != tc.remedy {
				t.Errorf("key %v remedy %v, want key %v remedy %q", items[0]["key"], items[0]["remedy"], tc.key, tc.remedy)
			}
			if tc.unread && stdin.Len() != len(tc.stdin) {
				t.Error("the refusal read stdin")
			}
			if files := storeFiles(t, dir); len(files) != 0 {
				t.Errorf("a refused add wrote %v", files)
			}
		})
	}
}

func TestAuthAddListRemoveRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "auth")
	for _, p := range []string{"zeta", "alpha"} {
		d := checkDocument(t, "auth add", runAuth(t, "auth add", authAdd, authOptions{Provider: p, Dir: dir,
			Configured: []string{"alpha", "zeta"}, Stdin: strings.NewReader("\n" + authSentinelKey + p + "\n")}))
		if d.Outcome != toolresult.Done || d.Values["provider"] != p {
			t.Fatalf("add %s: outcome %s provider %v", p, d.Outcome, d.Values["provider"])
		}
	}
	got, err := auth.NewStore(dir).Read("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if got != authSentinelKey+"alpha" {
		t.Errorf("the stored key does not match the piped one trimmed: length %d, want %d", len(got), len(authSentinelKey+"alpha"))
	}
	list := func() []any {
		d := checkDocument(t, "auth list", runAuth(t, "auth list", authList, authOptions{Dir: dir}))
		names, _ := d.Values["providers"].([]any)
		return names
	}
	if names := list(); !slices.Equal(names, []any{"alpha", "zeta"}) {
		t.Errorf("list %v, want [alpha zeta]", names)
	}
	for _, want := range []string{toolresult.Done, toolresult.Unchanged} {
		d := checkDocument(t, "auth remove", runAuth(t, "auth remove", authRemove, authOptions{Provider: "alpha", Dir: dir}))
		if d.Outcome != want {
			t.Errorf("remove alpha: outcome %s, want %s", d.Outcome, want)
		}
	}
	if names := list(); !slices.Equal(names, []any{"zeta"}) {
		t.Errorf("list after remove %v, want [zeta]", names)
	}
}

// TestAuthKeyAppearsNowhere: a key piped to add, with debug logging teed to a
// file, appears on neither stream, in the log, nor in any file of the store.
func TestAuthKeyAppearsNowhere(t *testing.T) {
	tmp := t.TempDir()
	dir, logPath := filepath.Join(tmp, "auth"), filepath.Join(tmp, "debug.log")
	var stderr bytes.Buffer
	logger, closer, err := log.New(log.Options{Level: log.LevelDebug, Console: &stderr, FilePath: logPath})
	if err != nil {
		t.Fatal(err)
	}
	opts := authOptions{Provider: "p", Dir: dir, Configured: []string{"p"}, Logger: logger}
	var stdout string
	for _, v := range []struct {
		name string
		verb func(authOptions) (string, []toolresult.Field)
	}{{"auth add", authAdd}, {"auth list", authList}, {"auth remove", authRemove}} {
		opts.Stdin = strings.NewReader(authSentinelKey)
		stdout += runAuth(t, v.name, v.verb, opts)
		if v.name == "auth add" && len(storeFiles(t, dir)) == 0 {
			t.Fatal("add stored no file, so the store's files prove nothing")
		}
		for _, f := range storeFiles(t, dir) {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(b, []byte(authSentinelKey)) {
				t.Errorf("after %s, store file %s holds the key", v.name, f)
			}
		}
	}
	if err := closer.Close(); err != nil {
		t.Fatal(err)
	}
	logged, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(logged) == 0 || stderr.Len() == 0 {
		t.Fatal("nothing was logged at debug, so the log proves nothing")
	}
	for name, text := range map[string]string{"stdout": stdout, "stderr": stderr.String(), "the log file": string(logged)} {
		if strings.Contains(text, authSentinelKey) {
			t.Errorf("%s holds the key", name)
		}
	}
}

// executeAuth runs the real root over args with stdin, and renders what it
// returns as main does: the document on stdout, the error on stderr.
func executeAuth(t *testing.T, stdin string, args ...string) (stdout, stderr string) {
	t.Helper()
	rootCmd.SetIn(strings.NewReader(stdin))
	t.Cleanup(func() { rootCmd.SetIn(nil) })
	out, errOut, err := executeRoot(t, args...)
	exitStatus(err, out, errOut)
	return out.String(), errOut.String()
}

// TestAuthUsageRefusalEchoesNoArgument: an argument-count refusal is fixed
// text, so a key passed as an extra argument is refused and neither stream
// repeats it.
func TestAuthUsageRefusalEchoesNoArgument(t *testing.T) {
	for _, args := range [][]string{
		{"auth", "add", "p", authSentinelKey},
		{"auth", "remove", "p", authSentinelKey},
		{"auth", "list", authSentinelKey},
		{"auth", authSentinelKey},
	} {
		verb := strings.Join(args[:len(args)-1], " ")
		t.Run(verb, func(t *testing.T) {
			stdout, stderr := executeAuth(t, "", args...)
			if strings.Contains(stdout, authSentinelKey) || strings.Contains(stderr, authSentinelKey) {
				t.Fatal("the refusal repeats the argument")
			}
			d := parseResult(t, stdout)
			items := d.items(toolresult.RefusalsKey)
			if d.Outcome != toolresult.Refused || len(items) != 1 || items[0]["check"] != toolresult.CheckUsage {
				t.Errorf("outcome %s, refusals %v; want one usage refusal", d.Outcome, items)
			}
		})
	}
}

// TestNoSurfaceNamesACredential: no flag of any command, at any depth, and no
// tool property or operation argument is named for a key or a credential, so
// nothing takes a key on argv or through a tool.
func TestNoSurfaceNamesACredential(t *testing.T) {
	named := func(name string) bool {
		name = strings.ToLower(name)
		return slices.ContainsFunc([]string{"key", "credential", "secret", "token", "password"}, func(w string) bool {
			return strings.Contains(name, w)
		})
	}
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		for _, fs := range []*pflag.FlagSet{c.Flags(), c.PersistentFlags()} {
			fs.VisitAll(func(f *pflag.Flag) {
				if named(f.Name) {
					t.Errorf("%s: flag --%s names a credential", c.CommandPath(), f.Name)
				}
			})
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(rootCmd)
	srv, _ := mcpServer(t)
	for _, tool := range toolsListedBy(t, srv) {
		if named(tool.Name) {
			t.Errorf("tool %s names a credential", tool.Name)
		}
		for p := range tool.InputSchema.Properties {
			if named(p) {
				t.Errorf("%s: tool property %s names a credential", tool.Name, p)
			}
		}
		operations, _ := tool.InputSchema.Properties["operation"]["enum"].([]any)
		for _, o := range operations {
			op, _ := o.(string)
			arguments, _ := operationArguments(t, srv, op)
			for _, a := range arguments {
				// insert-work-entry's key is the work's citation key, which
				// mints its id.
				if named(a) && (op != "insert-work-entry" || a != "key") {
					t.Errorf("%s: %s's argument %s names a credential", tool.Name, op, a)
				}
			}
		}
	}
}

// TestStoreLocationIsScratch: the store the CLI's add writes into lies under
// the scratch state home TestMain set, never the user's.
func TestStoreLocationIsScratch(t *testing.T) {
	const provider = "store-location-probe"
	if scratchStateHome == "" {
		t.Fatal("TestMain set no scratch state home")
	}
	dir, err := authStoreDir()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(dir, scratchStateHome+string(filepath.Separator)) {
		t.Fatalf("the store resolves to %s, outside the scratch state home %s", dir, scratchStateHome)
	}
	store := auth.NewStore(dir)
	t.Cleanup(func() {
		if _, err := store.Remove(provider); err != nil {
			t.Error(err)
		}
	})
	stdout, _ := executeAuth(t, authSentinelKey, "auth", "add", provider, "--config-dir", authConfigDir(t, "["+provider+"]\n"))
	if !strings.Contains(stdout, `outcome: "done"`) {
		t.Fatal("auth add through the CLI did not report done")
	}
	names, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(names, provider) {
		t.Errorf("the CLI's add stored nothing under %s: %v", dir, names)
	}
}

// authConfigDir is a configuration directory whose providers.toml holds
// providers, so a test reads no real home's configuration.
func authConfigDir(t *testing.T, providers string) string {
	t.Helper()
	dir := t.TempDir()
	writeValues(t, dir, config.ProvidersFileName, providers)
	return dir
}

// cliStore is the store the CLI resolves, under TestMain's scratch state home.
func cliStore(t *testing.T) auth.Store {
	t.Helper()
	dir, err := authStoreDir()
	if err != nil {
		t.Fatal(err)
	}
	return auth.NewStore(dir)
}

// TestAuthAddChecksTheConfiguration: through the CLI, add stores a key for a
// configured entry, even one a fault dropped from the pool, and refuses, writing
// nothing, a name no entry carries and a configuration that does not load.
func TestAuthAddChecksTheConfiguration(t *testing.T) {
	const entries = "[auth-configured]\nbaseUrl = \"http://configured.example/v1\"\n\n[auth-faulted]\n"
	for _, tc := range []struct {
		name, provider, providers, outcome string
		key                                any
	}{
		{"a configured entry", "auth-configured", entries, toolresult.Done, nil},
		{"an entry the pool dropped", "auth-faulted", entries, toolresult.Done, nil},
		{"an unconfigured name", "auth-unconfigured", entries, toolresult.Refused, "auth-unconfigured"},
		{"a configuration that does not load", "auth-configured", "[auth-configured\n", toolresult.Refused, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := cliStore(t)
			t.Cleanup(func() {
				if _, err := store.Remove(tc.provider); err != nil {
					t.Error(err)
				}
			})
			stdout, _ := executeAuth(t, authSentinelKey, "auth", "add", tc.provider, "--config-dir", authConfigDir(t, tc.providers))
			d := parseResult(t, stdout)
			if d.Outcome != tc.outcome {
				t.Fatalf("outcome %s, want %s", d.Outcome, tc.outcome)
			}
			if items := d.items(toolresult.RefusalsKey); tc.key != nil && (len(items) != 1 || items[0]["key"] != tc.key) {
				t.Errorf("refusals %v, want one keyed %v", items, tc.key)
			}
			got, err := store.Read(tc.provider)
			if stored := err == nil && got == authSentinelKey; stored != (tc.outcome == toolresult.Done) {
				t.Errorf("the piped key stored: %v, want %v (read: %v)", stored, !stored, err)
			}
		})
	}
}

// TestAuthRemoveClearsAnUnconfiguredName: remove reads no configuration, so an
// orphaned key file is cleared.
func TestAuthRemoveClearsAnUnconfiguredName(t *testing.T) {
	const provider = "auth-orphaned"
	store := cliStore(t)
	if err := store.Add(provider, authSentinelKey); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := store.Remove(provider); err != nil {
			t.Error(err)
		}
	})
	stdout, _ := executeAuth(t, "", "auth", "remove", provider, "--config-dir", authConfigDir(t, ""))
	if d := checkDocument(t, "auth remove", stdout); d.Outcome != toolresult.Done {
		t.Errorf("outcome %s, want done", d.Outcome)
	}
	if names, err := store.List(); err != nil || slices.Contains(names, provider) {
		t.Errorf("after remove the store lists %v (%v), want no %s", names, err, provider)
	}
}
