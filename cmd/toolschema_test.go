package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"kbase/internal/argmark"
	"kbase/internal/buildstore"
	"kbase/internal/config"
	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/mcp"
	toolresult "kbase/internal/result"
	"kbase/internal/write"
)

// briefLimit is the most characters a served argument's brief may have.
const briefLimit = 60

// TestEveryServedPropertyIsDescribed: every top-level property of every tool
// tools/list serves carries a description, and one of a command tool's
// arguments, its brief, is at most briefLimit characters; project-dir and the
// write and query tools' properties carry no brief. A mapping's sub-properties
// are not walked.
func TestEveryServedPropertyIsDescribed(t *testing.T) {
	srv := servedServer(rootCmd, mcp.Located{}, &bytes.Buffer{}, log.Discard())
	tools := toolsListedBy(t, srv)
	if len(tools) == 0 {
		t.Fatal("no tool is listed")
	}
	for _, tool := range tools {
		for name, p := range tool.InputSchema.Properties {
			description, _ := p["description"].(string)
			if strings.TrimSpace(description) == "" {
				t.Errorf("the %s tool's %s carries no description", tool.Name, name)
			}
			briefed := name != "project-dir" && tool.Name != write.Batch && tool.Name != queryFamily
			if n := utf8.RuneCountInString(description); briefed && n > briefLimit {
				t.Errorf("the %s tool's %s is described in %d characters, over %d: %q", tool.Name, name, n, briefLimit, description)
			}
		}
	}
}

// TestABriefForNoArgumentFailsTheServer: a brief a command declares for an
// argument its served schema does not take, as one a rename left behind is,
// stops the server serving, for a command tool and a detached launch alike.
func TestABriefForNoArgumentFailsTheServer(t *testing.T) {
	for _, c := range []*cobra.Command{probeCmd, buildCmd} {
		t.Run(c.Name(), func(t *testing.T) {
			saved := c.Annotations
			t.Cleanup(func() { c.Annotations = saved })
			c.Annotations = maps.Clone(saved)
			mcp.Brief(c, "renamed-away", "an argument no longer taken")
			srv := servedServer(rootCmd, mcp.Located{}, &bytes.Buffer{}, log.Discard())
			if err := srv.Serve(strings.NewReader(""), io.Discard); err == nil {
				t.Errorf("%s declares a brief for renamed-away, which its tool does not take, and the server serves", c.Name())
			}
		})
	}
}

// TestServedEnumsAreWhatTheCommandsAccept: each value tools/list gives as an
// argument's enum, a mapping key's included, is one the command's own check
// accepts; a value outside it the check refuses, and where that refusal lists
// what is allowed, the list is the enum.
func TestServedEnumsAreWhatTheCommandsAccept(t *testing.T) {
	enums := map[string]map[string][]string{}
	for _, tool := range toolsListedBy(t, servedServer(rootCmd, mcp.Located{}, &bytes.Buffer{}, log.Discard())) {
		enums[tool.Name] = map[string][]string{}
		for name, p := range tool.InputSchema.Properties {
			enums[tool.Name][name] = listedEnum(p)
			inner, _ := p["properties"].(map[string]any)
			for key, kp := range inner {
				kp, _ := kp.(map[string]any)
				enums[tool.Name][name+"."+key] = listedEnum(kp)
			}
		}
	}
	if got := enums["build"]["through"]; !slices.Equal(got, buildstore.StageIDs()) {
		t.Errorf("the build tool's through enum is %v, want the stage ids %v: an enum lists the canonical values alone, never an alias", got, buildstore.StageIDs())
	}
	bearings := bearingWalkRepo(t, enums["score"]["edge.relation"])
	scoreRefusals := func(opts scoreOptions) []toolresult.Item {
		var items []toolresult.Item
		for _, item := range scoreIn(t, bearings, t.TempDir(), opts, nil).doc.items(toolresult.RefusalsKey) {
			items = append(items, toolresult.Item{Detail: fmt.Sprint(item)})
		}
		return items
	}
	layerRefusals := func(v string) []toolresult.Item {
		_, _, items, _ := configureLocation(globalLayer(t.TempDir(), t.TempDir()), v, "/start")
		return items
	}
	for _, c := range []struct {
		tool, argument string
		// lists is whether the check's refusal of a value outside the set
		// lists the set as allowed.
		lists bool
		check func(value string) []toolresult.Item
	}{
		{"score", "pass", true, func(v string) []toolresult.Item { _, items := scoreUsage(scoreOptions{Pass: v}); return items }},
		{"score", "edge.relation", false, func(v string) []toolresult.Item {
			return scoreRefusals(scoreOptions{DryRun: true, Edge: bearingWalkEdge(enums["score"]["edge.relation"], v)})
		}},
		{"configure-providers", "api", true, func(v string) []toolresult.Item { return config.ProviderEntryRefusals("p", "", v, "") }},
		{"configure-providers", "decision-api", true, func(v string) []toolresult.Item { return config.ProviderEntryRefusals("p", "", "", v) }},
		{"probe", "role", true, func(v string) []toolresult.Item {
			_, _, err := probeRole(probeOptions{Role: config.Role(v)})
			var refusal toolresult.Refusal
			errors.As(err, &refusal)
			return refusal
		}},
		{"configure-providers", "layer", true, layerRefusals},
		{"configure-models", "layer", true, layerRefusals},
		{"build", "through", false, func(v string) []toolresult.Item {
			if _, ok := buildstore.ResolveStage(v); !ok {
				return []toolresult.Item{{Detail: v + " names no stage"}}
			}
			return nil
		}},
	} {
		t.Run(c.tool+" "+c.argument, func(t *testing.T) {
			enum := enums[c.tool][c.argument]
			if len(enum) == 0 {
				t.Fatalf("the %s tool's %s carries no enum", c.tool, c.argument)
			}
			for _, v := range enum {
				if items := c.check(v); len(items) != 0 {
					t.Errorf("%q, in the enum, is refused: %v", v, items)
				}
			}
			items := c.check("no-such-value")
			if len(items) != 1 {
				t.Fatalf("a value outside the enum: %d refusal items, want 1: %v", len(items), items)
			}
			if c.lists && !slices.Equal(items[0].Allowed, enum) {
				t.Errorf("the refusal allows %v, want the enum %v", items[0].Allowed, enum)
			}
		})
	}
}

