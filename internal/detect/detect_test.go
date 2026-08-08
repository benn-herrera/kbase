package detect

import (
	"slices"
	"testing"
)

// bucket names one Result field, so a table row can say where an id
// belongs without repeating four slices per case.
type bucket int

const (
	heavy bucket = iota
	light
	unclassified
	other
)

func (b bucket) String() string {
	switch b {
	case heavy:
		return "Heavy"
	case light:
		return "Light"
	case unclassified:
		return "UnclassifiedFamily"
	default:
		return "Others"
	}
}

// TestClassifyOne covers the matching grammar one id at a time: each case
// asserts the id lands in its bucket and, implicitly, in no other.
func TestClassifyOne(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want bucket
	}{
		// Naming variance the matcher must tolerate (SPEC.md §1).
		{"provider-prefixed dense", "google/gemma-4-31b-it", heavy},
		{"fused family, colon separator", "gemma4:31b-a4b", light}, // 31b and a4b both present: a4b wins — it is the MoE marker, and the MoE is named by both its total and active parameter counts
		{"canonical MoE", "gemma-4-26b-a4b", light},
		{"mixed case", "Gemma-4-31B-Instruct", heavy},
		{"prefixed, suffixed, quantized MoE", "some-org/gemma-4-26b-a4b-it-q4", light},
		{"underscore separators", "gemma_4_31b_it", heavy},

		// The MoE named by its total parameter count alone.
		{"bare MoE total size", "gemma-4-26b", light},
		{"prefixed, suffixed, quantized MoE total size", "org/gemma-4-26b-it-q4", light},

		// Family members the tier table does not cover: recognized, but
		// never assigned a tier by inference.
		{"uncovered size", "gemma-4-9b", unclassified},
		{"embedding variant", "gemma-4-embed", unclassified},

		// Non-members. Each is a near miss that a looser matcher admits.
		{"previous generation", "gemma-3-27b-it", other},
		{"digits fused into one token", "gemma34b", other},
		{"other family, same version", "llama-4-31b", other},
		{"unrelated model", "text-embedding-3-large", other},
		{"parameter count, not version", "gemma-4b-it", other},
		{"family-named organization", "gemma-labs/llama-3-8b", other},
		{"empty id", "", other},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := Classify([]string{tt.id})
			got := map[bucket][]string{
				heavy: res.Heavy, light: res.Light,
				unclassified: res.UnclassifiedFamily, other: res.Others,
			}
			for b, ids := range got {
				want := []string(nil)
				if b == tt.want {
					want = []string{tt.id}
				}
				if !slices.Equal(ids, want) {
					t.Errorf("%s = %v, want %v", b, ids, want)
				}
			}
		})
	}
}

// TestClassifyCatalogue: a realistic mixed catalogue sorts into all four
// buckets at once, each preserving the order the provider served.
func TestClassifyCatalogue(t *testing.T) {
	res := Classify([]string{
		"text-embedding-3-large",
		"google/gemma-4-31b-it",
		"gemma-4-26b-a4b",
		"gemma-3-27b-it",
		"gemma-4-embed",
		"gemma4:31b-a4b",
		"Gemma-4-31B-Instruct",
	})

	if want := []string{"google/gemma-4-31b-it", "Gemma-4-31B-Instruct"}; !slices.Equal(res.Heavy, want) {
		t.Errorf("Heavy = %v, want %v", res.Heavy, want)
	}
	if want := []string{"gemma-4-26b-a4b", "gemma4:31b-a4b"}; !slices.Equal(res.Light, want) {
		t.Errorf("Light = %v, want %v", res.Light, want)
	}
	if want := []string{"gemma-4-embed"}; !slices.Equal(res.UnclassifiedFamily, want) {
		t.Errorf("UnclassifiedFamily = %v, want %v", res.UnclassifiedFamily, want)
	}
	if want := []string{"text-embedding-3-large", "gemma-3-27b-it"}; !slices.Equal(res.Others, want) {
		t.Errorf("Others = %v, want %v", res.Others, want)
	}
}

// TestClassifyEmpty: no catalogue is not a classification failure — every
// bucket is empty, and the caller's tier resolution reports the no-match.
func TestClassifyEmpty(t *testing.T) {
	if res := Classify(nil); len(res.Heavy)+len(res.Light)+len(res.UnclassifiedFamily)+len(res.Others) != 0 {
		t.Errorf("Classify(nil) = %+v, want every bucket empty", res)
	}
}
