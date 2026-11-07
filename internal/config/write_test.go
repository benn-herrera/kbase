package config

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"kbase/internal/atomicfile"
)

// update runs UpdateConfig with the standard fixture values and fails the
// test if it errors.
func update(t *testing.T, path string) {
	t.Helper()
	if _, err := UpdateConfig(path, updateValues()); err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}
}

// updateValues is what every update in this file writes: the three categories.
func updateValues() Config {
	return Config{Models: ModelMap{Decision: decisionModel, Text: textModel, Scoring: scoringModel}}
}

// TestUpdateConfigLeavesTheSameValues: an update to the values the file
// already holds writes nothing and says so.
func TestUpdateConfigLeavesTheSameValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), ConfigFileName)
	cfg := updateValues()
	if wrote, err := UpdateConfig(path, cfg); err != nil || !wrote {
		t.Fatalf("first update: wrote %t, %v", wrote, err)
	}
	if wrote, err := UpdateConfig(path, cfg); err != nil || wrote {
		t.Errorf("second update: wrote %t, %v; want nothing written", wrote, err)
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
	want := Config{Models: ModelMap{Decision: decisionModel, Text: textModel, Scoring: scoringModel}}
	if got != want {
		t.Errorf("config = %+v, want %+v", got, want)
	}
}

// TestUpdateConfigFreshTemplate: with no config.toml on disk, the update
// creates one — the values, and the commented explanation of what the file
// is and which of its lines `kbase configure-models` will rewrite. A user's first
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
	// Field order is stable and matches the documented layout: the [models]
	// table and its three categories. A config.toml whose shape shifts
	// between writes makes every diff of it unreadable.
	models := strings.Index(body, "\n[models]\n")
	decision, text, scoring := strings.Index(body, "\ndecision ="), strings.Index(body, "\ntext ="), strings.Index(body, "\nscoring =")
	if models < 0 || decision < models || text < decision || scoring < text {
		t.Errorf("want [models] then decision, text and scoring, got:\n%s", body)
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
// It also pins the ruling of 2026-08-14: the file and the directory are
// created permissively and the user's umask narrows them. config.toml is a
// hand-editing surface, not a credential — the secrets live in the files
// providers.toml points at — so kbase does not second-guess the umask. The
// mask is set to a distinctive value first, so a hard-coded mode of any
// vintage fails here.
func TestUpdateConfigCreatesDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix file modes")
	}
	mask := setTestUmask(t)
	path := ConfigPath(filepath.Join(t.TempDir(), "nested", "kbase"))
	update(t, path)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat written config: %v", err)
	}
	if want := os.FileMode(atomicfile.CreateMode) &^ mask; info.Mode().Perm() != want {
		t.Errorf("mode = %v, want %v", info.Mode().Perm(), want)
	}
	dir, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat config dir: %v", err)
	}
	if want := os.FileMode(configDirPerm) &^ mask; dir.Mode().Perm() != want {
		t.Errorf("dir mode = %v, want %v", dir.Mode().Perm(), want)
	}
}

// annotated is a config.toml as a user leaves it: comments above and beside
// keys, keys and tables kbase does not model, quoting and spacing kbase
// would never emit. Every line of it except the three assignments must
// survive an update byte for byte — and the three keep their trailing
// comments, which are the notes a user is likeliest to have written.
//
// The decision value carries a `#` of its own: the comment scan must read it
// as part of the string, not as the start of the note that follows it.
const annotated = `# my kbase config — do not let a tool eat these notes
#   (second line of the header block)

# a key kbase knows nothing about
scratch = "keep me"

[models]
# decision answers the letter asks
decision = "old#decision"    # the id, hash and all
text='old-text'# no space before the marker
scoring = "old-scoring"

[experimental]
notes = "an entire table kbase does not model"
`

