// Package pandoc is the one place kbase execs pandoc: the LaTeX reader, the
// gfm writer, and the preflight that holds the binary to the accepted range.
package pandoc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

const (
	binary = "pandoc"

	// InstallPage is named by a refusal over a missing binary.
	InstallPage = "https://pandoc.org/installing.html"

	// bibliographyExit is pandoc's own exit code for a --bibliography it
	// could not parse, the one reader failure a build continues past.
	bibliographyExit = 25
)

// Exit codes pandoc gives a source it could not parse (PandocParseError,
// PandocParsecError).
var sourceExits = map[int]bool{64: true, 65: true}

// The accepted range: pandoc at or above minVersion, and a
// pandoc-api-version whose first two components are apiMajor.
var (
	minVersion = []int{3, 12}
	apiMajor   = []int{1, 23}
)

// unloadableInclude is pandoc's warning for an \input it could not read. It
// arrives at exit 0 with the file's content absent, so the warning text is
// the only evidence. The origin's filename is optional because the source
// arrives on stdin, which pandoc has no name for.
var unloadableInclude = regexp.MustCompile(
	`(?m)^\[WARNING\] Could not load include file (.+) at ((?:\S+ )?line \d+ column \d+)$`)

// MissingError is a pandoc absent from PATH.
type MissingError struct{}

func (MissingError) Error() string {
	return fmt.Sprintf("%s is not on PATH; install it (%s) and re-run", binary, InstallPage)
}

// VersionError is a pandoc outside the accepted range.
type VersionError struct {
	Version    string
	APIVersion string
}

func (e VersionError) Error() string {
	return fmt.Sprintf("%s %s (pandoc-api-version %s) is outside the accepted range: %s >= %s with pandoc-api-version %s.x",
		binary, e.Version, e.APIVersion, binary, dotted(minVersion), dotted(apiMajor))
}

// SourceError is a source the reader could not parse.
type SourceError struct{ Complaint string }

func (e SourceError) Error() string { return "the reader could not parse the source: " + e.Complaint }

// BibliographyError is a --bibliography the reader could not parse. The exit
// code says a file failed, not which; the complaint names it.
type BibliographyError struct{ Complaint string }

func (e BibliographyError) Error() string {
	return "the reader could not read a bibliography: " + e.Complaint
}

// Include is one file the source named that pandoc could not load.
type Include struct {
	Target string
	Origin string
}

// IncludeError is every include pandoc reported unloadable on one read.
type IncludeError struct{ Includes []Include }

func (e IncludeError) Error() string {
	parts := make([]string, len(e.Includes))
	for i, inc := range e.Includes {
		parts[i] = fmt.Sprintf("%s (named at %s)", inc.Target, inc.Origin)
	}
	return "the reader could not load: " + strings.Join(parts, "; ")
}

// Warner receives what pandoc writes to stderr on an otherwise successful run.
type Warner interface {
	Warn(msg string, kv ...any)
}

// Preflight holds the binary to the accepted range and returns its version.
func Preflight(ctx context.Context) (string, error) {
	out, _, err := run(ctx, "", nil, "--version")
	if err != nil {
		return "", err
	}
	fields := strings.Fields(strings.SplitN(out, "\n", 2)[0])
	if len(fields) < 2 || fields[0] != binary {
		return "", fmt.Errorf("unrecognized `%s --version` output: %q", binary, fields)
	}
	version := fields[1]

	doc, _, err := run(ctx, "", []byte{}, "-f", "markdown", "-t", "json")
	if err != nil {
		return "", err
	}
	var probe struct {
		API []int `json:"pandoc-api-version"`
	}
	if err := json.Unmarshal([]byte(doc), &probe); err != nil {
		return "", fmt.Errorf("reading pandoc-api-version: %w", err)
	}
	if !atLeast(parseVersion(version), minVersion) || !hasPrefix(probe.API, apiMajor) {
		return "", VersionError{Version: version, APIVersion: dotted(probe.API)}
	}
	return version, nil
}

// ReadLaTeX parses source to pandoc's JSON AST. dir is the volume's own
// directory: pandoc resolves \input against its working directory, not the
// source's. With bibliographies, citeproc runs and resolves citations against
// their union; without, no citeproc runs.
func ReadLaTeX(ctx context.Context, w Warner, dir, source string, bibliographies []string) ([]byte, error) {
	args := []string{"-f", "latex", "-t", "json"}
	if len(bibliographies) > 0 {
		args = append(args, "--citeproc")
		for _, b := range bibliographies {
			args = append(args, "--bibliography="+b)
		}
	}
	out, complaint, err := run(ctx, dir, []byte(source), args...)
	if err != nil {
		return nil, err
	}
	if found := unloadableInclude.FindAllStringSubmatch(complaint, -1); len(found) > 0 {
		includes := make([]Include, len(found))
		for i, m := range found {
			includes[i] = Include{Target: m[1], Origin: m[2]}
		}
		return nil, IncludeError{Includes: includes}
	}
	warn(w, complaint)
	return []byte(out), nil
}

// WriteGFM renders a JSON AST as GitHub-flavoured Markdown, standalone so the
// metadata arrives as a YAML block at its head, unwrapped.
func WriteGFM(ctx context.Context, w Warner, doc []byte) (string, error) {
	out, complaint, err := run(ctx, "", doc, "-f", "json", "-t", "gfm", "-s", "--wrap=none")
	if err != nil {
		return "", err
	}
	warn(w, complaint)
	return out, nil
}

func warn(w Warner, complaint string) {
	if complaint != "" {
		w.Warn("pandoc wrote to stderr on a successful run", "stderr", complaint)
	}
}

// run execs pandoc and returns its stdout and stderr, every failure shape as
// a typed error.
func run(ctx context.Context, dir string, stdin []byte, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = dir
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	complaint := strings.TrimSpace(stderr.String())
	if errors.Is(err, exec.ErrNotFound) {
		return "", "", MissingError{}
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		switch code := exit.ExitCode(); {
		case code == bibliographyExit:
			return "", "", BibliographyError{Complaint: complaint}
		case sourceExits[code]:
			return "", "", SourceError{Complaint: complaint}
		default:
			return "", "", fmt.Errorf("%s %s exited %d: %s", binary, strings.Join(args, " "), code, complaint)
		}
	}
	if err != nil {
		return "", "", fmt.Errorf("running %s: %w", binary, err)
	}
	return stdout.String(), complaint, nil
}

func parseVersion(v string) []int {
	var out []int
	for _, part := range strings.Split(v, ".") {
		n, err := strconv.Atoi(part)
		if err != nil {
			break
		}
		out = append(out, n)
	}
	return out
}

// atLeast compares version component-wise against floor.
func atLeast(version, floor []int) bool {
	for i, f := range floor {
		if i >= len(version) {
			return false
		}
		if version[i] != f {
			return version[i] > f
		}
	}
	return true
}

func hasPrefix(version, prefix []int) bool {
	if len(version) < len(prefix) {
		return false
	}
	for i := range prefix {
		if version[i] != prefix[i] {
			return false
		}
	}
	return true
}

func dotted(v []int) string {
	parts := make([]string, len(v))
	for i, n := range v {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, ".")
}
