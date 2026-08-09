package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newProvider is the provider name every update in this file sets, so a
// fixture's old value is visibly replaced.
const newProvider = "reaper"

// update runs UpdateConfig with the standard fixture values and fails the
// test if it errors.
func update(t *testing.T, path string) {
	t.Helper()
	cfg := Config{Provider: newProvider, Models: ModelMap{Heavy: heavyModel, Light: lightModel}}
	if err := UpdateConfig(path, cfg); err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}
}

// readBack returns the file's content as a string.
func readBack(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back %s: %v", path, err)
	}
	return string(data)
}

// assertValues checks the file parses to the values update wrote.
func assertValues(t *testing.T, path string) {
	t.Helper()
	got, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	want := Config{Provider: newProvider, Models: ModelMap{Heavy: heavyModel, Light: lightModel}}
	if got != want {
		t.Errorf("config = %+v, want %+v", got, want)
	}
}

// TestUpdateConfigFreshTemplate: with no config.toml on disk, the update
// creates one — the values, and the commented explanation of what the file
// is and which of its lines `kbase configure` will rewrite. A user's first
// sight of this file is usually this template.
func TestUpdateConfigFreshTemplate(t *testing.T) {
	dir := t.TempDir()
	path := ConfigPath(dir)
	update(t, path)
	assertValues(t, path)

	body := readBack(t, path)
	if !strings.HasPrefix(body, "#") {
		t.Errorf("template does not lead with its header comment:\n%s", body)
	}
	if !strings.Contains(body, "safe to hand-edit") {
		t.Errorf("template does not say the file is hand-editable:\n%s", body)
	}
	// Field order is stable and matches the documented layout: the
	// provider choice first, then the [models] table. A config.toml whose
	// shape shifts between writes makes every diff of it unreadable.
	// The table header, not the mention of it in the header comment.
	provider, models := strings.Index(body, "provider ="), strings.Index(body, "\n[models]\n")
	if provider < 0 || models < 0 || provider > models {
		t.Errorf("want `provider` before `[models]`, got:\n%s", body)
	}

	// No temp file may outlive the write; a stale config.toml.tmp* next
	// to the real file is debris a user has to reason about.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != ConfigFileName {
			t.Errorf("leftover file in config dir: %s", e.Name())
		}
	}
}

// TestUpdateConfigCreatesDir: a fresh install has no config directory at
// all, so the first write must make one rather than fail on the parent.
func TestUpdateConfigCreatesDir(t *testing.T) {
	path := ConfigPath(filepath.Join(t.TempDir(), "nested", "kbase"))
	update(t, path)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat written config: %v", err)
	}
	if info.Mode().Perm() != configFilePerm {
		t.Errorf("mode = %v, want %v", info.Mode().Perm(), os.FileMode(configFilePerm))
	}
}

// annotated is a config.toml as a user leaves it: comments above and beside
// keys, keys and tables kbase does not model, quoting and spacing kbase
// would never emit. Every line of it except the three assignments must
// survive an update byte for byte — and those three keep their trailing
// comments, which are the notes a user is likeliest to have written.
//
// The heavy value carries a `#` of its own: the comment scan must read it
// as part of the string, not as the start of the note that follows it.
const annotated = `# my kbase config — do not let a tool eat these notes
#   (second line of the header block)

provider   =    "old-provider"   # which providers.toml entry to use

# a key kbase knows nothing about
scratch = "keep me"

[models]
# heavy runs taxonomy design
heavy = "old#heavy"    # the id, hash and all
light='old-light'# no space before the marker

[experimental]
notes = "an entire table kbase does not model"
`

