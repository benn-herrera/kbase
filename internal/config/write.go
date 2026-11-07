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
	// configDirPerm is the mode a write ASKS FOR when it creates a missing
	// config directory: the ordinary permissive creation mode, left to the
	// user's umask to narrow. Neither file holds a credential — a key lives
	// in kbase's store or the file an entry's apiKeyFile names — so the
	// directory is ordinary user data, not a keystore.
	configDirPerm = 0o777

	// tableModels is the table holding the role→model-reference map; it mirrors
	// Config's `toml` tag for Models.
	tableModels = "models"

	// The providers.toml keys an entry's write sets; they mirror Provider's
	// `toml` tags.
	keyBaseURL     = "baseUrl"
	keyAPI         = "api"
	keyDecisionAPI = "decisionApi"

	// commentGap separates a rewritten assignment from the trailing
	// comment carried over from the line it replaced. The original spacing
	// is not reproduced — the value's width changed, so the alignment it
	// was chosen for is gone either way.
	commentGap = "  "
)

// keyValue is one key a table edit sets, and the string it sets it to.
type keyValue struct{ key, value string }

// UpdateConfig writes the [models] roles cfg names into the config.toml at
// path; a role cfg leaves "" stands as the file has it. A missing file, and
// its directory, are created from the commented template. It reports whether
// it wrote: a file already holding the values is left as it stands.
//
// It is an editor, not a serializer: see editTable.
func UpdateConfig(path string, cfg Config) (bool, error) {
	var set []keyValue
	for _, role := range Roles {
		if value := role.model(cfg.Models); value != "" {
			set = append(set, keyValue{string(role), value})
		}
	}
	return editTable(path, freshConfig(Config{}), tableModels, set)
}

// SetProvider writes p's baseUrl, and its api and decisionApi where p
// states them, into the providers.toml at path: into the table named p.Name,
// matched whatever its case, or a new one at the end of the file. A missing
// file is created holding the template's comments and this entry alone. Every
// other key, an entry's apiKeyFile included, stands. It reports whether it
// wrote: an entry already holding the values is left as it stands.
func SetProvider(path string, p Provider) (bool, error) {
	set := []keyValue{{keyBaseURL, p.BaseURL}}
	if p.API != "" {
		set = append(set, keyValue{keyAPI, p.API})
	}
	if p.DecisionAPI != "" {
		set = append(set, keyValue{keyDecisionAPI, p.DecisionAPI})
	}
	return editTable(path, providersTemplate(), p.Name, set)
}

// RemoveProvider removes the table named name, matched whatever its case,
// from the providers.toml at path, with the lines of its block, and reports
// whether one stood. Every other byte stands.
func RemoveProvider(path, name string) (bool, error) {
	before, err := readEditable(path)
	if err != nil {
		return false, err
	}
	prev, err := decodeEditable(path, before)
	if err != nil {
		return false, err
	}
	table := spelledTable(before, name)
	if _, ok := prev[table]; !ok {
		return false, nil
	}
	updated := removeLines(before, table)
	if !verifyEdit(prev, updated, func(doc map[string]any) { delete(doc, table) }) {
		return false, fmt.Errorf("kbase cannot remove the [%s] table without touching other lines; remove it from %s by hand", table, path)
	}
	return true, installConfig(path, updated)
}

// editTable sets each of set in the TOML file at path, under table — matched
// whatever its case — and writes the file atomically, creating it and its
// directory from template where it is absent or empty. It reports whether it
// wrote: keys already holding their values are not rewritten.
//
// The files are hand-editing surfaces first, so an existing one is updated
// line by line: the lines carrying the keys are rewritten, keeping their
// trailing comments at normalized spacing (commentGap); a missing key is added
// after its table's last key, and a missing table at the end of the file; every
// other byte — comments, grouping, spacing, other keys and tables — passes
// through untouched.
//
// The line editor understands the TOML layouts these files are written in,
// not all of TOML (a multiline string whose text looks like a table header is
// the known hazard). So the result is verified before it is installed: both
// versions are decoded, and unless they differ in exactly the intended values,
// nothing is written and the caller gets an error.
func editTable(path, template, table string, set []keyValue) (bool, error) {
	before, err := readEditable(path)
	if err != nil {
		return false, err
	}
	base := before
	if strings.TrimSpace(before) == "" {
		base = template
	}
	prev, err := decodeEditable(path, base)
	if err != nil {
		return false, err
	}
	table = spelledTable(base, table)
	set = unsettled(prev, table, set)
	if base == before && len(set) == 0 {
		return false, nil
	}
	updated := setLines(base, table, set)
	if !verifyEdit(prev, updated, func(doc map[string]any) { setKeys(doc, table, set) }) {
		return false, fmt.Errorf("kbase cannot rewrite these [%s] keys without touching other lines; set them in %s by hand", table, path)
	}
	return true, installConfig(path, updated)
}

