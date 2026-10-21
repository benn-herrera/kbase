package main

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"kbase/internal/config"
	"kbase/internal/harness"
)

// The built-in opencode back end. `--harness opencode` selects it: kbase
// composes the command and the environment opencode runs in here, and no
// harnesses.toml profile is read. It knows opencode — argv, environment,
// version — and nothing else, so this is the whole of what kbase knows of a
// harness's own interface.

const (
	// opencodeHarness is the --harness value selecting this back end.
	opencodeHarness = "opencode"
	// opencodeAgent is the agent the inline configuration defines for this
	// process alone. No profile and no file names it, and it stands nowhere
	// but the environment one run's process reads.
	opencodeAgent = "kbase-scoring"
	// opencodeVersion is the major this back end targets. opencode updates
	// often, so nothing is pinned below it.
	opencodeVersion = 2
	// opencodeConfigDir is the global configuration directory this back end
	// gives the process, under the directory one item runs in.
	opencodeConfigDir = "config"
)

// opencodeConfig is the inline configuration OPENCODE_CONFIG_CONTENT carries:
// one agent, with every tool denied to it, and snapshots off. opencode removes
// a tool from a request whose last matching rule denies every resource, so one
// deny over `*` is what leaves the model with no tool list at all. Snapshots
// are what opencode would otherwise keep of the item's directory in the user's
// data directory, and kbase empties that directory before every item: there is
// nothing there worth an object store. Nothing in this configuration states a
// system prompt, an instruction file, a plugin or a skill.
func opencodeConfig() string {
	return fmt.Sprintf(`{"snapshots":false,"agents":{%q:{"permissions":[{"action":"*","resource":"*","effect":"deny"}]}}}`, opencodeAgent)
}

// opencodeArgv is the command one item runs. The prompt reaches opencode on
// stdin — no element carries it — so one item's text is never an argv element,
// and the model, where the run names one, is opencode's own reference passed as
// its own element.
//
// --standalone is what makes the environment take effect: without it
// `opencode run` attaches to the background service, whose configuration and
// agent set were fixed when that service started and whose environment this
// process's assignments never reach.
func opencodeArgv(model string) ([]string, error) {
	argv := []string{opencodeHarness, "run", "--standalone", "--agent", opencodeAgent}
	switch {
	case model == "":
		// opencode asks its own configured model; the run names none.
		return argv, nil
	case strings.HasPrefix(model, "-"):
		return nil, config.Refusal{Check: config.CheckHarness, Key: "--harness",
			Detail: fmt.Sprintf("--harness %s hands opencode its model as an argv element, and %q begins with -: opencode would read it as an option rather than as a model id",
				opencodeHarness, model)}
	case !opencodeReference(model):
		return nil, config.Refusal{Check: config.CheckHarness, Key: "--harness",
			Detail: fmt.Sprintf("--harness %s runs opencode, which names a model <provider>/<model>: %q is not one",
				opencodeHarness, model)}
	}
	return append(argv, "--model", model), nil
}

// opencodeReference reports whether model is a reference opencode's own parser
// takes: <provider>/<model>, split at the first "/", and an optional #variant.
// A model id may itself contain "/" (openrouter/anthropic/...), a provider may
// not, and neither an id nor a variant may be empty.
func opencodeReference(model string) bool {
	provider, rest, ok := strings.Cut(model, "/")
	if !ok || provider == "" || rest == "" || strings.Contains(provider, "#") {
		return false
	}
	id, variant, variantGiven := strings.Cut(rest, "#")
	if id == "" {
		return false
	}
	return !variantGiven || (variant != "" && !strings.Contains(variant, "#"))
}

// opencodeEnv is the environment one item's process starts with: kbase's own,
// so the credential opencode already holds for the model is read where the user
// stored it, with the assignments that make the process this run's.
//
// OPENCODE_CONFIG_DIR is a directory under the item's working directory, which
// kbase empties before each item: the global configuration opencode reads is
// therefore empty, so no instruction file, agent, skill or plugin of the user's
// reaches this run, and the directory ends with the run's own records.
// OPENCODE_CONFIG_PROJECT_DISABLE closes the other hole: the item's directory
// may sit inside a user's project, and the upward walk for AGENTS.md and
// opencode.json starts there. PWD names the item's directory because `opencode
// run` takes its working directory from PWD, not from the one it is started in.
func opencodeEnv(dir string) []string {
	return withEnv(os.Environ(), map[string]string{
		"OPENCODE_CONFIG_CONTENT":         opencodeConfig(),
		"OPENCODE_CONFIG_DIR":             filepath.Join(dir, opencodeConfigDir),
		"OPENCODE_CONFIG_PROJECT_DISABLE": "1",
		"OPENCODE_DISABLE_CLAUDE_CODE":    "1",
		"PWD":                             dir,
	})
}

