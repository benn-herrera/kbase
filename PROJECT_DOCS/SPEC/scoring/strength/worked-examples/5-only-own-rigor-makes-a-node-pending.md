### 5. Only own rigor makes a node pending

The three states below are the Pending section's, in arithmetic. Two of them are the pair a reader
conflates — a node whose own judgement is missing, and a node one of whose edges is.

Pending `rigor` over a premise set:

```
founds     W  -> C1   bearing 1.00 x strength 0.70 = 0.70
D(C1)                 min(rigor *pending*, W 0.70) = *pending*
L(C1)                 −∞      (no lift edge)
σ(C1)                 *pending*
```

Strength `*pending*`, band `unknown` — the ladder has no rung for a value that is not a number. The premise
set is non-empty and judged; the node's own rigor is not, and `min(⊥, c) = ⊥` carries that through.

Pending `bearing` as the node's only edge:

```
supports   C2 -> C1   bearing *pending* — excluded, adds no operand
P(C1)                 empty
D(C1)                 0
L(C1), C(C1)          −∞, +∞   (no lift edge, no refute edge)
σ(C1)                 min(max(0, −∞), +∞) = 0.00
```

Strength `0.00`, band `unsubstantiated`. An unjudged bearing is not support yet, so the edge is no
supporter; the node's `1.00` does not enter, for case 4's reason.

The same pending bearing beside a premise that participates:

```
founds     W  -> C1   bearing 1.00 x strength 0.70 = 0.70
supports   C2 -> C1   bearing *pending* — excluded, adds no operand
D(C1)                 min(rigor 1.00, W 0.70) = 0.70
L(C1), C(C1)          −∞, +∞   (no lift edge, no refute edge)
σ(C1)                 0.70
```

Strength `0.70`, band `ok-with-caveats`. An edge excluded for a pending bearing does not make its target
`*pending*`: it removes its own operand, and the node reads what the rest of its premise set establishes.
`*pending*` reaches a node from the node's own `rigor`, and only where the premise set is otherwise
complete and nothing lifts — a lift would publish its own product instead. It does not reach a node from
an edge.

