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

// The instructions initialize carries: an opening sentence where the server
// has one to give, followed by the guidance text embedded from
// instructions.txt and the session rule. Plain text: no Markdown reaches the
// wire.
//
//go:embed instructions.txt
var instructionsGuidance string

const (
	instructionsServing         = "kbase serves the knowledge base at %s."
	instructionsBuilding        = "kbase is registered for %s and can build its knowledge base; kb-root/ does not exist yet."
	instructionsProjectRequired = "kbase serves no single project: name one on every tool call with project-dir, the absolute path of the project's git worktree root, the directory that holds kb-root/ or will once a build makes it."
)

// ServedInstructions is what the server tells a session at initialize, from
// its located project: an opening naming the KB's path where its kb-root/
// stands, the repository's where that directory has yet to be made, the
// project-dir every call names where the server requires one, and none where
// it located no project; then, always, the guidance and the session rule.
func ServedInstructions(located Located) string {
	rules := strings.TrimSpace(instructionsGuidance) + "\n\n" + kbdocs.SessionRule()
	if located.Required() {
		return instructionsProjectRequired + " " + rules
	}
	repo, ok := located.Root()
	if !ok {
		return rules
	}
	opening := fmt.Sprintf(instructionsBuilding, repo)
	kbRoot := filepath.Join(repo, kb.KBDir)
	if info, err := os.Stat(kbRoot); err == nil && info.IsDir() {
		opening = fmt.Sprintf(instructionsServing, kbRoot)
	}
	return opening + " " + rules
}
