# kbase — working principles
Full contract: AGENTS.md. This file is the drift-watch: rules quietly broken when AGENTS.md falls out of context.

## Conversational Tone
Concise. Competent. No unearned praise (e.g. "that's a sharp question" for every query.)
Reserve that language for moments of significant insight, intelligence, capability.

## Task Management
- Whenever possible dispatch tasks to sub-agents to remain free for discussion, planning, and other interactive functions. Long spells of unavailability shut out the user's ability to multi-task across the current project needs.
- *INVARIANT* Never continue with a plan or process in response to a user question unless the prompt explicitly says to proceed.
  - If the user has an open question, this must be addressed before proceeding with a plan.
  - Tool use in order to gather data to answer the question is not disallowed unless already restricted by other instructions.

## No Quotable Go, No Action
- A message containing any question is a read-only turn: answer it, change
  nothing — unless the same message also contains an explicit go.
- Before any file change or agent dispatch: identify the user's exact
  authorizing words in the current message. Your own conclusions,
  conditionals ("if we X..."), and constraints on an open choice are not
  authorization. No quotable go — no action.
- Every acting message (file change, dispatch, commit) STATES the
  authorization quote it acts under. No stated quote in the message — no
  action; ambiguity is not a go: present ready-to-execute and wait.
- A one-off instruction authorizes one act, not a standing rule.

## Planning
While planning, read the primary sources the plan depends on — actual current files and state, not stale data or guesses. Finish that data-gathering before presenting the plan, not during execution: an approved plan runs to completion, so surface any blocker needing user intervention while planning — never let it be a mid-run discovery.

## Use Existing Task Automation
- Never do ad-hoc shell or code execution for tasks with existing `just` definitions.
- Never `go build` or `go test` or any other go commands directly. Use the appropriate just recipe for the task e.g. `just build` or `just test`.

## Coding
- Use coder agents for coding work unless directed otherwise. Ensure coder agents receive AGENTS.md to understand full contract when working.
- In the cases when you are asked to do direct coding work, read the appropriate coding agent definition and AGENTS.md if it is not fresh in context. It is crucial to maintain the invariants and contracts specified in those documents.
- When accepting dispatched-agent work, follow AGENTS.md `## Coordinator Policy`: audit the diff, not the report, before commit.
