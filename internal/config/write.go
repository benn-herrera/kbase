package config

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"

	"kbase/internal/atomicfile"
)

const (
	// configDirPerm is the mode UpdateConfig ASKS FOR when it creates a
	// missing config directory: the ordinary permissive creation mode, left
	// to the user's umask to narrow. config.toml holds choices, never
	// credentials — the secrets live in the files providers.toml points at —
	// so the directory is ordinary user data, not a keystore.
	configDirPerm = 0o777

	// keyProvider is the top-level key the editor removes; it mirrors
	// Config's `toml` tag. The key is modelled so an old config.toml still
	// loads and the diagnostic that reports it superseded can name it, and
	// nothing writes it: a reference names its own pool entry. The [models]
	// table's keys are the category names themselves, so the Category
	// constants are their single source here as everywhere else.
	keyProvider = "provider"

	// tableModels is the table holding the category→model-id map; it mirrors
	// Config's `toml` tag for Models.
	tableModels = "models"

	// commentGap separates a rewritten assignment from the trailing
	// comment carried over from the line it replaced. The original spacing
	// is not reproduced — the value's width changed, so the alignment it
	// was chosen for is gone either way.
	commentGap = "  "
)

// CreateProvidersFile writes the annotated providers.toml template at path
// when no file stands there, creating the containing directory, and reports
// whether it wrote. It is the only writer of providers.toml: a file that
// stands is a hand-editing surface nothing here overwrites.
func CreateProvidersFile(path string) (bool, error) {
	return createTemplateFile(path, providersTemplate)
}

// CreateHarnessesFile writes the annotated harnesses.toml template at path when
// no file stands there, and reports whether it wrote. It is the only writer of
// harnesses.toml, and creation is all it does: a profile is added by editing the
// file, never by a verb, so a seed a user deleted stays deleted and a file they
// wrote keeps every entry they wrote.
func CreateHarnessesFile(path string) (bool, error) {
	return createTemplateFile(path, harnessesTemplate)
}

// createTemplateFile installs template at path where nothing stands.
func createTemplateFile(path, template string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("config: stat %s: %w", path, err)
	}
	if err := installConfig(path, template); err != nil {
		return false, err
	}
	return true, nil
}

// UpdateConfig writes the [models] categories into the config.toml at path,
// creating the containing directory and the file if they are missing, and
// removes the superseded top-level provider key. The write is atomic: the
// content lands in a temporary file in the destination directory and is
// renamed over path, so a reader sees either the previous config or the new
// one, never a half-written file, and a failed write leaves the previous
// config intact.
//
// It is an editor, not a serializer. config.toml is a primary user-editable
// file, so an existing one is updated line by line: the lines carrying the
// [models] keys are rewritten, the provider line is deleted, and every other
// byte — comments, blank-line grouping, key spelling and spacing, keys and
// tables Config does not model — passes through untouched. A rewritten line's
// own trailing comment survives too: those lines are the ones a user is
// likeliest to have annotated. Only its spacing is normalized (see
// commentGap).
//
// The line editor understands the TOML layouts config.toml is written in,
// not all of TOML (a multiline string whose text looks like a table header
// is the known hazard). So the result is verified before it is installed:
// both versions are decoded, and unless they differ in exactly the intended
// values, nothing is written and the caller gets an error. Silent
// corruption of a file the user edits by hand is the one outcome not on
// offer.
//
// It reports whether it wrote: a file already holding the values is left as
// it stands. cfg.Provider is not written — the key is the one the editor
// deletes, and a reference names its own pool entry.
func UpdateConfig(path string, cfg Config) (bool, error) {
	src, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("config: read %s: %w", path, err)
	}

	// An existing-but-empty file is treated as absent: there is nothing to
	// preserve, and the user gets the commented template either way.
	before := string(src)
	updated := freshConfig(cfg)
	if strings.TrimSpace(before) != "" {
		updated = updateLines(before, cfg)
	}
	if updated == before {
		return false, nil
	}

	if err := verifyUpdate(before, updated, cfg); err != nil {
		return false, fmt.Errorf("config: refusing to rewrite %s: %w", path, err)
	}
	return true, installConfig(path, updated)
}

