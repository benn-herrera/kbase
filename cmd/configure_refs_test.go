package main

import (
	"slices"
	"testing"

	"kbase/internal/config"
	"kbase/internal/model"
	"kbase/internal/model/modeltest"
)

// TestRunConfigureDialsEachProviderItsReferencesName: references naming two
// entries dial each entry's own endpoint, one request each, and need no
// --provider even where the pool holds more than one; the document then states
// no provider, as a null rather than by leaving the key out.
func TestRunConfigureDialsEachProviderItsReferencesName(t *testing.T) {
	opts := configureOptions{providerOptions: providerOptions{
		WriteDir: t.TempDir(),
		Providers: config.Providers{
			"a": provider("a", "http://a.example/v1"),
			"b": provider("b", "http://b.example/v1"),
		},
	}}
	opts.ModelMap = []string{"decision=a/d,text=b/t,scoring=a/s"}
	var dialled []string
	opts.NewClient = func(e model.Endpoint) model.Client {
		dialled = append(dialled, e.BaseURL)
		return modeltest.NewScriptedMock(nil, catalogue("d", "t", "s"))
	}
	got := runConfig(t, opts, nil)
	if got.code != 0 || got.doc.Outcome != "done" {
		t.Fatalf("configure over two referenced providers: exit %d, %v", got.code, got.doc.Values)
	}
	if want := []string{"http://a.example/v1", "http://b.example/v1"}; !slices.Equal(dialled, want) {
		t.Errorf("dialled %q, want each referenced provider once in role order", dialled)
	}
	if !slices.Contains(got.doc.Keys, "provider") || got.doc.Values["provider"] != nil {
		t.Errorf("keys %q, provider = %v: want the key present and null for a run naming two entries",
			got.doc.Keys, got.doc.Values["provider"])
	}
}
