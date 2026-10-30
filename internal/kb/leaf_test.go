package kb

import (
	"maps"
	"reflect"
	"testing"
)

func TestLineMarkers(t *testing.T) {
	cases := []struct {
		name, line string
		want       []LineMarker
	}{
		{"none", "plain text", nil},
		{"hand-spaced", "a <!--claim-rigor:  clm-aaaaaa--> b", []LineMarker{{"clm-aaaaaa", 2, 33}}},
		{"two markers", "x <!-- claim-rigor: clm-aaaaaa --> <!-- claim-rigor: ana-bbbbbb -->",
			[]LineMarker{{"clm-aaaaaa", 2, 34}, {"ana-bbbbbb", 35, 67}}},
		{"one marker, two ids", "a <!-- claim-rigor: clm-aaaaaa ana-bbbbbb -->",
			[]LineMarker{{"clm-aaaaaa", 2, 45}, {"ana-bbbbbb", 2, 45}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := LineMarkers(c.line)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("LineMarkers(%q) = %v, want %v", c.line, got, c.want)
			}
		})
	}
}

func TestStripLineMarkers(t *testing.T) {
	cases := []struct{ name, line, want string }{
		{"mid-line", "a <!-- claim-rigor: clm-aaaaaa --> b", "a b"},
		{"tab before", "a\t<!-- claim-rigor: clm-aaaaaa -->", "a"},
		{"hand-spaced", "a <!--claim-rigor:  clm-aaaaaa-->", "a"},
		{"several ids", "a <!-- claim-rigor: clm-aaaaaa ana-bbbbbb -->", "a"},
		{"two markers", "x <!-- claim-rigor: clm-aaaaaa --> <!-- claim-rigor: ana-bbbbbb -->", "x"},
		{"none", "plain", "plain"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := StripLineMarkers(c.line); got != c.want {
				t.Errorf("StripLineMarkers(%q) = %q, want %q", c.line, got, c.want)
			}
		})
	}
}

func TestHosts(t *testing.T) {
	g := Graph{
		Claims:      []ClaimNode{{ID: "clm-aaaaaa"}, {ID: "clm-bbbbbb"}},
		Analyses:    []AnalysisNode{{ID: "ana-cccccc"}},
		Experiments: []ExperimentNode{{ID: "exp-dddddd"}},
	}
	leaves := map[string]string{
		"v/z.md": "# Z\n<!-- claim-rigor: ana-cccccc -->\nA claim. <!-- claim-rigor: clm-aaaaaa -->\n",
		"v/a.md": "# A\n\n<!-- claim-rigor: ana-cccccc -->\n<!-- claim-rigor: clm-eeeeee -->\n",
		"v/m.md": "# M\nOne. <!-- claim-rigor: clm-bbbbbb -->\nTwo. <!-- claim-rigor: clm-bbbbbb -->\n",
	}
	hosts, faults := Hosts(g, leaves)
	wantHosts := map[string]string{"clm-aaaaaa": "v/z.md", "clm-bbbbbb": "v/m.md", "ana-cccccc": "v/a.md"}
	if !maps.Equal(hosts, wantHosts) {
		t.Errorf("hosts = %v, want %v", hosts, wantHosts)
	}
	wantFaults := []HostFault{
		{ID: "clm-bbbbbb", Markers: []MarkerSite{{"v/m.md", 1}, {"v/m.md", 2}}},
		{ID: "ana-cccccc", Markers: []MarkerSite{{"v/a.md", 2}, {"v/z.md", 1}}},
		{ID: "exp-dddddd"},
	}
	if !reflect.DeepEqual(faults, wantFaults) {
		t.Errorf("faults = %+v, want %+v", faults, wantFaults)
	}
}

func TestDocumentKind(t *testing.T) {
	for _, tc := range []struct {
		path        string
		hasChildren bool
		want        string
	}{
		{EntryPointFile, true, DocumentEntryPoint},
		{EntryPointFile, false, DocumentEntryPoint},
		{"v/index.md", true, DocumentIndex},
		{"v/leaf.md", true, DocumentIndex},
		{"v/index.md", false, DocumentLeaf},
		{"v/" + EntryPointFile, false, DocumentLeaf},
	} {
		if got := DocumentKind(tc.path, tc.hasChildren); got != tc.want {
			t.Errorf("DocumentKind(%q, %v) = %q, want %q", tc.path, tc.hasChildren, got, tc.want)
		}
	}
}
