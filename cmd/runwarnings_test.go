package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"kbase/internal/config"
	"kbase/internal/log"
	toolresult "kbase/internal/result"
)

// runWarningsConfig stages a configuration directory whose pool holds local, at
// baseURL and named by the scoring role, and — where faulted — broken, which
// declares no baseUrl and so drops out of the pool.
func runWarningsConfig(t *testing.T, baseURL string, faulted bool) string {
	t.Helper()
	dir := t.TempDir()
	providers := fmt.Sprintf("[local]\nbaseUrl = %q\napi = %q\n", baseURL, config.ProviderAPIOpenAI)
	if faulted {
		providers += fmt.Sprintf("\n[broken]\napi = %q\n", config.ProviderAPIOpenAI)
	}
	writeValues(t, dir, config.ProvidersFileName, providers)
	writeValues(t, dir, config.ConfigFileName, "[models]\nscoring = \"local/judge\"\n")
	return dir
}

// TestRunVerbsReportConfigurationFaults: build, probe and score report each
// fault their configuration load finds as an item of warnings — path the file
// it is against, detail what is wrong — and as one line on stderr, on a refusal
// as on a run that went through; with no fault, warnings is an empty list.
func TestRunVerbsReportConfigurationFaults(t *testing.T) {
	const fault = `provider "broken" unavailable: no baseUrl declared`
	srv, _ := probeRoleWire(t, config.RoleScoring)
	// Each verb is staged afresh per call: the directory it runs from, then a
	// run over opts that goes through, or one its arguments get refused.
	type verb func(t *testing.T, opts providerOptions, stdout *bytes.Buffer, refuse bool) (int, error)
	for _, tc := range []struct {
		name string
		run  verb
		want string
	}{
		{"build", func(t *testing.T, opts providerOptions, stdout *bytes.Buffer, refuse bool) (int, error) {
			repo, state := buildRepo(t)
			through := "spine-seed"
			if refuse {
				through = "phase-5"
			}
			return runBuild(context.Background(), buildOptions{VolumeRoots: []string{"paper.tex"}, NoInference: true, Through: through,
				StateDir: state, WorkDir: repo, Faults: opts.Faults, Stdout: stdout, Stderr: opts.Stderr, Logger: log.Discard()})
		}, toolresult.Bounded},
		{"probe", func(t *testing.T, opts providerOptions, stdout *bytes.Buffer, refuse bool) (int, error) {
			var role config.Role
			if refuse {
				role = "no-such-role"
			}
			return runProbe(context.Background(), probeOptions{providerOptions: opts, Role: role, Stdout: stdout, Logger: log.Discard()})
		}, toolresult.Done},
		{"score", func(t *testing.T, opts providerOptions, stdout *bytes.Buffer, refuse bool) (int, error) {
			return runScore(scoreOptions{providerOptions: opts, DryRun: true, Resume: refuse, StateDir: t.TempDir(), WorkDir: scoreRepo(t),
				Stdout: stdout, Logger: log.Discard()})
		}, toolresult.Done},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := func(cfgDir, want string, refuse bool) (parsedResult, string) {
				t.Helper()
				saved := flagConfigDir
				t.Cleanup(func() { flagConfigDir = saved })
				flagConfigDir = cfgDir
				opts, err := loadVerbContext(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				var stdout, stderr bytes.Buffer
				opts.Stderr = &stderr
				code, err := tc.run(t, opts, &stdout, refuse)
				if err != nil {
					t.Fatal(err)
				}
				doc := checkResult(t, tc.name, stdout.String())
				if doc.Outcome != want || code != toolresult.ExitCode(want) {
					t.Fatalf("outcome %s, exit %d; want %s\n%s", doc.Outcome, code, want, stdout.String())
				}
				return doc, stderr.String()
			}
			faulted := runWarningsConfig(t, srv.URL, true)
			for _, c := range []struct {
				want   string
				refuse bool
			}{{tc.want, false}, {toolresult.Refused, true}} {
				doc, stderr := run(faulted, c.want, c.refuse)
				if w := doc.items("warnings"); len(w) != 1 || w[0]["path"] != config.ProvidersPath(faulted) || w[0]["detail"] != fault {
					t.Errorf("%s: warnings = %v, want %q against %s", c.want, w, fault, config.ProvidersPath(faulted))
				}
				if n := strings.Count(stderr, fault); n != 1 {
					t.Errorf("%s: stderr names the fault %d times, want once:\n%s", c.want, n, stderr)
				}
			}
			doc, _ := run(runWarningsConfig(t, srv.URL, false), tc.want, false)
			if w, ok := doc.Values["warnings"].([]any); !ok || len(w) != 0 {
				t.Errorf("warnings = %v, want an empty list where there is no fault", doc.Values["warnings"])
			}
		})
	}
}
