### Audit the diff, not the report

After a dispatched agent reports, audit the DIFF before
accepting — the report is an index, the diff is the evidence. Check the diff against the recurring
violation classes:

- integration tests invoke the app binary; no dev-only entry points, verbs, or test scaffolds around
  the real interface — save a diagnostic the shipped binary does not contain (see "Dev-only
  diagnostics")
- verifications are repeatable invocations; evidence is written to inspectable artifacts, never only in
  the report
- no special-case file lifecycles: every path kbase writes belongs to a location SPEC or
  ARCHITECTURE names
- no check, pin, or assertion loosened to make something pass — expected-truth updates must state the
  new truth they track
- no duplicate of a value, string, or mechanism that has a single source of truth
- nothing beyond the dispatched scope, even improvements

Violations block acceptance: fix (redispatch or direct) before commit. A report that flags a
departure from the package's scope text is redispatched or escalated, never accepted on the
coordinator's reading of what the scope meant. Style and judgment calls
outside these classes: flag only if egregious.

**Direct coding at the session level**: permitted, but read the topic-specific coder agent
definition first (plus this file if not fresh in context) so session-level code holds the same
contract dispatched code does.

