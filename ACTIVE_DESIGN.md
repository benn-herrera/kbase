# Design: credentials — a store kbase can hold without being a keystore

**Why:** kbase needs an API key for `build` far more than for `score`, and build cannot delegate that
to a runtime: it is kbase's own pipeline, many calls per unit, and it runs in CI where no runtime is
logged in. So kbase will hold a credential. The question this design answers is to what standard.

## The problem

A provider's key reaches kbase today only through a path: `apiKeyFile` in a `providers.toml` entry,
read when the configuration loads (`internal/config/providers.go:258`, `:272`). The failure mode is
not an attacker: it is an agent being helpful, reading the file to check the key is configured, and
putting the key in a transcript.

## Decisions taken

1. **kbase holds one store, of the opencode kind,** one file per provider at
   `~/.local/state/kbase/auth/<provider>-key.enc` — `$XDG_STATE_HOME/kbase/auth/` where that
   variable is set, as kbase's state base already resolves. A key is encrypted at rest with a per-user key
   derived by HKDF from a key compiled into the binary, the username as HKDF's `info` (stdlib
   `crypto/hkdf`). The compiled-in key is a plain constant in the source: a build-time key would
   leave each release unable to read the last one's store, and disguising a constant in public
   source hides nothing. Nothing binds a ciphertext to its provider's name, so a user can rename a
   key file by hand when they rename a provider; kbase leaves an orphaned file where it is, so
   naming the provider back restores its key, and `kbase auth remove` clears it. Documented as obfuscation, explicitly not a boundary: a person running as the
   same user can recover it; a model doing `cat` gets ciphertext. Separation between users is the
   store file's `0600` mode, not the derivation.
2. **Adding a key is `kbase auth add <provider>`, and the key arrives on stdin, never as an
   argument.** `<provider>` is a `providers.toml` entry name, never a role: a key belongs to the
   endpoint that dials, and a role reaches it only through `config.toml`'s model reference.
   - `add` refuses a name that is not a configured entry, naming the name as given, with
     `kbase show-config` — which lists the entries — as its remedy; a configuration that does not
     load refuses as it does for any verb. `remove` takes any name, so an orphaned file can be
     cleared. Refusals and remedies name the provider as given.
   - A provider name matches `^[A-Za-z0-9_][A-Za-z0-9_-]*$`: a `providers.toml` entry that does
     not fails the configuration's load, and the store refuses such a name too. Two entries whose
     names differ only by case are refused, within a file or across layers: an override names its
     entry exactly. A key file's name is the provider's name lowercased, so a hand-renamed file is
     found only under a lowercase name, and `auth list` names the providers in lowercase.
   - The key is piped in: `<key-manager> --print-key | kbase auth add <provider>`. A key manager
     feeds it hands-free, and the key never enters an agent's context, even when the agent runs that
     pipeline.
   - Stdin that is a tty is refused, writing nothing: kbase never prompts for a key, so no key is
     typed where a terminal, a scrollback or a session transcript can hold it. The refusal names the
     piped form.
   - `auth add`'s help shows the key manager form and the clipboard form, for a key copied there by
     hand or by a program: `<your-system-clipboard-paster> | kbase auth add <provider>`. The help
     names no platform's command; the agent on the scene supplies the one its system has.
   - The acknowledgement says nothing about the key.
   - `kbase auth remove <provider>` clears the key; `kbase auth list` names the providers that have
     a stored key, by name only.
3. **The auth verbs are CLI-only.** An MCP tool call has no stdin, and no tool or tool argument
   accepts a key: the verbs are declared `cliOnly`.
