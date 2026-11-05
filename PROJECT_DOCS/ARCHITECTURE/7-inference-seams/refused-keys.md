### Refused keys

**Pattern:** Typed error classification at the boundary.

`StatusError.RefusedKey` (401 or 403) is the single predicate. Two sites match it: `asks/call`'s
`keyRemedy` and `modelclient.KeyRefused`, which `Registry.Catalogue`, `cmd/score.go`'s call and
`cmd/probe.go` apply. Each attaches `config.AuthAddRemedy` with the name of the provider the call went
to, and the result travels as data:

- `asks.IncompleteError.Remedy`, which `claimgraph`'s inference stop and the build's overview failure
  put on their items;
- `score.RemedyError`, whose remedy and `config.RefusedKeyNote` the attempt's ledger line records;
- `modelclient.KeyRefusedError`, which `cmd`'s `failed` puts on the failure item of `probe`, `models`
  and `configure-models`.

`asks/call` is told the provider it calls by `call.New`, from `build.Provider`'s per-role provider
names. No site recovers a status from a reason's text, and nothing checks a key before a call
(SPEC §9 A refused key).

