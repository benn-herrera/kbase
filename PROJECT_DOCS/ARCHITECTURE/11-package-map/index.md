## 11. Package map

The listing groups packages by role; the dependency rules below it are the whole rule, and the
import graph they allow is acyclic.

```
cmd/                      cobra root; one file per subcommand; surface.go, the one
                          registration file
internal/argmark          the one marker by which text names a tool argument (`Flag`,
                          `Positional`, `FlagValue` with its value) or holds a kbase
                          call (`Call`, an arguments mapping, lists of mappings
                          among its values; `Recorded`, a recorded
                          argv), and `Spelling.Spell`, each surface's one spelling:
                          `--name`, `<name>`, `--name value` at the command line,
                          `name`, `name: value` over MCP, a call through the
                          `Spelling.Call` its caller supplies; a marker inside other
                          text is backticked where its spelling is undelimited (every
                          one over MCP, a call at the command line), one that is the
                          whole text bare; imports no kbase package, and `log`
                          imports it to spell every line it writes for the command
                          line
internal/result           YAML result document + emitter (I5); the outcome-to-exit table;
                          a document's strings spelled by the writer it goes to
                          (`Spelled`, which spells any other text written on it
                          too: the root's stdout is one for every subcommand but
                          `mcp`, so what cobra prints itself — completion
                          descriptions, summaries, flag usages — is spelled; any
                          other writer keeps the marked form, a record's,
                          re-emitted spelled by `Reemit`), but a `Verbatim` — the
                          KB text a result passes through (a title, a rationale, a
                          context, a leaf's line, a conversion list's id and text),
                          written as it stands on every surface — and
                          `Items.Error` in the command line's spelling
internal/latex/pandoc     reader, writer and defaults-run seam (I1)
internal/latex/prepass    the two source scans (I3)
internal/docgraph         JSON transforms, whole-volume render+cut, outline,
                          records, partition checks, validate_build
internal/records          the record schema (§5.5), written by docgraph, read by claimgraph
internal/kb               kb-root model: names, exclusions, link primitives, dead-link
                          gate, the format stamp, its file and the frontmatter
                          recogniser, schema (the band ladder, spelled nowhere else;
                          the node kinds, the relation names, `Relation*` and
                          `Relations`, defined in `format400` and re-exported here
                          under the same names, as is the rest of the live format),
                          the record loader `LoadGraph` (the record files read
                          through `Source`, decoded by `format400`, a rigor above
                          `RigorMaximum` refused), the scoring faults' vocabulary
                          and the three weight tables (`faults.go`), `Source` (the one
                          read path), the derived row types `index.Derive` yields
                          (`indexstream.go`), the statement-marker reader
                          `MarkerLines` and the hosts it derives (`Hosts`), the
                          document kind (`DocumentKind`, from the child list),
                          `LabelLine` (the label-line recogniser), the child-list
                          reading of the tree relation (`ChildList`,
                          `ChildListLines`), the heading reader `FirstHeading`, the
                          reference-list entry reader `BibliographyEntries` and
                          the citation-key reader `CitationKeyRefs`, the
                          statement marker's one spelling (`StatementMarker`,
                          `StatementMarkerOpener`, `StatementMarkerRE`, re-exported
                          from `format400`), the heading anchors (`HeadingAnchors`
                          over `format400`'s `Slugify` and `Slugger`, re-exported),
                          the link grammar every link reader reads through
                          (`LinkRE`, `Links`, `Citation`, `BareIDs`, re-exported
                          from `format400`), the register pages' rule — the file
                          name, title and entry headings (`RegisterFile`,
                          `SummaryTitle`, `SummaryEntryHeadings`), the volumes
                          (`Volumes`) and a host's page (`SummaryPage`) —
                          re-exported from `format400`, which index and
                          claimgraph read it through
internal/kbload           `Open(kbRoot)`: reads the stamp, builds `kb.Source` (disk at
                          the current version, `formats.Chain` in memory for an older
                          one); the only importer of the `formats` root; imports `buildrecords`
                          for the root paths an older KB keeps the records at
                          (`buildrecords.RootRecordFile`); imported by `cmd`,
                          `build` and `write` only
internal/index            `Derive` (the derived rows in memory), the cycle load error
                          and the cycle walk (`BreakCycles`), strength,
                          the register pages, each node's page and
                          entry anchor there (`Rows.RegisterPages`,
                          `Rows.RegisterAnchors`), and their freshness,
                          the re-derivation (`RefreshReporting`), update-format's
                          save (`UpdateFormat`) and the pending-conversion refusal
                          (`PendingConversion`), the bare-id check,
                          the claim-id link check, the nodes a section's citation
                          can cite (`SectionNodes`), the path finder
                          (`DependsGraph.Path`)
internal/write            the write batch (`Run`: shape, apply in memory, the
                          derivation through `index.PlanDerivation`, then every
                          changed file written, or none under a dry run; §6) and
                          render / values / store / ops over the records and the
                          statement markers, provenance, the judged hash each value
                          set records (`judged.go`, through `asks`), the build-only
                          `add-build-edges`, `demote-claim-edge` and `split-claim`
internal/atomicfile       the one atomic file writer; `WriteMode` sets the file's
                          permission exactly, umask and a replaced file's mode aside
internal/query            the queries over `Derive`'s rows (SPEC §8): `find`'s
                          selection, sort and limit (`Find`, `FindUsage`; the
                          sort's checks and order in `rankUsage` and
                          `compareRanks`), `show`'s record — its host and section anchor
                          from the host leaf's marker, read once per leaf at
                          `Load`, its summary entry from `RegisterPages` and
                          `RegisterAnchors` — and `stats`, whose ranked sections
                          are `find` questions (`HighestLeverage`, `Weakest`);
                          `edges`' rows (`EdgeRowRecord`) and fixed order
                          (`compareEdgeRows`), and `trace`'s walk (`Trace`),
                          its contribution `index.EdgeContribution`'s
internal/sheet            DOT emitter, `dot` invocation: `Derive`'s rows, the leaves,
                          the entry point's and volumes' headings in; SVGs out
internal/export           the LaTeX export (§5.7): `export.Run`, the export's one
                          sequence, over unexported steps — the walk, the inputs of
                          `gen-tex/`, the embedded filter and template, the starter
                          `preamble.tex`, the report
internal/buildrecords     the four build records' readers (node-pass, unmarked,
                          classification, cull) and their paths under `kb-root/.build/`
                          (`Dir`, `RecordFiles`) and at the repository root, where a
                          KB before 4.0.0 keeps them (`RootRecordFile`); imported by
                          claimgraph, by kbdocs for whether a build made the KB, and
                          by kbload and `cmd/update-format.go` for the root paths the
                          conversion moves
internal/kbdocs           readiness stamp, the maintained blocks' fences and their
                          rewrite (`Maintain`), README assembly, the session rule
                          (`SessionRule`) and the `.index/` rule (`IndexRule`)
internal/claimgraph       conform, inventory (records + page-defined readings),
                          identify, attribute, shortlist, unmarked, order, cull,
                          split, elision (the cull's issuer), equation sites, classify,
                          equation(s), endcap, hand_named, assemble, write, gate,
                          the four build records
internal/asks             embedded templates + fragments, composer, caller
                          (captures, answer cache), multichoice-ask seam; the scoring
                          templates' line form, and the test that asserts the ids
                          each names against `kb`'s pass set; the judged hash's one
                          computation (`JudgedHash`; `NodeJudgedHash` and
                          `EdgeJudgedHash` name the template an item is judged under),
                          which score's dirty check and write's value sets both call
internal/gitledger        git checkpoint seam (I1), by exec
internal/filelock         the one advisory file lock (the KB write lock, the operation
                          lock in the git directory, the state store's holder lock)
internal/build            the walk and its steps' behaviour, the state store's writers,
                          resume, --no-inference, --through, progress and cancel,
                          and the development replay (`kbasedev`)
internal/buildstore       the build's state, read: the store's layout, holder record and
                          progress record, the stage vocabulary and the step table's
                          shape, status and the reading (holder lock, progress record,
                          the run it stands at) `wait` polls (`WaitState`), the commit
                          trail's reading, the newest build's (`CurrentTrail`) and
                          `build-trail`'s document of it (`BuildTrail`), the kb-root
                          refusal `init` and a fresh build share (`KBRootExists`),
                          and the progress logging both development replays share
                          (`ReplayProgress`, `kbasedev`); reaches no `model` or
                          `asks/call`
internal/score            the score passes' closed text, its guard and the passages it is
                          made of, a run's scope and the dirty check over the judged hash
                          (`scope.go`), the parse, the runner, the run's entry (`Invoke`) and
                          its landing through `write`, the run's ledger, its captures,
                          its re-pricing and the retention of runs, and the development
                          replay (`kbasedev`); imports not `model`; imported by cmd and
                          internal/mcp alone
internal/statedir         the state base (`Base`: $XDG_STATE_HOME/kbase,
                          $XDG_STATE_HOME defaulting to ~/.local/state where unset
                          or not an absolute path), and the
                          state directory of one KB: the override, else <key>
                          under the base, never inside kb-root/; imported by build,
                          buildstore and cmd
internal/auth             the credentials store: one file per provider,
                          `<provider>-key.enc`, the name lowercased, in the directory
                          its caller names, the key sealed by AES-GCM under a per-user
                          key HKDF derives from a constant in the source and the
                          username — obfuscation, not a boundary, separation between
                          users being the file's mode; the provider-name rule, stated
                          again from config's (`auth.NamePattern`,
                          `config.ProviderNamePattern`) and pinned equal by a cmd test;
                          imported by cmd alone
internal/secret           `Key`, the type every key travels in: fmt's verbs, a text,
                          JSON or YAML encoding and a log value render it
                          `<redacted>`, and `Reveal` alone returns the key; imports
                          no kbase package
internal/asks/call        the caller: the multichoice asks through a Slot's Decision behind the multichoice-ask
                          seam, the overview call through a Slot's Chat; captures, answer
                          cache; imported by build alone
internal/formats          (SPEC §10) a tree of packages, one per format and one per
                          converter; no file I/O. The root: `Chain`, the `converters`
                          list and the version arithmetic (`ParseVersion`,
                          `CompareMinor`, and `Chain` taking every patch of a minor
                          to that minor's converter), imported from outside the
                          tree by `kbload` alone.
                          `common`: what two or more of its superseded formats and
                          converters use and no format defines (`Files`, and
                          `Warning`, whose list, node id and text a conversion
                          result groups its dropped fields' warnings by).
                          `format<major><minor><patch>`: one format's definition —
                          stamp, field names, token patterns, vocabulary, record
                          shapes, its reader where one exists — defined once there
                          (`format200` covers every 2.0 patch). A format package may
                          change until a kbase release ships its version, and is
                          frozen after in what it accepts and what a conversion
                          produces. `format400` is the live format's: the record
                          types, field names, the judged hash, id grammar, node
                          kinds, relations, the statement marker, the heading
                          slugging, the link and citation grammar and the summary
                          pages' names, title, entry headings and volume rule
                          (`citation.go`) and the record files' encoder and decoder
                          over bytes; it imports no kbase package, `common` included,
                          and `kb` alone imports it from outside the tree. `format300`
                          is frozen, read by `from300to400`.
                          `from<from>to<to>`: one N→N+1 converter over the loose
                          representation, returning the next form plus the paths it
                          made obsolete
internal/mcp/protocol     (SPEC §11) JSON-RPC framing, dispatch, the Tool, Schema and
                          Property types and the frame checks; it declares no subscribe
                          capability and answers no subscribe method; imports `log`
                          only. The tool table, the resources and the server version
                          are injected by internal/mcp through Serve(in, out, version,
                          instructions, tools, resources, lg)
internal/mcp              the MCP exposure of the command tree: the `kbase.mcp`,
                          `kbase.mcp.family` and `kbase.readOnly` annotations, the
                          help notes' `kbase.help.mcp` and `kbase.help.cli`
                          annotations and the three help renderings (SPEC §11)
                          composed from a command's `Short`, `Long` and notes —
                          a tool's description, its help resource, and
                          `CLIHelp`, the `--help` text the root's help function
                          prints, the usage line, `Long`, the generated argument
                          list and the CLI note — the list (`ArgumentList`) built
                          from the definitions a tool's schema reads: a command's
                          positionals and flags (`commandArguments`), a values
                          op's `write.OpVocabulary`, the batch's `batchSchema`,
                          the root's persistent and own flags, each surface
                          typing a list and a mapping its own way; the CLI note
                          (`CLINote`) closed by the flags excluded from MCP and,
                          below the root, a line naming the global flags — each
                          rendering, and each schema description, spelling the
                          markers for its surface through `mcp.Spelling`, which
                          renders a call from the tree's annotations (`spelling.go`:
                          a bound command's tool call over MCP, `write`'s against
                          the batch schema; `CommandLine`'s command line at the
                          command line, where `write` and each write op take every
                          argument but their own flags as the values document,
                          piped in as `printf '<yaml>' | kbase <op>`), and each tool's
                          result spelled over MCP and checked for an item keyed
                          `--name` or `<name>` of its own arguments (`warnCLIKeys`,
                          a warning only) — the
                          tool table derived from the bound commands and their
                          families, a call run as its command's RunE, a family
                          tool's dispatch to its operation and the check of its
                          arguments against that operation's own schema, the
                          `write` tool, `kbase write`'s, whose arguments are the
                          batch and whose family's members, the write ops, have
                          no tool of their own, the
                          project a call runs in, the resources (each family's
                          help, `kbase://help/<family>`, and each bound
                          command's help, `kbase://help/<command>`, among them),
                          and the detached build and score launches, each reporting a
                          run still starting after 5 s (`launch.go`, the one self-exec
                          owner, I1), and the
                          instructions `initialize` reports; imported by cmd alone
internal/modelclient      the client registry (`Registry`, one `model.Client` per
                          endpoint, wire protocol and sampling settings) and the
                          mapping of configuration's `api`, flavour and sampling
                          onto `model`'s protocol, decision surface and sampling
                          (`chatProtocol`, `decisionsSurface`, `samplingOf`);
                          imports `model`, `config`, `result` and
                          `log`; imported by cmd alone
kept: config, model (the `Queue` and its `Slot`, the only exported entry to inference), log,
      version
```

Contents:

- [§11 Loading](loading.md)
- [§11 The loader's file set and its other refusals](the-loader-s-file-set-and-its-other-refusals.md)
- [§11 The zero-value hazard](the-zero-value-hazard.md)
- [§11 Dependency rules](dependency-rules.md)
