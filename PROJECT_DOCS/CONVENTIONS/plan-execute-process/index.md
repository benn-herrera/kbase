## Plan & Execute Process

Plans are tracked: progress and intent are worth more than a tidy tree, and a plan in an ignored
file is one power-cycle from gone.

- **`ACTIVE_PLAN.md`** at the repo root is the one plan under way: decisions confirmed at kickoff,
  work-package boundaries, sequencing and status. Dispatched agents execute out of it; status
  updates as work packages land. It carries outcomes, their consequences and the principles still
  steering open choices — not decision history, which lives in commits. When the work lands, its
  outcomes move into the contract documents and the file is replaced by the next plan.
- **`ROADMAP_PLANS/`** holds partly-planned work that is not under way: a plan with enough design
  done that someone could pick it up, usually behind a ROADMAP.md item. A plan leaves when it
  becomes the active plan, is finished, or is abandoned. A plan there cites only tracked paths.
- **A plan may name a normative page outside the contract documents, and that page outranks the
  plan.** The contract documents are THESIS.md, SPEC.md, ARCHITECTURE.md and CONVENTIONS.md. A plan
  may make a page of its own subject normative — `RIGOR_AND_BEARING_SCORING_SCHEME.md` is such a
  page, stating the scoring rubric the contract cites — and on
  that subject the page is the requirement while the plan is the route. Where a plan row and the
  page disagree, the work implements the page and the plan's line is reported as moved, in the
  commit that lands it; the page is not edited by the landing, and the contract documents cite it
  rather than restate it. A plan that needs such a page names it in the plan, so a reader knows which
  source settles a disagreement.
- Working drafts and half-formed notes are scratch (`.claude-temp/`), not plans.
- Any plan that is activated, regardless of its location, gets a post-sprint review section
  at the end. 
  - That section contains the review, revision, and burndown steps. 
  - Unless otherwise specified the reviewers are an architect and a coder reviewing with THESIS.md, SPEC.md,
  ARCHITECTURE.md, and CONVENTIONS.md as their pre-task reading list. 
  - They do NOT get the plan doc. Their job is to review the changes against standards and suitability for purpose, not rehash the implementation. 
  - Their review brief includes reviewing the tests of all touched code
  - The architect also gets asked, in a separate step, to review the results of `just measure-mcp-surface`
    to look for bloat, redundancy, user friction, and general rot.
  - The burndown list is written to a separate `<plan>_REVIEW_BURNDOWN.md` and gets the same
    prompt engineer review and revision pass as any other document containing agent instructions.
Contents:

- [Surface audit, every third retired plan](surface-audit-every-third-retired-plan.md)
- [Plan work rating and dispatch](plan-work-rating-and-dispatch.md)
- [Plan dispatchability](plan-dispatchability.md)
