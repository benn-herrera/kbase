// Package ledger is the build's record in git: each stage boundary a commit
// whose subject names the stage, scoped by pathspec to the paths the build
// owns, and the trail of those commits read back as the build's position. It
// is the only package that runs git, and it runs the host's, so the user's
// configuration, hooks and attributes apply.
package ledger

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"kbase/internal/log"
)

// Prefix opens every boundary commit's subject.
const Prefix = "kb-build:"

// subjectRE reads a boundary's stage id off its subject: the grep finds
// candidates and this decides, so a body quoting the prefix forges nothing.
var subjectRE = regexp.MustCompile(`^` + regexp.QuoteMeta(Prefix) + ` ([^\s|]+) \| `)

// NotWorktreeError is a directory outside any git worktree.
type NotWorktreeError struct{ Dir, Detail string }

func (e NotWorktreeError) Error() string {
	return fmt.Sprintf("%s is not inside a git worktree (%s); run `git init` there first", e.Dir, e.Detail)
}

// Repo is one git worktree and the paths the build owns in it, relative to
// its root.
type Repo struct {
	Root  string
	owned []string
	lg    log.Logger
}

// Open is the worktree holding dir, owning the given root-relative paths.
func Open(dir string, owned []string, lg log.Logger) (*Repo, error) {
	r := &Repo{owned: owned, lg: lg}
	out, err := r.git(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, NotWorktreeError{dir, err.Error()}
	}
	root := strings.TrimSpace(out)
	if root == "" {
		return nil, NotWorktreeError{dir, "git names no top level"}
	}
	r.Root = filepath.FromSlash(root)
	return r, nil
}

// Entry is one boundary commit.
type Entry struct {
	Stage, Commit, Body string
}

// Trail is every boundary commit reachable from HEAD, newest first. A branch
// with no commits has an empty trail.
func (r *Repo) Trail() ([]Entry, error) {
	if _, err := r.run("rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
		return nil, nil
	}
	out, err := r.run("log", "--grep=^"+Prefix, "--format=%H%x1f%s%x1f%b%x1e")
	if err != nil {
		return nil, err
	}
	var trail []Entry
	for _, rec := range strings.Split(out, "\x1e") {
		parts := strings.SplitN(strings.TrimLeft(rec, "\n"), "\x1f", 3)
		if len(parts) != 3 {
			continue
		}
		if m := subjectRE.FindStringSubmatch(parts[1]); m != nil {
			trail = append(trail, Entry{Stage: m[1], Commit: parts[0], Body: strings.TrimSpace(parts[2])})
		}
	}
	return trail, nil
}

// Record commits the owned paths as the boundary of stage, its subject
// `kb-build: <stage> | <display>` and body as given, and returns the commit.
// Nothing else the user staged rides along, and a boundary that changed
// nothing is still recorded.
func (r *Repo) Record(stage, display, body string) (string, error) {
	paths, err := r.present()
	if err != nil {
		return "", err
	}
	if len(paths) > 0 {
		if _, err := r.run(append([]string{"add", "-A", "--"}, paths...)...); err != nil {
			return "", err
		}
	}
	args := []string{"commit", "--quiet", "--allow-empty", "--only", "-m", fmt.Sprintf("%s %s | %s", Prefix, stage, display)}
	if body != "" {
		args = append(args, "-m", body)
	}
	if _, err := r.run(append(append(args, "--"), paths...)...); err != nil {
		return "", err
	}
	out, err := r.run("rev-parse", "HEAD")
	return strings.TrimSpace(out), err
}

// Dirty is every owned path git reports changed, staged or untracked, root
// relative; ignored files are not dirt.
func (r *Repo) Dirty() ([]string, error) {
	out, err := r.run(append([]string{"status", "--porcelain=v1", "-z", "--untracked-files=all", "--"}, r.owned...)...)
	if err != nil {
		return nil, err
	}
	var dirty []string
	fields := strings.Split(out, "\x00")
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		if len(f) < 4 {
			continue
		}
		dirty = append(dirty, f[3:])
		if f[0] == 'R' || f[0] == 'C' {
			i++
		}
	}
	return dirty, nil
}

// Restore returns every owned path to its state at commit — HEAD where commit
// is empty — tracked content restored, index and worktree alike, and
// untracked files removed. On a branch with no commits it unstages them.
func (r *Repo) Restore(commit string) error {
	if commit == "" {
		if _, err := r.run("rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
			return r.unstageAndClean()
		}
		commit = "HEAD"
	}
	var known []string
	for _, p := range r.owned {
		inCommit, err := r.run("ls-tree", "--name-only", commit, "--", p)
		if err != nil {
			return err
		}
		inIndex, err := r.run("ls-files", "--", p)
		if err != nil {
			return err
		}
		if strings.TrimSpace(inCommit+inIndex) != "" {
			known = append(known, p)
		}
	}
	if len(known) > 0 {
		if _, err := r.run(append([]string{"restore", "--source=" + commit, "--staged", "--worktree", "--"}, known...)...); err != nil {
			return err
		}
	}
	return r.clean()
}

func (r *Repo) unstageAndClean() error {
	paths, err := r.present()
	if err != nil || len(paths) == 0 {
		return err
	}
	if _, err := r.run(append([]string{"rm", "-r", "--cached", "--quiet", "--ignore-unmatch", "--"}, paths...)...); err != nil {
		return err
	}
	return r.clean()
}

// clean removes the untracked files under the owned paths; ignored files
// stay.
func (r *Repo) clean() error {
	paths, err := r.present()
	if err != nil || len(paths) == 0 {
		return err
	}
	_, err = r.run(append([]string{"clean", "--force", "-d", "--quiet", "--"}, paths...)...)
	return err
}

// Files is every file under dir, root relative, as commit holds it, keyed by
// its path relative to dir.
func (r *Repo) Files(commit, dir string) (map[string][]byte, error) {
	out, err := r.run("archive", "--format=tar", commit, "--", dir)
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	tr := tar.NewReader(strings.NewReader(out))
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return files, nil
		}
		if err != nil {
			return nil, fmt.Errorf("reading %s at %s: %w", dir, commit, err)
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		var b bytes.Buffer
		if _, err := io.Copy(&b, tr); err != nil {
			return nil, err
		}
		rel := strings.TrimPrefix(path.Clean(h.Name), path.Clean(dir)+"/")
		files[rel] = b.Bytes()
	}
}

// IndexLock is the path of the repository's index lock where one stands, ""
// where none does. While it stands every commit and restore fails.
func (r *Repo) IndexLock() (string, error) {
	out, err := r.run("rev-parse", "--git-path", "index.lock")
	if err != nil {
		return "", err
	}
	p := filepath.FromSlash(strings.TrimSpace(out))
	if !filepath.IsAbs(p) {
		p = filepath.Join(r.Root, p)
	}
	if _, err := os.Lstat(p); errors.Is(err, os.ErrNotExist) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return p, nil
}

// present is the owned paths git would accept in a pathspec: on disk, or
// tracked.
func (r *Repo) present() ([]string, error) {
	var paths []string
	for _, p := range r.owned {
		if _, err := os.Lstat(filepath.Join(r.Root, p)); err == nil {
			paths = append(paths, p)
			continue
		}
		tracked, err := r.run("ls-files", "--", p)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(tracked) != "" {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

func (r *Repo) run(args ...string) (string, error) { return r.git(r.Root, args...) }

// git runs one git command in dir. It takes no context: a commit cut short
// would leave the repository's index locked, so a cancelled build lets the
// one in flight finish.
func (r *Repo) git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	r.lg.Debug("git", "dir", dir, "args", strings.Join(args, " "))
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
