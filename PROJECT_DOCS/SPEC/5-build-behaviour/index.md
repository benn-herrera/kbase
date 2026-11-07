## 5. Build behaviour

```
kbase build <volume-root>... [--bibliography FILE]... [--no-inference]
            [--through <stage>] [--state-dir DIR] [--ask-concurrency N]
kbase build --resume [--no-inference] [--through <stage>] [--state-dir DIR]
            [--ask-concurrency N]
```

A value of `--ask-concurrency` under 1 is a usage refusal naming `--ask-concurrency`, and the
`resume` call a build returns includes `--ask-concurrency` where it was given.

Contents:

- [§5 Output and placement](output-and-placement.md)
- [§5 Inputs](inputs.md)
- [§5 Starting over](starting-over.md)
- [§5 Determinism](determinism.md)
- [§5 Inference preflight](inference-preflight.md)
- [§5 Bibliographies](bibliographies.md)
- [§5 Monitoring](monitoring.md)
- [§5 A build marks every claim](a-build-marks-every-claim.md)
- [§5 A build records no judgement](a-build-records-no-judgement.md)
- [§5 Co-linearity cull (`claims-culled`)](co-linearity-cull-claims-culled.md)
- [§5 Dependency attribution (`depends-attributed`)](dependency-attribution-depends-attributed.md)
- [§5 Analysis split (`analyses-split`)](analysis-split-analyses-split.md)
- [§5 Records and inference](records-and-inference.md)
