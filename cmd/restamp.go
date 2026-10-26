package main

import (
	"path/filepath"

	"kbase/internal/kb"
	"kbase/internal/kbdocs"
)

// restamp puts in a migrated src the stamped documents its save writes, named
// for the repository its kb-root sits in, as init names them.
func restamp(src *kb.Source) error {
	return kbdocs.Restamp(src, filepath.Base(src.Repo()))
}
