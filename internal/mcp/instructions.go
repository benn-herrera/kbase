package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kbase/internal/argmark"
	"kbase/internal/kb"
	"kbase/internal/kbdocs"
)

// EdgeDirection is how an edge reads: the one sentence both the instructions
// and the root command's help carry.
const EdgeDirection = `An edge reads "source *relation* target", so "A supports B" means A's strength bears on B.`

// The openings of a server that located a project, by whether its kb-root/
// stands.
const (
	instructionsServing  = "kbase serves the knowledge base at %s."
	instructionsBuilding = "kbase serves the repository at %s and can build its knowledge base; kb-root/ does not exist yet."
)

// instructionsNotLocated is the opening of a server that located no project.
var instructionsNotLocated = "kbase serves no single project: every tool call this session names one with " +
	argmark.Flag(projectDirProperty) + ", the absolute path of the project's git worktree root, the directory holding .git, " +
	"where kb-root/ is or will be. Get that path from the user, by asking or by proposing one for them to confirm."

// instructionsGuidance is what the instructions hold after the opening and
// its sentence naming the skill, whatever the mode: the .index/ rule and the
// session rule the stamped AGENTS.md also carries, then the paragraphs only
// the instructions carry.
var instructionsGuidance = []string{
	kbdocs.IndexRule(),
	kbdocs.SessionRule(),
	EdgeDirection,
	"Pass " + argmark.Flag(commitProperty) + " on every write batch: it is the only way to make the batch undoable as one unit. " +
		"It commits all of `kb-root/`.",
	"The claim-graph sheets are drawn for people. Offer them for display, as files in kb-root/ or, where this server lists them, " +
		"as kbase://sheet/ resources. Never read one for information; the queries answer what a sheet shows.",
	"A command kbase prints for handling a key, such as the upgrade field show-config reports, is run as printed. " +
		"Never compose one from a key file's path, and never ask for a key or read a key file into the conversation.",
}

// ServedInstructions is what the server tells a session at initialize, spelled
// over MCP: an opening saying where the project is — the KB's path where the
// located project's kb-root/ stands, the repository's where that directory has
// yet to be made, and how a session gets the project-dir every call names
// where the server located none — then the skill, then the guidance.
func ServedInstructions(located Located) string {
	opening := instructionsNotLocated
	if repo, ok := located.Root(); ok {
		opening = fmt.Sprintf(instructionsBuilding, repo)
		kbRoot := filepath.Join(repo, kb.KBDir)
		if info, err := os.Stat(kbRoot); err == nil && info.IsDir() {
			opening = fmt.Sprintf(instructionsServing, kbRoot)
		}
	}
	text := opening + " The kbase skill carries the workflow.\n\n" + strings.Join(instructionsGuidance, "\n\n")
	return argmark.Spelling{MCP: true}.Spell(text)
}
