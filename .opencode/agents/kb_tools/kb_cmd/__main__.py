# !INSTALLED! from kb_tools/kb_cmd/__main__.py — do not edit here: request changes from the adjagent project; local needs belong in your CONVENTIONS.md.
"""Module entry point so ``python -m kb_cmd <args>`` works."""

import sys

from .cli import main

if __name__ == "__main__":
    sys.exit(main())
