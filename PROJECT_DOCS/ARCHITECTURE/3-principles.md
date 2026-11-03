## 3. Principles

These steer choices the invariants leave open.

- **Propose-and-verify at every model seam.** The model never mutates canonical bytes. It emits data
  with a mechanically checkable post-condition; deterministic code validates and executes.
- **The model supplies values; the machine owns syntax.** An ask presents its subject fully
  resolved, states which values are open, and takes the answer in a grammar the machine wrote. No
  ask asks a model to construct a document.
- **Monotone safety.** Every model-refined artifact has a mechanical fallback that was already
  acceptable. Model failure degrades quality, never correctness. No stage exits on a model's
  opinion; every stage exits on a comparison of artifacts or a return code.
- **Provenance and reproducibility are artifact properties.** The same inputs and pandoc version
  give a byte-identical document graph — tree and records — and a claim graph identical modulo the
  ids it mints (SPEC §5); derived state is regenerable and freshness-gated.
- **No LaTeX parser is written here, partial or whole.** The reader is pandoc.
- **The kbase and personant integration is the target.** A prior decision that prevents it is
  removed, not designed around.
- **The served surface is the operating contract.** What an agent needs to drive kbase reaches it
  from kbase itself: a command's own facts in its help; rules spanning commands in the MCP
  instructions, kept short because every session carries them; longer guidance as MCP resources,
  read on demand. The `kbase` skill carries the workflow, as the instructions say (SPEC §11),
  chiefly when to engage kbase at all; a rule a command or the server can state is served, not
  left to the skill.

---

