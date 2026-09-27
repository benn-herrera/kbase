package config

import (
	"fmt"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// unknownKeyHint closes every unknown-key refusal. The keys are named, so
// what the user needs next is where the real spelling is written down.
const unknownKeyHint = "check spelling against the documented schema"

// rejectUnknownKeys turns anything the decode did not consume into a load
// error naming the file and every offending key, and returns nil when the
// file decoded whole.
//
// It exists because the alternative is silence: a typo'd key or table
// ("modles" for "models") decodes into nothing at all, so the tweak the
// user made never takes effect and nothing says so — a discovery that
// otherwise arrives three hours into a pipeline run.
//
// ALL unknown keys are listed in one refusal, not just the first: a user
// fixing a config by trial and error is the failure mode this is meant to
// end, not to install one level up.
//
// Only key NAMES appear in the message, never values — providers.toml
// carries credentials, and this error travels the same console and log
// path every other loader error does.
func rejectUnknownKeys(path string, md toml.MetaData) error {
	undecoded := md.Undecoded()
	names := make([]string, 0, len(undecoded))
	for _, key := range undecoded {
		// An unknown table is reported alongside the unknown keys inside
		// it. Naming the leaf ("modles.heavy") locates the typo; naming
		// its parent as well only pads the message.
		if slices.ContainsFunc(undecoded, func(other toml.Key) bool { return isChildKey(key, other) }) {
			continue
		}
		names = append(names, `"`+key.String()+`"`)
	}
	if len(names) == 0 {
		return nil
	}
	noun := "unknown key"
	if len(names) > 1 {
		noun = "unknown keys"
	}
	return fmt.Errorf("%s: %s %s — %s", path, noun, strings.Join(names, ", "), unknownKeyHint)
}

// isChildKey reports whether child sits under parent (strictly: a key is
// not its own child).
func isChildKey(parent, child toml.Key) bool {
	return len(child) > len(parent) && slices.Equal(child[:len(parent)], parent)
}
