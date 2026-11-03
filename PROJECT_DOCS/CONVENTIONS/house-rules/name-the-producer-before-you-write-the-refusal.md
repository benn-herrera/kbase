### Name the producer before you write the refusal

Before writing code that refuses, raises, or
reports a finding on a state, name what produces that state. If nothing on a reachable path
produces it, the code restates a design the rest of the code already states; delete it. If it can
fire on something else — a guard firing on a corpus that spells its token in prose — its only
reachable outcome is a build failing for a reason that does not exist. Making a failure survivable
is a different act; this rule does not reach it.
- **External corruption is not guarded against op by op.** kbase does not detect or repair damage
  that another process, such as a merge or a hand edit, does to its records. No op special-cases a
  state that only such damage produces; the load's own checks are the only guard. This robustness is
  deferred, not unconsidered: it is taken on whole, at a cost not yet accepted, never one op at a
  time.
- **A check earns its place by what it reads.** It reads what arrived from somewhere kbase did not
  compute: pandoc's output, an author's LaTeX, values on stdin,
  a model's answer to one of our asks, a KB this version did not write. A check over a value kbase
  computed restates the computation. That list is not closed; a new source is argued against the same test.
  Process separation is not the test. Each check argues its own case in the report that lands it; a
  comment asserting boundary status is not that argument.
