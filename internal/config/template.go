package config

// The two templates a configuration directory is created with — the only
// prose kbase writes about its own file formats, in one home. Each is what a
// file that does not exist yet is created as, and both read as unset to every
// loader: what a template states is commented out, and what a fresh install
// must fill in is left blank.

// configTemplate is what a config.toml that does not exist yet is created
// as. It is commented because the file is a hand-editing surface first and
// a program output second: a user who opens it should find out from the
// file itself what may be changed and what `kbase configure` will rewrite.
// The [models] guidance it carries is hints.go's, rendered here and as the
// kbase://model-hints resource.
const configTemplate = `# kbase configuration — safe to hand-edit; ` + "`kbase configure`" + ` rewrites the
# [` + tableModels + `] values and leaves everything else in this file exactly as it found it.

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
// created as. One entry is live, so the pool holds something a selection
// resolves without a choice; the rest is commented, so a fresh install never
// has two usable entries and no selection between them. It names no key
// field, because a key is added with `kbase auth add`; an apiKeyFile an entry
// states still loads. The %s is AuthAddRemedy's form for a placeholder name,
// filled in by freshProviders. The live base URL is a lab
// host, and .example is a reserved TLD that can never resolve, so no
// placeholder can reach a real vendor.
const providersTemplate = `# Each provider is one top-level table; ` + ProviderNameRule + `;
# no two names may differ by case alone. The name is what a [models]
# reference's provider part and --provider name. An entry here replaces the
# entry of exactly the same name from ~/.config/kbase/providers.toml whole.
# This file holds no API key: an entry's key goes in kbase's store, under the
# entry's name:
#   %s

[local]
baseUrl = "http://lab-server.local:8000/v1"
# type = "inference"  # optional; the only kind kbase can use
# api = "anthropic"   # optional; the chat protocol: "anthropic" (the default),
#                     # the Messages API, or "openai"
# decisionsFlavor = "systemone"  # optional; which decision surface this server
#                                # speaks: "systemone" (the default) or
#                                # "decisions"

# [cloud]
# baseUrl = "https://api.inference.example/v1"
`
