// Package kbdocs writes the documents a build stamps into kb-root/ from their
// templates: the readiness documents, each only where none stands, and the
// overview document around a passage written from excerpts of the tree.
package kbdocs

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"kbase/internal/atomicfile"
	"kbase/internal/kb"
	"kbase/internal/result"
)

//go:embed templates/*.tmpl.md templates/digest-chunk.tmpl.md
var templates embed.FS

// The documents, by the name each is written under in kb-root/.
const (
	ConventionsFile = "CONVENTIONS.md"
	OverviewFile    = "README.md"
)

// readinessDocs are the documents stamped from their templates, in this order:
// init stamps them at a KB's birth, the build at its validation gate.
var readinessDocs = []string{kb.AgentsFile, ConventionsFile}

// The slots the templates name.
const (
	slotProjectName = "project-name"
	slotScopePin    = "scope-pin"
	slotNodeKinds   = "node-kinds"
	slotPending     = "pending-literal"
	slotPassage     = "overview-passage"
	slotDigestChunk = "digest-chunk"
)

// digestChunkName names the template sub-piece holding the digest sentence
// the overview fills its digest-chunk slot with, present only where the KB
// carries the digest sheet (README.tmpl.md: everything between the "DIGEST"
// and "END DIGEST" markers). The wording lives in the template, never in Go;
// a one-volume KB — the common shape — fills the slot with the empty string.
const digestChunkName = "digest-chunk.tmpl.md"

func digestChunk() (string, error) {
	b, err := templates.ReadFile("templates/" + digestChunkName)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// AuthoredKBPin is the scope pin of a KB `init` created empty: no corpus was
// distilled because there is no corpus — what the KB holds, its author writes
// into it, a claim at a time.
const AuthoredKBPin = "This KB was created empty by kbase init, authored, not distilled: " +
	"there is no corpus. What it holds its author writes into it, a claim at a time, " +
	"through the ops `CONVENTIONS.md` states."

var slotRE = regexp.MustCompile(`\{([a-z][a-z0-9]*(?:-[a-z0-9]+)*)\}`)

// blankRunRE matches a run of three or more newlines: an empty value leaves
// one where its slot stood alone on its line.
var blankRunRE = regexp.MustCompile(`\n{3,}`)

// fill substitutes values into the named template in one pass, so a value
// holding brace text is content. A slot with no value refuses the fill. A run
// of three or more newlines — what an empty value leaves — collapses to one
// blank line.
func fill(name string, values map[string]string) (string, error) {
	b, err := templates.ReadFile("templates/" + strings.TrimSuffix(name, ".md") + ".tmpl.md")
	if err != nil {
		return "", err
	}
	text := string(b)
	var missing []string
	for _, m := range slotRE.FindAllStringSubmatch(text, -1) {
		if _, ok := values[m[1]]; !ok && !slices.Contains(missing, m[1]) {
			missing = append(missing, m[1])
		}
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("%s: the template names slot(s) nothing fills: %s", name, strings.Join(missing, ", "))
	}
	text = slotRE.ReplaceAllStringFunc(text, func(s string) string { return values[s[1:len(s)-1]] })
	return blankRunRE.ReplaceAllString(text, "\n\n"), nil
}

// Stamp writes the readiness documents into the KB's kb-root, each only
// where none stands, and CLAUDE.md as the redirect to AGENTS.md likewise;
// scopePin is what AGENTS.md says the KB distills, empty where its caller has
// no statement of its own to make. It returns the kb-root-relative paths it
// wrote, and one report line per document; on an error, both as far as it
// got. A CLAUDE.md that is not the redirect is a result.Refusal, before
// anything is written.
func Stamp(src *kb.Source, projectName, scopePin string) (written, reports []string, err error) {
	kbRoot := src.Root()
	msg, err := kb.UnmigratedAgentsFile(src)
	if err != nil {
		return nil, nil, err
	}
	if msg != "" {
		return nil, nil, result.Refusal{{Check: kb.UnmigratedAgentsCheck, Path: kb.AgentsRedirectFile, Detail: msg}}
	}
	kinds := make([]string, len(kb.NodeKinds))
	for i, k := range kb.NodeKinds {
		kinds[i] = "`" + k + "`"
	}
	values := map[string]string{slotProjectName: projectName, slotScopePin: scopePin, slotNodeKinds: strings.Join(kinds, ", ")}
	for _, name := range readinessDocs {
		target := filepath.Join(kbRoot, name)
		if exists(target) {
			reports = append(reports, name+": present, left as authored")
			continue
		}
		text, err := fill(name, values)
		if err != nil {
			return written, reports, err
		}
		if err := atomicfile.Write(target, []byte(text), nil); err != nil {
			return written, reports, err
		}
		written = append(written, name)
		reports = append(reports, name+": written from its template")
	}
	redirect := filepath.Join(kbRoot, kb.AgentsRedirectFile)
	if exists(redirect) {
		return written, append(reports, kb.AgentsRedirectFile+": present, already the redirect"), nil
	}
	if err := atomicfile.Write(redirect, []byte(kb.AgentsRedirect+"\n"), nil); err != nil {
		return written, reports, err
	}
	return append(written, kb.AgentsRedirectFile), append(reports, kb.AgentsRedirectFile+": written as the redirect to "+kb.AgentsFile), nil
}

// Overview is the overview document: its template around the passage.
// digestDrawn is whether the KB carries the volume digest sheet — two or more
// volumes hold nodes (kb.SheetDigestDrawn); false, the common one-volume
// shape, leaves the digest sentence out of the README entirely.
func Overview(projectName, passage string, digestDrawn bool) (string, error) {
	digest := ""
	if digestDrawn {
		var err error
		if digest, err = digestChunk(); err != nil {
			return "", err
		}
	}
	return fill(OverviewFile, map[string]string{slotProjectName: projectName, slotPending: kb.PendingLiteral,
		slotPassage: passage, slotDigestChunk: digest})
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return !errors.Is(err, os.ErrNotExist)
}
