# ROADMAP – kbase

Future intent only. Not part of the contract doc set; not handed to coding dispatches.
A finished item leaves nothing. Do not mark it as done and let it persist. Just remove it.

## Reader fidelity

### Display names from `\input`-ed preambles
That's it.
  
### Resolved numbering
theorem, equation and section numbers as the author's counter scheme produces
them, where within-section numbering and `\numberwithin` make pandoc's counter disagree.

### The `-latex_macros` reading 
pandoc's LaTeX reader with macro expansion disabled (`-f
latex-latex_macros`), which may preserve author environment names.

### A LaTeX reader in place of pandoc's 
`../polytexnical` - taken up only if it shows coverage pandoc's
reader lacks and the pre-pass registry can't supply, measured on real papers. pandoc is actively
maintained and may close those gaps itself.

## Exporter

### Nested table bug

see [EXPORT_LATEX_BUG](ROADMAP_DESIGNS/EXPORT_LATEX_BUG.md)

## Configuration

### Lockstep viability check

Test for whether lockstep schedule for the ask queue, beside the continuous one would be more efficient

- N calls launch together and the next N wait for the slowest, so a local endpoint prefills a whole
  wave at once instead of admitting calls one by one. 
- Worth it only if measured faster on local inference; a group's first ask still goes alone to warm
  the prefix cache.

## Distribution

### A `winget` distribution for Windows

Currently only weakly supported. Requires devpro to get tested for windows use as well.

## Chorres

### Update README argument graph images

The README's argument-graph images are snapshots of another project's graph. They are replaced by
`refresh`'s sheets from kbase's own KB once a KB argues the scoring mechanism, so the
pictures become kbase's own graph rather than an illustration of its capability.
