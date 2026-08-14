package ingest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/log"
	"kbase/internal/log/logtest"
)

// writeFile creates dir/name (parents included) with the given contents.
func writeFile(t *testing.T, root, name, body string) string {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", name, err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

// mdExts is the extension set most cases walk with. It is spelled out here
// rather than imported from the Markdown adapter: this package knows nothing
// about formats, and a test that borrowed the adapter's constant would be
// asserting the adapter's opinion instead of the walk's behavior.
var mdExts = []string{".md"}

// paths lists the ingested document ids in order, which is the order tests assert
// against: sorted, slash-separated, relative to the root.
func paths(c Corpus) []string {
	out := make([]string, 0, len(c.Docs))
	for _, u := range c.Docs {
		out = append(out, u.Path)
	}
	return out
}

// TestWalkSelection covers what the walk collects and what it leaves
// alone: nested Markdown in path order, a case-variant extension, and the
// three categories that are not documents (dot-directories, dot-files,
// non-Markdown files).
func TestWalkSelection(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Index\n")
	writeFile(t, root, "guide/setup.md", "# Setup\n")
	writeFile(t, root, "guide/deep/nested.MD", "# Nested\n")
	writeFile(t, root, "notes.txt", "not markdown\n")
	writeFile(t, root, ".hidden/secret.md", "# Hidden\n")
	writeFile(t, root, ".dotfile.md", "# Dotfile\n")
	writeFile(t, root, "guide/.draft.md", "# Draft\n")

	lg := &logtest.Capture{}
	c, err := Walk(root, mdExts, lg)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	want := []string{"guide/deep/nested.MD", "guide/setup.md", "index.md"}
	if got := paths(c); !slices.Equal(got, want) {
		t.Errorf("docs: got %v, want %v", got, want)
	}
	if !c.Has("guide/setup.md") || c.Has("notes.txt") {
		t.Error("Has must answer for ingested paths only")
	}

	// Every skip is silent in the corpus and visible in the log — the
	// "why is that file not in my knowledge base" channel.
	for _, skipped := range []string{".hidden", ".dotfile.md", "guide/.draft.md", "notes.txt"} {
		if !lg.Has(t, "debug", "path", skipped) {
			t.Errorf("no debug record for skipped entry %q", skipped)
		}
	}
}

// TestWalkExtensions: which files are documents is the caller's format
// knowledge, so the walk honours the set it is given — several extensions at
// once, case-insensitively — and refuses an empty one rather than walking a
// tree it can collect nothing from.
func TestWalkExtensions(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "doc.md", "# Doc\n")
	writeFile(t, root, "paper.tex", "\\section{Paper}\n")
	writeFile(t, root, "shout.TEX", "\\section{Shout}\n")
	writeFile(t, root, "notes.txt", "neither\n")

	c, err := Walk(root, []string{".md", ".tex"}, log.Discard())
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	want := []string{"doc.md", "paper.tex", "shout.TEX"}
	if got := paths(c); !slices.Equal(got, want) {
		t.Errorf("docs: got %v, want %v", got, want)
	}

	if _, err := Walk(root, nil, log.Discard()); err == nil {
		t.Error("an empty extension set must be refused")
	} else if !strings.Contains(err.Error(), "at least one") {
		t.Errorf("error = %v, want it to say the caller must name an extension", err)
	}

	_, err = Walk(root, []string{".rst"}, log.Discard())
	if err == nil || !strings.Contains(err.Error(), "no .rst files") {
		t.Errorf("error = %v, want it to name the extensions it looked for", err)
	}
}

