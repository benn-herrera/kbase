### The band ladder

The *band ladder* is the five *rungs* a `strength` is read against, each a threshold bucket with an
inclusive lower bound: `0.85` *ok to build on*, `0.65` *ok to build on, see caveats*, `0.45` *use as
input only*, `0.20` *do not build on, rework needed*, and `0.00` **unsubstantiated**. The bottom
rung states an outcome and not a cause: every strength below `0.20` falls in it from either
direction — a refuter's cap, or a minimum no premise carries — and what it says is that nothing a
claim built on this would inherit substantiates the claim. The cause, where a reader needs it, is
the run's fault record and the derivation trace. One value has one band wherever a band is
shown: a claim's or an analysis's record (`build_status`, `strength_band`), the `stats`
distribution's row, and the sheet's fill and tooltip. A pending strength has no band, and a strength
below `0.00` cannot occur.

