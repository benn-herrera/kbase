# CONVENTIONS — KBase

Instructions for AI agents using the kbase CLI. The adjacent README.md is the
human-facing guide; this file is the operating contract.

kbase converts a Markdown documentation corpus into an agent-friendly
knowledge base. It is a non-interactive batch tool: every verb runs to
completion and exits. Exit code 0 is success; nonzero is failure with the
reason on stderr. It never prompts.

## Current verbs

The KB-building pipeline verbs are not yet released. Available today:

- `kbase models [--provider NAME] [--timeout DUR]` — list the provider's
  model ids, one per line, sorted, on stdout. Summary line on stderr.
- `kbase configure [--provider NAME] [--model-map heavy=ID,light=ID]` —
  detect gemma-4 models on the endpoint and write tier assignments to
  config.toml.
- `kbase --version` — version string only.
- Every verb supports `--help`; trust it as current.

## Configuration

Config directory resolution: `--config-dir` flag > `$KBASE_CONFIG_DIR` >
`~/.config/kbase`.

`providers.toml` — endpoint pool. Name-keyed tables:

```toml
[local]
baseUrl = "http://localhost:8000/v1"
apiKeyFile = "local.key"   # path relative to this file — PREFER this form
# apiKeyUnsafe = "sk-..."  # inline key; avoid writing secrets into config
```

`config.toml` — active choices (`provider = "name"`, `[models]` with
`heavy`/`light`). Safe to edit by hand and safe to edit programmatically:
`kbase configure` rewrites only the values it owns and preserves all other
lines, comments included.

Provider selection for any verb: `--provider` > config.toml `provider` >
sole pool entry. With several entries and none chosen, kbase lists the
candidates and fails — rerun with `--provider`.

## Failure semantics you should rely on

- **No silent guessing.** If gemma-4 tier detection finds zero or several
  candidates for a tier, `configure` fails, prints every model id the
  endpoint offered, and states the fix. Recover by rerunning with explicit
  assignment: `kbase configure --model-map heavy=<id>,light=<id>`.
- **No partial writes.** A failed `configure` leaves config.toml untouched.
- **Unusable pool entries warn, never abort** the whole pool; the warning
  names the entry and reason (never key material).
- kbase requires an OpenAI-compatible endpoint serving gemma-4-family
  models; other model families are usable only via explicit `--model-map`.

## Handling secrets

Never write API keys into providers.toml inline, into config.toml, or into
shell history when a key file works: put the key in a file beside
providers.toml and reference it with `apiKeyFile`. kbase never prints key
material; neither should you.
