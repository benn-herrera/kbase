### Test data layout

`test-data/fixtures/` holds version-controlled test data we author, own, and
maintain. `test-data/transient/` (gitignored) holds cloud-sourced corpus clones, generated test
data, and test output. Never write generated or downloaded data into `fixtures/`, with one
exception: a committed fixture that pins kbase's own output — `init`'s skeleton,
`test-data/fixtures/init/skeleton/` — is written only by `just regenerate-fixtures`, from the
binary, never by hand; a change to what kbase writes there re-runs it.

---