// freshConfig renders the commented template for a config.toml that does
// not exist yet.
func freshConfig(cfg Config) string {
	return fmt.Sprintf(configTemplate,
		modelHintsComment(),
		assign(CategoryDecision, cfg.Models.Decision),
		assign(CategoryText, cfg.Models.Text),
		assign(CategoryScoring, cfg.Models.Scoring))
}

// updateLines rewrites the [models] lines of an existing config.toml, removes
// a top-level provider line, and returns the new content. Keys the file does
// not have are added: missing categories at the end of the [models] table, and
// a missing [models] table at the end of the file.
func updateLines(src string, cfg Config) string {
	lines := strings.Split(src, "\n")
	// A trailing newline splits into a final empty element. Holding it
	// aside keeps "the end of the file" meaning the last line of text, and
	// restores the file's original ending exactly.
	endsWithNewline := len(lines) > 0 && lines[len(lines)-1] == ""
	if endsWithNewline {
		lines = lines[:len(lines)-1]
	}

	var (
		table       string
		modelsFound bool
		modelsEnd   = -1 // first line past the [models] table
		providerAt  = -1 // the superseded top-level line, removed below
		found       = map[string]bool{}
	)
	for i, line := range lines {
		code, comment := splitComment(line)
		trimmed := strings.TrimSpace(code)
		if name, isHeader := tableHeader(trimmed); isHeader {
			if table == tableModels && modelsEnd < 0 {
				modelsEnd = i
			}
			table = name
			modelsFound = modelsFound || name == tableModels
			continue
		}
		key := lineKey(trimmed)
		switch {
		case table == "" && key == keyProvider && providerAt < 0:
			providerAt = i
			found[keyProvider] = true
		case table == tableModels && isCategory(key) && !found[key]:
			lines[i] = withComment(assign(key, categoryValue(cfg, key)), comment)
			found[key] = true
		default:
			continue
		}
	}
	if modelsFound && modelsEnd < 0 {
		modelsEnd = len(lines)
	}

	// The editor's one deletion, and the only change that is not a value
	// rewrite. Nothing writes the key: a reference names its own pool entry.
	if providerAt >= 0 {
		lines = slices.Delete(lines, providerAt, providerAt+1)
		if modelsEnd > providerAt {
			modelsEnd--
		}
	}

	var missing []string
	for _, role := range Roles {
		category := string(role)
		if !found[category] {
			missing = append(missing, assign(category, categoryValue(cfg, category)))
		}
	}
	if len(missing) > 0 {
		if !modelsFound {
			modelsEnd = len(lines)
			missing = append([]string{"[" + tableModels + "]"}, missing...)
			if at := lastTextLine(lines, modelsEnd); at > 0 {
				missing = append([]string{""}, missing...)
			}
		}
		lines = slices.Insert(lines, lastTextLine(lines, modelsEnd), missing...)
	}

	out := strings.Join(lines, "\n")
	if endsWithNewline {
		out += "\n"
	}
	return out
}

// isCategory reports whether key names one of the [models] table's categories.
// The roles are the categories typed, so the file and every document that
// reports them state the same set in the same order.
func isCategory(key string) bool {
	for _, role := range Roles {
		if key == string(role) {
			return true
		}
	}
	return false
}

