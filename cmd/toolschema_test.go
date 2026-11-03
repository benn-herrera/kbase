package main

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"kbase/internal/argmark"
	"kbase/internal/log"
	"kbase/internal/mcp"
	"kbase/internal/write"
)

// TestEveryServedPropertyIsDescribed: every top-level property of every tool
// tools/list serves carries a description. A mapping's sub-properties are not
// walked.
func TestEveryServedPropertyIsDescribed(t *testing.T) {
	srv := servedServer(rootCmd, mcp.Located{}, &bytes.Buffer{}, log.Discard())
	tools := toolsListedBy(t, srv)
	if len(tools) == 0 {
		t.Fatal("no tool is listed")
	}
	for _, tool := range tools {
		for name, p := range tool.InputSchema.Properties {
			if description, _ := p["description"].(string); strings.TrimSpace(description) == "" {
				t.Errorf("the %s tool's %s carries no description", tool.Name, name)
			}
		}
	}
}

// listedArgument is one line of an argument list as a help renders it: two
// spaces, the name, as the body spells it, its type, required and repeatable
// in parentheses, then a colon and its description.
var listedArgument = regexp.MustCompile(`^  (\S+) \(([^)]+)\): (\S.*)$`)

// TestEveryServedArgumentIsListedWithATypeAndADescription: the argument list
// of every bound command's help resource lists exactly the arguments its tool takes — kbase write's batch keys, a values
// op's entry keys, a command's positionals and flags — each on a line naming
// its type and carrying a description; and each flag a development build
// carries and no tool takes is named in the command's CLI note.
func TestEveryServedArgumentIsListedWithATypeAndADescription(t *testing.T) {
	srv := servedServer(rootCmd, mcp.Located{}, &bytes.Buffer{}, log.Discard())
	batchKeys := map[string]bool{}
	for _, tool := range toolsListedBy(t, srv) {
		if tool.Name == write.Batch {
			for name := range tool.InputSchema.Properties {
				batchKeys[name] = name != "project-dir"
			}
		}
	}
	for _, c := range boundCommandsOf(rootCmd) {
		want := map[string]bool{}
		switch fields, values := write.OpVocabulary(c.Name()); {
		case c.Name() == write.Batch:
			want = batchKeys
		case values:
			for _, f := range fields {
				want[f.Name] = true
			}
		default:
			_, schema, err := mcp.CommandSchema(c)
			if err != nil {
				t.Fatal(err)
			}
			for name := range schema.Properties {
				want[name] = true
			}
		}
		help := readResource(t, srv, mcp.HelpURIPrefix+c.Name())
		listed := map[string]bool{}
		if _, list, ok := strings.Cut(help, "\nArguments:\n"); ok {
			for _, line := range strings.Split(list, "\n") {
				m := listedArgument.FindStringSubmatch(line)
				if m == nil {
					break
				}
				listed[strings.Trim(m[1], "`")] = true
			}
		}
		for name, served := range want {
			if served && !listed[name] {
				t.Errorf("%s: %s is no argument list line with a type and a description:\n%s", c.Name(), name, help)
			}
		}
		for name := range listed {
			if !want[name] {
				t.Errorf("%s: its argument list names %s, which its tool does not take", c.Name(), name)
			}
		}
		note, err := mcp.CLINote(c)
		if err != nil {
			t.Fatal(err)
		}
		c.LocalNonPersistentFlags().VisitAll(func(f *pflag.Flag) {
			if mcp.ExcludedFlag(f) && !strings.Contains(note, argmark.Flag(f.Name)+" (") {
				t.Errorf("%s: --%s, which no tool takes, is not named in its CLI note:\n%s", c.Name(), f.Name, note)
			}
		})
	}
}

// helpDefault matches the default pflag appends to a flag's line in --help's
// flags block.
var helpDefault = regexp.MustCompile(`\(default (.+)\)$`)

// TestServedFlagDescriptionsStateHelpsDefault: for every flag a command's tool
// schema derives, a query operation's included, the default --help's flags
// block prints after its usage is stated in its schema description as
// "default: X", quoted or not, and a
// duration flag's description says it is a Go duration.
func TestServedFlagDescriptionsStateHelpsDefault(t *testing.T) {
	defaulted := 0
	for _, c := range boundCommandsOf(rootCmd) {
		if _, values := write.OpVocabulary(c.Name()); values || c.Name() == write.Batch || c.Annotations[mcp.FamilyAnnotation] == write.Batch {
			continue
		}
		_, schema, err := mcp.CommandSchema(c)
		if err != nil {
			t.Fatal(err)
		}
		c.LocalNonPersistentFlags().VisitAll(func(f *pflag.Flag) {
			p, ok := schema.Properties[f.Name]
			if !ok {
				return
			}
			alone := pflag.NewFlagSet(c.Name(), pflag.ContinueOnError)
			alone.AddFlag(f)
			line := strings.TrimSpace(alone.FlagUsages())
			if m := helpDefault.FindStringSubmatch(line); m != nil && !strings.HasSuffix(f.Usage, m[0]) {
				defaulted++
				value := m[1]
				if unquoted, err := strconv.Unquote(value); err == nil {
					value = unquoted
				}
				if !strings.Contains(p.Description, "default: "+value) && !strings.Contains(p.Description, "default: "+strconv.Quote(value)) {
					t.Errorf("%s --%s: --help prints (default %s), which its schema description %q does not state", c.Name(), f.Name, m[1], p.Description)
				}
			}
			if f.Value.Type() == "duration" && !strings.Contains(p.Description, "a Go duration") {
				t.Errorf("%s --%s: a duration flag whose schema description %q does not say it is a Go duration", c.Name(), f.Name, p.Description)
			}
		})
	}
	if defaulted == 0 {
		t.Fatal("no served flag has a default --help prints")
	}
}
