package skeleton

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

func TestSlugGrammar(t *testing.T) {
	for _, tc := range []struct {
		name  string
		title string
		want  string
	}{
		{"plain", "Installation", "installation"},
		{"spaces collapse", "  The   Four-Scale   Framework ", "the-four-scale-framework"},
		{"a non-breaking space is a space", "Scale 2 Summary", "scale-2-summary"},
		{"digits survive", "Rojo 7 API", "rojo-7-api"},
		{"leading and trailing separators trimmed", "--- Setup ---", "setup"},
		{"an em dash is a word break", "§4.2: Scale 2 — Summary", "§4-2-scale-2-summary"},
		{"slashes separate, underscores do not", "src/lib_utils.md", "src-lib_utils-md"},
		{"link-hostile characters separate", "50% Off: What? #2", "50-off-what-2"},
		{"windows-illegal characters separate", `a<b>c|d*e"f`, "a-b-c-d-e-f"},
		{"generated part titles keep their ordinals", partTitle("Domain", 1, 2), "domain-1-2"},
		{"word cap", "one two three four five six seven eight nine ten", "one-two-three-four-five-six-seven-eight"},
		{"math symbols survive", "The σ₂ < 1 Condition", "the-σ₂-1-condition"},
		{"accents survive", "Café Résumé", "café-résumé"},
		{"only ascii case folds", "ÉCOLE Normale", "École-normale"},
		{"cjk survives", "日本語のドキュメント", "日本語のドキュメント"},
		{"kept punctuation survives", "!!! ???", "!!!"},
		{"an all-blacklisted title falls back", "/// ???", slugFallback},
		{"empty falls back", "", slugFallback},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Slug(tc.title); got != tc.want {
				t.Errorf("Slug(%q) = %q, want %q", tc.title, got, tc.want)
			}
		})
	}
}

// The byte cap is a BYTE cap over text that is no longer bytes-per-character.
// `日` is three bytes, so a cap of 64 lands mid-rune: 21 of them are 63
// bytes and 22 are 66. A cut that ignored rune boundaries would leave a
// dangling continuation byte in a filename — a byte sequence no filesystem,
// terminal or comparison agrees about.
func TestSlugTruncatesAtARuneBoundary(t *testing.T) {
	got := Slug(strings.Repeat("日", 30))
	if want := strings.Repeat("日", slugByteCap/3); got != want {
		t.Errorf("Slug of a long CJK title = %q, want %q", got, want)
	}
	if !utf8.ValidString(got) {
		t.Errorf("Slug of a long CJK title = %q, which is not valid UTF-8", got)
	}
}

// A slug is a filename and a link destination, so whatever the title did it
// must never carry a path separator, a character Windows refuses, one that a
// link renderer reads as structure, or an invisible rune.
func TestSlugIsAlwaysAFilename(t *testing.T) {
	for _, title := range []string{
		"../../etc/passwd", "a/b/c", "index.md", "  ", "C:\\Windows", "a\tb\nc",
		"σ₂ < 1", "Café", "日本語のドキュメント", "50% #frag", "not\xff utf8\xfe",
		strings.Repeat("verylongword", 20), strings.Repeat("日", 40), strings.Repeat("word ", 50),
	} {
		got := Slug(title)
		if got == "" {
			t.Errorf("Slug(%q) is empty", title)
		}
		if len(got) > slugByteCap {
			t.Errorf("Slug(%q) = %q is %d bytes, over the %d cap", title, got, len(got), slugByteCap)
		}
		if !utf8.ValidString(got) {
			t.Errorf("Slug(%q) = %q is not valid UTF-8", title, got)
		}
		// Spelled out rather than asking slugSeparator: a test that reuses
		// the predicate under test agrees with it even when it is wrong.
		for _, r := range got {
			hostile := strings.ContainsRune(slugHostile, r) ||
				unicode.IsSpace(r) || unicode.IsControl(r) || unicode.Is(unicode.Cf, r)
			if hostile {
				t.Errorf("Slug(%q) = %q contains %q, which a name may not carry", title, got, string(r))
			}
		}
		if strings.HasPrefix(got, "-") || strings.HasSuffix(got, "-") || strings.Contains(got, "--") {
			t.Errorf("Slug(%q) = %q has a stray separator", title, got)
		}
	}
}

// index.md is the file an index node renders to, so a leaf that slugged to
// `index` would render over its own parent. The namer reserves it, and the
// reservation is what makes the collision impossible rather than merely
// noticed.
func TestNamerReservesTheIndexAndEntryPointNames(t *testing.T) {
	nm := newNamer()
	if got := nm.claim(Slug("Index"), "", 1); got == "index" {
		t.Fatalf("namer handed out %q, which is the parent's own page", got)
	}
	if got := nm.claim(Slug("Entry Point"), "", 1); got == "entry-point" {
		t.Fatalf("namer handed out %q, which is the corpus root", got)
	}
}

func TestNamerDisambiguates(t *testing.T) {
	nm := newNamer()
	first := nm.claim("setup", "", 1)
	second := nm.claim("setup", "", 1)
	third := nm.claim("setup", "", 1)
	if first != "setup" {
		t.Fatalf("first claim = %q, want setup", first)
	}
	if second == first || third == first || second == third {
		t.Fatalf("collisions not disambiguated: %q %q %q", first, second, third)
	}
}

// §2.7: a node re-parented by chain collapse carries the slug of the container
// it came out of, and the namer tries it as a prefix before falling back to an
// ordinal — so the disambiguated name says where the node came from.
func TestNamerPrefersTheOriginPrefix(t *testing.T) {
	nm := newNamer()
	nm.claim("overview", "", 1)
	got := nm.claim("overview", "sync", 1)
	if got != "sync-overview" {
		t.Fatalf("re-parented claim = %q, want sync-overview", got)
	}
	// A second collision on the same origin still has to resolve.
	again := nm.claim("overview", "sync", 1)
	if again == "overview" || again == "sync-overview" {
		t.Fatalf("second re-parented claim collides: %q", again)
	}
}

// A split family's names are reserved together, so a sibling can never take a
// name one of the parts is going to need.
func TestNamerReservesTheWholeSplitFamily(t *testing.T) {
	nm := newNamer()
	base := nm.claim("guide", "", 3)
	if base != "guide" {
		t.Fatalf("family base = %q, want guide", base)
	}
	for k := 1; k <= 3; k++ {
		if got := nm.claim(partName(base, k), "", 1); got == partName(base, k) {
			t.Fatalf("a sibling took %q, which part %d needs", got, k)
		}
	}
}

// O-8: a part's name and title are functions of the group's title and the
// part's ordinal, and of nothing else. Stage 4 moves the boundaries inside a
// group, so a name that knew where a boundary landed would falsify the
// skeleton the moment refinement ran.
func TestPartNamesAreIndependentOfBoundaries(t *testing.T) {
	if got, want := partName("guide", 2), "guide-2"; got != want {
		t.Errorf("partName = %q, want %q", got, want)
	}
	if got, want := partTitle("Sync Details", 2, 3), "Sync Details (2/3)"; got != want {
		t.Errorf("partTitle = %q, want %q", got, want)
	}
}