// TestFoldedPath: link resolution has to reach a `.MD` document written as
// `.md`, and has to refuse when a fold cannot pick between two documents.
func TestFoldedPath(t *testing.T) {
	c, err := New([]SourceDoc{
		{Path: "guide/Setup.MD", Bytes: []byte("s")},
		{Path: "twin.md", Bytes: []byte("a")},
		{Path: "TWIN.md", Bytes: []byte("b")},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for _, tc := range []struct {
		name, in, wantID string
		wantAmbiguous    bool
	}{
		{name: "case-variant extension", in: "guide/setup.md", wantID: "guide/Setup.MD"},
		{name: "case-variant path", in: "GUIDE/SETUP.MD", wantID: "guide/Setup.MD"},
		{name: "exact spelling still folds to itself", in: "guide/Setup.MD", wantID: "guide/Setup.MD"},
		{name: "absent", in: "guide/nope.md"},
		{name: "two documents fold to one key", in: "twin.md", wantAmbiguous: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id, ambiguous := c.FoldedPath(tc.in)
			if id != tc.wantID || ambiguous != tc.wantAmbiguous {
				t.Errorf("FoldedPath(%q) = (%q, %v), want (%q, %v)", tc.in, id, ambiguous, tc.wantID, tc.wantAmbiguous)
			}
		})
	}
}

// TestWalkCustody: the bytes under custody are the file's bytes, and the
// digest is over exactly those bytes. Unicode normalization is the ONE
// transform (see TestNFCPrePass); everything else is left alone. CRLF and a
// missing trailing newline are the cases a "helpful" reader would silently
// fix — fixing them would invalidate every byte offset the survey records.
func TestWalkCustody(t *testing.T) {
	const body = "# Title\r\n\r\ntext without trailing newline"
	root := t.TempDir()
	writeFile(t, root, "doc.md", body)

	c, err := Walk(root, mdExts, log.Discard())
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	u := c.Docs[0]
	if string(u.Bytes) != body {
		t.Errorf("custody bytes = %q, want %q", u.Bytes, body)
	}
	sum := sha256.Sum256([]byte(body))
	if want := hex.EncodeToString(sum[:]); u.SHA256 != want {
		t.Errorf("SHA256 = %q, want %q", u.SHA256, want)
	}
}

// The two spellings of the same document. Both are written out of explicit
// code points rather than pasted literals: an invisible combining mark in
// source is a test nobody can review.
const (
	combiningAcute = string(rune(0x0301)) // the decomposed accent
	eAcute         = string(rune(0x00e9)) // é, the composed one
	iAcute         = string(rune(0x00ed)) // í
)

// NFD is what a macOS filesystem and several editors hand back; NFC is what
// most of the world writes. They render identically and no reader can tell
// them apart.
const (
	nfdBody = "# Cafe" + combiningAcute + " Re" + combiningAcute + "sume" + combiningAcute + "\n\nsi" + combiningAcute + "\n"
	nfcBody = "# Caf" + eAcute + " R" + eAcute + "sum" + eAcute + "\n\ns" + iAcute + "\n"
)

// TestNFCPrePass: custody bytes are NFC bytes, and provenance keeps both
// digests. The payoff is the last assertion — two corpora that differ only in
// Unicode spelling have ONE identity, so every hash, title, slug and
// comparison downstream sees one document rather than two.
func TestNFCPrePass(t *testing.T) {
	t.Run("normalizes and records both hashes", func(t *testing.T) {
		c, err := New([]SourceDoc{{Path: "doc.md", Bytes: []byte(nfdBody)}})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		u := c.Docs[0]
		if string(u.Bytes) != nfcBody {
			t.Errorf("custody bytes = %q, want the NFC spelling %q", u.Bytes, nfcBody)
		}
		if want := hexDigest(nfcBody); u.SHA256 != want {
			t.Errorf("SHA256 = %q, want the custody digest %q", u.SHA256, want)
		}
		if want := hexDigest(nfdBody); u.UploadSHA256 != want {
			t.Errorf("UploadSHA256 = %q, want the digest of the bytes as read %q", u.UploadSHA256, want)
		}
		if !u.BytesNormalized() {
			t.Error("BytesNormalized() = false for a document the pre-pass rewrote")
		}
	})

	t.Run("already-NFC content is untouched and records no transform", func(t *testing.T) {
		// The common case, ASCII included: no transform happened, so the two
		// digests must be the same string and not merely both present.
		for _, body := range []string{nfcBody, "# Plain ASCII\n", ""} {
			c, err := New([]SourceDoc{{Path: "doc.md", Bytes: []byte(body)}})
			if err != nil {
				t.Fatalf("New(%q): %v", body, err)
			}
			u := c.Docs[0]
			if string(u.Bytes) != body {
				t.Errorf("custody bytes = %q, want %q unchanged", u.Bytes, body)
			}
			if u.SHA256 != u.UploadSHA256 || u.BytesNormalized() {
				t.Errorf("%q: hashes must be equal when nothing was transformed: %q vs %q",
					body, u.SHA256, u.UploadSHA256)
			}
		}
	})

	t.Run("invalid utf-8 is never touched", func(t *testing.T) {
		// Normalizing bytes that are not UTF-8 would be a guess about an
		// encoding this package cannot verify, and a guess that rewrites
		// custody is the one thing custody exists to prevent.
		body := "# Title\n\n\xff\xfe not utf-8 \x80\n"
		c, err := New([]SourceDoc{{Path: "doc.md", Bytes: []byte(body)}})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		u := c.Docs[0]
		if string(u.Bytes) != body {
			t.Errorf("custody bytes = %q, want the bytes as read %q", u.Bytes, body)
		}
		if u.BytesNormalized() {
			t.Error("BytesNormalized() = true for bytes the pre-pass must not have touched")
		}
	})

	t.Run("both spellings reach one corpus identity", func(t *testing.T) {
		nfd, err := New([]SourceDoc{{Path: "doc.md", Bytes: []byte(nfdBody)}})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		nfc, err := New([]SourceDoc{{Path: "doc.md", Bytes: []byte(nfcBody)}})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if nfd.ContentHash != nfc.ContentHash {
			t.Errorf("content hash differs by Unicode spelling alone: %q vs %q",
				nfd.ContentHash, nfc.ContentHash)
		}
		if nfd.Docs[0].UploadSHA256 == nfc.Docs[0].UploadSHA256 {
			t.Error("UploadSHA256 must still tell the two uploads apart")
		}
	})
}

// The two spellings of one document NAME, built the same explicit way the
// body constants are. A filesystem picks the spelling with no more author
// involvement than an editor picks the one in the text.
const (
	nfdName = "guide/caf" + "e" + combiningAcute + ".md"
	nfcName = "guide/caf" + eAcute + ".md"
)

// TestNFCPathPrePass: the id is normalized on the same terms the bytes are.
// The payoff is the same too — one document has ONE identity however the
// filesystem spelled its name — but ordering makes it sharper here: path
// order is the corpus's canonical order, so the order has to be over the ids.
func TestNFCPathPrePass(t *testing.T) {
	t.Run("normalizes the id and keeps the spelling as given", func(t *testing.T) {
		c, err := New([]SourceDoc{{Path: nfdName, Bytes: []byte("x")}})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		u := c.Docs[0]
		if u.Path != nfcName {
			t.Errorf("Path = %+q, want the NFC spelling %+q", u.Path, nfcName)
		}
		if u.UploadPath != nfdName {
			t.Errorf("UploadPath = %+q, want the path as given %+q", u.UploadPath, nfdName)
		}
		if !u.PathNormalized() {
			t.Error("PathNormalized() = false for a path the pre-pass rewrote")
		}
		// The id is the id: the corpus answers to it and to nothing else.
		if !c.Has(nfcName) || c.Has(nfdName) {
			t.Error("the corpus must answer for the NFC id, not for the spelling it was handed")
		}
	})

	t.Run("an already-NFC id records no transform", func(t *testing.T) {
		c, err := New([]SourceDoc{{Path: nfcName, Bytes: []byte("x")}, {Path: "plain.md", Bytes: []byte("y")}})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		for _, u := range c.Docs {
			if u.Path != u.UploadPath || u.PathNormalized() {
				t.Errorf("%+q: path fields must be equal when nothing was transformed, got %+q",
					u.Path, u.UploadPath)
			}
		}
	})

	t.Run("order is over the ids, not the spellings", func(t *testing.T) {
		// The witness: `cafe` + combining acute sorts BEFORE `cafz.md` on its
		// source bytes (`e` < `z`) and AFTER it on its NFC id (the composed
		// e-acute starts 0xc3). Normalizing after the sort would leave the
		// corpus — and the content hash, which reads this order — spelling-
		// dependent.
		c, err := New([]SourceDoc{
			{Path: nfdName, Bytes: []byte("x")},
			{Path: "guide/cafz.md", Bytes: []byte("y")},
		})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if got, want := paths(c), []string{"guide/cafz.md", nfcName}; !slices.Equal(got, want) {
			t.Errorf("docs: got %+q, want %+q (sorted by the NFC id)", got, want)
		}
	})

	t.Run("both spellings reach one corpus identity", func(t *testing.T) {
		hash := func(name string) string {
			t.Helper()
			c, err := New([]SourceDoc{
				{Path: name, Bytes: []byte("body")},
				{Path: "guide/cafz.md", Bytes: []byte("other")},
			})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			return c.ContentHash
		}
		if hash(nfdName) != hash(nfcName) {
			t.Error("content hash differs by the Unicode spelling of a filename alone")
		}
	})

	t.Run("two spellings of one name are one document, and refused as two", func(t *testing.T) {
		_, err := New([]SourceDoc{
			{Path: nfdName, Bytes: []byte("x")},
			{Path: nfcName, Bytes: []byte("y")},
		})
		if err == nil {
			t.Fatal("two spellings of one path must be refused as a duplicate")
		}
		// Both spellings render identically, so an error that names only the
		// id would be unactionable; it has to show the code points.
		if !strings.Contains(err.Error(), "two Unicode spellings") {
			t.Errorf("error = %v, want it to say the two paths are one name in two spellings", err)
		}
		// The assertion is over the ESCAPED spelling the error was asked to
		// print, constructed here rather than pasted: a combining acute in a
		// source literal is invisible to a reviewer.
		for _, want := range []string{fmt.Sprintf("%+q", nfdName), fmt.Sprintf("%+q", nfcName)} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error = %v, want it to contain %s, the form that tells the spellings apart", err, want)
			}
		}
	})

	t.Run("an id that is not utf-8 is never touched", func(t *testing.T) {
		// A filesystem may hand back a name that is raw bytes. Normalizing it
		// would be a guess about an encoding this package cannot verify.
		raw := "guide/\xff\xfe.md"
		c, err := New([]SourceDoc{{Path: raw, Bytes: []byte("x")}})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		u := c.Docs[0]
		if u.Path != raw || u.PathNormalized() {
			t.Errorf("Path = %+q, want the bytes as given %+q untouched", u.Path, raw)
		}
	})
}

