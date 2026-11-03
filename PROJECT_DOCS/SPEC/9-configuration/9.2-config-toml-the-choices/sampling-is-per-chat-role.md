#### Sampling is per chat role

`[sampling.text]` and `[sampling.scoring]` each take
`temperature` and `thinking`, both optional. A key left unset is kbase's default —
temperature 0 and thinking off — never the provider's. They merge field by field like every
`config.toml` value, each keeping the layer that supplied it. There is no
`[sampling.decision]`: the decision role's closed-set probabilities take no sampling, and a
`sampling.decision` key fails strict load as unmodeled ([§9.3](../9.3-strict-load-contract.md)). A `thinking` value other than
the four fails the load, naming `sampling.<role>.thinking` and the four. Resolving a role
refuses, naming `sampling.<role>.temperature`, a temperature outside the range its entry's
chat protocol accepts — 0 to 2 for `openai`, 0 to 1 for `anthropic`.

