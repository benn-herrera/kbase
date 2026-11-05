# THESIS - kbase

What this project is and why

## kbase in plain terms

kbase reads a set of LaTeX papers and produces a knowledge base: a directory of markdown files
describing the papers' structure and the argument they make, as a graph of claims. It then keeps
that knowledge base correct as people and agents edit it, and scores how well the argument holds
up.

Almost all of that is assembled from well-known building blocks, used at small scale. A few
pieces are new, and they are the reason the project exists. This page names both, so that work on
a standard piece gets the standard answer and attention goes to the new ones.

## The building blocks

Each row is a standard pattern at the size kbase needs. The last column names the larger system
the piece resembles and is not — the direction work on it must not drift.

| kbase piece | The standard pattern | It is not |
|---|---|---|
| Reading LaTeX | Run pandoc; walk its JSON AST in Go and transform the tree. Before pandoc, a registry of narrow pre-passes, each filling one named gap in pandoc's reading | A LaTeX parser, macro engine or renderer. pandoc parses; kbase never does |
| The knowledge base | A directory of markdown files with YAML frontmatter and JSONL record files, loaded whole into memory, validated on load | A database engine: no indexes, no query language, no storage layer |
| Writes | A batch of operations validated all-or-none in memory, then written file by file with atomic renames; git is the undo log and the recovery | A transaction manager or a version-control system |
| Derived pages and sheets | Views regenerated from the source files after every write, checked for staleness | A build system or a reactive framework |
| Queries | Scans and lookups over the loaded set | A query planner or a search engine |
| The claim graph | A DAG: cycle detection, reachability, a topological walk that folds values from sources to sinks | A general graph library or graph database |
| Format versions | Numbered schema migrations, each a pure N→N+1 conversion, applied on read and saved by an explicit command | A migration framework |
| Builds and scoring runs | A detached child process with a lockfile, a status file, an append-only log, cancel and resume | A job scheduler or workflow engine |
| Model calls | A chat-API client behind a fixed-size worker pool, with retry and backoff, and a cache keyed on the request | An agent framework, an eval harness or a model router |
| Asking a model | A prompt template that asks a closed question and parses the answer against a fixed grammar | Free-form generation; the model never writes the knowledge base |
| Configuration and credentials | Layered TOML files (explicit, repository, global) and a local credentials file | A secrets manager |
| The CLI and MCP server | A cobra command tree; the MCP tools are generated from it over JSON-RPC on stdio | A plugin system or a general-purpose tool server |
| Checks | Unit tests, golden fixtures, and tests that enforce the package dependency rules | — |

Where a piece is in this table, its conventional answer is the default. A design that departs
from the standard pattern states why, in ARCHITECTURE. A design that grows a piece toward its
"is not" column is out of scope, however natural the next step looks.

## What is new

Three things, and only these:

1. **What a claim graph is made of.** Five kinds of node and four scoring relations between them,
   and the rule that a claim is a bare statement: any argument for it is a separate node.
2. **The local judgments.** Each node and each edge gets a narrow, checklist-style judgment —
   rigor for a node, bearing for an edge — from a closed list of faults, each fault costing a
   fixed amount.
3. **How the judgments combine.** A formula that carries those local values along the graph's
   edges into a strength for every node.

The machinery that computes all three is standard, and listed above. What each of the three is, is
not, and SPEC's scoring schemes own it.

### Building: the draft is mechanical, inference only refines it

A LaTeX document already encodes most of its own structure. The author built a hierarchy and the
format serialized it; reading it back is deserialization, not interpretation.

- **The navigation tree** — title, parts, chapters, sections — is a transcription of the sectioning
  commands. A second implementation produces the same tree.
- **The argument's nodes** — theorems, lemmas, propositions, displayed equations —
  arrive as LaTeX environments with identifiers.
- **The argument's edge candidates** arrive as `\ref`-family references to those identifiers. A
  reference does not decide an edge: it may be a mention, supported by, or in support of.

Authors under-mark their arguments, so a model closes three gaps:

1. claims argued in prose with no environment (new nodes);
2. references made in prose without `\ref` (new edge candidates);
3. for each candidate, whether it is an edge and which way it points: mention, supported by, or in
   support of.

**Inference refines a mechanical draft; it never produces one.** Every model-refined step has the
mechanical result to fall back on.

In the finished graph, base claims sit at the bottom — asserted outright, or resting on a cited
work outside the document set — and every other edge connects claims within it.

### Scoring: small judgments, combined

An argument is a graph of claims and relations. Judge each node and each edge locally, with the
narrowest question that decides it, and combine those judgments into a picture of the whole.

- **Intelligence-neutral.** The scheme gives usefully consistent results whoever applies it, human
  or model. Narrow questions are what make that possible: the less arguable a question, the more
  consistent its answers.
- **Arrangement-invariant.** Two arguments made of the same points, arranged differently — one
  linear in three medium steps, one built bottom-up from two foundations in five small steps —
  must score essentially the same. Edge count, step size and direction of construction must not
  be ways to game the score. This is a requirement the combining formula is measured against;
  the current formula does not yet meet it and this is a topic of active development.

## Targets

A build today takes a couple of hours of local inference and still needs manual adjustments afterwards;
scoring a full knowledge base takes minutes on a 128 GB machine.

The target is a build of an average arXiv math paper in about 30 minutes, and scoring in about
five, on models a 36 GB M3 Max MacBook Pro can host — the machine common in university labs and on
researchers' desks.
