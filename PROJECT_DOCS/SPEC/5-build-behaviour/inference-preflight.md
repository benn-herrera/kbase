### Inference preflight

A build with a stage left to run that makes model calls refuses ([§7](../7-tool-result-contract-and-exit-codes/index.md), `refused`) before the first
stage it runs when no provider is configured ([§9](../9-configuration/index.md)), naming the configuration it lacks
(`check: provider`); its `remedy` names `kbase show-config` as the diagnostic that shows how the
configuration resolved ([§9.6](../9-configuration/9.6-kbase-show-config/index.md)). A build making no model calls needs no provider. Every model call a
build makes is one tool-less chat request answered from its prompt alone — except the claim-graph
multichoice model calls, which are each one `POST` to the decision endpoint the selected entry
names ([§9.1](../9-configuration/9.1-providers-toml-the-endpoint-pool/index.md)) containing a fixed-set question (a `choice` or `noul` question over the offered letters)
and answered by probabilities over that set, not by reply text ([§9](../9-configuration/index.md)).

**Refusals** (outcome `refused`):

- Unparseable source (`check: source`): names the paper and the reader error.
- Unloadable include (`check: include`): names each file and the line that named it.
- Include outside the volume (`check: include`): `\input` and `\include` resolve only inside the
  top `.tex` file's directory, the one pandoc runs in; one that resolves outside it — by an
  absolute path, by `..`, through a symlink, or below a first-level include — is refused, naming
  the include and the file that named it. A target that resolves inside and names no file remains
  the reader's own warning.
- An unclassified metadata key (`check: metadata-key`): a key the rendering's metadata block has
  that is neither content nor apparatus. A key pandoc's writer drops — an empty
  entry — reaches no block and is not refused.

