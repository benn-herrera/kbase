package mcp

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kbase/internal/kb"
	"kbase/internal/kbdocs"
)

// The instructions initialize carries: an opening sentence naming the served
// path, in one of two forms, followed by the guidance text embedded from
// instructions.txt and the session rule. Plain text: no Markdown reaches the
// wire.
//
//go:embed instructions.txt
var instructionsGuidance string

const (
	instructionsServing  = "kbase serves the knowledge base at %s."
	instructionsBuilding = "kbase is registered for %s and can build its knowledge base; kb-root/ does not exist yet."
)

// ServedInstructions is what the server tells a session at initialize, from
// the directory the tools find their KB from: the KB's path where its kb-root/
// stands, the repository's where that directory has yet to be made, and
// nothing where no repository holds the directory. A --kb-root naming the
// repository root and one naming its kb-root/ both reach the same answer.
func ServedInstructions(dir string) (string, error) {
	repo, err := kb.RepositoryRoot(dir)
	if err != nil || repo == "" {
		return "", err
	}
	opening := fmt.Sprintf(instructionsBuilding, repo)
	kbRoot := filepath.Join(repo, kb.KBDir)
	if info, err := os.Stat(kbRoot); err == nil && info.IsDir() {
		opening = fmt.Sprintf(instructionsServing, kbRoot)
	}
	return opening + " " + strings.TrimSpace(instructionsGuidance) + "\n\n" + kbdocs.SessionRule(), nil
}
