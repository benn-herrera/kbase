package prompt

import (
	"errors"
	"strings"
	"testing"
)

func TestParseDefinition(t *testing.T) {
	for _, tc := range []struct {
		name string
		md   string
		want string // expected Critical
	}{
		{
			name: "no critical section",
			md:   "# Distiller\n\nTranslate the span faithfully.\n",
			want: "",
		},
		{
			name: "section runs to EOF",
			md:   "# Distiller\n\n## CRITICAL\n\nReproduce every fact.\nNever summarize.\n",
			want: "Reproduce every fact.\nNever summarize.",
		},
		{
			name: "section ends at next level-2 heading",
			md:   "## CRITICAL\n\nNever summarize.\n\n## Notes\n\nIgnored.\n",
			want: "Never summarize.",
		},
		{
			name: "section ends at next level-1 heading",
			md:   "## CRITICAL\n\nNever summarize.\n\n# Appendix\n\nIgnored.\n",
			want: "Never summarize.",
		},
		{
			name: "deeper headings stay inside the section",
			md:   "## CRITICAL\n\nNever summarize.\n\n### Exception\n\nQuoted text.\n",
			want: "Never summarize.\n\n### Exception\n\nQuoted text.",
		},
		{
			name: "list indentation is preserved",
			md:   "## CRITICAL\n\n  - never summarize\n  - never paraphrase\n",
			want: "  - never summarize\n  - never paraphrase",
		},
		{
			name: "heading match is case-sensitive",
			md:   "## Critical\n\nNot the section.\n",
			want: "",
		},
		{
			name: "heading match is exact, not a prefix",
			md:   "## CRITICALITY\n\nNot the section.\n",
			want: "",
		},
		{
			name: "indented heading is not a heading",
			md:   "  ## CRITICAL\n\nNot the section.\n",
			want: "",
		},
		{
			name: "empty section is legal and empty",
			md:   "## CRITICAL\n\n## Notes\n\nIgnored.\n",
			want: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			def, err := ParseDefinition(tc.md)
			if err != nil {
				t.Fatalf("ParseDefinition: %v", err)
			}
			if def.Critical != tc.want {
				t.Errorf("Critical = %q, want %q", def.Critical, tc.want)
			}
			// Slot 2 renders the definition as authored: the CRITICAL
			// section is extracted, never removed.
			if def.Body != tc.md {
				t.Errorf("Body was modified:\n got %q\nwant %q", def.Body, tc.md)
			}
		})
	}
}

func TestParseDefinitionMultipleCritical(t *testing.T) {
	md := "## CRITICAL\n\nOne.\n\n## Notes\n\n## CRITICAL\n\nTwo.\n"
	_, err := ParseDefinition(md)
	var target ErrMultipleCritical
	if !errors.As(err, &target) {
		t.Fatalf("err = %v, want ErrMultipleCritical", err)
	}
	if target.Count != 2 {
		t.Errorf("Count = %d, want 2", target.Count)
	}
}

func TestValidateDefinitionWordCap(t *testing.T) {
	atCap := "## CRITICAL\n\n" + strings.TrimSpace(strings.Repeat("word ", CriticalWordCap)) + "\n"
	overCap := "## CRITICAL\n\n" + strings.TrimSpace(strings.Repeat("word ", CriticalWordCap+1)) + "\n"

	if err := ValidateDefinition(atCap); err != nil {
		t.Errorf("definition at the cap should validate, got %v", err)
	}

	err := ValidateDefinition(overCap)
	var target ErrCriticalOverCap
	if !errors.As(err, &target) {
		t.Fatalf("err = %v, want ErrCriticalOverCap", err)
	}
	if target.Words != CriticalWordCap+1 || target.Cap != CriticalWordCap {
		t.Errorf("got %d words / cap %d, want %d / %d",
			target.Words, target.Cap, CriticalWordCap+1, CriticalWordCap)
	}
	if strings.Contains(err.Error(), "word word") {
		t.Error("error message must not echo the section content")
	}
}

// TestValidateDefinitionWordCountIsWhitespaceBased pins the documented word
// rule against scripts that do not space-separate: the cap counts what a
// human counts when they eyeball the section, not runes or bytes.
func TestValidateDefinitionWordCountIsWhitespaceBased(t *testing.T) {
	// Well over CriticalWordCap runes, three whitespace-separated words.
	long := strings.Repeat("日", 200) + " " + strings.Repeat("é", 200) + " " + strings.Repeat("x", 200)
	if got := wordCount(long); got != 3 {
		t.Fatalf("wordCount = %d, want 3", got)
	}
	if err := ValidateDefinition("## CRITICAL\n\n" + long + "\n"); err != nil {
		t.Errorf("multi-byte section under the word cap should validate, got %v", err)
	}
}

func TestValidateDefinitionPlaceholders(t *testing.T) {
	for _, tc := range []struct {
		name    string
		md      string
		wantErr bool
	}{
		{"well-formed placeholder", "Job: {{job_name}} for {{corpus}}.\n", false},
		{"single braces pass through", "Use `map[string]{int}` and {this}.\n", false},
		{"json body passes through", "Send {\"a\": 1} to the endpoint.\n", false},
		{"go template collides", "Example: {{if .X}}yes{{end}}\n", true},
		{"unclosed double brace", "Job: {{job_name\n", true},
		{"spaces inside placeholder", "Job: {{ job_name }}\n", true},
		{"empty placeholder", "Job: {{}}\n", true},
		{"triple brace", "Job: {{{job_name}}}\n", true},
		{"name starting with a digit", "Job: {{1job}}\n", true},
		{"dashed name", "Job: {{job-name}}\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateDefinition(tc.md)
			if tc.wantErr {
				var target ErrBadPlaceholder
				if !errors.As(err, &target) {
					t.Fatalf("err = %v, want ErrBadPlaceholder", err)
				}
				if strings.ContainsAny(target.Snippet, "\r\n") {
					t.Errorf("snippet %q must be single-line", target.Snippet)
				}
				if len(target.Snippet) > snippetLen {
					t.Errorf("snippet %q exceeds %d bytes", target.Snippet, snippetLen)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateDefinition: %v", err)
			}
		})
	}
}
