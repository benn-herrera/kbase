package prompt

import (
	"errors"
	"reflect"
	"testing"
)

func TestInterpolate(t *testing.T) {
	for _, tc := range []struct {
		name     string
		template string
		vars     map[string]string
		want     string
	}{
		{
			name:     "no placeholders and no vars",
			template: "Plain text.",
			vars:     nil,
			want:     "Plain text.",
		},
		{
			name:     "multiple placeholders",
			template: "Job {{job}} over {{corpus}}.",
			vars:     map[string]string{"job": "distill", "corpus": "guide"},
			want:     "Job distill over guide.",
		},
		{
			name:     "repeated placeholder uses one var",
			template: "{{job}}/{{job}}",
			vars:     map[string]string{"job": "distill"},
			want:     "distill/distill",
		},
		{
			name:     "single braces are content",
			template: "{keep} {{job}} {also keep}",
			vars:     map[string]string{"job": "distill"},
			want:     "{keep} distill {also keep}",
		},
		{
			name:     "empty value is a value",
			template: "[{{job}}]",
			vars:     map[string]string{"job": ""},
			want:     "[]",
		},
		{
			// Substitution is single-pass: braces in a value are content,
			// so a corpus fragment can never become a placeholder.
			name:     "value braces are not re-scanned",
			template: "{{job}}",
			vars:     map[string]string{"job": "{{corpus}}"},
			want:     "{{corpus}}",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Interpolate(tc.template, tc.vars)
			if err != nil {
				t.Fatalf("Interpolate: %v", err)
			}
			if got != tc.want {
				t.Errorf("Interpolate = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestInterpolateLoudBothDirections: a silent partial fill is how a prompt
// drifts from what its author believes it says, so both an unfilled
// placeholder and an unused variable are errors — reported together.
func TestInterpolateLoudBothDirections(t *testing.T) {
	for _, tc := range []struct {
		name        string
		template    string
		vars        map[string]string
		wantUnknown []string
		wantUnused  []string
	}{
		{
			name:        "unfilled placeholder",
			template:    "{{job}} over {{corpus}}",
			vars:        map[string]string{"job": "distill"},
			wantUnknown: []string{"corpus"},
		},
		{
			name:       "unused variable",
			template:   "{{job}}",
			vars:       map[string]string{"job": "distill", "corpus": "guide"},
			wantUnused: []string{"corpus"},
		},
		{
			name:        "both directions at once",
			template:    "{{job}} {{stage}}",
			vars:        map[string]string{"corpus": "guide", "job": "distill"},
			wantUnknown: []string{"stage"},
			wantUnused:  []string{"corpus"},
		},
		{
			name:        "repeated unfilled placeholder reports once",
			template:    "{{stage}}/{{stage}}",
			vars:        nil,
			wantUnknown: []string{"stage"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Interpolate(tc.template, tc.vars)
			var target ErrInterpolate
			if !errors.As(err, &target) {
				t.Fatalf("err = %v, want ErrInterpolate", err)
			}
			if got != "" {
				t.Errorf("failed interpolation must return no text, got %q", got)
			}
			if !reflect.DeepEqual(target.Unknown, tc.wantUnknown) {
				t.Errorf("Unknown = %v, want %v", target.Unknown, tc.wantUnknown)
			}
			if !reflect.DeepEqual(target.Unused, tc.wantUnused) {
				t.Errorf("Unused = %v, want %v", target.Unused, tc.wantUnused)
			}
		})
	}
}

func TestInterpolateMalformed(t *testing.T) {
	for _, tc := range []struct {
		name     string
		template string
	}{
		{"unclosed", "{{job"},
		{"spaces in name", "{{ job }}"},
		{"go template", "{{range .Files}}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Interpolate(tc.template, map[string]string{"job": "distill"})
			var target ErrBadPlaceholder
			if !errors.As(err, &target) {
				t.Fatalf("err = %v, want ErrBadPlaceholder", err)
			}
			if got != "" {
				t.Errorf("failed interpolation must return no text, got %q", got)
			}
		})
	}
}
