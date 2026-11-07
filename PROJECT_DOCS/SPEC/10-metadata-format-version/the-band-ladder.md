### The band ladder

The *band ladder* is the five *rungs* a `strength` is read against, each a threshold bucket with an
inclusive lower bound: `0.85` *ok to build on* (`ok-to-build`), `0.65` *ok to build on, see
caveats* (`ok-with-caveats`), `0.45` *use as input only* (`input-only`), `0.20` *do not build on,
rework needed* (`do-not-build`), and `0.00` **unsubstantiated** (`unsubstantiated`). The bottom
rung states an outcome and not a cause: every strength below `0.20` falls in it from either
direction — a refuter's cap, or a minimum no premise carries — and what it says is that nothing a
claim built on this would inherit substantiates the claim. The cause, where a reader needs it, is
the run's fault record and the derivation trace. One value has one band wherever a band is
shown: a claim's or an analysis's record (`build-status`, `strength-band`), the `stats`
distribution's row, and the sheet's fill and tooltip. A pending strength has no band: a result's band
reads `unknown`, and the sheet labels it `*pending*`
([§3](../3-compatibility-contract/claim-graph-svg/what-is-drawn.md)). A strength below `0.00` cannot occur.