// TestWalkPathNFC: the walk reads by the on-disk name and hands the id to New
// to normalize, so a decomposed filename on disk becomes a composed id with
// the disk's own spelling kept beside it.
func TestWalkPathNFC(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, nfdName, "# Cafe\n")

	c, err := Walk(root, mdExts, log.Discard())
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	u := c.Docs[0]
	if u.Path != nfcName {
		t.Errorf("Path = %+q, want the NFC id %+q", u.Path, nfcName)
	}
	// Filesystems disagree about what they store: APFS and ext4 keep the
	// decomposed name, others hand back a composed one. Only the id above is
	// a property of this package; what UploadPath holds is a property of the
	// disk, and asserting it unconditionally would be asserting the disk's
	// behavior.
	switch u.UploadPath {
	case nfdName:
		if !u.PathNormalized() {
			t.Error("PathNormalized() = false for a name the disk stored decomposed")
		}
	case nfcName:
		t.Log("this filesystem composed the name on write; the pre-pass had nothing to do")
	default:
		t.Errorf("UploadPath = %+q, want one of the two spellings of the name written", u.UploadPath)
	}
}

// hexDigest is the test's own sha256, spelled out rather than borrowed from
// the package: a test that reuses the implementation's helper cannot catch
// the implementation hashing the wrong bytes.
func hexDigest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// TestWalkSymlinks pins the symlink policy, which splits on one
// question: could this link have been hiding a document? A file link is
// content and is followed. A directory link is refused, because a directory
// can hold documents and WalkDir will not look inside it. A broken link is
// fatal when it is named like a document and a skip when it is not.
func TestWalkSymlinks(t *testing.T) {
	t.Run("file link is followed", func(t *testing.T) {
		outside := t.TempDir()
		target := writeFile(t, outside, "external.md", "# External\n")
		root := t.TempDir()
		writeFile(t, root, "index.md", "# Index\n")
		if err := os.Symlink(target, filepath.Join(root, "linked.md")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}

		c, err := Walk(root, mdExts, log.Discard())
		if err != nil {
			t.Fatalf("Walk: %v", err)
		}
		if got, want := paths(c), []string{"index.md", "linked.md"}; !slices.Equal(got, want) {
			t.Errorf("docs: got %v, want %v", got, want)
		}
		u, _ := c.Doc("linked.md")
		if string(u.Bytes) != "# External\n" {
			t.Errorf("linked document bytes = %q, want the link target's content", u.Bytes)
		}
	})

	t.Run("directory link is refused", func(t *testing.T) {
		outside := t.TempDir()
		writeFile(t, outside, "external.md", "# External\n")
		root := t.TempDir()
		writeFile(t, root, "index.md", "# Index\n")
		if err := os.Symlink(outside, filepath.Join(root, "elsewhere")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}

		_, err := Walk(root, mdExts, log.Discard())
		if err == nil {
			t.Fatal("expected a symlinked-directory refusal, got nil")
		}
		if !strings.Contains(err.Error(), "symlink to a directory") {
			t.Errorf("error = %v, want it to name the symlinked directory policy", err)
		}
	})

	t.Run("broken link fails loudly", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "index.md", "# Index\n")
		if err := os.Symlink(filepath.Join(root, "gone.md"), filepath.Join(root, "dangling.md")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}

		_, err := Walk(root, mdExts, log.Discard())
		if err == nil {
			t.Fatal("expected a dangling-symlink failure, got nil")
		}
		if !strings.Contains(err.Error(), "dangling.md") {
			t.Errorf("error = %v, want it to name the offending path", err)
		}
	})

	t.Run("broken non-document link is skipped", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "index.md", "# Index\n")
		if err := os.Symlink(filepath.Join(root, "gone.png"), filepath.Join(root, "logo.png")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}

		lg := &logtest.Capture{}
		c, err := Walk(root, mdExts, lg)
		if err != nil {
			t.Fatalf("a broken link that cannot hold a document must not fail the corpus: %v", err)
		}
		if got, want := paths(c), []string{"index.md"}; !slices.Equal(got, want) {
			t.Errorf("docs: got %v, want %v", got, want)
		}
		if !lg.Has(t, "debug", "path", "logo.png") {
			t.Error("a skipped broken link must leave a debug record naming it")
		}
	})
}