// listedEnum is the enum a listed property p carries, nil where none.
func listedEnum(p map[string]any) []string {
	values, _ := p["enum"].([]any)
	var enum []string
	for _, v := range values {
		s, _ := v.(string)
		enum = append(enum, s)
	}
	return enum
}

// bearingWalkRepo is a KB whose one experiment has an edge of each of
// relations to a claim of its own (bearingWalkEdge), every bearing pending.
func bearingWalkRepo(t *testing.T, relations []string) string {
	t.Helper()
	repo := t.TempDir()
	leaf := "[↑ KB](entry-point.md)\n\n# A\n\nThe experiment's own paragraph.<!-- kbase-id: exp-aaaaaa -->\n"
	g := kb.Graph{Experiments: []kb.ExperimentNode{{ID: "exp-aaaaaa", Title: "An Experiment", Status: kb.ExperimentRun}}}
	for i, r := range relations {
		target := bearingWalkTarget(i)
		leaf += "\nA claim's own paragraph " + strconv.Itoa(i) + ".<!-- kbase-id: " + target + " -->\n"
		g.Claims = append(g.Claims, kb.ClaimNode{ID: target, Title: "Claim " + strconv.Itoa(i), Rationale: "Shown."})
		g.Scorable = append(g.Scorable, kb.ScorableEdge{Relation: r, Source: "exp-aaaaaa", Target: target, Bearing: kb.Bearing{Pending: true}})
	}
	files := map[string]string{
		".git/HEAD":              "ref: refs/heads/main\n",
		"kb-root/entry-point.md": maintenanceEntryPoint,
		maintenanceStampFile:     maintenanceFormatStamp,
		"kb-root/a.md":           leaf,
	}
	maps.Copy(files, recordFiles(g))
	writeRepo(t, repo, files)
	updateFormatKB(t, repo)
	return repo
}

// bearingWalkTarget is the claim bearingWalkRepo's i-th relation targets.
func bearingWalkTarget(i int) string { return fmt.Sprintf("clm-rel%03d", i) }

// bearingWalkEdge is score's edge argument naming bearingWalkRepo's edge of
// relation, over relations; one of a relation outside them targets the first
// claim.
func bearingWalkEdge(relations []string, relation string) string {
	return "exp-aaaaaa," + bearingWalkTarget(max(slices.Index(relations, relation), 0)) + "," + relation
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

// TestArgumentListDescriptionsStateHelpsDefault: for every flag a command's
// tool takes, a query operation's included, the default --help's flags block
// prints after its usage is stated in the description the help's argument list
// carries (mcp.CommandSchema's), as "default: X", quoted or not, and a duration
// flag's description says it is a Go duration.
func TestArgumentListDescriptionsStateHelpsDefault(t *testing.T) {
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
