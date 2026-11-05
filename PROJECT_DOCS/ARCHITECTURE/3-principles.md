## 3. Principles

These steer choices the invariants leave open.

- **Pattern:** Generate-then-validate. **Propose-and-verify at every model seam.** The model never
  mutates canonical bytes. It emits data with a deterministically checkable post-condition; code
  validates and executes.
- **Pattern:** Constrained output. **The model supplies values; the machine owns syntax.** A prompt
  presents its subject fully resolved, states which values are open, and takes the answer in a grammar
  the machine wrote. No prompt asks a model to construct a document.
- **Pattern:** Graceful degradation to a deterministic baseline. **Monotone safety.** Every
  model-refined artifact has a deterministic fallback that was already acceptable. Model failure
  degrades quality, never correctness. No stage exits on a model's opinion; every stage exits on a
  comparison of artifacts or a return code.
- **Pattern:** Hermetic, deterministic build. **Provenance and reproducibility are artifact
  properties.** The same inputs and pandoc version give a byte-identical document graph (tree and
  records) and a claim graph identical modulo the ids it mints (SPEC §5 Determinism); derived views
  are regenerable and freshness-checked. Ids are minted randomly, hence "modulo ids".
- **Pattern:** Reuse an existing parser. **No LaTeX parser is written here, partial or whole.** The
  reader is pandoc.
- **Pattern:** Single source of truth. **Metadata first.** A need the `.index/` records can meet reads
  the records. A leaf is read only for what records cannot hold without restating the leaf: the text
  itself, where a statement marker sits, and how the text is laid out.
- **Pattern:** Self-describing API. **The served text is the operating contract.** What an agent needs
  to drive kbase reaches it from kbase itself: a command's own facts in its help; rules spanning
  commands in the MCP instructions, kept short because every session reads them; longer guidance as
  MCP resources, read on demand. The `kbase` skill supplies the workflow, as the instructions say
  (SPEC §11 Served text), chiefly when to engage kbase at all; a rule a command or the server can
  state is served, not left to the skill.

---

