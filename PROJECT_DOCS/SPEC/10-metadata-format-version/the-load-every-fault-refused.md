### The load: every fault refused

The load refuses a KB whose records hold any of these faults, `check: metadata`, naming each with its
file and its line or record:

1. a record file that does not parse;
2. an unknown or missing field;
3. a value of the wrong type or outside its domain, a `rigor` above `1.0` included;
4. an id outside its kind's grammar;
5. one node id on two records;
6. a pairing that no relation joins (the relations table below);
7. one edge identity on two records;
8. an edge naming a node that has no record;
9. a cycle among the scoring edges.

Faults 7 to 9 are found only once the record files decode without fault. A record file missing from
a KB stamped at the current version is refused `check: index`, remedy "restore it from git". Faults
7 to 9, which no write op produces, have the remedy "return the record files to a consistent
commit in git, such as the one before a merge, and reapply the other side's changes through write
ops". The remedy of a `rigor` above `1.0` says kbase never writes such a value, so whatever wrote
it is the way to undo it. No op resolves a load error.

