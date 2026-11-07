### Required bearings

Every scoring edge record has its `bearing` field, and the strict load refuses one that lacks it,
has a value outside `[0, 1] ∪ {*pending*}`, or has it on a `references` record, naming the file, the
line and the field: a missing bearing is never read as `0`, nor as pending.

