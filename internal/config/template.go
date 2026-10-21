package config

// The three templates a configuration directory is created with — the only
// prose kbase writes about its own file formats, in one home. Each is what a
// file that does not exist yet is created as, and all three read as unset to
// every loader: what a template states is commented out, and what a fresh
// install must fill in is left blank.

// configTemplate is what a config.toml that does not exist yet is created
// as. It is commented because the file is a hand-editing surface first and
// a program output second: a user who opens it should find out from the
// file itself what may be changed and what `kbase configure` will rewrite.
// The [models] guidance it carries is hints.go's, rendered here and as the
// kbase://model-hints resource.
const configTemplate = `# kbase configuration — safe to hand-edit; ` + "`kbase configure`" + ` rewrites the
# [` + tableModels + `] values and removes a superseded ` + "`" + keyProvider + "`" + ` key, and leaves
# everything else in this file exactly as it found it.

%s

[` + tableModels + `]
%s
%s
%s

# [asks]
# readerConcurrency = 4  # asks of one group in flight once its first has
#                        # returned; a whole number, at least 1.
`

// providersTemplate is what a providers.toml that does not exist yet is
// created as. One entry is live, so the pool holds something a selection
// resolves without a choice; the rest is commented, so a fresh install never
// has two usable entries and no selection between them. apiKeyFile is
// commented because a live one naming a file that is not there drops its
// entry — a fault where an endpoint was meant. The live base URL is a lab
// host, and .example is a reserved TLD that can never resolve, so no
// placeholder can reach a real vendor.
const providersTemplate = `# Each provider is one top-level table. The names are arbitrary: they are
# what a [models] reference's provider part and --provider name. An entry
# here replaces a same-named entry from ~/.config/kbase/providers.toml whole.

[local]
baseUrl = "http://lab-server.local:8000/v1"
# apiKeyFile = "~/.api_keys/lab-server.txt"  # relative paths resolve beside this file
# type = "inference"  # optional; the only kind kbase can use
# api = "openai"      # optional; the only protocol it speaks
# decisionsFlavor = "systemone"  # optional; which decision surface this server
#                                # speaks: "systemone" (the default) or
#                                # "decisions"

# [cloud]
# baseUrl = "https://api.inference.example/v1"
# apiKeyFile = "~/.api_keys/cloud.txt"
`

// harnessesTemplate is what a harnesses.toml that does not exist yet is created
// as. One profile is live — `claude`, which takes the prompt on stdin with its
// tools removed — so `kbase score --harness claude` runs from a fresh install.
// The second ships commented, with the limitation of its command stated in the
// file the user edits: `opencode run` has no flag that removes tools or
// suppresses instructions, so enabling it means naming an agent that cannot
// read the KB. Neither command names {model}: each runs on the harness's own
// configured model, which is what a fresh install can rely on, and the header
// says how to opt a profile into --scoring-model.
const harnessesTemplate = `# kbase harness profiles — safe to hand-edit. Each profile is one top-level
# table, named the way ` + "`--harness`" + ` names it, holding the command kbase runs
# for one item: one process per item, the prompt on its stdin or in the
# element carrying ` + "`{prompt}`" + `, and its stdout the reply. An entry here
# replaces a same-named entry from ~/.config/kbase/harnesses.toml whole.
#
# A profile whose command carries ` + "`{model}`" + ` is handed the scoring role's
# model, and ` + "`--scoring-model`" + ` overrides it. A profile naming no ` + "`{model}`" + `
# runs on its own configured model and kbase records none: add
# --model {model} (claude) or -m {model} (opencode) to have it take one.

[claude]
command = ["claude", "-p", "--tools", "", "--strict-mcp-config", "--safe-mode"]

# [opencode]
# command = ["opencode", "run", "--format", "default", "{prompt}"]
# Off by default: ` + "`opencode run`" + ` (v2.0.32) has no flag that removes tools or
# suppresses instructions, so a profile runs the default ` + "`build`" + ` agent with
# its tools, its own system prompt and the instruction files opencode loads —
# and an agent with tools can read the KB the ask is about. Run an agent that
# cannot instead, by naming it with ` + "`--agent`" + `: that agent comes from the
# agent set you install, not from kbase.
`
