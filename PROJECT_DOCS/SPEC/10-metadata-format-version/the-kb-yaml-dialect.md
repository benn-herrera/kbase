### The KB YAML dialect

kbase writes, and a reader needs, only this subset: block mappings and block lists, lists of flat
mappings, a block list of scalars as a mapping's value, plain or double-quoted scalars, no anchors,
tags, folded or multi-line scalars; flow style only for a list of scalars (`[clm-a, clm-b]`) and an
empty list or mapping (`[]`, `{}`); a scalar is never wrapped. A reader of the dialect, in any
language, is conformant when it reads the record files of the `4.0.0` golden trees
to the values their decoded-values goldens hold.

