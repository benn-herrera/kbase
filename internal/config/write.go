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

	// keyProvider is the top-level key holding the provider choice; it
	// mirrors Config's `toml` tag. The [models] table's keys are the tier
	// names themselves, so TierHeavy and TierLight are their single
	// source here as everywhere else.
	keyProvider = "provider"

	// tableModels is the table holding the tier→model-id map; it mirrors
	// Config's `toml` tag for Models.
	tableModels = "models"

	// commentGap separates a rewritten assignment from the trailing
	// comment carried over from the line it replaced. The original spacing
	// is not reproduced — the value's width changed, so the alignment it
	// was chosen for is gone either way.
	commentGap = "  "
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
# ` + TierHeavy + `: the overview passage.
# ` + TierLight + `: the claim graph's letter asks.
%s
%s

# [asks]
# readerConcurrency = 4  # asks of one group in flight once its first has
#                        # returned; a whole number, at least 1.
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
// grouping, key spelling and spacing, keys and tables Config does not model —
// passes through untouched. A rewritten line's own trailing comment
// survives too: those three lines are the ones a user is likeliest to have
// annotated. Only its spacing is normalized (see commentGap).
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
// it stands.
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
		case table == "" && key == keyProvider && !found[keyProvider]:
			lines[i] = withComment(assign(keyProvider, cfg.Provider), comment)
		case table == tableModels && (key == TierHeavy || key == TierLight) && !found[key]:
			lines[i] = withComment(assign(key, tierValue(cfg, key)), comment)
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

// tierValue returns the model id cfg maps to a tier key.
func tierValue(cfg Config, tier string) string {
	id, _ := cfg.ModelFor(tier)
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
