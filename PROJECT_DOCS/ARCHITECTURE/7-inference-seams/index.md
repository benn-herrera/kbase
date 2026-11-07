## 7. Inference seams

**Pattern:** Role-based model routing behind provider adapters.

`internal/config` loads the providers and the role choices, `internal/model` contains the wire clients
and the call queue, and `internal/modelclient` is the only constructor of a `model.Client`. Three roles
route calls: `decision` for the multichoice questions, `text` for the overview call and `scoring` for
`kbase score`'s calls (SPEC §9 The files are the one source of inference). Each role names its own
provider and model in `[models]` as a `<provider>/<model>` reference. No role takes another role's
endpoint, model or provider, and no call fails over to another provider when its own fails. Failover
is a pattern for services that must stay up; kbase is a user's tool, and a failover would change which
model judges or refines the KB without the user deciding that it may. The role is the user's
declaration of which model does the work, so a call that cannot reach it fails and says so.

Every model call is made through a `model.Slot`. `model.Queue` has N permits; `Queue.Go` waits for a free
permit and starts the item with a `Slot` on it ([§2](../2-invariants.md), I12). `Slot.Chat` makes the overview call and
each scoring call, and `Slot.Decision` makes each multichoice question. The `Client` interface's
inference methods are unexported, so the compiler restricts every other package to a slot, and a slot is
spent when its item returns. `build`, `score` and both replays build the queue from `--ask-concurrency`,
else `[asks] concurrency`, else `config.DefaultAskConcurrency` ([§12](../12-constants/index.md)). `askConcurrency`
in `cmd/askconcurrency.go` is the only resolver and refuses a flag under 1. The same N is the idle
connections kept per host: `modelclient.New` takes it and sets `model.Endpoint.IdleConns`, so parallel
calls reuse connections. A build shares one queue across its roles; `probe` runs a queue of one.
`walk.launch` checks that a provider resolves before the first stage when a stage left to run makes
model calls (SPEC §5 Inference preflight).

Contents:

- [§7 Probe](probe.md)
- [§7 Wire clients](wire-clients.md)
- [§7 Retry and attempt outcomes](retry-and-attempt-outcomes.md)
- [§7 Refused keys](refused-keys.md)
- [§7 Layered configuration](layered-configuration.md)
- [§7 Resolution and the client registry](resolution-and-the-client-registry.md)
- [§7 The decision API](the-decision-api.md)
- [§7 The multichoice-ask interface](the-multichoice-ask-interface.md)
- [§7 Credentials](credentials.md)
