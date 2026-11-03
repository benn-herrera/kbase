package argmark

import (
	"slices"
	"testing"
)

func TestSpellSpellsEachArgumentForItsSurface(t *testing.T) {
	text := "use " + Flag("state-dir") + " with " + Positional("volume-root") + "."
	for _, tc := range []struct {
		name string
		s    Spelling
		want string
	}{
		{"the command line", Spelling{}, "use --state-dir with <volume-root>."},
		{"MCP", Spelling{MCP: true}, "use `state-dir` with `volume-root`."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.s.Spell(text); got != tc.want {
				t.Errorf("Spell = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSpellHandsACallToItsSurface(t *testing.T) {
	var got []Invocation
	s := Spelling{Call: func(inv Invocation) string { got = append(got, inv); return "<call>" }}
	text := s.Spell("run " + Call("find", map[string]any{"kind": []string{"claim"}, "max": 0.65}) + " or " +
		Recorded("build", []string{"--resume", "--state-dir", "/s"}))
	if text != "run `<call>` or `<call>`" || len(got) != 2 {
		t.Fatalf("Spell = %q, calls %v", text, got)
	}
	if got[0].Command != "find" || got[0].Args["max"] != 0.65 || !slices.Equal(got[0].Args["kind"].([]any), []any{"claim"}) || got[0].Argv != nil {
		t.Errorf("the find call = %+v", got[0])
	}
	if got[1].Command != "build" || got[1].Args != nil || !slices.Equal(got[1].Argv, []string{"--resume", "--state-dir", "/s"}) {
		t.Errorf("the recorded call = %+v", got[1])
	}
}

// TestACallCarriesAListOfMappings: a write batch's entries, a list of
// mappings, reach the surface's call spelling as they were given.
func TestACallCarriesAListOfMappings(t *testing.T) {
	entry := map[string]any{"operation": "add-citation", "id": "clm-aaaaaa", "line": 7}
	got := Calls("cite: " + Call("write", map[string]any{"entry": []map[string]any{entry}}))
	if len(got) != 1 || got[0].Command != "write" {
		t.Fatalf("Calls = %+v", got)
	}
	entries, _ := got[0].Args["entry"].([]any)
	if len(entries) != 1 {
		t.Fatalf("the call's entry = %#v", got[0].Args["entry"])
	}
	if e, _ := entries[0].(map[string]any); e["operation"] != "add-citation" || e["id"] != "clm-aaaaaa" || e["line"] != 7.0 {
		t.Errorf("the entry = %#v, want %v", entries[0], entry)
	}
}

// TestAMarkerInsideTextIsBacktickedAndAWholeOneBare: a marker that is the
// whole text — a key, a resume, a remedy that is only a call — stays bare; one
// inside other text is wrapped in backticks over MCP, an argument and a call
// alike, and at the command line a call alone, since --name and <name> are
// delimited already.
func TestAMarkerInsideTextIsBacktickedAndAWholeOneBare(t *testing.T) {
	call := func(inv Invocation) string { return "run " + inv.Command }
	cli, mcp := Spelling{Call: call}, Spelling{MCP: true, Call: call}
	for _, tc := range []struct {
		name string
		s    Spelling
		text string
		want string
	}{
		{"a whole flag over MCP", mcp, Flag("limit"), "limit"},
		{"a whole call over MCP", mcp, Call("find", nil), "run find"},
		{"a whole call at the command line", cli, Call("find", nil), "run find"},
		{"an embedded flag over MCP", mcp, Flag("limit") + " needs " + Positional("id"), "`limit` needs `id`"},
		{"an embedded flag at the command line", cli, Flag("limit") + " needs " + Positional("id"), "--limit needs <id>"},
		{"an embedded call over MCP", mcp, "list with " + Call("find", nil), "list with `run find`"},
		{"an embedded call at the command line", cli, "list with " + Call("find", nil), "list with `run find`"},
		{"a flag with its value over MCP", mcp, "keep " + FlagValue("direction", "out"), "keep `direction: out`"},
		{"a flag with its value at the command line", cli, "keep " + FlagValue("direction", "out"), "keep --direction out"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.s.Spell(tc.text); got != tc.want {
				t.Errorf("Spell(%q) = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}

func TestSpellLeavesWhatNoMarkerOpens(t *testing.T) {
	for _, text := range []string{
		"<id> and --flag stay as written",
		open + "-Bad Name" + close,
		open + "?x" + close,
		open + "-unclosed",
		open + "!find {not json}" + close,
	} {
		if got := (Spelling{MCP: true}).Spell(text); got != text {
			t.Errorf("Spell(%q) = %q, want it unchanged", text, got)
		}
	}
}

func TestACallStaysMarkedWithNoCallSpelling(t *testing.T) {
	marked := Call("update-format", nil)
	if got := (Spelling{}).Spell(marked); got != marked {
		t.Errorf("Spell = %q, want the marker kept", got)
	}
}

func TestArgumentsAndCallsListTheMarkers(t *testing.T) {
	text := Flag("a") + Positional("b") + Call("c", nil) + Flag("d")
	if got, want := Arguments(text), []Argument{{"a", false}, {"b", true}, {"d", false}}; !slices.Equal(got, want) {
		t.Errorf("Arguments = %v, want %v", got, want)
	}
	if got := Calls(text); len(got) != 1 || got[0].Command != "c" || len(got[0].Args) != 0 {
		t.Errorf("Calls = %v", got)
	}
}
