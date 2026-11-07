### No refresh moves a judgement

A rebuild of the derived views, `update-format`'s included, makes no model call, so a derived value
changes only when the records do: a write op, or a scoring run writing its results. `verify`'s
freshness check is between the records and a summary page, never between two judgements.

