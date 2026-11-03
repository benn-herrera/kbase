### Required bearings

Every scoring edge record carries its `bearing` field, and the strict load
refuses one that lacks it, carries a value outside `[0, 1] ∪ {*pending*}`, or carries it on a
`references` record (above), naming the file, the line and the field: a missing bearing is never
read as `0`, nor as pending.