// TestUpdateConfigPreservesHandEdits is the deal-breaker case: an update
// rewrites the three lines it owns and nothing else. config.toml is a
// primary hand-editing surface, so a write that eats a user's comments,
// grouping, or unmodelled keys is a defect, not a documented limitation.
// That holds for the owned lines too: only the value changes, and the note
// beside it comes along (at normalized spacing — the value's width moved).
func TestUpdateConfigPreservesHandEdits(t *testing.T) {
	path := writeFile(t, t.TempDir(), ConfigFileName, annotated)
	update(t, path)
	assertValues(t, path)

	// The rewritten lines, keyed by their line number in the fixture.
	rewritten := map[int]string{
		3:  `provider = "` + newProvider + `"` + commentGap + `# which providers.toml entry to use`,
		10: `heavy = "` + heavyModel + `"` + commentGap + `# the id, hash and all`,
		11: `light = "` + lightModel + `"` + commentGap + `# no space before the marker`,
	}
	before, after := strings.Split(annotated, "\n"), strings.Split(readBack(t, path), "\n")
	if len(before) != len(after) {
		t.Fatalf("line count changed: %d → %d\n%s", len(before), len(after), strings.Join(after, "\n"))
	}
	for i := range before {
		want, ok := rewritten[i]
		if !ok {
			want = before[i]
		}
		if after[i] != want {
			t.Errorf("line %d = %q, want %q", i, after[i], want)
		}
	}
}

