### The form

One line per occurrence:

```
flt-id: 12
```

An id, a colon, and the number of the line where the fault first shows. The prompt asks for that one
line and never a range; the parse still reads a range of lines within one section, `flt-id: 12-14`.
Leading and trailing whitespace is trimmed, and the field after the colon is trimmed. The number before the bar is the template wrapper's, not the text's: `N| ` is a gutter the call writes,
and the words behind it are the text's own. A line is a charge line or it is not: blank lines are
ignored, and any other line — an explanatory sentence, a heading, a preamble — is `malformed`. Nothing
is read out of prose. The brackets of the format sketch are delimiters, not characters: a line may
have `[…]:` and `[…]` around the two fields or not, and a bracket pair around a whole field is
stripped before the field is read. Order is free; charges are recorded in the order written, and no
arithmetic depends on it.

