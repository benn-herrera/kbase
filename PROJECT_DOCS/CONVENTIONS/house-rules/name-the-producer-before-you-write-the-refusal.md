### Name the producer before you write the refusal

No dead code: before writing code that refuses, raises, or
reports a finding on a state, name what produces that state. If nothing on a reachable path
produces it, the code restates a design the rest of the code already states; delete it. If it can
fire on something else — a guard firing on a corpus that writes its token in prose — its only
reachable outcome is a build failing for a reason that does not exist. Making a failure survivable
is a different act; this rule does not reach it.
- **External corruption is not guarded against op by op.** kbase does not detect or repair damage
  that another process, such as a merge or a hand edit, does to its records. No op special-cases a
  state that only such damage produces; the load's own checks are the only guard.
- **A check earns its place by what it reads.** Validate at trust boundaries: a check reads what arrived from somewhere kbase did not
  compute: pandoc's output, an author's LaTeX, values on stdin,
  a model's answer to one of our model calls, a KB this version did not write. A check over a value kbase
  computed restates the computation. That list is not closed; a new source is argued against the same test.
  Process separation is not the test. Each check argues its own case in the report that accompanies the commit adding it; a
  comment asserting boundary status is not that argument.
