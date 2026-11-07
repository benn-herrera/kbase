### Surface audit, every third surface-changing sprint

Every third sprint, counting only sprints that change the CLI or MCP surface, separate from any
sprint's own review, and executing nothing:
given only what a session is served — the MCP tools, instructions and help resources, the CLI's
help, the skill — review the logical user flows an agent or user follows to do their work, and
report every operation that is missing or inverted, every flow that takes more calls than its
intent, and every step where the obvious call is not the right one. Its findings feed the next
design; no post-sprint review covers it.
- Lenses, applied to every operation and flow:
  - **redundant**: another op, or a batch of another op's entries, already does it;
  - **over-composed**: it bundles steps a caller could issue one at a time, with no atomicity or
    consistency reason to bundle them;
  - **over-fine-grained**: it leaves the caller to keep the records consistent, or to run a step
    that should follow automatically;
  - **abstraction inversion**: a simple intent takes a chain of calls instead of one;
  - **unnecessary**: no current or foreseeable need calls for it;
  - **poorly named**: its name misdescribes what it does, or names a thing differently from its
    siblings.

