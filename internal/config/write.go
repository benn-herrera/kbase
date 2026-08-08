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
)

const (
	// configDirPerm is the mode UpdateConfig creates a missing config
	// directory with. config.toml holds choices, never credentials — the
	// secrets live in the files providers.toml points at — so the
	// directory is ordinary user data, not a keystore.
	configDirPerm = 0o755

	// configFilePerm is the mode of the written config.toml. It is a file
	// the user is expected to open and hand-edit.
	configFilePerm = 0o644

	// configTempPattern names the temporary file UpdateConfig writes
	// before renaming it into place. It sits in the destination directory
	// so the rename is within one filesystem, and therefore atomic.
	configTempPattern = ConfigFileName + ".tmp*"

	// keyProvider is the top-level key holding the provider choice; it
	// mirrors Config's `toml` tag. The [models] table's keys are the tier
	// names themselves, so TierHeavy and TierLight are their single
	// source here as everywhere else.
	keyProvider = "provider"

	// tableModels is the table holding the tier→model-id map; it mirrors
	// Config's `toml` tag for Models.
	tableModels = "models"
)

// configTemplate is what a config.toml that does not exist yet is created
// as. It is commented because the file is a hand-editing surface first and
// a program output second: a user who opens it should find out from the
// file itself what may be changed and what `kbase configure` will rewrite.
const configTemplate = `# kbase configuration — safe to hand-edit; ` + "`kbase configure`" + ` updates only
# the ` + keyProvider + ` and [` + tableModels + `] values, and leaves everything else in this
# file exactly as it found it.

%s

[` + tableModels + `]
# ` + TierHeavy + `: taxonomy design, hierarchical summaries, regeneration.
# ` + TierLight + `: dissection, distillation, review.
%s
%s
`

// UpdateConfig sets the provider choice and the [models] tiers in the
// config.toml at path, creating the containing directory and the file if
// they are missing. The write is atomic: the content lands in a temporary
// file in the destination directory and is renamed over path, so a reader
// sees either the previous config or the new one, never a half-written
// file, and a failed write leaves the previous config intact.
//
// It is an editor, not a serializer. config.toml is a primary user-editable
// file, so an existing one is updated line by line: the lines carrying the
// keys above are rewritten and every other byte — comments, blank-line
// grouping, key spelling and spacing, keys and tables Config does not model
// — passes through untouched.
//
// The line editor understands the TOML layouts config.toml is written in,
// not all of TOML (a multiline string whose text looks like a table header
// is the known hazard). So the result is verified before it is installed:
// both versions are decoded, and unless they differ in exactly the intended
// values, nothing is written and the caller gets an error. Silent
// corruption of a file the user edits by hand is the one outcome not on
// offer.
func UpdateConfig(path string, cfg Config) error {
	src, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("config: read %s: %w", path, err)
	}

	// An existing-but-empty file is treated as absent: there is nothing to
	// preserve, and the user gets the commented template either way.
	before := string(src)
	updated := freshConfig(cfg)
	if strings.TrimSpace(before) != "" {
		updated = updateLines(before, cfg)
	}

	if err := verifyUpdate(before, updated, cfg); err != nil {
		return fmt.Errorf("config: refusing to rewrite %s: %w", path, err)
	}
	return installConfig(path, updated)
}

// freshConfig renders the commented template for a config.toml that does
// not exist yet.
func freshConfig(cfg Config) string {
	return fmt.Sprintf(configTemplate,
		assign(keyProvider, cfg.Provider),
		assign(TierHeavy, cfg.Models.Heavy),
		assign(TierLight, cfg.Models.Light))
}

