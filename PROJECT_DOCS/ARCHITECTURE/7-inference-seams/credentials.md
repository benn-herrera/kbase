### Credentials

**Pattern:** Local secrets file with a redacting secret type.

**Store.** `internal/auth` keeps one file per provider, `<provider>-key.enc` (`auth.FileSuffix`), in
`auth.DirName` under `statedir.Base`. The directory sits beside the per-KB state directories, whose
16-hex-digit names it cannot collide with. The name is lowercased, and `auth.CheckName` runs before any
path is built from it. `auth` imports no `config`, so `auth.NamePattern` restates
`config.ProviderNamePattern`, and `cmd/auth_test.go` pins the two equal. A key is sealed with
AES-256-GCM under a per-user key that HKDF derives from a constant in the source and the OS username
(`Store.sealer`). The constant is public on purpose: a key chosen at build time would leave each
release unable to read the last one's store, and disguising a shipped constant hides nothing. The
sealing is obfuscation, not a boundary: any process of the same user recovers the key, and separation
between users is the file's mode. `auth.FileMode` applies on creation and on every replace
(`atomicfile.WriteMode`), and `auth.DirMode` on every `Add`. The provider's name is not bound into the
file, so a file renamed by hand opens under its new name. The threat the store answers is an agent
being helpful — reading a key file and putting the key in a transcript — not an attacker: an agent
reading the file gets ciphertext. kbase uses no OS keychain, which would take three resolvers with
three failure modes (no session bus, no gpg-agent, no PowerShell module) for the same ceiling, nor
gpg, which would make gpg a dependency and a passphrase a runtime condition; and a stored key serves
builds that run in CI, where no one is logged in.

**Secret type.** `secret.Key` is the type every key travels in. fmt's verbs, a text, JSON or YAML
encoding and a log value all render `<redacted>`, and `Reveal` alone returns the text.
`cmd/revealpolicy_test.go` allows `Reveal` in four production files: `internal/auth/auth.go`, the two
wire clients, which set the auth header and scrub an echoed key, and `internal/model/transport.go`,
which truncates a status error's body around a key.

**Read path.** `cmd`'s `loadConfigContext` reads every file of the store through `internal/auth`
(`readStoredKeys`) before the load and gives `config.Load` a `config.StoredKeys`: each key that
opened, by the lowercase name its file is stored under, and a `config.StoreFault` per file that did
not, built from the `*auth.FileError`'s path and error apart. A store directory that exists and does
not list is one nameless `StoreFault` against the directory, with no keys, and the load goes on.
`config` imports no `auth`.

**Precedence.** `keyKind` is the single source of truth for which key an entry uses: the key stored
under the provider's name, whatever layer supplied its table; else the table's `apiKeyFile`, which
`LoadProviders` then reads, and which a provider with a stored key never reads, so it raises neither a
read fault nor a permission warning; else none (SPEC §9 The files are the one source of inference).
`keyFileExposure` warns on a key file readable beyond its owner, except on Windows (SPEC §9.4 Key-file
permission warning).

**Faults.** `config.Load` turns each store fault whose name a configured provider has into a warning
`ProviderFault` naming that provider, with `Source` `SourceStoreFile`, `Path` the store file and
`DoesNotOpen` the store fault's. A file no provider names is read by nothing and not reported. The
nameless directory fault names no provider and becomes a warning with `Source` `SourceStoreDir`,
`Path` the directory, which `warnFaults` writes as the credentials store's. A `ProviderFault`'s `Reason`
never repeats its `Path`; `ProviderFault.LocatedReason` leads a store fault's reason with its path for a
line that is complete on its own (`warnFaults`' and a provider's `fault` in `show-config`). `config.Report` reports
a store fault as its path and reason, with `config.AuthAddRemedy` as the remedy where the file does not
open and for a provider dropped because its `apiKeyFile` could not be read (`ProviderFault.KeyFileUnread`),
the stored key outranking the file. It alone computes a `file` key's `upgrade` and the warning that a
stored key leaves an `apiKeyFile` unread, so `show-config` alone emits them.

**Input and transport.** `kbase auth add` reads the key from stdin only, at most `authAddInputLimit`
bytes ([§12](../12-constants/index.md)); it reads one byte past the limit to tell more from exactly. A configured key
sent to an `http://` base URL on a host other than loopback travels in cleartext. `build`, `score` and
`probe` write one stderr line saying so through `warnCleartextKey` (a build writes one per endpoint),
which names the host and never the key, and proceed. No switch turns the warning off.

---

