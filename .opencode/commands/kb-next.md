---
# !GENERATED! from adjagent/definitions/kb_session.py — do not edit here: request changes from the adjagent project; local needs belong in your CONVENTIONS.md.
# !TUNING! family=claude seat=none member=none tier=highest=inherit,high=inherit,medium=inherit,low=inherit,lowest=inherit stock=highest,high,medium,low,lowest harness=opencode
---
Continue a knowledge-base reading session on a new topic: locate the KB, load the docent, and answer
the question left in the handoff.

## Locate the KB

Before reading anything, establish where the KB is. Probe in order:

1. `./kb-root/entry-point.md` **and** `./kb-root/.index/` both exist → **KB root** `./kb-root`, **project root** `.`
2. otherwise `./entry-point.md` **and** `./.index/` both exist → **KB root** `.`, **project root** `..`
3. otherwise **stop and ask.** Name both probes you ran, and say that this command runs from the project root (the directory holding `kb-root/`) or from `kb-root/` itself. Do not search the tree for a KB — section directories carry their own `entry-point.md`, and a search finds the wrong one.

The `.index/` sibling is what the probes turn on: it is what makes a directory a viable KB root rather than a section inside one.

From here, every file path resolves against the located **KB root**, and every tool invocation anchors to the located **project root** — `PYTHONPATH=<project-root>/.opencode/agents`.

Then, in order:

1. Read `<project-root>/.opencode/agents/kb-docent.md` — that definition is who
   you are for this session.
2. Read `session/new-topic.md` from the KB root.

You are the docent. Respond to the question at the end.
