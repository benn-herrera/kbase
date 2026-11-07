### Integration tests invoke the app binary

Black-box testing: an integration test runs `./bin/kbase` directly — the
system end-to-end as delivered. Behavior switches (`--state-dir`, `--config-dir`) are flags in the
recipe's invocation. In-process calls to verb functions are unit tests of verb logic, never
integration tests.

