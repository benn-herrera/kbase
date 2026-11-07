### 6. Colocated claim and analysis

```
  W  --founds 1.00--> C1   claim,    rigor 1.00
  W  --founds 1.00--> A1   analysis, rigor 1.00
```

```
founds     W  -> C1   bearing 1.00 x strength 0.70 = 0.70
D(C1)                 min(rigor 1.00, W 0.70) = 0.70
σ(C1)                 min(max(0.70, −∞), +∞) = 0.70
founds     W  -> A1   bearing 1.00 x strength 0.70 = 0.70
D(A1)                 min(rigor 1.00, W 0.70) = 0.70
σ(A1)                 min(max(0.70, −∞), +∞) = 0.70
```

Both `0.70`, both band `ok-with-caveats`, and the arithmetic is identical term for term. The `1.00` inside each
min is a different judgement: on C1 it is the statement vocabulary charged nothing — the assertion's text
is fixed, consistent and single-sensed, and says nothing about an argument, which a claim does not display;
on A1 it is the method vocabulary charged nothing — the derivation engages what it sets out to establish
and states its steps. A claim has no method and an analysis no statement quality, so one number stands for
two different absences of fault; the node's class decides which criteria priced it, and strength propagation reads only
the number.

