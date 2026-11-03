### Three quantities

`rigor` is a node's local value — a claim's, an analysis's, a work's or an
experiment's — a number in `[0, 1]` or `*pending*`; `bearing` is a scoring edge's scalar, a number in
`[0, 1]` or `*pending*`, and the one token on every scoring edge; `strength` is the derived value, a
number in `[0, 1]` or pending, on a claim and on an analysis. `*pending*` is the format's one
no-number literal: written for a value nobody has judged, and read wherever a value is absent. A work,
an experiment and an axiom have no derived value — their `rigor` is terminal, and an axiom's is `1.0`
by rule, which no authored file carries — so each one's strength is its `rigor`.

