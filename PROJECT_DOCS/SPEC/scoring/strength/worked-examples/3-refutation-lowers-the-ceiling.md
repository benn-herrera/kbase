### 3. A refutation caps — and sometimes does not

One refuter serves both readings: a claim `R` at `rigor 1.00`, founded by the same work, so
`σ(R) = min(1.00, 1.00 × 0.70) = 0.70`, refuting at `bearing 1.00`. The cap is `0.30` in both.

Binding:

```
founds     W  -> R    bearing 1.00 x strength 0.70 = 0.70
D(R)                  min(rigor 1.00, W 0.70) = 0.70
σ(R)                  min(max(0.70, −∞), +∞) = 0.70
founds     W  -> C1   bearing 1.00 x strength 0.70 = 0.70
D(C1)                 min(rigor 1.00, W 0.70) = 0.70
lifts                 none, so −∞
refutes    R  -> C1   cap = 1 - (bearing 1.00 x strength 0.70) = 0.30
C(C1)                 min{ R 0.30 } = 0.30
σ(C1)                 min(max(0.70, −∞), 0.30) = 0.30
```

`σ(C1) = 0.30`, band `do-not-build`. Its base alone was `0.70`, `ok-with-caveats`: the cap costs two
bands as
well as the number, and it is the only station that can put a node below both its derivation and its lift.

Not binding:

```
founds     W  -> R    bearing 1.00 x strength 0.70 = 0.70; σ(R) = 0.70
founds     W  -> C2   bearing 1.00 x strength 0.70 = 0.70
D(C2)                 min(rigor 1.00, W 0.70) = 0.70
supports   C2 -> C1   bearing 0.20 x strength 0.70 = 0.14
D(C1)                 min(rigor 1.00, C2 0.14) = 0.14
refutes    R  -> C1   cap = 1 - (bearing 1.00 x strength 0.70) = 0.30
C(C1)                 min{ R 0.30 } = 0.30
σ(C1)                 min(max(0.14, −∞), 0.30) = 0.14
```

`σ(C1) = 0.14`, band `unsubstantiated` — case 1's node with an attack on it. The cap participates, is at
full bearing and full strength, and is not the binding term: `min(0.14, 0.30)` is the premise's number. A
refutation can be worth a great deal and change nothing, because a cap is a ceiling and this node was
already under it.