// withEnv is base with overrides set once each: every assignment base already
// carries for a name overrides names is dropped first, so the child reads one
// value per name however the host's environment is spelled.
func withEnv(base []string, overrides map[string]string) []string {
	out := make([]string, 0, len(base)+len(overrides))
	for _, assignment := range base {
		if name, _, ok := strings.Cut(assignment, "="); ok {
			if _, overridden := overrides[name]; overridden {
				continue
			}
		}
		out = append(out, assignment)
	}
	for _, name := range slices.Sorted(maps.Keys(overrides)) {
		out = append(out, name+"="+overrides[name])
	}
	return out
}

// opencodeSetup is --harness opencode's half of the setup: the version gate,
// the scoring role's model in opencode's form, and the probed command every
// item starts.
//
// A harnesses.toml profile of the same name is not run and says so once, naming
// its file: the name selects the built-in back end, and a user's profile
// standing behind a name kbase also uses is exactly the substitution silence
// would hide.
func opencodeSetup(ctx context.Context, in config.Inputs, opts scoreOptions) (config.Profile, config.Endpoint, *harness.Command, error) {
	if err := opencodeCheck(ctx, opts); err != nil {
		return config.Profile{}, config.Endpoint{}, nil, err
	}
	if p, ok := opts.Harnesses[opencodeHarness]; ok {
		fmt.Fprintf(opts.Stderr, "%s holds an %q harness profile, and --harness %s runs the built-in opencode back end: the profile is not run\n",
			config.HarnessesPath(harnessLayerPath(opts.Layers, p.Layer)), opencodeHarness, opencodeHarness)
	}
	written, _, source, err := config.HarnessModel(in, config.RoleScoring, opts.ScoringModel)
	if err != nil {
		return config.Profile{}, config.Endpoint{}, nil, err
	}
	argv, err := opencodeArgv(written)
	if err != nil {
		return config.Profile{}, config.Endpoint{}, nil, err
	}
	command, err := harness.New(argv, opts.Logger)
	if err != nil {
		return config.Profile{}, config.Endpoint{}, nil, config.Refusal{Check: config.CheckHarness, Key: "--harness",
			Detail: fmt.Sprintf("--harness %s runs the opencode back end: %v", opencodeHarness, err)}
	}
	return config.Profile{Name: opencodeHarness},
		config.Endpoint{Role: config.RoleScoring, Model: written, ModelSource: source}, command, nil
}

// opencodeCheck refuses a program that is not opencode v2, before any item is
// asked: one version print, and nothing else about the program is inspected.
func opencodeCheck(ctx context.Context, opts scoreOptions) error {
	probe, err := harness.New([]string{opencodeHarness, "--version"}, opts.Logger)
	if err != nil {
		return config.Refusal{Check: config.CheckHarness, Key: "--harness",
			Detail: fmt.Sprintf("--harness %s runs the opencode back end: %v", opencodeHarness, err)}
	}
	callCtx, cancel := context.WithTimeout(ctx, opts.callTimeout())
	defer cancel()
	out, err := probe.Ask(callCtx, opts.WorkDir)
	if err != nil {
		return config.Refusal{Check: config.CheckHarness, Key: "--harness",
			Detail: fmt.Sprintf("--harness %s runs the opencode back end, and `opencode --version` failed: %v", opencodeHarness, err)}
	}
	if major, ok := opencodeMajor(out); !ok || major != opencodeVersion {
		return config.Refusal{Check: config.CheckHarness, Key: "--harness",
			Detail: fmt.Sprintf("--harness %s runs opencode v%d, and `opencode --version` printed %q",
				opencodeHarness, opencodeVersion, out)}
	}
	return nil
}

// opencodeMajor reads the major version from `opencode --version`'s output:
// "opencode v2.1.0" is 2. A line this cannot read as a version names none.
func opencodeMajor(out string) (int, bool) {
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return 0, false
	}
	major, _, _ := strings.Cut(strings.TrimPrefix(fields[len(fields)-1], "v"), ".")
	n, err := strconv.Atoi(major)
	return n, err == nil
}
