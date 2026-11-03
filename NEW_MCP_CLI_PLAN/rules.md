# Rules every package keeps

These rules hold in every stage. Rule numbers are the same in every stage: Rules 5, 6, 8, 12 and
13 differ between stages and are stated in each stage's `index.md`.

**What each dispatch receives:** [`PROJECT_DOCS/entry-point.md`](../PROJECT_DOCS/entry-point.md), and the
`PROJECT_DOCS/` leaves its package leaf links; this file, `NEW_MCP_CLI_PLAN/rules.md`; its stage's `index.md`; its own
package leaf; every leaf of [`NEW_MCP_CLI_DESIGN/2-conventions/`](../NEW_MCP_CLI_DESIGN/2-conventions/index.md), the conventions every op
follows, whatever the package links; and the other `NEW_MCP_CLI_DESIGN/` leaves its package leaf links to. Nothing else of
the plan or the design.

1. **The normative design.** The [`NEW_MCP_CLI_DESIGN/`](../NEW_MCP_CLI_DESIGN/entry-point.md) leaves that a package links to are this plan's normative
   text ([CONVENTIONS "Plan & Execute Process"](../PROJECT_DOCS/CONVENTIONS/plan-execute-process/index.md)). Their subject is the served surface: the CLI, the
   MCP tools, the write and query ops, the served text, and KB format 4.0.0. On that subject the
   leaves are the requirement and outrank the package text where they disagree, and this plan is
   only the route.
   - "page §N" means section §N of the design, at the leaf its link names.
   - Where a package's text and a design leaf it links to disagree, implement the leaf, and name
     the package's line as moved in the Commit body (Rule 3).
   - Nothing else of the design applies to the package.
   - The design is never edited by the work.
2. **Line numbers and grep answers.** Cites of the contract documents are links to the
   `PROJECT_DOCS/` leaf that held the cited line at commit `68021ba`; find the text in that leaf by its
   symbol or its quoted text. Other line numbers are as of that commit; where a line has
   moved, find it by its symbol or its quoted text. Every `git grep` answer was taken at that
   commit. Re-run the command before acting on it, because an earlier package, this stage's or an
   earlier stage's, may have moved the answer.
3. **The report.** The executor's report ends with two sections.
   - `Stale contract text`: contract text outside the package that the executor found stale. It is
     listed there and not edited, and the package completes (Rule 6).
   - `Commit body`, which the coordinator copies into the commit. It names:
     - every truth the package moved, each fixture and what it now answers included (CONVENTIONS
       "Testing");
     - every line of this plan the page moved (Rule 1);
     - every model-facing text the package changed: help text, cobra `Short`, MCP tool and schema
       descriptions, instructions, resource text, the stamped-document templates, and a refusal's
       `detail`, `allowed` and remedy. The coordinator sends that list to prompt-engineer review
       before the commit lands.
4. **A pin moves with its fact.** A test, transcript or fixture that pins a fact the package moves
   is updated in the same package. A pin is never loosened to pass. An expected-truth update states
   the new truth it tracks.
7. **No guard against a state that nothing produces** (CONVENTIONS "Name the producer before you
   write the refusal").
9. **Keys.**
   - An API key is never accepted on argv or through any MCP tool. It arrives only piped into
     `kbase auth add <provider>`, which refuses a terminal on stdin.
   - A key never appears in a log line, an error string or a result.
   - No option anywhere takes a key file's path ([page §3.7](../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/index.md)).
10. **Files.**
    - No package reads or writes anything under `kb-root/.index/` in any KB by hand. kbase's own
      writes, and `just regenerate-fixtures` copying them, are the exception.
    - Scratch goes in `.claude-temp/` only.
    - No git command that rewrites the working tree: `stash`, `checkout`, `restore`, `reset`,
      `clean`.
    - Every edit is made with the edit and write tools.
11. **Runner targets only.** Builds, tests and integration runs go through `just` recipes.
    `./bin/kbase` is run directly only inside a recipe's script. A LIVE recipe whose script a
    package changes is checked by `just --dry-run <recipe>`, never run ([CONVENTIONS "Testing"](../PROJECT_DOCS/CONVENTIONS/testing/index.md)).
