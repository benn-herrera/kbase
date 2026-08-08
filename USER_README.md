# kbase

**Turn a human-targeted documentation corpus into an agent-friendly knowledge
base.**

Point kbase at a Markdown documentation set and it produces a navigable
Markdown tree — entry point → domain index → subtopic index → leaf — built for
AI agents to consume:

- **Verbatim leaves.** Leaf pages are faithful translations of the source, not
  paraphrases. Summaries exist only to route navigation: *summaries route,
  leaves answer.*
- **Mechanically generated navigation.** Tree links are emitted
  deterministically — dead links are impossible by construction. Cross-
  references come only from the source's own links; nothing is invented.
- **Self-describing artifact.** Every generated KB ships a `.agents/`
  directory with agent definitions and a routing eval — any agent that picks
  up the KB finds its operating manual inside.
- **Provenance.** Every KB records the app version, model IDs, and source
  identity that produced it.

kbase is a batch appliance: it runs its pipeline to completion and exits.
There is no interactive mode.

## Status

Early release. The `models` and `configure` verbs below work today; the
KB-building pipeline verbs are landing next.

## Install

Pick the binary for your platform from this distribution, put it on your
`PATH`, and rename it to `kbase` (or `kbase.exe` on Windows):

| Platform | Binary |
|---|---|
| macOS (Apple silicon) | `kbase-darwin-arm64` |
| Windows (x86-64) | `kbase-windows-amd64.exe` |
| Linux (x86-64) | `kbase-linux-amd64` |

## Setup

kbase needs an OpenAI-compatible API endpoint serving **gemma-4-family
models** — local (vLLM, llama.cpp, Ollama, …) or any cloud host.

1. Create `~/.config/kbase/providers.toml` (the directory can be overridden
   with `$KBASE_CONFIG_DIR` or `--config-dir`). Each entry names an endpoint:

   ```toml
   [local]
   baseUrl = "http://reaper.local:8000/v1"
   apiKeyFile = "local.key"     # path relative to this file; preferred

   [cloud]
   baseUrl = "https://api.example.com/v1"
   apiKeyFile = "cloud.key"
   # apiKeyUnsafe = "sk-..."    # inline key; discouraged (secret in config)
   ```

2. Check what the endpoint serves:

   ```sh
   kbase models --provider local
   ```

3. Auto-detect and assign the gemma-4 model tiers:

   ```sh
   kbase configure --provider local
   ```

   This scans the provider's model list and writes the tier assignments to
   `~/.config/kbase/config.toml`. If detection is ambiguous (or the endpoint
   serves no gemma-4 models), kbase lists everything it found and fails
   rather than guessing; assign manually with
   `kbase configure --model-map heavy=<id>,light=<id>`.

`config.toml` is yours to hand-edit; `kbase configure` updates only the
values it owns and preserves everything else, comments included.

## License

MIT (see `LICENSE`). Generated KBs inherit the license of their source
corpus — a KB built from CC-BY-4.0 docs is itself a derivative of CC-BY-4.0
material.