// TestWalkRootFailures: a mistyped or non-directory root is a loud
// failure, and so is a directory holding no Markdown at all.
func TestWalkRootFailures(t *testing.T) {
	root := t.TempDir()
	file := writeFile(t, root, "doc.md", "# Doc\n")

	for _, tc := range []struct {
		name, root, want string
	}{
		{"missing root", filepath.Join(root, "nope"), "corpus root"},
		{"root is a file", file, "is not a directory"},
		{"no markdown", t.TempDir(), "no .md files"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Walk(tc.root, mdExts, log.Discard())
			if err == nil {
				t.Fatalf("expected an error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

// TestNewOrdersAndRejects: New is the single constructor, so ordering and the
// two malformed-input refusals are pinned here rather than through the walk.
func TestNewOrdersAndRejects(t *testing.T) {
	c, err := New([]SourceDoc{
		{Path: "b.md", Bytes: []byte("b")},
		{Path: "a/deep.md", Bytes: []byte("d")},
		{Path: "a.md", Bytes: []byte("a")},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got, want := paths(c), []string{"a.md", "a/deep.md", "b.md"}; !slices.Equal(got, want) {
		t.Errorf("docs: got %v, want %v (sorted by path)", got, want)
	}

	if _, err := New([]SourceDoc{{Path: "x.md"}, {Path: "x.md"}}); err == nil {
		t.Error("duplicate paths must be refused")
	}
	if _, err := New([]SourceDoc{{Bytes: []byte("x")}}); err == nil {
		t.Error("a document without a path must be refused")
	}
}

// TestContentHashIdentity: the corpus hash is the source identity in the
// provenance stamp, so it must be stable for identical input, insensitive to
// the order docs arrive in, and sensitive to a rename that moves no bytes.
func TestContentHashIdentity(t *testing.T) {
	base := []SourceDoc{
		{Path: "a.md", Bytes: []byte("alpha")},
		{Path: "b.md", Bytes: []byte("beta")},
	}
	shuffled := []SourceDoc{base[1], base[0]}
	renamed := []SourceDoc{{Path: "a.md", Bytes: []byte("alpha")}, {Path: "c.md", Bytes: []byte("beta")}}
	edited := []SourceDoc{{Path: "a.md", Bytes: []byte("alpha!")}, {Path: "b.md", Bytes: []byte("beta")}}

	hash := func(us []SourceDoc) string {
		t.Helper()
		c, err := New(us)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		return c.ContentHash
	}

	want := hash(base)
	if got := hash(shuffled); got != want {
		t.Error("content hash must not depend on input order")
	}
	if got := hash(renamed); got == want {
		t.Error("content hash must change when a file is renamed")
	}
	if got := hash(edited); got == want {
		t.Error("content hash must change when a file's bytes change")
	}
}