// TestUpdateConfigPreservesHandEdits is the deal-breaker case: an update
// rewrites the three lines it owns and changes nothing else. config.toml is a primary hand-editing surface, so a write
// that eats a user's comments, grouping, or unmodelled keys is a defect, not
// a documented limitation. That holds for the owned lines too: only the value
// changes, and the note beside it comes along (at normalized spacing — the
// value's width moved).
func TestUpdateConfigPreservesHandEdits(t *testing.T) {
	path := writeFile(t, t.TempDir(), ConfigFileName, annotated)
	update(t, path)

	// Not read back through LoadConfig: this fixture carries keys Config
	// does not model, and a strict load refuses those on purpose. The
	// line-by-line comparison below covers the written values exactly.
	//
	// The rewritten lines, keyed by their line number in the fixture.
	rewritten := map[int]string{
		8:  `decision = "` + decisionModel + `"` + commentGap + `# the id, hash and all`,
		9:  `text = "` + textModel + `"` + commentGap + `# no space before the marker`,
		10: `scoring = "` + scoringModel + `"`,
	}
	before, after := strings.Split(annotated, "\n"), strings.Split(readBack(t, path), "\n")
	var want []string
	for i, line := range before {
		if replaced, ok := rewritten[i]; ok {
			line = replaced
		}
		want = append(want, line)
	}
	if !slices.Equal(after, want) {
		t.Errorf("file =\n%s\nwant\n%s", strings.Join(after, "\n"), strings.Join(want, "\n"))
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
		name: "missing categories are added to the end of their table",
		src:  "[models]\ndecision = \"old-decision\"\n\n[other]\nk = 1\n",
		want: "[models]\ndecision = \"" + decisionModel + "\"\ntext = \"" + textModel + "\"\n" +
			"scoring = \"" + scoringModel + "\"\n\n[other]\nk = 1\n",
	}, {
		name: "a missing [models] table is added at the end of the file",
		src:  "# note\n",
		want: "# note\n\n[models]\ndecision = \"" + decisionModel + "\"\ntext = \"" + textModel + "\"\n" +
			"scoring = \"" + scoringModel + "\"\n",
	}, {
		name: "a missing category is added to the table",
		src:  "# note\n\n[models]\ndecision = \"old-decision\"\ntext = \"old-text\"\n",
		want: "# note\n\n[models]\ndecision = \"" + decisionModel + "\"\ntext = \"" + textModel + "\"\n" +
			"scoring = \"" + scoringModel + "\"\n",
	}, {
		name: "an unrelated file keeps its content and gains the table",
		src:  "[experimental]\nnotes = \"kbase does not model this\"\n",
		want: "[experimental]\nnotes = \"kbase does not model this\"\n" +
			"\n[models]\ndecision = \"" + decisionModel + "\"\ntext = \"" + textModel + "\"\nscoring = \"" + scoringModel + "\"\n",
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeFile(t, t.TempDir(), ConfigFileName, tt.src)
			update(t, path)
			// The whole-file comparison is the assertion: some of these
			// sources carry tables Config does not model, which a strict
			// LoadConfig refuses by design.
			if got := readBack(t, path); got != tt.want {
				t.Errorf("file =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

// TestUpdateConfigIdempotent: writing the same values twice must produce
// the same bytes. An updater that drifts — re-inserting a key, growing a
// blank line — turns every `kbase configure-models` into a spurious diff.
func TestUpdateConfigIdempotent(t *testing.T) {
	for _, src := range []string{"", annotated} {
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
	const src = `note = """
[models]
decision = "this is prose, not configuration"
"""

[models]
decision = "old-decision"
text = "old-text"
`
	path := writeFile(t, t.TempDir(), ConfigFileName, src)
	_, err := UpdateConfig(path, Config{Models: ModelMap{Decision: decisionModel, Text: textModel}})
	if err == nil {
		t.Fatal("multiline string shaped like a table: got nil error, want a refusal")
	}
	if want := "set them in " + path + " by hand"; !strings.Contains(err.Error(), want) {
		t.Errorf("error: got %v, want a refusal naming the file (%q)", err, want)
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
	if _, err := UpdateConfig(path, Config{}); err == nil {
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
	real := writeFile(t, dotfiles, ConfigFileName, "[models]\n")
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
	if got := readBack(t, real); !strings.Contains(got, decisionModel) {
		t.Errorf("the linked-to file did not receive the update:\n%s", got)
	}
}

// TestUpdateConfigLeavesAsksTableAlone: [asks] is hand-added and hand-owned.
// `kbase configure-models` edits the [models] values; a setting the user chose must
// not be reverted, reformatted, or moved by a verb that has no opinion about
// it.
func TestUpdateConfigLeavesAsksTableAlone(t *testing.T) {
	const src = `[models]
decision = "old-decision"
text = "old-text"

[asks]
# one ask at a time on this box
concurrency = 1
`
	path := writeFile(t, t.TempDir(), ConfigFileName, src)
	update(t, path)

	_, asksTable, found := strings.Cut(readBack(t, path), "\n[asks]\n")
	if !found {
		t.Fatalf("the [asks] table did not survive the update:\n%s", readBack(t, path))
	}
	if want := "# one ask at a time on this box\nconcurrency = 1\n"; asksTable != want {
		t.Errorf("[asks] table = %q, want %q", asksTable, want)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if n := cfg.Asks.Concurrency; n == nil || *n != 1 {
		t.Errorf("concurrency = %v after an update that does not own it, want 1", n)
	}
}

// TestUpdateConfigLeavesPreviousOnFailure: an unwritable destination must
// fail without disturbing the config already there. The pipeline reads
// this file; a failed write that empties it is worse than no write.
func TestUpdateConfigLeavesPreviousOnFailure(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, ConfigFileName, "[models]\ndecision = \"old\"\n")
	// A read-only directory blocks the temp-file create, which is the
	// first step of the write.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	if _, err := UpdateConfig(path, Config{Models: ModelMap{Decision: "new"}}); err == nil {
		t.Fatal("UpdateConfig into a read-only directory: got nil error, want failure")
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Models.Decision != "old" {
		t.Errorf("previous config was disturbed: decision = %q, want old", cfg.Models.Decision)
	}
}

// TestUpdateConfigLeavesRolesNotGiven: a role the update leaves "" is not
// rewritten, inserted or blanked.
func TestUpdateConfigLeavesRolesNotGiven(t *testing.T) {
	const src = "[models]\ndecision = \"old-decision\"\ntext='old-text'\n"
	path := writeFile(t, t.TempDir(), ConfigFileName, src)
	if _, err := UpdateConfig(path, Config{Models: ModelMap{Decision: decisionModel}}); err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}
	if got, want := readBack(t, path), "[models]\ndecision = \""+decisionModel+"\"\ntext='old-text'\n"; got != want {
		t.Errorf("file = %q, want %q", got, want)
	}
}

// handProviders is a providers.toml as a user leaves it: a header, a quoted
// table header, an entry with a key file and a note, and a second entry.
const handProviders = `# my pool

[Lab]  # the box under the desk
baseUrl = "http://old:8000/v1"  # moved in May
apiKeyFile = "lab.key"
# a note on lab

# the cloud one
[cloud]
baseUrl = "https://cloud.example/v1"
`

// TestSetProvider: an entry's fields are set in place in the table matching
// its name whatever its case, a field not given and every other byte stand; a
// new entry is added at the end; the same values again write nothing.
func TestSetProvider(t *testing.T) {
	for _, tc := range []struct {
		name string
		p    Provider
		want string
	}{
		{"an existing entry, whatever its case", Provider{Name: "lab", BaseURL: "http://new:8000/v1", API: "openai"},
			strings.Replace(handProviders, `baseUrl = "http://old:8000/v1"  # moved in May
apiKeyFile = "lab.key"`, `baseUrl = "http://new:8000/v1"  # moved in May
apiKeyFile = "lab.key"
api = "openai"`, 1)},
		{"a new entry", Provider{Name: "edge", BaseURL: "http://edge/v1", DecisionAPI: "decisions"},
			handProviders + "\n[edge]\nbaseUrl = \"http://edge/v1\"\ndecisionApi = \"decisions\"\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeProviders(t, t.TempDir(), handProviders)
			if wrote, err := SetProvider(path, tc.p); err != nil || !wrote {
				t.Fatalf("SetProvider: wrote %t, %v", wrote, err)
			}
			if got := readBack(t, path); got != tc.want {
				t.Errorf("file =\n%s\nwant\n%s", got, tc.want)
			}
			if wrote, err := SetProvider(path, tc.p); err != nil || wrote {
				t.Errorf("the same entry again: wrote %t, %v; want nothing written", wrote, err)
			}
		})
	}
}

// TestSetProviderCreatesTheFile: an absent providers.toml is created holding
// the entry written and no other, and loads as a pool of that one entry.
func TestSetProviderCreatesTheFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := SetProvider(ProvidersPath(dir), Provider{Name: "lab", BaseURL: "http://lab:8000/v1"}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	pool, faults, err := LoadProviders(ProvidersPath(dir), nil)
	if err != nil || len(faults) != 0 || len(pool) != 1 || pool["lab"].BaseURL != "http://lab:8000/v1" {
		t.Errorf("pool = %+v, faults %v, %v; want the one entry written", pool, faults, err)
	}
	if body := readBack(t, ProvidersPath(dir)); !strings.HasPrefix(body, providersTemplate()) {
		t.Errorf("the created file does not open with the template's comments:\n%s", body)
	}
}

// TestRemoveProvider: the entry matching the name whatever its case goes,
// with its block, and the rest stands; an entry that stands nowhere is
// reported as not removed, and so is an absent file.
func TestRemoveProvider(t *testing.T) {
	dir := t.TempDir()
	path := writeProviders(t, dir, handProviders)
	if removed, err := RemoveProvider(path, "lab"); err != nil || !removed {
		t.Fatalf("RemoveProvider: removed %t, %v", removed, err)
	}
	if got, want := readBack(t, path), "# my pool\n\n# the cloud one\n[cloud]\nbaseUrl = \"https://cloud.example/v1\"\n"; got != want {
		t.Errorf("file =\n%s\nwant\n%s", got, want)
	}
	if removed, err := RemoveProvider(path, "lab"); err != nil || removed {
		t.Errorf("a second removal: removed %t, %v; want none", removed, err)
	}
	if removed, err := RemoveProvider(ProvidersPath(t.TempDir()), "lab"); err != nil || removed {
		t.Errorf("an absent file: removed %t, %v; want none", removed, err)
	}
}