// verifyUpdate is the guard on the line editor: it decodes the previous
// content and the proposed replacement and requires that they differ in
// exactly the values UpdateConfig was asked to set — the three [models] keys,
// and the removal of the provider key. Anything else the edit touched — an
// unknown key, a table, the text inside a multiline string the editor mistook
// for config — shows up here as a mismatch, and a mismatch refuses the write.
func verifyUpdate(before, after string, cfg Config) error {
	var prev, next map[string]any
	if strings.TrimSpace(before) != "" {
		if _, err := toml.Decode(before, &prev); err != nil {
			return fmt.Errorf("the file it already holds is not valid TOML: %w", err)
		}
	}
	if _, err := toml.Decode(after, &next); err != nil {
		return fmt.Errorf("the update would not be valid TOML: %w", err)
	}

	want := maps.Clone(prev)
	if want == nil {
		want = map[string]any{}
	}
	models := map[string]any{}
	if m, ok := want[tableModels].(map[string]any); ok {
		models = maps.Clone(m)
	}
	delete(want, keyProvider)
	models[CategoryDecision] = cfg.Models.Decision
	models[CategoryText] = cfg.Models.Text
	models[CategoryScoring] = cfg.Models.Scoring
	want[tableModels] = models

	if !reflect.DeepEqual(want, next) {
		return fmt.Errorf("the update would change more than the [%s] values and the removal of %s — "+
			"this layout is beyond the line editor; set them by hand", tableModels, keyProvider)
	}
	return nil
}

// installConfig writes content to path atomically, creating the containing
// directory if it is missing. A path that is a symlink is followed: the
// write lands on the file the link names, and the link itself survives.
func installConfig(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, configDirPerm); err != nil {
		return fmt.Errorf("config: create %s: %w", dir, err)
	}

	// A config.toml symlinked into a dotfiles repo is a common setup for
	// exactly this kind of file; the write follows the link, so the dotfiles
	// copy is the one rewritten and the link survives.
	if err := atomicfile.Write(path, []byte(content), nil); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	return nil
}

// assign renders one `key = "value"` line. Go's quoting and TOML's basic
// strings agree on everything a model id or provider name can contain; a
// value they disagree on fails verifyUpdate rather than landing on disk.
func assign(key, value string) string { return key + " = " + strconv.Quote(value) }

// categoryValue returns the model id cfg maps to a category key.
func categoryValue(cfg Config, category string) string {
	id, _ := cfg.ModelFor(category)
	return id
}

// splitComment cuts a line at its trailing comment, returning the text
// before the `#` and the comment from the `#` onward (empty when there is
// none). It is what lets a commented table header still read as one, and
// what lets a rewritten assignment keep the note beside it.
//
// The scan is quote-aware, because `#` is an ordinary character inside a
// value: a basic string honours backslash escapes, a literal string does
// not. A line whose quotes never close — in a real config.toml that means
// a multiline string, whose interior this editor does not follow — yields
// no comment, and matches nothing.
func splitComment(line string) (code, comment string) {
	var quote byte
	for i := 0; i < len(line); i++ {
		switch c := line[i]; {
		case quote == '"' && c == '\\':
			i++ // an escaped character cannot close the string
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '#':
			return line[:i], line[i:]
		}
	}
	return line, ""
}

// withComment re-attaches a preserved trailing comment to a rewritten
// assignment.
func withComment(assignment, comment string) string {
	if comment == "" {
		return assignment
	}
	return assignment + commentGap + comment
}

// tableHeader reports whether a trimmed line opens a table and, if so,
// names it. An array-of-tables header (`[[x]]`) counts: the keys under it
// belong to x, which is all the caller needs to know to leave them alone.
func tableHeader(trimmed string) (string, bool) {
	if !strings.HasPrefix(trimmed, "[") || !strings.HasSuffix(trimmed, "]") {
		return "", false
	}
	return strings.TrimSpace(strings.Trim(trimmed, "[]")), true
}

// lineKey returns the key a trimmed line assigns, or "" if it assigns
// nothing. Quotes around the key are dropped so the quoted spelling of a
// bare key is recognized as the same key.
func lineKey(trimmed string) string {
	key, _, ok := strings.Cut(trimmed, "=")
	if !ok {
		return ""
	}
	return strings.Trim(strings.TrimSpace(key), `"'`)
}

// lastTextLine walks back from end over blank lines and returns the index
// just past the last line of text. Appending there keeps a new key inside
// its table and leaves the blank-line grouping that follows intact.
func lastTextLine(lines []string, end int) int {
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return end
}
