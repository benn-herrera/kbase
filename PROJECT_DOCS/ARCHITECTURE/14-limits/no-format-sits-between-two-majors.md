### No format sits between two majors

`formats.Chain` walks converters version by version and
refuses a step no converter leads, so a KB stamped `1.1.0` is refused at load rather than read; no
`1.1.0` converter exists and none is reachable.

