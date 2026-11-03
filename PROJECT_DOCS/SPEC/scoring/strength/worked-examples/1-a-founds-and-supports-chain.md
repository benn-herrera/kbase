### 1. A chain: a work founds a claim, the claim supports another

```
  W  --founds 1.00--> C2  --supports 0.20--> C1
 0.70                 1.00                   1.00
```

```
founds     W  -> C2   bearing 1.00 x strength 0.70 = 0.70
D(C2)                 min(rigor 1.00, W 0.70) = 0.70
σ(C2)                 min(max(0.70, −∞), +∞) = 0.70
supports   C2 -> C1   bearing 0.20 x strength 0.70 = 0.14
D(C1)                 min(rigor 1.00, C2 0.14) = 0.14
σ(C1)                 min(max(0.14, −∞), +∞) = 0.14
```

`σ(C1) = 0.14`, band `unsubstantiated`. At neither node does the `1.00` bind — a rigor caps what the premises
establish, and at both nodes the premise is the lower operand — while the second hop costs exactly the
bearing: `0.70` in, `0.14` out. The work is cited at full relevance and `σ(C2)` carries its `0.70`
unchanged; what puts C1 under the `0.85` cut is the link it stands on, not the work.

