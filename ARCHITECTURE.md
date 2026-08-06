# Kbase Architecture Design

A single-purpose AI appliance application for the dissection of standard documentation into an AI-friendly knowledge base markdown graph.

This document is the authoritative design reference.

---

## 1. Invariants

These must hold regardless of implementation choices. Violating any invariant is a
blocking defect.

**Build and Deployment**

1. **Single self-contained binary, embedded prompts.** No external runtime
   dependencies or shipped data files.

---

TBD
