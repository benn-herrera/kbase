package mcp

import (
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"kbase/internal/mcp/protocol"
)

func TestDeclaredPositionals(t *testing.T) {
	for _, tc := range []struct {
		name, use string
		want      []Positional
		wantErr   bool
	}{
		{"none", "verb", []Positional{}, false},
		{"one", "verb <paper>", []Positional{{"paper", false, false}}, false},
		{"a variadic last", "verb <paper> <roots>...", []Positional{{"paper", false, false}, {"roots", true, false}}, false},
		{"an optional variadic last", "verb <paper> [<roots>...]", []Positional{{"paper", false, false}, {"roots", true, true}}, false},
		{"a bare word", "verb paper", nil, true},
		{"an optional in brackets", "verb [<paper>]", nil, true},
		{"an optional variadic before the last", "verb [<roots>...] <paper>", nil, true},
		{"a variadic before the last", "verb <roots>... <paper>", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DeclaredPositionals(&cobra.Command{Use: tc.use})
			if (err != nil) != tc.wantErr {
				t.Fatalf("error %v, want an error %v", err, tc.wantErr)
			}
			if !tc.wantErr && !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestCommandSchema: the positionals are required string properties, a
// variadic one an array and an optional variadic one not required, each
// described as its command declares; every local flag of a carried type is a
// property, optional unless RequireFlag declares it, described by its usage,
// then the default --help prints
// where it prints one, and for a duration that it is a Go duration; an
// excluded flag and a persistent one are no property; and a flag no tool
// argument carries, or one named as a positional, is an error.
func TestCommandSchema(t *testing.T) {
	cmd := DescribePositional(&cobra.Command{Use: "verb <paper> [<roots>...]"}, "paper", "paper description")
	cmd.Flags().Bool("dry", false, "dry usage")
	cmd.Flags().String("name", "", "name usage")
	cmd.Flags().String("order", "up", "order usage")
	cmd.Flags().String("label", "two words", "label usage")
	cmd.Flags().Int("limit", 0, "limit usage")
	cmd.Flags().Int("size", 50, "size usage")
	cmd.Flags().Float64("ratio", 0, "ratio usage")
	cmd.Flags().StringArray("tag", nil, "tag usage")
	cmd.Flags().Duration("timeout", time.Second, "timeout usage")
	cmd.Flags().Duration("wait", 0, "wait usage")
	cmd.Flags().String("dev", "", "dev usage")
	ExcludeFlag(cmd.Flags().Lookup("dev"))
	cmd.Flags().String("must", "", "must usage")
	RequireFlag(cmd.Flags().Lookup("must"))
	cmd.PersistentFlags().String("global", "", "global usage")

	positionals, schema, err := CommandSchema(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if want := []Positional{{"paper", false, false}, {"roots", true, true}}; !reflect.DeepEqual(positionals, want) {
		t.Errorf("positionals %+v, want %+v", positionals, want)
	}
	strs := &protocol.Property{Type: "string"}
	want := protocol.Schema{
		Properties: map[string]protocol.Property{
			"paper":   {Type: "string", Description: "paper description"},
			"roots":   {Type: "array", Items: strs},
			"dry":     {Type: "boolean", Description: "dry usage"},
			"name":    {Type: "string", Description: "name usage"},
			"order":   {Type: "string", Description: "order usage (default: up)"},
			"label":   {Type: "string", Description: `label usage (default: "two words")`},
			"limit":   {Type: "integer", Description: "limit usage"},
			"size":    {Type: "integer", Description: "size usage (default: 50)"},
			"ratio":   {Type: "number", Description: "ratio usage"},
			"tag":     {Type: "array", Items: strs, Description: "tag usage"},
			"timeout": {Type: "string", Description: "timeout usage (a Go duration; default: 1s)"},
			"wait":    {Type: "string", Description: "wait usage (a Go duration)"},
			"must":    {Type: "string", Description: "must usage"},
		},
		Required: []string{"paper", "must"},
	}
	if !reflect.DeepEqual(schema, want) {
		t.Errorf("schema %+v\nwant %+v", schema, want)
	}

	for _, tc := range []struct {
		name  string
		build func(*cobra.Command)
	}{
		{"a flag type no argument carries", func(c *cobra.Command) { c.Flags().Uint("count", 0, "") }},
		{"a flag named as a positional", func(c *cobra.Command) { c.Flags().String("paper", "", "") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &cobra.Command{Use: "verb <paper>"}
			tc.build(c)
			if _, _, err := CommandSchema(c); err == nil {
				t.Error("no error")
			}
		})
	}
}

// TestToolsRequireFamiliesAndMembersToMatch: the tool table and the help
// resources must agree on which families exist, so a member naming a family
// no Family names, and a Family no bound command names, are each an error.
func TestToolsRequireFamiliesAndMembersToMatch(t *testing.T) {
	for _, tc := range []struct {
		name     string
		member   string
		families []Family
	}{
		{"a member whose family is absent", "absent", nil},
		{"a family with no member", "", []Family{{Name: "lonely"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := &cobra.Command{Use: "root"}
			cmd := Bind(&cobra.Command{Use: "verb", RunE: func(*cobra.Command, []string) error { return nil }})
			if tc.member != "" {
				InFamily(cmd, tc.member)
			}
			root.AddCommand(cmd)
			if _, err := Tools(root, Located{}, io.Discard, Calls{Families: tc.families}); err == nil {
				t.Error("no error")
			}
		})
	}
}

// TestCommandLine: the positionals in declared order, an array spread into
// its items; then each other argument as flag settings in name order, one per
// array item; a bool and a number spelled as the command line spells them; an
// absent argument nothing.
func TestCommandLine(t *testing.T) {
	positionals := []Positional{{"paper", false, false}, {"roots", true, false}}
	for _, tc := range []struct {
		name      string
		args      map[string]any
		wantArgv  []string
		wantFlags []FlagArgument
	}{
		{"no arguments", map[string]any{}, nil, nil},
		{"positionals alone",
			map[string]any{"paper": "p.tex", "roots": []any{"a", "b"}},
			[]string{"p.tex", "a", "b"}, nil},
		{"flags in name order",
			map[string]any{"zeta": true, "alpha": []any{"x", "y"}, "mid": "m"},
			nil, []FlagArgument{{"alpha", "x"}, {"alpha", "y"}, {"mid", "m"}, {"zeta", "true"}}},
		{"numbers as the command line spells them",
			map[string]any{"limit": float64(3), "ratio": 0.25, "big": 1e21},
			nil, []FlagArgument{{"big", "1000000000000000000000"}, {"limit", "3"}, {"ratio", "0.25"}}},
		{"an absent argument is nothing",
			map[string]any{"paper": nil, "dry": nil, "off": false},
			nil, []FlagArgument{{"off", "false"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			argv, flags := CommandLine(positionals, nil, tc.args)
			if !reflect.DeepEqual(argv, tc.wantArgv) {
				t.Errorf("argv %q, want %q", argv, tc.wantArgv)
			}
			if !reflect.DeepEqual(flags, tc.wantFlags) {
				t.Errorf("flags %+v, want %+v", flags, tc.wantFlags)
			}
		})
	}
}
