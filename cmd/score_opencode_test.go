package main

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// TestOpenCodeArgv: one item's command carries the standalone server, the
// agent the inline configuration defines, and — where the run names a model —
// --model and the value as two elements, the value unaltered.
func TestOpenCodeArgv(t *testing.T) {
	for _, tc := range []struct {
		name  string
		model string
		want  []string
	}{
		{name: "a reference", model: "openrouter/deepseek/deepseek-v4.1-flash",
			want: []string{"opencode", "run", "--standalone", "--agent", opencodeAgent, "--model", "openrouter/deepseek/deepseek-v4.1-flash"}},
		{name: "a variant", model: "anthropic/claude#high",
			want: []string{"opencode", "run", "--standalone", "--agent", opencodeAgent, "--model", "anthropic/claude#high"}},
		{name: "no model", want: []string{"opencode", "run", "--standalone", "--agent", opencodeAgent}},
		{name: "a bare id is handed over whole", model: "clef",
			want: []string{"opencode", "run", "--standalone", "--agent", opencodeAgent, "--model", "clef"}},
		{name: "a value beginning with - is handed over as it stands", model: "-openrouter/deepseek",
			want: []string{"opencode", "run", "--standalone", "--agent", opencodeAgent, "--model", "-openrouter/deepseek"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			argv := opencodeArgv(tc.model)
			if !slices.Equal(argv, tc.want) {
				t.Errorf("= %q; want %q", argv, tc.want)
			}
		})
	}
}

// TestOpenCodeEnv: the environment one item's process starts with reads the
// user's credential where opencode stored it, points opencode's global
// configuration at a directory under the item's own, closes the upward walk,
// and states the item's directory as the one the run works in.
func TestOpenCodeEnv(t *testing.T) {
	dir := "/state/kbase/score/run-abc/harness"
	env := opencodeEnv(dir)
	got := map[string]string{}
	for _, assignment := range env {
		name, value, ok := strings.Cut(assignment, "=")
		if !ok {
			t.Fatalf("assignment %q carries no =", assignment)
		}
		if _, twice := got[name]; twice {
			t.Errorf("%s is assigned twice", name)
		}
		got[name] = value
	}
	for name, want := range map[string]string{
		"OPENCODE_CONFIG_DIR":             dir + "/config",
		"OPENCODE_CONFIG_PROJECT_DISABLE": "1",
		"OPENCODE_DISABLE_CLAUDE_CODE":    "1",
		"PWD":                             dir,
		"OPENCODE_CONFIG_CONTENT":         opencodeConfig(),
	} {
		if got[name] != want {
			t.Errorf("%s = %q, want %q", name, got[name], want)
		}
	}
	// kbase's own environment is the base: the credential is opencode's, so
	// nothing here may strip the environment the user's install reads.
	if len(env) <= 5 {
		t.Errorf("the environment holds %d assignments: kbase's own are not carried", len(env))
	}
}

// TestOpenCodeConfigDeniesEveryTool: the inline configuration defines one agent
// with every tool denied and nothing else — no system prompt, no instruction, no
// plugin — which is what leaves the model a request with no tool list.
func TestOpenCodeConfigDeniesEveryTool(t *testing.T) {
	type rule struct {
		Action   string `json:"action"`
		Resource string `json:"resource"`
		Effect   string `json:"effect"`
	}
	var config struct {
		Agents map[string]struct {
			System       *string   `json:"system"`
			Instructions *[]string `json:"instructions"`
			Tools        *[]string `json:"tools"`
			Permissions  []rule    `json:"permissions"`
		} `json:"agents"`
	}
	if err := json.Unmarshal([]byte(opencodeConfig()), &config); err != nil {
		t.Fatalf("the inline configuration is no JSON: %v", err)
	}
	agent, ok := config.Agents[opencodeAgent]
	if !ok || len(config.Agents) != 1 {
		t.Fatalf("agents = %v; want %q alone", config.Agents, opencodeAgent)
	}
	if agent.System != nil || agent.Instructions != nil || agent.Tools != nil {
		t.Errorf("the agent states an instruction of its own: %+v", agent)
	}
	want := []rule{{Action: "*", Resource: "*", Effect: "deny"}}
	if !slices.Equal(agent.Permissions, want) {
		t.Errorf("permissions = %v; want one deny over every action", agent.Permissions)
	}
}

// TestWithEnv: an override replaces what the base carried for its name, the
// base's other assignments stand, and the overrides are appended in name order.
func TestWithEnv(t *testing.T) {
	got := withEnv([]string{"PATH=/bin", "PWD=/elsewhere", "KEEP=1"}, map[string]string{"PWD": "/work", "B": "2", "A": "1"})
	want := []string{"PATH=/bin", "KEEP=1", "A=1", "B=2", "PWD=/work"}
	if !slices.Equal(got, want) {
		t.Errorf("= %q, want %q", got, want)
	}
}

// TestOpenCodeMajor: the version is read from the last word of the line, and a
// line with none names none.
func TestOpenCodeMajor(t *testing.T) {
	for _, tc := range []struct {
		out   string
		major int
		ok    bool
	}{
		{out: "opencode v2.1.0", major: 2, ok: true},
		{out: "opencode v3.0.0-rc.1\n", major: 3, ok: true},
		{out: "opencode vlocal"},
		{out: ""},
	} {
		if major, ok := opencodeMajor(tc.out); major != tc.major || ok != tc.ok {
			t.Errorf("%q = %d, %v; want %d, %v", tc.out, major, ok, tc.major, tc.ok)
		}
	}
}