// readEditable is the file at path, "" where none stands.
func readEditable(path string) (string, error) {
	src, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("config: read %s: %w", path, err)
	}
	return string(src), nil
}

// decodeEditable decodes src, the content of the file at path, refusing the
// edit where it is not valid TOML.
func decodeEditable(path, src string) (map[string]any, error) {
	doc := map[string]any{}
	if _, err := toml.Decode(src, &doc); err != nil {
		return nil, fmt.Errorf("config: refusing to rewrite %s: the file it already holds is not valid TOML: %w", path, err)
	}
	return doc, nil
}

// unsettled is the members of set whose key table does not already hold at
// that value.
func unsettled(doc map[string]any, table string, set []keyValue) []keyValue {
	current, _ := doc[table].(map[string]any)
	var out []keyValue
	for _, kv := range set {
		if v, ok := current[kv.key].(string); !ok || v != kv.value {
			out = append(out, kv)
		}
	}
	return out
}

// setKeys sets each of set under table in doc, replacing doc's table with a
// copy so the decoded original it was cloned from stands.
func setKeys(doc map[string]any, table string, set []keyValue) {
	current, _ := doc[table].(map[string]any)
	next := maps.Clone(current)
	if next == nil {
		next = map[string]any{}
	}
	for _, kv := range set {
		next[kv.key] = kv.value
	}
	doc[table] = next
}

// verifyEdit is the guard on the line editor: whether after decodes to
// exactly prev with edit applied. Anything else the edit touched — an unknown
// key, a table, the text inside a multiline string the editor mistook for
// configuration, TOML the edit broke — is a mismatch, and a mismatch refuses
// the write.
func verifyEdit(prev map[string]any, after string, edit func(doc map[string]any)) bool {
	want := maps.Clone(prev)
	edit(want)
	next := map[string]any{}
	if _, err := toml.Decode(after, &next); err != nil {
		return false
	}
	return reflect.DeepEqual(want, next)
}

// freshConfig renders the commented template for a config.toml that does
// not exist yet.
func freshConfig(cfg Config) string {
	return fmt.Sprintf(configTemplate,
		modelHintsComment(),
		assign(string(RoleDecision), cfg.Models.Decision),
		assign(string(RoleText), cfg.Models.Text),
		assign(string(RoleScoring), cfg.Models.Scoring),
		DefaultAskConcurrency)
}

// spelledTable is the name of src's first table header matching name
// whatever its case, else name.
func spelledTable(src, name string) string {
	for _, line := range strings.Split(src, "\n") {
		code, _ := splitComment(line)
		if header, ok := tableHeader(strings.TrimSpace(code)); ok && strings.EqualFold(header, name) {
			return header
		}
	}
	return name
}

// splitLines is src's lines, a trailing newline held aside so "the end of the
// file" means its last line of text, and whether src ended with one.
func splitLines(src string) ([]string, bool) {
	lines := strings.Split(src, "\n")
	endsWithNewline := len(lines) > 0 && lines[len(lines)-1] == ""
	if endsWithNewline {
		lines = lines[:len(lines)-1]
	}
	return lines, endsWithNewline
}

// joinLines is splitLines' inverse.
func joinLines(lines []string, endsWithNewline bool) string {
	out := strings.Join(lines, "\n")
	if endsWithNewline {
		out += "\n"
	}
	return out
}

