// Package kbdocs writes the documents a build stamps into kb-root/ from their
// templates: the readiness documents, each only where none stands, and the
// overview document around a passage written from excerpts of the tree; and
// rewrites the block kbase maintains in each of them.
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
	"kbase/internal/buildrecords"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/result"
)

//go:embed templates/*.tmpl.md templates/digest-chunk.tmpl.md
var templates embed.FS

//go:embed templates/session-rule.txt
var sessionRule string

// SessionRule is the rule a session works under for rigor and bearing values:
// plain text, the one copy that the stamped AGENTS.md and the MCP instructions
// both carry.
func SessionRule() string { return strings.TrimSpace(sessionRule) }

//go:embed templates/index-rule.txt
var indexRule string

// IndexRule is the rule a session works under for kb-root/.index/: plain
// text, the one copy that the stamped AGENTS.md and the MCP instructions both
// carry.
func IndexRule() string { return strings.TrimSpace(indexRule) }

// The documents, by the name each is written under in kb-root/.
const (
	ConventionsFile = kb.ConventionsFile
	OverviewFile    = "README.md"
)

// The slots the templates name.
const (
	slotProjectName = "project-name"
	slotScopePin    = "scope-pin"
	slotPending     = "pending-literal"
	slotPassage     = "overview-passage"
	slotDigestChunk = "digest-chunk"
	slotSessionRule = "session-rule"
	slotIndexRule   = "index-rule"
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
// distilled because there is no corpus — its author writes what it holds, a
// claim at a time.
const AuthoredKBPin = "This KB was created empty by kbase init, authored, not distilled: " +
	"there is no corpus. Its author writes what it holds, a claim at a time."

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
	values := readinessValues(projectName, scopePin)
	for _, name := range kb.StampedFiles {
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

// readinessValues is what the readiness documents' slots are filled with.
func readinessValues(projectName, scopePin string) map[string]string {
	return map[string]string{slotProjectName: projectName, slotScopePin: scopePin,
		slotSessionRule: SessionRule(), slotIndexRule: IndexRule()}
}

// The maintained block's fence lines, each recognised by its two ends alone,
// so the text between the ends may differ between releases.
const (
	fenceOpenHead, fenceOpenTail   = "<!-- vvv-kbase", "kbase-vvv -->"
	fenceCloseHead, fenceCloseTail = "<!-- ^^^-kbase", "kbase-^^^ -->"
)

// blockSpan is the byte range of text between its one ordered pair of fence
// lines: from the start of the line after the opening fence to the start of
// the closing fence's line. ok is false where text holds other than one
// opening and one closing fence line, the opening first.
func blockSpan(text string) (start, end int, ok bool) {
	opens, closes := 0, 0
	for at := 0; at < len(text); {
		n := strings.IndexByte(text[at:], '\n')
		next := len(text)
		if n >= 0 {
			next = at + n + 1
		}
		line := strings.TrimSpace(text[at:next])
		switch {
		case strings.HasPrefix(line, fenceOpenHead) && strings.HasSuffix(line, fenceOpenTail):
			opens++
			start = next
		case strings.HasPrefix(line, fenceCloseHead) && strings.HasSuffix(line, fenceCloseTail):
			closes++
			end = at
		}
		at = next
	}
	return start, end, opens == 1 && closes == 1 && start <= end
}

// Maintain rewrites the maintained block of each maintained-block document
// of src's kb-root to what its template renders, writing only a document whose
// bytes change: AGENTS.md and CONVENTIONS.md, each written whole where absent,
// and README.md only where it stands — a build alone writes one, carrying its
// overview passage outside the block. A document holding other than one
// ordered pair of fence lines is left untouched and listed in unfenced. The
// templates are filled with projectName; AGENTS.md's scope pin is
// AuthoredKBPin where init made the KB, else none; README.md's digest
// sentence stands where digestDrawn. Both lists are kb-root-relative.
func Maintain(src *kb.Source, projectName string, digestDrawn bool) (written, unfenced []string, err error) {
	pin := ""
	if bornByInit(src) {
		pin = AuthoredKBPin
	}
	readme, err := Overview(projectName, "", digestDrawn)
	if err != nil {
		return nil, nil, err
	}
	rendered := map[string]string{OverviewFile: readme}
	for _, name := range kb.StampedFiles {
		if rendered[name], err = fill(name, readinessValues(projectName, pin)); err != nil {
			return nil, nil, err
		}
	}
	for _, name := range append(slices.Clone(kb.StampedFiles), OverviewFile) {
		target := src.KBPath(name)
		current, err := src.ReadFile(target)
		if errors.Is(err, os.ErrNotExist) {
			if name == OverviewFile {
				continue
			}
			if err := index.WriteKBFile(src, target, []byte(rendered[name])); err != nil {
				return written, unfenced, err
			}
			written = append(written, name)
			continue
		}
		if err != nil {
			return written, unfenced, err
		}
		start, end, ok := blockSpan(string(current))
		if !ok {
			unfenced = append(unfenced, name)
			continue
		}
		blockStart, blockEnd, _ := blockSpan(rendered[name])
		text := string(current[:start]) + rendered[name][blockStart:blockEnd] + string(current[end:])
		if text == string(current) {
			continue
		}
		if err := index.WriteKBFile(src, target, []byte(text)); err != nil {
			return written, unfenced, err
		}
		written = append(written, name)
	}
	return written, unfenced, nil
}

// bornByInit is whether init made the KB src holds: no build record stands.
// init writes no build record, and build refuses the populated kb-root it
// leaves; a build writes its node-pass record at claims-declared, before it
// stamps AGENTS.md at phase-3a.
func bornByInit(src *kb.Source) bool {
	return !slices.ContainsFunc(buildrecords.RecordFiles, func(rel string) bool { return src.IsFile(src.Path(rel)) })
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
