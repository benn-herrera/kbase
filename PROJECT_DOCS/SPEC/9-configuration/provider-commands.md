### Provider commands

`kbase models [--provider NAME] [--timeout DURATION]` lists the selected
provider's model identifiers, sorted ascending, in its result document ([§7](../7-tool-result-contract-and-exit-codes/index.md); [page §3.7](../../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/models.md)).
Every input naming a provider — `--provider`, a model reference's provider part, `auth add` and
`auth remove` — is folded to lowercase, and every output names it so ([page §2.7](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.7-provider-names.md)).
`kbase configure-providers` and `kbase configure-models` write `providers.toml` and `config.toml` as
[page §3.7](../../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/configure-providers.md) and its [`configure-models`](../../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/configure-models.md) leaf state, where they write as "The configure commands' write
location" states, and under [§9.5](9.5-the-configure-commands-targeted-update-byte-preservation-contract.md). What the page leaves to this implementation:

- `configure-providers` refuses, `check: usage`: a write with no `--base-url`; a `--base-url`
  carrying credentials (`user:key@` or `key@`), the `kbase auth add` pipe as its remedy,
  its detail ending as a refused key's does ([§9](a-refused-key.md), "A refused key"), checked
  before any other fault of it; a `--base-url` that is no absolute `http` or `https` URL; no
  refusal of `--base-url` names any part of it. Also refused: an `--api` other than `anthropic` or `openai`, or a
  `--decisions-flavor` other than `systemone` or `decisions`, each with those values as `allowed`;
  and `--remove` beside an entry field. A provider name breaking the provider-name rule ([§9.1](9.1-providers-toml-the-endpoint-pool/index.md)) is
  refused keyed `<provider>`. `--remove` of a provider the file holds no entry for is refused
  `check: provider`, `path` that `providers.toml`, detail ``no provider `<name>` in <path>``, and
  `kbase show-config` as the remedy.
- `configure-providers` dials where the written entry has a key — stored, or read from the file
  its `apiKeyFile` names — under a 30 s deadline, and gives the catalogue, sorted ascending, as
  `models`. With no key, `next` is two commands: the `kbase auth add` pipe, the provider's name
  filled in, then `kbase models --provider <name>`. Where the dial fails, `next` is one entry
  naming the failure, then the `configure-providers` call for a wrong URL, the `kbase auth add`
  pipe for a wrong key, and `kbase models --provider <name>`; where the stack written
  into does not reload, one entry naming that and `kbase show-config`; where the written entry
  does not load, one entry naming its fault's reason and `kbase show-config`.
- `configure-providers --remove` gives `next` as page `configure-providers` "Result" states, where
  neither the layer written nor a layer beneath it still holds the provider: one entry per role
  whose reference names it, giving the role, the reference and the `configure-models` call,
  carrying the removal's location flag, that replaces it; then, where a key file is stored under
  the name, one entry giving `kbase auth remove <name>`, saying where the file does not open. With
  nothing to list there is no `next`.
- `configure-providers` and `configure-models` read only the stack they write into: a layer
  outside it that fails the strict load does not stop them. Each reports every fault of that
  stack as an item of `warnings`, as `kbase models` reports them — `configure-providers` on every
  outcome from its write or removal on, `configure-models` on every outcome once the stack loads.
- `configure-models` folds its provider part to lowercase ([page §2.7](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.7-provider-names.md)) and writes it so. It refuses a value that is no reference and a provider
  the written layer's stack does not hold — the layer written and the layers beneath it: the
  global layer beneath the repository one, nothing beneath any other — each keyed by its role's
  flag, every one at once, before any dial. The latter's `path` is the written layer's
  `providers.toml`, its remedy the `configure-providers <provider> --base-url <url>` call carrying
  the location flag the call carried, then `kbase show-config`; a provider dropped from that stack
  by a fault keeps the fault's reason, as "Provider selection" below states. A run naming no role
  is a usage refusal. A catalogue that
  cannot be listed is `failed`, nothing written; a model missing from its catalogue is an item
  of `warnings`, its `key` the role's flag, and a line on stderr. `models` is the three roles the
  written file holds, null where it holds none.

`kbase show-config [--provider NAME]` reports how the configuration
resolves and changes nothing ([§9.6](9.6-kbase-show-config/index.md)). `kbase probe [--model REF] [--decision | --text | --scoring]
[--prompt TEXT] [--timeout DURATION]`, also the `probe` tool, asks one selection — a role flag's
role, else the `scoring` role, or the reference `--model` names — one question and reports the
call, reading no KB and writing no state ([§8](../8-maintenance-subcommands/probe.md)). Each writes its result under [§7](../7-tool-result-contract-and-exit-codes/index.md).

`kbase auth add <provider>` stores the API key piped on its stdin for `<provider>`, a
`providers.toml` entry name, replacing any key stored for it: one file per provider,
`$XDG_STATE_HOME/kbase/auth/<provider>-key.enc` (`~/.local/state` where the variable is unset or not an absolute path),
the file named for `<provider>` in lowercase, each file encrypted under an encryption key derived
for the OS user — obfuscation against a reader of the file, not a boundary against that user — and
readable by its owner alone. `add` reads the configuration as the provider verbs do, and a
configuration that does not load refuses as it does for them; a name, folded to lowercase, that no
`providers.toml` entry carries — in the pool or dropped from it by a fault — is refused,
`check: usage`, `key` the folded name, detail ``no provider `<name>` in providers.toml (names match in any case)``, `remedy: kbase show-config`. The key arrives on stdin and
no other way: stdin that is a terminal is refused, as is input of more than 65536 bytes, input that
is empty once surrounding whitespace is trimmed, and a key that, so trimmed, still holds a
whitespace or control character, each `check: usage`, naming no part of the input, with the piped
form, the provider's name filled in, as its `remedy`. A refusal writes nothing, and the name refusals come before stdin is read. `kbase auth remove <provider>` clears the stored key; where none is stored it is refused
([page §2.10](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)), `check: usage`, `key` the folded name, detail ``no key stored for provider `<name>` ``,
`remedy: kbase auth list`. `kbase auth list` names the providers with a stored key, in lowercase; neither reads the
configuration, so a key file whose entry is gone is still removed. A name that breaks the
provider-name rule ([§9.1](9.1-providers-toml-the-endpoint-pool/index.md)) is a usage refusal, `key` the name as given and the detail the rule,
never the name; its `remedy` is `kbase show-config` on `add`, `kbase auth list` on `remove`. A
refusal of the argument count names no argument given. No result names a stored key.

