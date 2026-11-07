### Records and inference

The reader and placement facts the claim graph needs are held as records in the build state store,
not in `kb-root/`. Each inference pass is one fixed-set decision per
model call under its embedded template, and the calls are made as
[Inference preflight](inference-preflight.md) states. A stage's exit never depends on a model's
opinion.

---

