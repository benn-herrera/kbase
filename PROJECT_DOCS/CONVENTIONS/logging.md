## Logging

Project-specific bindings only (the general logging doctrine lives in the coder agent definitions):

- `internal/log` is the **only** logging interface (four-method `Logger`: `Debug`/`Info`/`Warn`/`Error`,
  alternating key/value pairs). Built **once**, in the composition root (`cmd/main.go`), from the
  persistent `--log-level` (default warn) and `--log-file` flags. `--log-file` tees; it never
  redirects.
- **Threaded, not global**: dependency injection — a component that logs takes a `log.Logger` parameter, so its logging is
  visible in its constructor signature. No package-level logger, no setter; `log.Discard()` covers a
  caller with nothing to hand it.
- Never call `slog` directly, and never use `fmt.Fprintf(os.Stderr, ...)` for a diagnostic. Verb
  *output* — a summary line, a user-facing warning — is a different channel and stays on the verb's
  `Stderr` writer.
