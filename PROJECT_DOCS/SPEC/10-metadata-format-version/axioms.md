### Axioms

An axiom is declared by its node record, which `add-axiom` writes ([§8](../8-maintenance-subcommands/index.md)). `axioms.md` is prose stating
it, one heading each, `### Axiom <label>: <title><!-- kbase-id: <id> -->`; the marker pairs the
heading with its record, and `axioms.md` is not read for declarations. That marker is not a
statement marker, and the axiom stays unhosted. `verify`'s `axiom-heading` check holds the heading
and the record together by three checks: each axiom record has exactly one heading ending in its
marker; that heading's text is `Axiom ` followed by the record's label, `: ` and its title; and each
marked heading names an axiom record. A fault of these checks is one the author's prose edit clears
([§8](../8-maintenance-subcommands/verify.md)).

