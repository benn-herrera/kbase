# Plan: credentials — a store kbase can hold without being a keystore

**Date:** 2026-10-21
**State:** design only. Nothing here is built.
**Why now:** kbase needs an API key for `build` far more than for `score`, and build cannot delegate
that to a runtime: it is kbase's own pipeline, many calls per unit, and it runs in CI where no runtime
is logged in. So kbase will hold a credential. The question the plan answers is to what standard.

## The problem

A provider's key reaches kbase today through a path — `apiKeyFile` in `providers.toml`, or
`KBASE_API_KEY_FILE` — and `clients.key` reads that file at dial time (`cmd/provider.go:149`). The
failure mode is not an attacker: it is an agent being helpful, reading the file to check the key is
configured, and putting the key in a transcript.

## Decisions taken

1. **kbase holds one store, of the opencode kind.** A key is encrypted at rest with a key compiled into
   the binary, under kbase's config directory. Documented as obfuscation, explicitly not a boundary: a
   person running as the same user can recover it; a model doing `cat` gets ciphertext.
2. **Adding a key is the user's act, and only theirs.** `kbase auth add <provider>` prompts on the
   terminal, hides the echo, and refuses when stdin is not a tty. There is no `--key`, and no
   non-interactive form. The refusal is the safety property, not a convenience: an agent invoking it
   in a tool call is refused, so the key cannot reach a context.
3. **`providers.toml` names a provider, never a key.** No path for a model to follow and nothing to
   `cat`. `apiKeyFile` and `KBASE_API_KEY_FILE` survive only for CI and mounted secrets, where nothing
   is curious and a file is exactly right.
4. **No key means no header.** A provider configured with a base URL and no credential sends no
   `Authorization` header — already true: `model.APIKey` empty sends none
   (`internal/model/openai_client.go:55`). That is what makes a loopback credential proxy possible later
   without a code change.
5. **The skill asks; the agent does not.** adjagent's reference says: if a verb refuses naming a
   missing key, ask the user to run `kbase auth add <provider>` in their own terminal. Do not run it
   for them, and do not ask for the key.

## Considered and set aside

- **OS keychain per platform** (`security`, `secret-tool`, Windows Credential Manager): three resolvers,
  three failure modes (no session bus, no gpg-agent, no PowerShell module), and the same ceiling.
- **gpg-encrypted key file + gpg-agent**: portable and honest, but it makes gpg a kbase dependency and
  the passphrase's availability a runtime condition.
- **Loopback credential proxy**: the only shape where kbase never holds the key, and the only one that
  handles providers needing signed requests. It needs a process, a start-up story and its own per-
  provider auth shapes. Worth revisiting when a signed-request provider matters.

## Steps

Each is a dispatch. Rating decides the model (project `AGENTS.md`).

| # | step | rating |
|---|---|---|
| 1 | **The store and the verb.** `internal/auth`: the on-disk blob (AES-GCM under a key compiled into the binary, nonce per write, file mode 0600 under the config dir), read and write; then `kbase auth add <provider>` — prompt on the tty with echo off, refuse without a tty, print `stored` and nothing about the key, and `kbase auth rm <provider>`. The refusal path is the tested one. | expert |
| 2 | **Resolution.** `clients.key` (`cmd/provider.go:149`) consults the store when no path is set: `apiKeyFile` wins where a path is given, then the store, then nothing. A missing entry is the refusal that names `kbase auth add`, naming the provider. | expert |
| 3 | **Reporting.** `show-config` records that a key came from the store, by name and never by value; the provenance receipt likewise. | mechanical |
| 4 | **Contract.** SPEC §9.3's provider and key-file text (from `SPEC.md:1069` and `:1236`), `CONVENTIONS`, `USER_CONVENTIONS`, README's setup, and the skill line above for adjagent. | tech-writer seat |

## Acceptance

- `kbase auth add x < /dev/null` refuses, naming the tty requirement, and writes nothing.
- A round trip stores and resolves a key; `show-config` and the receipt name the store, never the key.
- `git grep` over the suite finds no test that prints a key into a failure message.
- No verb accepts a key on argv or from a non-tty stdin.
- `just checkpoint` green.

## Open

- **Which provider names the verb takes**: a role (`scoring`), a pool entry name, or both. The role is
  what the user thinks in; the pool entry is what dials.
- **What the fixed key is**: a constant in the source, or derived at build time. Neither is a boundary;
  the first is simpler to reproduce across a release.
- **Whether the store is one file or one file per provider**, and what happens to it when a provider is
  renamed in `providers.toml`.
- **`auth list`**: whether showing *which* providers have a key is worth a verb, given it tells a model
  what is configured.