// updateLines rewrites the provider and [models] lines of an existing
// config.toml and returns the new content. Keys the file does not have are
// added: a missing provider goes in after the leading comment block (a
// top-level key must precede the first table), missing tiers at the end of
// the [models] table, and a missing [models] table at the end of the file.
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
		found       = map[string]bool{}
	)
	for i, line := range lines {
		trimmed := stripComment(strings.TrimSpace(line))
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
		case table == "" && key == keyProvider && !found[keyProvider]:
			lines[i] = assign(keyProvider, cfg.Provider)
		case table == tableModels && (key == TierHeavy || key == TierLight) && !found[key]:
			lines[i] = assign(key, tierValue(cfg, key))
		default:
			continue
		}
		found[key] = true
	}
	if modelsFound && modelsEnd < 0 {
		modelsEnd = len(lines)
	}

	// Tail insertions first: they are indexed from the [models] table,
	// which the head insertion below would shift.
	var missing []string
	for _, tier := range []string{TierHeavy, TierLight} {
		if !found[tier] {
			missing = append(missing, assign(tier, tierValue(cfg, tier)))
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
	if !found[keyProvider] {
		lines = slices.Insert(lines, endOfLeadingComments(lines), assign(keyProvider, cfg.Provider))
	}

	out := strings.Join(lines, "\n")
	if endsWithNewline {
		out += "\n"
	}
	return out
}

// verifyUpdate is the guard on the line editor: it decodes the previous
// content and the proposed replacement and requires that they differ in
// exactly the values UpdateConfig was asked to set. Anything else the edit
// touched — an unknown key, a table, the text inside a multiline string the
// editor mistook for config — shows up here as a mismatch, and a mismatch
// refuses the write.
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
	want[keyProvider] = cfg.Provider
	models[TierHeavy] = cfg.Models.Heavy
	models[TierLight] = cfg.Models.Light
	want[tableModels] = models

	if !reflect.DeepEqual(want, next) {
		return fmt.Errorf("the update would change more than the %s and [%s] values — "+
			"this layout is beyond the line editor; set them by hand", keyProvider, tableModels)
	}
	return nil
}

// installConfig writes content to path atomically, creating the containing
// directory if it is missing.
func installConfig(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, configDirPerm); err != nil {
		return fmt.Errorf("config: create %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, configTempPattern)
	if err != nil {
		return fmt.Errorf("config: create temp file in %s: %w", dir, err)
	}
	// Removing the temp file is a no-op once the rename has consumed it,
	// and the cleanup that matters on every failure path below.
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return fmt.Errorf("config: write %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("config: write %s: %w", path, err)
	}
	// CreateTemp opens at 0600; config.toml is meant to be readable and
	// hand-editable, so the mode is set before the file becomes visible
	// under its real name.
	if err := os.Chmod(tmp.Name(), configFilePerm); err != nil {
		return fmt.Errorf("config: chmod %s: %w", path, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("config: install %s: %w", path, err)
	}
	return nil
}

// assign renders one `key = "value"` line. Go's quoting and TOML's basic
// strings agree on everything a model id or provider name can contain; a
// value they disagree on fails verifyUpdate rather than landing on disk.
func assign(key, value string) string { return key + " = " + strconv.Quote(value) }

// tierValue returns the model id cfg maps to a tier key.
func tierValue(cfg Config, tier string) string {
	id, _ := cfg.ModelFor(tier)
	return id
}

// stripComment removes a trailing comment from a trimmed line, so that a
// commented table header still reads as one. It gives up when a quote
// precedes the `#`, where the marker may be part of a value rather than a
// comment — the resulting line simply matches nothing.
func stripComment(trimmed string) string {
	if i := strings.IndexByte(trimmed, '#'); i >= 0 && !strings.ContainsAny(trimmed[:i], `"'`) {
		return strings.TrimSpace(trimmed[:i])
	}
	return trimmed
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

// endOfLeadingComments returns the index of the first line that is neither
// blank nor a comment — where a missing top-level key is inserted, since it
// must sit above the first table header and below the file's own preamble.
func endOfLeadingComments(lines []string) int {
	for i, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			return i
		}
	}
	return len(lines)
}
