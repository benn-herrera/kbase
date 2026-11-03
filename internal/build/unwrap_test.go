package build

import (
	"os"
	"testing"
)

// TestAWrappedCrossReferenceIsUnwrappedToItsInnerLink: the link pandoc wraps
// around a cross-reference, its destination the bare LaTeX label, is replaced
// by its text, so the cross-reference inside it — which links the target
// document's heading — is the one link left; the prose around it stands.
func TestAWrappedCrossReferenceIsUnwrappedToItsInnerLink(t *testing.T) {
	data, err := os.ReadFile("../export/testdata/excerpts/link-around-anchor.md")
	if err != nil {
		t.Fatal(err)
	}
	inner := `<a href="../appendix-omitted-details-in-appendix-6-1.md#appendix-omitted-details-in-appendix-61" data-reference-type="ref"` +
		"\n" + `data-reference="apx:omitted_details">7</a>`
	want := "Appendix " + inner + " proves\nthis inequality based on direct calculation.\n"
	got := unwrapReferences(string(data))
	if got != want {
		t.Errorf("unwrapped:\n%s\nwant:\n%s", got, want)
	}
	if again := unwrapReferences(got); again != got {
		t.Errorf("unwrapping again changed the text:\n%s", again)
	}
}
