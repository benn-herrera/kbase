#### The value

kbase computes the value from the charge list: the item's class's maximum — `1.00` under each of the three tables — less each id's weight per occurrence under that class's table, floored at zero and exact at two decimals, summed in integer hundredths. `none` is the empty list and reaches the maximum; a ledger line recorded with `flt-pending` gives `*pending*` whatever the table, though no table holds that id. The weights are defined in [RIGOR_AND_BEARING_SCORING_SCHEME.md](../../scoring/rigor-and-bearing/entry-point.md). No prompt template contains a weight, a maximum or a band: the ids and the criteria are the prompt's, the value is kbase's, and nothing else is an input. A written value is therefore checkable against its own ledger line without calling a model.

