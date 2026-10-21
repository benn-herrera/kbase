package config

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// harnessCommandKey is the one field a profile models, spelled as
// harnesses.toml spells it. The schema is closed: a profile states the command
// a run executes and nothing else, so a misspelled field refuses the load
// rather than leaving the command it was meant to change unread.
const harnessCommandKey = "command"

// profileEntry is one harnesses.toml table as the file states it.
type profileEntry struct {
	Command []string `toml:"command"`
}

// LoadHarnesses reads harnesses.toml at path: one profile per top-level table,
// keyed by the table's name, an open set — the grammar providers.toml uses for
// pool entries, and no name reserved. There is no nested form: a profile is
// `<name>` at the top level, never `[harness.<name>]`.
//
// A nonexistent or empty file yields no profiles and a nil error — a fresh
// install has no harnesses.toml yet. The error return is a file-level failure:
// the file unreadable, malformed TOML, a field the schema does not model, or a
// profile naming no command. A profile is a command; one that states none would
// load as a name a run could select and nothing to execute, so it refuses the
// whole load as a malformed file of either other kind does.
func LoadHarnesses(path string) (Harnesses, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Harnesses{}, nil
		}
		return nil, fmt.Errorf("harnesses: read %s: %w", path, err)
	}
	raw := map[string]profileEntry{}
	md, err := toml.Decode(string(data), &raw)
	if err != nil {
		return nil, fmt.Errorf("harnesses: parse %s: %w", path, err)
	}
	if err := rejectUnknownKeys(path, md); err != nil {
		return nil, fmt.Errorf("harnesses: %w", err)
	}
	out := make(Harnesses, len(raw))
	for name, entry := range raw {
		if len(entry.Command) == 0 {
			return nil, fmt.Errorf("harnesses: %s: the %q profile names no %s — a profile is the command a run executes, an array of strings that may not be empty",
				path, name, harnessCommandKey)
		}
		out[name] = Profile{Name: name, Command: entry.Command}
	}
	return out, nil
}

// CarriesModel reports whether the profile's command names the {model} token.
// Where it does not, the command asks its own configured model, which kbase
// cannot name: a document's model is then null even though the role is
// configured.
func (p Profile) CarriesModel() bool {
	return slices.ContainsFunc(p.Command, func(element string) bool { return strings.Contains(element, tokenModel) })
}

// takeHarnesses adds over's profiles to the merge, each naming the layer that
// supplied it. A profile a higher layer already supplied stands as it is: a
// same-named profile is replaced whole, so a repository cannot borrow the global
// profile's command for a name it chose. The standing profile records the layer
// whose entry it displaced — the loser drops out of the merge, and where the
// stack holds several lower layers it keeps the highest of them.
func takeHarnesses(merged Harnesses, over Harnesses, source string) {
	for name, p := range over {
		if standing, ok := merged[name]; ok {
			if standing.ReplacedLayer == "" {
				standing.ReplacedLayer = source
				merged[name] = standing
			}
			continue
		}
		p.Layer = source
		merged[name] = p
	}
}

// ProfileNames returns the loaded profiles' names, sorted — a stable list for a
// refusal the user is expected to choose from, and the order a report lists
// them in.
func ProfileNames(harnesses Harnesses) []string {
	names := make([]string, 0, len(harnesses))
	for name := range harnesses {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
