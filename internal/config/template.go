package config

// The two templates a configuration directory is created with — the only
// prose kbase writes about its own file formats, in one home. Each is what a
// file that does not exist yet is created as, and both read as unset to every
// loader: what a template states is commented out, and what a fresh install
// must fill in is left blank.

// configTemplate is what a config.toml that does not exist yet is created
// as. It is commented because the file is a hand-editing surface first and
// a program output second: a user who opens it should find out from the
// file itself what may be changed and what `kbase configure-models` will
// rewrite. The [models] guidance it carries is hints.go's, rendered here and as
// the kbase://model-hints resource.
const configTemplate = `# kbase configuration — safe to hand-edit; ` + "`kbase configure-models`" + ` rewrites
# the [` + tableModels + `] values it is given and leaves everything else in this file
# exactly as it found it.

%s

[` + tableModels + `]
%s
%s
%s

# [asks]
# concurrency = %d        # model calls one build or score has in flight at once,
#                        # across every role; a whole number, at least 1.

# [sampling.text]          # the overview passage; [sampling.scoring] takes the
#                          # same two keys for kbase score. Unset: temperature 0,
#                          # thinking "off" — kbase's default, not the server's.
# temperature = 0          # 0 to 2 on an "openai" entry; 0 to 1 on an "anthropic"
#                          # one
# thinking = "off"         # off | low | medium | high
`

// providersTemplate is what a providers.toml that does not exist yet is
// created as, before the entry being written is added at its end: comments
// alone, its example entry commented out, so the file holds the entries
// written to it and no other. It names no key field, because a key is added
// with `kbase auth add`; an apiKeyFile an entry states still loads. It is
// built by concatenation, so no character in it is a format verb; it carries
// AuthAddRemedy's form for a placeholder name. The example's base URL is a
// lab host, so no placeholder can reach a real vendor.
func providersTemplate() string {
	return `# Each provider is one top-level table; ` + ProviderNameRule + `;
# no two names may differ by case alone. The name is what a [models]
# reference's provider part and --provider name. An entry in a repository's
# file replaces the global entry of the same name (any case) whole.
# This file holds no API key: an entry's key goes in kbase's store, under the
# entry's name:
#   ` + AuthAddRemedy("<name>") + `
# kbase configure-providers adds, edits and removes an entry. An entry reads:
#
# [local]
# baseUrl = "http://lab-server.local:8000/v1"
# type = "inference"  # optional; the only kind kbase can use
# api = "anthropic"   # optional; the chat protocol: "anthropic" (the default),
#                     # the Messages API, or "openai"
# decisionApi = "systemone"  # optional; which decision API this server
#                            # speaks: "systemone" (the default) or
#                            # "decisions"
`
}