4. **`providers.toml` names a provider, never a key, on the served surface.** `apiKeyFile` stays
   supported — for CI and mounted secrets, where nothing is curious and a file is exactly right —
   but kbase's served surface does not document it: no command help, MCP instructions, refusal
   remedy or template text an agent reads names it. Agents guide users through `kbase auth add`.
   The one exception is `show-config`'s report, which documents where each key came from: while
   `apiKeyFile` is supported, it keeps reporting a key from a file (`key: file` and its `key-file`
   path) beside one from the store. For each such entry it also gives the upgrade, values filled
   in: `cat <key-path> | kbase auth add <provider>`. The store wins over a stated `apiKeyFile`, so
   that one command completes the upgrade. Where an entry has both, `show-config` reports
   `key: store` and a warning that the entry's `apiKeyFile` is not read, its remedy: remove the
   `apiKeyFile` line from the `providers.toml` that states it, or `kbase auth remove <provider>` to
   use the file.
5. **No key means no header.** A provider configured with a base URL and no credential sends no
   `Authorization` header — already true: an empty `APIKey` sends none
   (`internal/model/openai_client.go:89`). That is what makes a loopback credential proxy possible
   later without a code change.
6. **The agent names the command; the key stays out of the conversation.** Where a provider
   refuses a call for its key, the failure's remedy is the piped form,
   `<key-manager> --print-key | kbase auth add <provider>` (`config.AuthAddRemedy`), the name filled
   in. An agent tells the user to run it, or runs it against the user's key manager; it never asks
   for the key itself.

## Considered and set aside

- **OS keychain per platform** (`security`, `secret-tool`, Windows Credential Manager): three
  resolvers, three failure modes (no session bus, no gpg-agent, no PowerShell module), and the same
  ceiling.
- **gpg-encrypted key file + gpg-agent**: portable and honest, but it makes gpg a kbase dependency
  and the passphrase's availability a runtime condition.
- **Loopback credential proxy**: the only shape where kbase never holds the key, and the only one
  that handles providers needing signed requests. It needs a process, a start-up story and its own
  per-provider auth shapes. Worth revisiting when a signed-request provider matters.

## Outline of the work

The plan built from this design rates and scopes each part.

1. **The store and the verbs.** `internal/auth`: the on-disk blob (AES-GCM under the per-user key,
   a nonce per write, mode `0600`), read and write; `kbase auth add <provider>`,
   `kbase auth remove <provider>` and `kbase auth list` as decision 2 states; all `cliOnly`.
2. **Resolution.** An entry's key is the store's, looked up by the entry's name whatever layer
   supplied the entry, else its `apiKeyFile` where one is given, else none (decision 5). Nothing is
   checked before dialling: a provider that answers 401 or 403 fails the call as `provider-refused`
   does today, and that failure's item carries decision 6's piped remedy, the provider's name
   filled in — for a missing key and a wrong one alike, since adding a key replaces
   the stored one. A project that overrides a provider and needs a different key manages that key
   itself; there is no per-project store. SPEC §9's "a repository cannot inherit a credential for
   an endpoint it chose" narrows to `apiKeyFile`.
3. **Reporting.** `show-config` reports a key's source: `store`, or `file` with its path as today,
   never its value; for a `file` key, the upgrade to the store; for an entry with both, the
   unread-`apiKeyFile` warning (decision 4). Both are `show-config`'s alone: no other verb's output
   carries them.
4. **The served surface** moves off `apiKeyFile` (decision 4), but for `show-config`'s report:
   today it is named in a refusal detail (`internal/config/resolve.go:291`) and the
   `providers.toml` template `configure --create` writes (`internal/config/template.go:51`,
   `:61`).
5. **Contract.** SPEC §9.1, §9.3 and §9.4's key-file text, CONVENTIONS, README's setup; an adjagent
   change request for the skill.

## Acceptance

- `kbase auth add x` with a tty on stdin refuses, naming the piped form, and writes nothing; with
  empty piped stdin it refuses and writes nothing.
- A round trip pipes, stores and resolves a key; `show-config` names the store, never the key;
  `auth list` names the provider.
- No verb accepts a key on argv, and no MCP tool accepts one at all.
- No served text an agent reads names `apiKeyFile`.
- `git grep` over the suite finds no test that prints a key into a failure message.
- `just checkpoint` green.