// setLines rewrites the lines of src's table that assign a key of set and
// returns the new content. Keys the table does not have are added after its
// last key, and a missing table at the end of the file.
func setLines(src, table string, set []keyValue) string {
	if len(set) == 0 {
		return src
	}
	lines, endsWithNewline := splitLines(src)
	var (
		current string
		found   bool
		after   = -1 // the line a missing key goes after: the table's last key, else its header
		done    = map[string]bool{}
	)
	for i, line := range lines {
		code, comment := splitComment(line)
		trimmed := strings.TrimSpace(code)
		if name, isHeader := tableHeader(trimmed); isHeader {
			current = name
			if name == table && !found {
				found, after = true, i
			}
			continue
		}
		if current != table || trimmed == "" {
			continue
		}
		after = i
		key := lineKey(trimmed)
		for _, kv := range set {
			if kv.key == key && !done[key] {
				lines[i] = withComment(assign(key, kv.value), comment)
				done[key] = true
			}
		}
	}

	var missing []string
	for _, kv := range set {
		if !done[kv.key] {
			missing = append(missing, assign(kv.key, kv.value))
		}
	}
	if len(missing) == 0 {
		return joinLines(lines, endsWithNewline)
	}
	at := after + 1
	if !found {
		at = lastTextLine(lines, len(lines))
		missing = append([]string{"[" + table + "]"}, missing...)
		if at > 0 {
			missing = append([]string{""}, missing...)
		}
	}
	return joinLines(slices.Insert(lines, at, missing...), endsWithNewline)
}

// removeLines removes table's block from src: its header, through its last
// key line and the lines directly below that line with no blank between, and
// one blank line that would otherwise be left doubled. A comment set off by a
// blank line before the next header is left for that header.
func removeLines(src, table string) string {
	lines, endsWithNewline := splitLines(src)
	start, next := -1, len(lines)
	for i, line := range lines {
		code, _ := splitComment(line)
		name, isHeader := tableHeader(strings.TrimSpace(code))
		if !isHeader {
			continue
		}
		if start >= 0 {
			next = i
			break
		}
		if name == table {
			start = i
		}
	}
	if start < 0 {
		return src
	}
	lastKey := start
	for i := start + 1; i < next; i++ {
		if code, _ := splitComment(lines[i]); strings.TrimSpace(code) != "" {
			lastKey = i
		}
	}
	end := lastKey + 1
	for end < next && strings.TrimSpace(lines[end]) != "" {
		end++
	}
	blankBefore := start == 0 || strings.TrimSpace(lines[start-1]) == ""
	blankAfter := end == len(lines) || strings.TrimSpace(lines[end]) == ""
	switch {
	case blankBefore && blankAfter && start > 0:
		start--
	case blankBefore && blankAfter && end < len(lines):
		end++
	}
	return joinLines(slices.Delete(lines, start, end), endsWithNewline)
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
	if err := atomicfile.Write(path, []byte(content)); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	return nil
}

// assign renders one `key = "value"` line. Go's quoting and TOML's basic
// strings agree on everything a model id, a provider name or a URL can
// contain; a value they disagree on fails verifyEdit rather than landing on
// disk.
func assign(key, value string) string { return key + " = " + strconv.Quote(value) }

// splitComment cuts a line at its trailing comment, returning the text
// before the `#` and the comment from the `#` onward (empty when there is
// none). It is what lets a commented table header still read as one, and
// what lets a rewritten assignment keep the note beside it.
//
// The scan is quote-aware, because `#` is an ordinary character inside a
// value: a basic string honours backslash escapes, a literal string does
// not. A line whose quotes never close — in a real file that means a
// multiline string, whose interior this editor does not follow — yields no
// comment, and matches nothing.
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
// names it, unquoted. An array-of-tables header (`[[x]]`) counts: the keys
// under it belong to x, which is all the caller needs to know to leave them
// alone.
func tableHeader(trimmed string) (string, bool) {
	if !strings.HasPrefix(trimmed, "[") || !strings.HasSuffix(trimmed, "]") {
		return "", false
	}
	return strings.Trim(strings.TrimSpace(strings.Trim(trimmed, "[]")), `"'`), true
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
