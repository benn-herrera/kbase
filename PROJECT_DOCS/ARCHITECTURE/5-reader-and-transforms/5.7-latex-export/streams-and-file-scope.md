#### Streams and file scope

The stream is `kbase-export-front.md`, then one `kbase-export-vol{N}.md` per volume in order.

- The front file is the entry point's prose: its first heading is taken as the document `title`, its
  up-link and volume-list lines are removed, and every remaining heading is marked `{.unnumbered}`.
- A volume file contains the volume's index and every document of its navigation tree with
  navigation stripped (`export.Strip`).

`defaults.yaml` lists the files as `input-files` with `file-scope: true`, so pandoc parses each file
apart and prefixes its identifiers and `#`-link targets with `<file>__`. The scope is the volume,
for two reasons: one concatenated parse would collide the footnote labels of different volumes, and
a footnote's definition in a built KB can sit in a different document of its volume than the
reference, which a per-document scope would sever. The filter's first act removes the prefix, for
the file names in the tables' `streams` and no other `__`.