// TestUpdateConfigInsertsMissingKeys: a config.toml can be missing any of
// the keys — a hand-written one usually is. Each is added where it belongs
// rather than triggering a rewrite of the file.
func TestUpdateConfigInsertsMissingKeys(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string // the whole file after the update
	}{{
		name: "a tier is added to the end of its table",
		src:  "provider = \"old\"\n\n[models]\nheavy = \"old-heavy\"\n\n[other]\nk = 1\n",
		want: "provider = \"" + newProvider + "\"\n\n[models]\nheavy = \"" + heavyModel + "\"\n" +
			"light = \"" + lightModel + "\"\n\n[other]\nk = 1\n",
	}, {
		name: "a missing [models] table is added at the end of the file",
		src:  "# note\nprovider = \"old\"\n",
		want: "# note\nprovider = \"" + newProvider + "\"\n\n[models]\nheavy = \"" + heavyModel + "\"\n" +
			"light = \"" + lightModel + "\"\n",
	}, {
		name: "a missing provider goes under the leading comments, above the first table",
		src:  "# note\n\n[models]\nheavy = \"old-heavy\"\nlight = \"old-light\"\n",
		want: "# note\n\nprovider = \"" + newProvider + "\"\n[models]\nheavy = \"" + heavyModel + "\"\n" +
			"light = \"" + lightModel + "\"\n",
	}, {
		name: "an unrelated file keeps its content and gains both",
		src:  "[experimental]\nnotes = \"kbase does not model this\"\n",
		want: "provider = \"" + newProvider + "\"\n[experimental]\nnotes = \"kbase does not model this\"\n" +
			"\n[models]\nheavy = \"" + heavyModel + "\"\nlight = \"" + lightModel + "\"\n",
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeFile(t, t.TempDir(), ConfigFileName, tt.src)
			update(t, path)
			assertValues(t, path)
			if got := readBack(t, path); got != tt.want {
				t.Errorf("file =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

// TestUpdateConfigIdempotent: writing the same values twice must produce
// the same bytes. An updater that drifts — re-inserting a key, growing a
// blank line — turns every `kbase configure` into a spurious diff.
func TestUpdateConfigIdempotent(t *testing.T) {
	for _, src := range []string{"", annotated, "provider = \"old\"\n"} {
		path := writeFile(t, t.TempDir(), ConfigFileName, src)
		update(t, path)
		first := readBack(t, path)
		update(t, path)
		if second := readBack(t, path); second != first {
			t.Errorf("second update changed the file:\n%s\nwas\n%s", second, first)
		}
	}
}

// TestUpdateConfigRefusesUnparsableLayout: the line editor understands the
// TOML config.toml is written in, not all of TOML. A multiline string whose
// text looks like a table header is the case it gets wrong — so the guard
// catches it, the write is refused, and the file is left alone. A loud
// refusal is recoverable by hand; silent corruption of a hand-edited file
// is not.
func TestUpdateConfigRefusesUnparsableLayout(t *testing.T) {
	const src = `provider = "old"

note = """
[models]
heavy = "this is prose, not configuration"
"""

[models]
heavy = "old-heavy"
light = "old-light"
`
	path := writeFile(t, t.TempDir(), ConfigFileName, src)
	err := UpdateConfig(path, Config{Provider: newProvider, Models: ModelMap{Heavy: heavyModel, Light: lightModel}})
	if err == nil {
		t.Fatal("multiline string shaped like a table: got nil error, want a refusal")
	}
	if !strings.Contains(err.Error(), "refusing to rewrite") {
		t.Errorf("error: got %v, want a refusal naming the file", err)
	}
	if got := readBack(t, path); got != src {
		t.Errorf("refused update still touched the file:\n%s", got)
	}
}

// TestUpdateConfigRefusesMalformedFile: a config.toml that does not parse
// is not a file to edit blind. LoadConfig already fails on it; the writer
// must not paper over it by overwriting what the user cannot see.
func TestUpdateConfigRefusesMalformedFile(t *testing.T) {
	const src = "[models\nbroken"
	path := writeFile(t, t.TempDir(), ConfigFileName, src)
	if err := UpdateConfig(path, Config{Provider: newProvider}); err == nil {
		t.Fatal("malformed config.toml: got nil error, want a refusal")
	}
	if got := readBack(t, path); got != src {
		t.Errorf("refused update still touched the file:\n%s", got)
	}
}

// TestUpdateConfigFollowsSymlink: a config.toml symlinked into a dotfiles
// repo is a common setup for exactly this kind of file. The update must
// land on the file the link names — replacing the link with a regular file
// leaves the dotfiles copy stale, and nothing tells the user.
func TestUpdateConfigFollowsSymlink(t *testing.T) {
	root := t.TempDir()
	dotfiles := filepath.Join(root, "dotfiles")
	confDir := filepath.Join(root, "kbase")
	for _, dir := range []string{dotfiles, confDir} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	real := writeFile(t, dotfiles, ConfigFileName, "provider = \"old\"\n")
	link := ConfigPath(confDir)
	if err := os.Symlink(real, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	update(t, link)
	assertValues(t, link)

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("lstat link: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the update replaced the symlink with a %v", info.Mode().Type())
	}
	if got := readBack(t, real); !strings.Contains(got, newProvider) {
		t.Errorf("the linked-to file did not receive the update:\n%s", got)
	}
}

// TestUpdateConfigLeavesDevTableAlone: [dev] is hand-added and hand-owned.
// `kbase configure` edits the provider and [models] values; a switch the
// user turned on must not be reverted, reformatted, or moved by a verb
// that has no opinion about it.
func TestUpdateConfigLeavesDevTableAlone(t *testing.T) {
	const src = `provider = "old"

[models]
heavy = "old-heavy"
light = "old-light"

[dev]
# local inference-timing diagnostics
telemetry = true
`
	path := writeFile(t, t.TempDir(), ConfigFileName, src)
	update(t, path)

	_, devTable, found := strings.Cut(readBack(t, path), "\n[dev]\n")
	if !found {
		t.Fatalf("the [dev] table did not survive the update:\n%s", readBack(t, path))
	}
	if want := "# local inference-timing diagnostics\ntelemetry = true\n"; devTable != want {
		t.Errorf("[dev] table = %q, want %q", devTable, want)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !cfg.Dev.Telemetry {
		t.Error("Dev.Telemetry was turned off by an update that does not own it")
	}
}

// TestUpdateConfigLeavesPreviousOnFailure: an unwritable destination must
// fail without disturbing the config already there. The pipeline reads
// this file; a failed write that empties it is worse than no write.
func TestUpdateConfigLeavesPreviousOnFailure(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, ConfigFileName, "provider = \"old\"\n")
	// A read-only directory blocks the temp-file create, which is the
	// first step of the write.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	if err := UpdateConfig(path, Config{Provider: "new"}); err == nil {
		t.Fatal("UpdateConfig into a read-only directory: got nil error, want failure")
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Provider != "old" {
		t.Errorf("previous config was disturbed: Provider = %q, want old", cfg.Provider)
	}
}
