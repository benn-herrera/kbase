## Worked example

```
  supports   C2 -> C1   bearing 0.40 x strength 0.65 = 0.26
  founds     W  -> C1   bearing 0.70 x strength 0.72 = 0.50
  founds     S1 -> C1   bearing 0.90 x strength 1.00 = 0.90
  D(C1)                 min(rigor 0.84, C2 0.26, W 0.50, S1 0.90) = 0.26
  lifts      E  -> C1   bearing 0.75 x strength 0.64 = 0.48
  max(D, L)             max(0.26, 0.48) = 0.48
  refutes    R  -> C1   cap = 1 - (bearing 0.80 x strength 0.75) = 0.40
  σ(C1)                 min(0.48, 0.40) = 0.40
```

W's product is `0.504`: the min is the same at full precision and at the two decimals the page prints.
S1 is the axiom shape: its `rigor` is `1.00` by rule, so the contribution is the bearing alone. Its
`0.90` sits in the min beside the others and does not bind — a premise stronger than the weakest link
changes nothing.

Change one bearing and the graph answers differently:

- `supports` bearing `1.0` (C2) → contribution `0.65`, derivation `min(0.84, 0.65, 0.50, 0.90) = 0.50`.
- `founds` bearing `0.0` (W) → contribution `0`, derivation `0`: the work is not what the claim stands
  on.
- `lifts` bearing `1.0` → lift `0.64`, so `max(D, L) = 0.64` before any cap.
- every premise excluded (C2, W and S1 all pending) → `P(C1)` is empty and `D(C1) = 0`, so
  `max(D, L) = 0.48` is the lift's product alone; the cap still holds C1 at `0.40`.

