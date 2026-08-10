package markdown

import (
	"slices"
	"strings"
	"testing"

	"kbase/internal/survey"
)

// TestFrontMatter: the block is recorded as the artifact's metadata byte
// range and excluded from the parse, and the bytes before the first heading
// remain accounted for.
func TestFrontMatter(t *testing.T) {
	corpus, art, lg := surveyFixturesLogged(t, fixtures)
	u, _ := corpus.Unit("front.md")
	f := fileOf(t, art, "front.md")

	if f.Metadata == nil {
		t.Fatal("front matter block was not detected")
	}
	got := string(u.Bytes[f.Metadata.Start:f.Metadata.End])
	want := "---\ntitle: Front Matter Doc\ndescription: What this document is for\ntags: [a, b]\n---\n"
	if got != want {
		t.Errorf("metadata range holds %q, want %q", got, want)
	}
	if f.Preamble == nil || f.Preamble.Start != f.Metadata.End {
		t.Errorf("preamble must resume where the front matter ends; got %+v", f.Preamble)
	}
	if plain := fileOf(t, art, "front-unterminated.md"); plain.Metadata != nil {
		t.Errorf("an unterminated block is not front matter; got %+v", plain.Metadata)
	}
	if !lg.has(t, "debug", "file", "front.md") {
		t.Error("a front-matter decision must leave a debug record naming the file")
	}
}

// TestFrontMatterRejection: a block that is delimited like front matter but
// does not parse as a YAML MAPPING is content. This is the gate that keeps a
// document opening with a thematic break from losing everything above its
// second rule into an opaque range — a loss no tiling check can see, because
// an opaque range tiles perfectly well.
func TestFrontMatterRejection(t *testing.T) {
	corpus, art, lg := surveyFixturesLogged(t, fixtures)

	for _, path := range []string{"front-thematic.md", "front-blank-first.md", "plain-bom.md"} {
		if f := fileOf(t, art, path); f.Metadata != nil {
			t.Errorf("%s: recorded metadata %+v, want none", path, f.Metadata)
		}
	}
	if !lg.has(t, "debug", "file", "front-thematic.md") {
		t.Error("a rejected block must leave a debug record naming the file")
	}

	// The rejected bytes stay in the parse: the prose between the rules is
	// still the file's preamble, and the whole file is still tiled.
	f := fileOf(t, art, "front-thematic.md")
	u, _ := corpus.Unit("front-thematic.md")
	if f.Preamble == nil || f.Preamble.Start != 0 {
		t.Fatalf("preamble must start at byte 0 when there is no front matter; got %+v", f.Preamble)
	}
	if !strings.Contains(string(u.Bytes[f.Preamble.Start:f.Preamble.End]), "delimiter-only scan") {
		t.Error("the rejected block's prose must remain inside the surveyed preamble")
	}
}

// TestFrontMatterFields: the labels stage 3 routes on are lifted out of the
// block, because stage 3 consumes the survey and never the source. A block
// that parses but carries none of them yields none, and no error.
func TestFrontMatterFields(t *testing.T) {
	_, art := surveyFixtures(t, fixtures)

	f := fileOf(t, art, "front.md")
	if f.Title != "Front Matter Doc" {
		t.Errorf("title = %q", f.Title)
	}
	if f.Description != "What this document is for" {
		t.Errorf("description = %q", f.Description)
	}
	if got := strings.Join(f.Tags, ","); got != "a,b" {
		t.Errorf("tags = %q, want %q", got, "a,b")
	}
	if bom := fileOf(t, art, "front-bom.md"); bom.Title != "BOM Doc" {
		t.Errorf("a BOM must not cost the file its title; got %q", bom.Title)
	}
	if empty := fileOf(t, art, "front-empty.md"); empty.Title != "" || empty.Tags != nil {
		t.Errorf("an empty block carries no labels; got %+v", empty)
	}

	for _, tc := range []struct {
		name, body string
		want       survey.File
	}{{
		name: "a mapping without the keys yields no fields",
		body: "---\nsidebar_position: 3\n---\n\n# H\n",
	}, {
		name: "a mis-shaped optional value costs only that value",
		body: "---\ntitle: Kept\ntags: sometimes\n---\n\n# H\n",
		want: survey.File{Title: "Kept"},
	}, {
		name: "values are collapsed and capped like gists",
		body: "---\ntitle: |\n  wrapped\n  across lines\ntags: [\"  spaced  \", \"\"]\n---\n\n# H\n",
		want: survey.File{Title: "wrapped across lines", Tags: []string{"spaced"}},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			f := surveyOne(t, "fm.md", tc.body)
			if f.Metadata == nil {
				t.Fatal("block was not recorded as the file's metadata range")
			}
			if f.Title != tc.want.Title || f.Description != tc.want.Description {
				t.Errorf("title/description = %q/%q, want %q/%q", f.Title, f.Description, tc.want.Title, tc.want.Description)
			}
			if !slices.Equal(f.Tags, tc.want.Tags) {
				t.Errorf("tags = %v, want %v", f.Tags, tc.want.Tags)
			}
		})
	}
}
