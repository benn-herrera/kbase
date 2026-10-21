package harness

import (
	"path/filepath"
	"strings"
	"testing"
)

// probe is a probed command over a program every supported host has, so the
// token arithmetic is tested without a process of its own.
func probe(t *testing.T, argv ...string) *Command {
	t.Helper()
	c, err := New(argv, nil)
	if err != nil {
		t.Fatalf("New(%q): %v", argv, err)
	}
	return c
}

// TestItemFillsThePromptWhereTheCommandNamesIt: {prompt} is replaced in the one
// element that names it and nowhere else, the command is not
// rewritten, and the argv-borne prompt leaves stdin empty.
func TestItemFillsThePromptWhereTheCommandNamesIt(t *testing.T) {
	c := probe(t, "sh", "-c", "true", "--message", "{prompt}", "{model}")
	item, err := c.Item("say {prompt} back")
	if err != nil {
		t.Fatalf("Item: %v", err)
	}
	got := item.Argv()
	want := []string{"sh", "-c", "true", "--message", "say {prompt} back", "{model}"}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("argv = %q, want %q: the prompt lands once, and an unknown token stands", got, want)
	}
	if item.prompt != "" {
		t.Errorf("an argv-borne prompt is also held for stdin: %q", item.prompt)
	}
	if prof := c.Argv(); strings.Join(prof, " ") != "sh -c true --message {prompt} {model}" {
		t.Errorf("the probed command was rewritten: %q", prof)
	}
}

// TestItemHoldsThePromptForStdin: a command naming no {prompt} receives it on
// stdin, and the argv is the command's unchanged.
func TestItemHoldsThePromptForStdin(t *testing.T) {
	c := probe(t, "sh", "-c", "true")
	item, err := c.Item("the item's text")
	if err != nil {
		t.Fatalf("Item: %v", err)
	}
	if item.prompt != "the item's text" {
		t.Errorf("the prompt held for stdin = %q", item.prompt)
	}
	if got := strings.Join(item.Argv(), " "); got != "sh -c true" {
		t.Errorf("argv = %q, want the command's own", got)
	}
	// {prompt} is decided once, at the command: the item is not rescanned for
	// a token the command does not carry.
	if strings.Contains(strings.Join(item.Argv(), " "), "the item's text") {
		t.Error("a prompt reached an argv naming no token")
	}
}

// TestItemRefusesWhatThePromptCannotBe: a prompt the argv cannot carry, one past
// the bound, and an argv naming the token twice fail the item before a process
// starts.
func TestItemRefusesWhatThePromptCannotBe(t *testing.T) {
	for _, tc := range []struct {
		name    string
		command []string
		prompt  string
		want    string
	}{
		{"a NUL", []string{"sh", "-c", "true", "{prompt}"}, "a\x00b", "NUL"},
		{"a long prompt", []string{"sh", "-c", "true", "{prompt}"}, strings.Repeat("x", PromptLimit+1), "over the"},
		{"a long prompt on stdin", []string{"sh", "-c", "true"}, strings.Repeat("x", PromptLimit+1), "over the"},
		{"two elements", []string{"sh", "{prompt}", "{prompt}"}, "p", "in 2 elements"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := probe(t, tc.command...).Item(tc.prompt)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Item = %v, want a refusal naming %q", err, tc.want)
			}
		})
	}
	if _, err := probe(t, "sh").Item(strings.Repeat("x", PromptLimit)); err != nil {
		t.Errorf("a prompt at the bound = %v, want it accepted", err)
	}
}

// TestNewProbesTheProgram: a program that is not there is refused at setup, and
// a command naming no program at all with it.
func TestNewProbesTheProgram(t *testing.T) {
	if _, err := New([]string{"kbase-no-such-harness-program", "--ask"}, nil); err == nil {
		t.Error("a program that does not stand was accepted")
	}
	if _, err := New(nil, nil); err == nil {
		t.Error("an empty command was accepted")
	}
	c, err := New([]string{"sh"}, nil)
	if err != nil {
		t.Fatalf("New(sh) = %v, want the probe to find it", err)
	}
	// The probe's answer is the path an item starts, and an item runs elsewhere:
	// a program named relative to kbase's directory would name another program
	// there, so what the probe certified is held absolutely.
	if !filepath.IsAbs(c.program) {
		t.Errorf("program = %q, want the probed path resolved absolutely", c.program)
	}
	if got := c.Argv(); len(got) != 1 || got[0] != "sh" {
		t.Errorf("argv = %q, want the command's own argv[0] recorded", got)
	}
}

// TestNewRefusesThePromptTwiceInOneElement: a command element naming {prompt}
// twice would hand the prompt to the command twice, so it fails setup rather
// than every item — as the two-element form already does.
func TestNewRefusesThePromptTwiceInOneElement(t *testing.T) {
	for _, element := range []string{"{prompt}{prompt}", "--message={prompt} {prompt}"} {
		_, err := New([]string{"sh", "-c", "true", element}, nil)
		if err == nil || !strings.Contains(err.Error(), "once") {
			t.Errorf("New over element %q = %v, want a refusal naming the token's one place", element, err)
		}
	}
	// One element, one token: the item's case, and New still accepts it.
	if _, err := New([]string{"sh", "-c", "true", "{prompt}"}, nil); err != nil {
		t.Errorf("New over one {prompt} = %v, want it accepted", err)
	}
}
