### 2. A lift over a weak base

```
  W  --founds 1.00--> C2  --supports 0.20--> C1
 0.70                 1.00                   1.00
  E  --lifts 1.00-------------------------> C1
 1.00   (run)
```

```
founds     W  -> C2   bearing 1.00 x strength 0.70 = 0.70
supports   C2 -> C1   bearing 0.20 x strength 0.70 = 0.14
D(C1)                 min(rigor 1.00, C2 0.14) = 0.14
lifts      E  -> C1   bearing 1.00 x strength 1.00 = 1.00
L(C1)                 max{ E 1.00 } = 1.00
C(C1)                 none — no refute edge, so +∞
σ(C1)                 min(max(0.14, 1.00), +∞) = 1.00
```

`σ(C1) = 1.00`, band `ok-to-build`. `D(C1)` is case 1's `0.14`, untouched: a lift is not a premise and
does not enter the min, so the derivation still says the node stands on `0.14`. What `max` does is stand
the lift's `1.00` beside that derivation and keep the larger — the only station in the walk that can raise
a value, and here it carries the node from `unsubstantiated` to `ok-to-build`, four rungs. Take the edge away
and this is case 1 exactly; that is the case the station exists for.

