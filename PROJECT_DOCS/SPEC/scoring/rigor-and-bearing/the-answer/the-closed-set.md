### The closed set

The item's class fixes the accepted set before the call:

- `rigor` on a claim (`clm-`) accepts the four statement ids;
- `rigor` on an analysis or an experiment (`ana-`, `exp-`) accepts the six method ids;
- `bearing` accepts the five `bearing` ids, on any scoring edge.

No set holds `flt-pending`: a reply naming it is an id outside the set, and the `allowed` list the
refusal carries is the class's own.

`flt-contradiction` and `flt-overreach` are accepted by both rigor vocabularies. An id from the other
rigor vocabulary — `flt-underspecified` on an analysis, `flt-unaddressed` on a claim — is
`malformed`, and the refusal names the id and the call's ids as `allowed`. So is the other pass's id,
a letter from the retired alphabet, and a misspelling. An id is case-folded before matching and
recorded in its own lower-case spelling.

