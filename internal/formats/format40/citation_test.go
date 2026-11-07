package format40

import (
	"slices"
	"strings"
	"testing"
)

// TestLinksReadCitations: each of a citation's three forms reads as one link
// citing the id its title names; a link with no title, or with a title that
// is no node id, is an ordinary link.
func TestLinksReadCitations(t *testing.T) {
	for _, tc := range []struct {
		name, line, text, destination, id string
		cites                             bool
	}{
		{"quote", `See ["the bound is tight"](leaf.md#bounds "ana-012xyz").`, `"the bound is tight"`, "leaf.md#bounds", "ana-012xyz", true},
		{"paraphrase", `So [the bound holds](leaf.md#bounds "ana-012xyz") here.`, "the bound holds", "leaf.md#bounds", "ana-012xyz", true},
		{"naming", `[The Bound Is Tight](leaf.md#bounds "ana-012xyz")`, "The Bound Is Tight", "leaf.md#bounds", "ana-012xyz", true},
		{"a work, single-quoted", `[Smith 2020](../kb-summary.md#smith-2020 'wrk-s00000')`, "Smith 2020", "../kb-summary.md#smith-2020", "wrk-s00000", true},
		{"an axiom, angle-bracketed", `[Axiom S3](<axioms.md#axiom-s3-a-rule> "axm-s30000")`, "Axiom S3", "<axioms.md#axiom-s3-a-rule>", "axm-s30000", true},
		{"no title", "[the bound](leaf.md#bounds)", "the bound", "leaf.md#bounds", "", false},
		{"a title naming no node", `[the bound](leaf.md#bounds "see the bound")`, "the bound", "leaf.md#bounds", "", false},
		{"a title of an id's shape run on", `[the bound](leaf.md#bounds "ana-012xyz7")`, "the bound", "leaf.md#bounds", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			links := Links(tc.line)
			if len(links) != 1 {
				t.Fatalf("Links(%q) = %+v, want one link", tc.line, links)
			}
			l := links[0]
			if l.Text != tc.text || l.Destination != tc.destination || tc.line[l.DestStart:l.DestEnd] != tc.destination {
				t.Errorf("link %+v, want text %q and destination %q", l, tc.text, tc.destination)
			}
			id, cites := l.CitedID()
			if cites != tc.cites || cites && id != tc.id {
				t.Errorf("CitedID = %q, %t; want %q, %t", id, cites, tc.id, tc.cites)
			}
			if !tc.cites {
				return
			}
			rendered := Citation(tc.text, tc.destination, tc.id)
			back := Links(rendered)
			if len(back) != 1 || back[0].Text != tc.text || back[0].Destination != tc.destination || back[0].Title != tc.id {
				t.Errorf("Citation renders %q, which reads back as %+v", rendered, back)
			}
		})
	}
}

// TestBareIDs: an id of any kind standing in prose is bare, and so is a link
// whose whole text is an id, its token spanning the link; an id that is only
// part of a link's text is not, nor one in a statement marker, a citation's
// link title or a link's destination, nor one run into a longer word.
func TestBareIDs(t *testing.T) {
	for _, tc := range []struct {
		name, line string
		want       []string
	}{
		{"every kind", "clm-aaaaaa, ana-bbbbbb and exp-cccccc rest on wrk-s00000 and axm-s30000.", []string{"clm-aaaaaa", "ana-bbbbbb", "exp-cccccc", "wrk-s00000", "axm-s30000"}},
		{"a 3.0.0 work or axiom id is none", "wrk-smith2020 and axm-S3", nil},
		{"a link's text", "[clm-aaaaaa](leaf.md)", []string{"[clm-aaaaaa](leaf.md)"}},
		{"part of a link's text", "[see clm-aaaaaa](leaf.md)", nil},
		{"beside a link", "clm-aaaaaa and [see clm-bbbbbb](leaf.md) then [ana-cccccc](leaf.md#c)",
			[]string{"clm-aaaaaa", "[ana-cccccc](leaf.md#c)"}},
		{"a statement marker", "Holds." + StatementMarker("clm-aaaaaa") + StatementMarker("ana-aaaaaa"), nil},
		{"a citation's link title", `[One](leaf.md#one "clm-aaaaaa")`, nil},
		{"a link's destination", "[One](leaf.md#one-clm-aaaaaa)", nil},
		{"a reference definition", "[one]: leaf.md#one-clm-aaaaaa", nil},
		{"run into a word", "xclm-aaaaaa clm-aaaaaab", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, tok := range BareIDs(tc.line) {
				span := tc.line[tok.Start:tok.End]
				if span != tok.ID && !strings.HasPrefix(span, "["+tok.ID+"](") {
					t.Errorf("token %+v spans %q, neither its id nor a link whose text it is", tok, span)
				}
				got = append(got, span)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("BareIDs(%q) = %q, want %q", tc.line, got, tc.want)
			}
		})
	}
}

// TestBlankInlineCodeKeepsOffsets: a blanked span keeps its byte length, a
// multi-byte character in it included, so an offset into the result is one
// into the text.
func TestBlankInlineCodeKeepsOffsets(t *testing.T) {
	text := "a `é clm-aaaaaa` and $`x`$ then [c](d)\n`` x ` ``"
	got := BlankInlineCode(text)
	want := "a " + strings.Repeat(" ", len("`é clm-aaaaaa`")) + " and " + strings.Repeat(" ", len("$`x`$")) + " then [c](d)\n" +
		strings.Repeat(" ", len("`` x ` ``"))
	if got != want || len(got) != len(text) {
		t.Errorf("BlankInlineCode(%q) = %q, want %q", text, got, want)
	}
}
