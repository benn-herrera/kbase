# Survey: rehoming the retired served-surface design into SPEC

Abbreviations: `D:` = `DOCS_REVIEW/reference/NEW_MCP_CLI_DESIGN.md`, `S:` = `DOCS_REVIEW/SPEC.md`,
`A:` = `DOCS_REVIEW/ARCHITECTURE.md`, `C:` = `DOCS_REVIEW/CONVENTIONS.md`.

How the citations were found, so the counts can be re-run from `DOCS_REVIEW/`:

- `grep -o 'NEW_MCP_CLI_DESIGN/[^)#]*' SPEC.md ARCHITECTURE.md CONVENTIONS.md | wc -l` returns **206** links:
  196 in SPEC, 7 in ARCHITECTURE and 3 in CONVENTIONS. The grouped `file:line` list in the table comes from
  `grep -n -o` over the same files.
- `grep -n -i 'page §' … | grep -v 'NEW_MCP_CLI_DESIGN/'` finds one unlinked mention, at S:2171 ("the three checks
  page §1.1 lists"). The other unlinked hits (S:180, S:490) are links that wrap onto the next line.

## 1. Summary

1. The 206 dead links and 1 unlinked mention point at 55 design sections. In most §8 ops SPEC says the base
   rule "is page §X's" and then lists only "what this release adds" (e.g. S:803, S:829, S:863, S:1111). Until
   it moves, the base rule exists only in the retired design.
2. The heaviest items are §2.2 write batches (17 citations), §2.9 manually scored and the judged hash (16),
   §2.10 existence (11), §1.4 statement markers (9), §1.1 nodes (8), §2.3 the derivation (8), §2.4 load errors
   (8), `score` (8) and §4.2 help (8).
3. Much of the content those links point at now appears in SPEC only in the interim-release notes, §12
   (S:2812-2829 and S:2877-3004). §12 is a changelog. The rewrite has to lift these rules into the normative
   sections rather than cite §12.
4. For these sections SPEC already states the content, so the links can simply be deleted: §1.2, §2.5, §2.6,
   §2.7, `cancel`, `init`, `export-latex`, `citations-of`, `stats`, `show-config`, `auth`, §3.7 index and §4.5.
5. Proposed new SPEC sections:
   - §8 "Write batches", from design §2.2, §2.3 and §2.10;
   - §10 "Documents of a KB", from the design's §1 intro and the summary-page half of §1.6;
   - §10 "Statements and anchors", from §1.4;
   - §10 "Provenance and the judged hash", from §2.9;
   - §11 "Served text", from §4.1 and §4.2;
   - §6 "`kbase build-trail`".
6. Settle these before rewriting:
   - S:695-696 says the per-op key vocabulary "is not copied into prose". That is a mechanism rule sitting in
     SPEC, and it blocks moving the design's key tables into SPEC (§4 item 7).
   - SPEC disagrees with itself on whether `probe` is a tool: S:686-688 and S:2724-2725 against S:1468,
     S:2580 and S:3073.

## 2. Cited design sections

The class column says where the content belongs: B = behaviour (SPEC), M = mechanism (ARCHITECTURE), Mx = mixed.

| Design section (D: lines) | Cited at (count) | Class | Recommended home | Overlap with SPEC; superseded text |
|---|---|---|---|---|
| §1.1 Nodes (D:32-76) | A:541; S:2058, 2103, 2105, 2170, 2171 (unlinked), 2189, 2892 (8) | B. Exception: the hint "what would strengthen a node is `trace` with sort contribution" (D:52-53) is help guidance | §10 "The record files" (S:2086) takes the record-field table (D:37-43), which S:2102-2103 only delegates. §10 "Five node types" (S:2184) takes the title and label rules. §10 "Axioms" (S:2166) takes the heading form and `verify`'s three checks (D:64-69); S:2171 points at those checks without stating them | Bands: stated in full at S:2276-2286. Rules for authors: in full at S:2288-2296. Minted ids: S:2188. Axiom rigor 1.0: S:2205. Not in SPEC: the field table, the citation-key grammar `[A-Za-z0-9](?:[A-Za-z0-9_.:+/-]*[A-Za-z0-9])?` (D:57-58), and "a label is never an id" (D:60-61). Title uniqueness within a kind appears only in §12 (S:2883, S:3045) |
| §1.2 Edges and relations (D:77-103) | S:864, 2060, 2303 (3) | B | §10 "Four scoring relations, one arithmetic" (S:2208) | Mostly stated already: the relations table (S:2213-2218), the arithmetic and exclusions (S:2229-2257), identity (S:2122-2124) and context reported as null (S:2120). Two gaps. (a) `references` from any kind to any kind appears only inside a sentence at S:2059; it needs a table row. (b) The definition of *dependents* (D:103: the distinct targets of a node's outgoing scoring edges) is missing, although `find` and `stats` use it (S:967, S:993) |
| §1.3 Demoted edges (D:104-126) | S:789, 864, 1016, 2523, 2534 (5) | B | §10 "The `demoted` relation" (S:2521). Give it a heading of its own: it is currently a bold paragraph inside "The migration announces itself" | Mostly stated already (S:2116, S:2124, S:2521-2538). Gaps: a demoted record is named by `source`, `target` and `relation: demoted`, and that name covers every record between the two nodes (D:115-116). `restore-demoted` puts back the relation, context, bearing and mark exactly (D:118-119). Both appear only in §12 (S:2803-2805). Standard framing: an edge set aside to break a cycle, keeping its full payload |
| §1.4 Statement markers (D:127-179) | S:67, 806, 915, 932, 2061, 2107, 2126, 2148, 2165 (9) | B | New §10 "Statements and anchors" (§4 item 3) | Partly stated: the marker form (S:67, S:2127); no marker on a fence's opening or closing line (S:2134-2136); snippet comparison and the nearby lines a refusal names (S:812-814); the first marker hosts (S:985); GitHub's slugger, by name only (S:2106, S:2837). Missing: the definitions of paragraph and section, and how each kind's statement is read (D:150-166); a node with no marker gets no host, no section anchor and no summary entry (D:167-169); the slug algorithm (D:171-177); the section anchor (D:179). Standard: GitHub heading anchors (github-slugger) |
| §1.5 Citations (D:180-212) | S:831, 916, 931, 2063, 2138 (5) | B | §10 "Citations" (S:2138) | Partly stated: the forms are named (S:2139) and the bare-id fault is stated (S:2140, S:1515). The link target for each kind appears only in §12 (S:2942-2943). "Checked once when placed, then like any link" appears only in §12 (S:2843). "A citation is not an edge" (D:212) is missing. A citation's target is not the same thing as `canonical_path`: for a claim the citation target is its host leaf and section anchor (D:201-202), while its canonical path is its register entry (S:2161-2162). State both so the two are not confused. Standard: a Markdown link whose title attribute is the node id |
| §1.6 Derived files (D:213-239) | S:69, 1048, 2062, 2144 (4) | B | Summary-page half: §10 "Register pages" (S:2144), or the new §10 "Documents of a KB". Sheet half: nothing to move | Sheets are fully stated in §3 (S:103-155), and a volume is defined at S:115. S:2144-2149 delegates the summary-page rules: which page (the root page where a node is hosted outside every volume or any work exists), the `# KB summary` title, the preamble line, the entry heading, no entry for an axiom or an unmarked node, and entry order (marker reading order, then works by label, D:230). Only part of this appears in §12 (S:2839). S:2153-2155 rightly calls an entry's body presentation |
| §1.7 Maintained-block documents (D:240-257) | C:284; S:71, 85, 1478, 2470 (5) | B | §3 "Stamped documents" (S:76) | Partly stated: the fence names and `unfenced`, in §12 only (S:2866-2870); `CLAUDE.md` as bytes (S:71); `init` writes only absent files (S:758). Missing: a marker line is recognised by its two ends; one ordered pair; an absent `AGENTS.md` or `CONVENTIONS.md` is written whole; `README.md` is written only by a build; an empty pair is valid; when the block is rewritten (D:243-255). Standard: generated-region fences (the BEGIN/END GENERATED idiom) |
| §1.8 Format version (D:258-287) | S:90, 1984, 2054 (3) | B | §10 "The stamp" and "One version read and written" (S:1961-2008) | Mostly stated: the current version (S:1984), the break in both directions (S:1992-1998), migration at load (S:2000-2008). The list of what `4.0.0` defines (D:271-283) is only partly summarised at S:2053-2063, which leaves out the `.build/` placement and the minted work and axiom ids; §12 has those (S:2871, S:2890). Standard: a semver major bump is a breaking change |
| §2.1 Naming (D:302-324) | S:3078 (1) | Mx. Naming nodes and edges, closed key sets and the model-configuration vocabulary are B. The advice on copying a row into an op (D:311-314) is help guidance | §8 "Values input" (S:691): add that a query row's op keys name the edge for any edge op. The copying advice goes to the new §11 "Served text" | Edge naming: S:697-699. Role: S:642-648. Model reference: S:1832. The one citation is a §12 note, so the link can go |
| §2.2 Write batches (D:325-408) | A:443, A:471; S:622, 693, 703, 715, 734, 774, 776, 807, 835, 867, 888, 1492, 2582, 2596, 2879 (17) | Mx. B: batch shape, the two surfaces, order, no references between entries, all-or-nothing, the two error phases, refusal-item keys, dry run, the `commit` flow and labels, locks, values, result. M: read the whole KB into memory, apply in memory, write each changed file once, temp-then-rename (D:347-350). ARCHITECTURE already has these (A:443-475). The consumer-visible limit stays in SPEC: a crash between two files' writes is not atomic, and git covers it (D:350-351) | New §8 "Write batches" (§4 item 1) | Locks: stated in full (S:718-734). Result keys: stated in full (S:622). Values: partly (S:691-700 refuses U+0008 and U+000C only; line breaks are covered only through the record format, S:2094). Commit: partly (S:776-786 adds to the design's rules but delegates the base flow). Order, shape first, stop at the first refusal, dry run and the empty batch: §12 only (S:2812-2829). No references between entries: §12 only (S:2878-2879). Standard: atomic batch (transaction); validate everything, then apply and stop at the first failure; ids assigned by the server and returned per entry (no forward references); advisory lock with a bounded wait that yields a retryable error; dry run |
| §2.3 The derivation after every write (D:409-415) | A:475; S:181, 703, 715, 734, 793, 1260, 1383 (8) | B | §8 "Writes leave nothing stale" (S:713), merged into the new "Write batches" | Mostly stated (S:713-717, S:181, S:1259-1260). "The derivation does not repair or convert" (D:414) is stated only per verb (S:1485, S:2183). Standard: derived files recomputed and written in the same transaction as the records |
| §2.4 Load errors (D:416-435) | A:1906; S:600, 954, 1378, 1486, 1521, 2177, 2544 (8) | B | §10 "The load: every fault refused" (S:2174) | Partly stated: the three faults no op produces, with their remedy (S:2180-2183); a missing record file (S:2179); rigor above 1 (S:2271). The nine-item fault list (D:420-428) is not in SPEC: S:2177 only delegates to it. Standard: strict schema and referential-integrity check at load that refuses on any fault |
| §2.5 The domain rule (D:436-445) | S:174, 407 (2) | B | §5 "Output and placement" (S:171) and §6 (S:397) | Fully stated: S:173-175, S:407-417, and commit scope at S:777-778. Inconsistency: D:438-439 says nothing outside `kb-root/` is checked, `update-format`'s clean check included. Yet `update-format` checks the build records at the repository root that it would move (D:1277, S:1475-1476). The rule should state that exception |
| §2.6 Format conversion (D:446-458) | S:576, 1380, 1479, 2001 (4) | Mx. B, except "nothing cached between loads" (D:449-450), which is M | §10 "One version read and written" (S:1990) | Fully stated: S:1992-2008, S:2480-2481 (deterministic minted ids), S:2862-2865. The links can go. Standard: migration at read time, saved by an explicit migrate command, with writes blocked until it is saved |
| §2.7 Provider names (D:459-471) | S:1549, 1589, 1622, 1740, 1836, 1927, 3070 (7) | B | §9.1 (S:1737) | Fully stated: S:1738-1740, S:1549-1551, S:1588-1589. The links can go |
| §2.8 Results and exit codes (D:472-531) | S:666, 958, 968, 1010, 1026, 3052 (6) | B | §7 "Item" and "Bounded results" (S:592, S:664) | Fully stated: document shape (S:564-582), exit table (S:671-679), `isError` (S:2630). Check values: S:600 names them but defines none; D:498-517 gives a condition for each. S:600 also lacks `agents-redirect`, which S:71 and S:717 use. The design lacks `include`, `metadata-key`, `inputs`, `pandoc`, `export-dir` and `write`; SPEC is newer on those. Lists: only partly at S:666-668. The sorting rules are missing: ascending by default, every key in both directions, pending below every number, null last (D:527-531). A negative `limit` as a Python slice (D:523-525) appears only in §12 (S:2963). Standard: no pagination; a top-N limit with slice semantics; nulls last |
| §2.9 Manually scored, and the judged hash (D:532-552) | C:272; S:314, 807, 816, 835, 846, 866, 883, 1112, 1133, 1143, 1251, 1323, 1364, 2056, 2110 (16) | B. The provenance mechanism at C:272 stays in CONVENTIONS | New §10 "Provenance and the judged hash" (§4 item 4) | Partly stated: field shapes (S:2108-2110, S:2116-2118); per-op hash writes (S:815-817, S:845-848, S:881-884); build writes `false` (S:312-314); landings (S:1248-1251); a §12 summary (S:2993-2998). Missing from the normative text: which records carry the mark (D:534-536); `true` on a value given and `false` on one defaulted (D:537-539); re-asserting a value on a record marked `false` marks it `true` with outcome `done` (D:540-541); no key sets the mark (D:542); the hash covers the text plus the ask template's digest and not the model (D:543-545); what makes an item dirty (D:546-547). Standard: content fingerprint for staleness (ETag-style); provenance flag that pins a value |
| §2.10 Existence (D:553-571) | A:29; S:703, 791, 806, 834, 866, 889, 988, 1021, 1037, 1653 (11) | B | The new §8 "Write batches", replacing §8 "Idempotence" (S:701) | Mostly stated: the rule is summarised at S:703-712; each op's error string is given (S:795-796, 841, 869-872, 892, 939, 963, 988, 1020, 1036); provider and key existence (S:1601-1603, S:1652-1653). Missing: lookups match exactly, with no near-miss (D:560-561); the remedy names the query that lists what exists (only at S:872, and in §12 at S:2988). The heading "Idempotence" mislabels this: a re-issued add or removal is refused, not repeated. Standard: strict create (conflict where the item exists, no upsert), not-found on removing a missing item, and an edit that changes nothing reports `unchanged` |
| §3.1 Node writes, index (D:615-634) | S:803 (1) | B | §8 "`add-<kind>`" (S:801) | The identity rule is fully restated at S:803-806. But S:803 also delegates "each op's keys, defaults, result and refusals", and those live in the leaves this link does not reach (§3 below) |
| `add-axiom` (D:676-692) | S:2889 (1) | B | §8 "`add-<kind>`" | Mostly stated: S:818-826, S:622 (`heading`), and §12 (S:2885-2887). Missing: the keys (`label`, `title`) |
| `edit-<kind>` (D:693-744) | S:829 (1) | B | §8 "`edit-<kind>`" (S:827) | Partly stated: refusals (S:838-860) and a §12 summary (S:2897-2911). Missing: the table of fields each kind's edit may change (D:712-718). The re-pointing rules (D:696-709) are only partly at S:849-854. The label-change rewrite targets (D:721-729) are only partly at S:855-856 and in §12 |
| `remove-node` (D:745-765) | S:887 (1) | B | §8 "`remove-node`" (S:885) | The details are stated (S:892-910). The core effect (it removes the record, the marker, every edge and every demoted record naming the node) appears only in §12 (S:2927-2930) |
| `move-node` (D:766-780) | S:913 (1) | B | §8 "`move-node`" (S:911) | Nearly fully stated: S:915-925 and §12 (S:2936-2940). Missing: the keys |
| §3.2 Edge writes, index (D:781-788) | S:863 (1) | B | §8 "Edge ops" (S:861) | The index holds only a list of ops. S:863 delegates keys, results and refusals to the leaves (§3 below). The refusal details are at S:869-884 and in §12 (S:2912-2921) |
| `restore-demoted` (D:836-847) | S:789 (1) | B | §8 "`restore-demoted`" (S:787) | Mostly stated (S:790-800 and §12 S:2922-2926). Missing: `relation` must be `demoted`, and any other relation is refused (D:841, D:845) |
| `add-citation` (D:854-889) | S:928 (1) | B | §8 "`add-citation`" (S:926) | Partly stated: S:934-948 and §12 (S:2941-2950). Missing: the keys, and the output syntax of each of the three forms (D:860-874); `snippet` must occur exactly once on the line (D:863-866), which appears only in §12 (S:2946) |
| `citations-of` (D:890-898) | S:957 (1) | B | §8 "`citations-of`" (S:955) | Fully stated: S:957-963 and S:623. The link can go |
| `find` (D:912-941) | S:624, 966 (2) | B | §8 "`find`" (S:964) | Partly stated: rows (S:624), tie-breaks (S:974-975), refusals (S:976-978). The filter semantics (D:917-935) appear only in §12 and only in part (S:2957-2965): case-insensitive substring match on title and label; `path` as a directory or a leaf, with `""` or `.` meaning `kb-root/`; `min` means at or above and `max` strictly below, both comparing scored nodes only; `min: 0` keeps the scored nodes; the sort keys |
| `show` (D:942-961) | S:627, 981 (2) | B | §8 "`show`" (S:979) | Fully stated: S:627, S:985-988. The form of `summary`, `<page>#<slug>`, is stated only in §12 (S:2967) |
| `edges` (D:962-981) | S:625, 900, 1009 (3) | B | §8 "`edges`" (S:1007) | Partly stated: rows (S:625), refusals (S:1017-1021), part of the order (S:1013). Missing from the normative text: the arguments (`id`; `direction` `in`, `out` or `both`, default `both`; repeatable `relation`) and the base order (relation, source, target). §12 has them (S:2973-2979). The *dependents* line (D:978-979) belongs with §1.2's definition |
| `trace` (D:982-1003) | S:626, 1024 (2) | Mx. B, except that `contribution` reuses the scoring code and no restated formula (D:994-996), which is M and goes to ARCHITECTURE | §8 "`trace`" (S:1022) | Partly stated: S:1029-1037 and §12 (S:2980-2984). Missing: the `contribution` sort, including that a `refutes` edge's contribution is its cap (D:990-994); `*pending*` versus null contribution (D:1000-1001); the walk never follows `references` or `demoted` (D:985-986) |
| `stats` (D:1004-1020) | S:991 (1) | B | §8 "`stats`" (S:989) | Fully stated: S:627 and S:991-1006. The link can go |
| `build` (D:1045-1086) | S:181, 203, 751, 2646 (4) | B | §5 "Starting over" (S:200) and §7 (S:616) | Nearly fully stated: synopsis (S:162-165), inputs (S:191-198), result (S:616), MCP result (S:2648-2657), refusals spread across S:179, 185-187, 220-234, 258-260, 441-446 and 2007. Rewrite "are page §3.5's" (S:202-204) as a single list of refusals. Title uniqueness appears only by delegation (S:180) and in §12 (S:3045) |
| `score` (D:1087-1137) | S:1111, 1129, 1144, 1248, 1308, 1395, 1422, 2671 (8) | B | §8 "`score`" (S:1102) | Mostly stated: S:1104-1431, S:636, S:2663-2681. Missing from the normative text: the default scope (D:1092-1096; S:1110 names it but delegates); what `rescore-manual` reaches (D:1103-1105), only in §12 (S:3001-3003); the base refusal list (D:1122-1135), of which S:1395-1397 states only the additions; "no score batch and no item limit: the scope is the budget" (D:1136-1137), only in §12 (S:3003) |
| `status` (D:1138-1160) | S:491 (1) | B | §6 "`kbase status`" (S:488) | Partly stated: keys (S:617) and the score shape (S:514-526). The build `state` vocabulary (`none`, `running`, `cancelled`, `interrupted`, `failed`, `bounded`, `finished`; D:1145) is not listed anywhere; S:617 says only "§6's vocabulary". Refusals: a newer store is at S:455-457; a missing worktree and a `--state-dir` inside `kb-root/` appear only for the MCP tools (S:2661) |
| `cancel` (D:1161-1169) | S:536 (1) | B | §6 "`kbase cancel`" (S:534) | Fully stated: S:536-544 and S:618. The link can go |
| `wait` (D:1170-1176) | S:547 (1) | B | §6 "`kbase wait`" (S:545) | Behaviour fully stated (S:547-551). Missing: `--pid` is required and positive, and its usage refusal (D:1175-1176) |
| `build-trail` (D:1177-1192) | S:205, 494, 2688 (3) | B | New §6 "`kbase build-trail`", beside status, cancel and wait | Partly stated: the commands (S:205-207, S:2688-2693) and `check: trail` (S:230-234). Missing: the result keys (`state-dir`, `head`, `boundaries` oldest first with `stage`, `commit`, `subject` and `command`, `started-at`, and `dirty` naming the first 20 paths; D:1188-1190), because the §7 table (S:614-636) has no `build-trail` row; and the refusals for outside a worktree and a store that `status` would refuse |
| `verify` (D:1200-1226) | S:621, 1502, 2153, 2172, 3057, 3061 (6) | B | §8 "`verify`" (S:1500) | Partly stated: S:1506-1523 adds to the base rules. The base list of faults with their check names (D:1204-1218) appears only in §12 (S:3049-3057). The remedies (D:1219-1223) are only partly at S:1511-1518 |
| `update-format` (D:1227-1282) | A:2147; S:620, 1475, 1488, 2024, 2476, 2896 (7) | B | §8 "`update-format`" (S:1473) and §10 "The `3.0.0 → 4.0.0` migration" (S:2472) | Mostly stated: the conversion steps are restated in full at S:2474-2508; result (S:620); commit (S:1487-1492). The refusal list (D:1276-1282) is scattered: S:1475-1476, S:1485, S:1997, and the lock rule at S:720. Collect it in one place. Standard: a migrate command that requires a clean tree |
| `export-latex` (D:1283-1301) | S:1047, 1092, 3065 (3) | B | §8 "`export-latex`" (S:1038) | Fully stated: S:1038-1100 and S:635. The links can go |
| §3.7 Setup and configuration, index (D:1302-1323) | S:1540, 2729, 2761 (3) | B | §9 (S:1532) | Fully stated: S:1534-1541; key handling at S:1891-1894 and S:1949. The link at S:2729 cites the index for catalogue behaviour that lives in the `configure-*` leaves |
| `init` (D:1324-1341) | S:204, 750 (2) | B | §8 "`init`" (S:736) | Fully stated: S:736-769 and S:619. The links can go |
| `configure-providers` (D:1342-1367) | S:630, 1591, 1696 (3) | B | §9 "Provider commands" (S:1584) | Fully stated: S:1594-1621 and S:1694-1710. But the §7 row (S:630) has no `warnings` key, although S:1618-1621 says the command reports them and D:1360-1365 lists the key. Add it to the row |
| `configure-models` (D:1368-1384) | S:631, 1591, 1887 (3) | B | §9 "Provider commands" | Fully stated: S:1622-1632 and §9.5. The §7 row (S:631) has no `warnings` key, although S:1631 says a catalogue miss is a `warnings` item (D:1378-1382). Add it to the row |
| `models` (D:1385-1393) | S:1587 (1) | B | §9 "Provider commands" | Fully stated: S:1586-1587, S:1677-1691 and S:628. The §7 row has no `warnings` key, although S:1620 says `models` reports them (D:1390-1392). Add it to the row |
| `show-config` (D:1394-1407) | S:1926 (1) | B | §9.6 (S:1909) | Fully stated: S:1909-1955 and S:629. The link can go |
| `probe` (D:1408-1417) | S:1468, 3074 (2) | B | §8 "`probe`" (S:1434) | Fully stated: S:1434-1471 and S:632. Superseded text: S:686-688 says "`probe` excepted" from the tools, and S:2724-2725 says no inference runs from any tool "but `build` and `score`". Both contradict S:1468, S:2580, S:2622 and S:3073, and D:598-600 and D:1440-1441. §12 is the newest, so fix S:686 and S:2724 |
| `auth` (D:1418-1428) | S:3072 (1) | B | §9 "Provider commands" (S:1640) | Fully stated: S:1640-1658 and S:633-634. The link can go |
| §4.1 The instructions (D:1451-1482) | S:81, 2737, 3083 (3) | B: the served text an agent reads. S:2737 requires the design's "quoted wording" | New §11 "Served text" (§4 item 5) | Partly stated: §12 lists the eight topics only (S:3079-3084). S:78-81 requires `AGENTS.md` to carry items 3 and 4 "in the same words", but no words are in SPEC. Item 6's quoted sentence (D:1475-1476) and item 5's shared sentence with `kbase --help` (D:1472-1474) are also missing. The same text appears in the instructions, `AGENTS.md` and `--help`, and only a test can guarantee the three stay byte-identical. SPEC should state the text once and require that it be identical in each place, not repeat it |
| §4.2 Tool descriptions, help resources and `--help` (D:1483-1547) | C:211; S:603, 1469, 2580, 2626, 2713, 2717, 2754 (8) | Mx. B: the three renderings and what each holds; the content each MCP note must carry (the `probe` sentence, D:1530-1531, which S:1469 delegates and nothing restates); the `write` description's batch contract (D:1514-1523); the obligations on shared help bodies (D:1540-1547). M or process: the four stored texts (D:1485-1494); the test that each rendering is exactly its parts (D:1502); `measure-mcp-surface` (D:1512). These go to ARCHITECTURE or CONVENTIONS; C:211 already holds the test | New §11 "Served text" | The renderings are fully stated (S:2623-2629, S:2708-2721, S:2751-2756). The rest of the B part is missing, as listed in the class column |
| §4.3 Server modes and `project-dir` (D:1548-1562) | S:2635, 3087 (2) | B | §11 "Two modes" (S:2635) | Mostly stated: S:2607-2613, S:2635-2639, and §12 (S:3085-3087). Missing: "startup never fails for want of a project" (D:1562) |
| §4.4 CLI-only commands (D:1563-1569) | S:2580, 3089 (2) | B | §11 "Tools" (S:2577) | Fully stated: S:2577-2580. "The root help and `mcp`'s help name the three CLI-only commands" appears only in §12 (S:3088) |
| §4.5 Served resources (D:1570-1583) | S:2704 (1) | B | §11 "Resources" | Fully stated: S:2694-2723. The link can go |

### ARCHITECTURE and CONVENTIONS links to re-point once SPEC holds the rule

- A:29 (invariant I6) → the new SPEC §8 "Write batches", existence rule.
- A:443 and A:471 → the new SPEC §8 "Write batches".
- A:475 → SPEC §8 "Writes leave nothing stale", or its successor.
- A:541 → SPEC §10 "Five node types".
- A:1906 → SPEC §10 "The load".
- A:2147 → SPEC §10 "The `3.0.0 → 4.0.0` migration".
- C:211 → the new SPEC §11 "Served text".
- C:272 → the new SPEC §10 "Provenance and the judged hash".
- C:284 → SPEC §3 "Stamped documents".

## 3. Uncited design sections

| Section (D: lines) | Holds a rule SPEC needs? |
|---|---|
| §1 intro (D:7-31) | **Yes, and it would be lost.** It lists the document classes: entry point, index, `axioms.md`, the maintained-block documents and the `CLAUDE.md` redirect, the summary pages, and leaves ("every other Markdown document"). It also says markers are read from leaves only, and that a summary entry's marker hosts nothing. SPEC defines document kinds only inside the `3.0.0` migration paragraph (S:2048-2049), which leaves out `axioms.md`, the maintained-block documents and the summary pages. Yet `add-<kind>`'s "not a leaf" refusal (D:656) depends on the definition. Home: new §10 "Documents of a KB" |
| §2 intro (D:288-301), §3.3 index (D:848-853), §3.6 index (D:1193-1199), §4 intro (D:1442-1450) | No. These are lists of sections only |
| §3 intro (D:572-614) | No. The surface kinds, global flags, MCP tool set and read-only set are already stated (S:685-689, S:1566-1577, S:2577-2629, read-only set S:2621-2622). The op and tool counts (42 and 16) are derived by test (S:2627-2629) and need no prose |
| `add-claim`, `add-analysis`, `add-experiment` (D:635-664); `add-work` (D:665-675) | **Yes.** These are reached only through S:803's link to the §3.1 index. They hold the key table (D:640-648), `add-work`'s keys (D:670-671) and the refusal lists. SPEC has a partial summary only in §12 (S:2877-2885). The citation-key grammar a work's label must match is nowhere in SPEC. Home: §8 "`add-<kind>`" |
| `add-edge` (D:789-812), `edit-edge` (D:813-825), `remove-edge` (D:826-835) | **Yes.** These are reached only through S:863's link to the §3.2 index. They hold the keys (D:793-799, D:817, D:832) and the rule that `remove-edge` leaves every other record byte for byte (D:829-830). Home: §8 "Edge ops" |
| §3.4 index (D:899-911) | No. Already stated at S:949-954 and S:730 (no lock) |
| §3.5 index (D:1021-1044) | No. Already stated: the state-store key and the operation lock (S:437-449); foreground at the CLI, detached over MCP (S:1120-1123, S:2640) |
| §3.8 / `mcp` (D:1429-1441) | No, but it shows that S:2724-2725 is stale on `probe` (see the `probe` row) |
| §5 Out of scope (D:1584-1588) | No. It points at another document and states no rule |

## 4. Proposed new SPEC sections

1. **§8 "Write batches"**, the first subsection of §8. It replaces the delegating text of "Values input",
   "Idempotence", "Writes leave nothing stale", "One write lock" and "Write ops" (S:691-786); those rules can
   stay as its own subsections. It should state:
   - batch shape and the two surfaces;
   - entries apply in the order given, each checked against the state the earlier entries left;
   - no entry refers to an id another entry of the same batch mints, and each entry's minted id is returned
     beside it;
   - every shape error is reported at once, then entries apply and the batch stops at the first refusal;
   - all or nothing, the derivation included, plus the stated limit that a crash between two files' writes is
     not atomic;
   - the existence rule: an add of what stands is a conflict; a removal or edit of what does not stand is
     not-found; an edit that changes nothing is `unchanged`; lookups match exactly;
   - the value domains and the prose-value rules;
   - a dry run's result is identical to a real run's;
   - the `commit` flow: pre-flight commits, labels, and the outcome when a commit fails;
   - locks.

   Reading the whole KB into memory and temp-then-rename stay in ARCHITECTURE §6 "Write API".
2. **§10 "Documents of a KB"**, placed before "The record files" (S:2086). It defines the document classes
   (from the design's §1 intro): what a leaf is, where markers are read, and which documents kbase generates
   or partly maintains. It defines a volume, citing S:115, and states the summary-page rules from §1.6:
   placement, title, preamble, entry heading, which nodes get no entry, entry order and removal. What an
   entry's body shows stays declared presentation, as at S:2153-2155.
3. **§10 "Statements and anchors"**, placed after "The record files". It replaces the "Statement markers"
   paragraph (S:2126-2136) and states:
   - the marker's form and placement, including the abutting pair and the fence-line rule;
   - how `document`, `line` and `snippet` name a line, and the refusal that names nearby lines;
   - how each kind's statement is read (paragraph, section, the two layouts the analysis split writes);
   - what a missing or extra marker means for host, anchor and summary entry;
   - the GitHub slug algorithm and the section anchor.

   "Citations" (S:2138) and `canonical_path` (S:2157) then cite this section.
4. **§10 "Provenance and the judged hash"**, placed after "Required bearings" (S:2309). It states:
   - which records carry `manually-scored` and `judged-hash`;
   - when a write op writes `true` or `false`, and the re-assertion rule;
   - that no argument sets the mark;
   - what the hash covers: the item's statement, or an edge's two end statements, plus the ask template's
     digest, but not the model;
   - the all-zeros value for a value never judged, and what makes an item dirty.

   The per-op notes in §8 and `score`'s scope then cite it, and the 16 citations reduce to one section.
5. **§11 "Served text"**, a subsection of §11 that takes over the "Instructions" bullet (S:2736-2739). It
   states:
   - the eight instruction items, with the wording SPEC requires quoted once;
   - which items `AGENTS.md` and `kbase --help` repeat byte for byte;
   - the help renderings and what each holds;
   - the required content of the MCP and CLI notes: the `write` batch contract, the `probe` sentence, and the
     detached `build`/`score` note;
   - the obligations on shared help bodies, including the advice on copying a row into an edge op from §2.1.

   How the texts are stored and the test that each rendering is exactly its parts go to ARCHITECTURE and
   CONVENTIONS.
6. **§6 "`kbase build-trail`"**, beside `status`, `cancel` and `wait`. It gives the result keys and
   refusals, which are currently in neither §6 nor the §7 table, and moves the trail rules from S:205-207
   and S:2687-2693 into one place.
7. **Decide before rewriting:** S:695-696 ("stated once, in kbase's write package; it is not copied into
   prose").
   - The problem: this is a rule about where the code keeps its key list, sitting in SPEC. SPEC's own Must-see
     (S:7) says any implementation that satisfies SPEC is valid, and that cannot hold if the op keys are
     stated only in code. So SPEC must state the keys, and the single-source rule belongs in ARCHITECTURE or
     CONVENTIONS.
   - The risk: tables in prose drift from the code, which is the concern behind S:695.
   - Recommendation: have a test check SPEC's key tables against the op definitions. If no such test is
     built, the rewrite should accept the drift risk explicitly rather than leave the keys out of SPEC.

Also to fix in the rewrite:

- **§7 table**: add a `build-trail` row; add `warnings` to the `configure-providers`, `configure-models` and
  `models` rows; add `agents-redirect` to S:600's list of check values.
- **`probe`**: remove "`probe` excepted" at S:686-688, and add `probe` to "no inference runs from any tool
  but…" at S:2724-2725.
