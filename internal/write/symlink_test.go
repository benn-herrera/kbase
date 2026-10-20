package write

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/result"
)

// TestResolveTargetRefusesASymlink: a kb-root-relative path that is itself a
// symbolic link is refused there, not resolved to the file it names.
func TestResolveTargetRefusesASymlink(t *testing.T) {
	root := seedKB(t)
	outside := filepath.Join(t.TempDir(), "leaf.md")
	if err := os.WriteFile(outside, []byte("outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "leaf.md")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	_, err := resolveTarget(root, "leaf.md")
	var sym *kb.SymlinkError
	if !errors.As(err, &sym) || sym.Path != link {
		t.Fatalf("resolveTarget = %v, want a SymlinkError naming %s", err, link)
	}
}

// TestAnOpOverASymlinkedDocumentRefuses: an op on a KB whose document is a
// symbolic link refuses with the symlink check, and the file the link names
// keeps its bytes.
func TestAnOpOverASymlinkedDocumentRefuses(t *testing.T) {
	root := seedKB(t)
	register := snapshot(t, root)[kb.RegisterFile]
	outside := filepath.Join(t.TempDir(), kb.RegisterFile)
	if err := os.WriteFile(outside, []byte(register), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, kb.RegisterFile)
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	res := run(t, root, "set-rigor", "entry:\n- id: "+seedClaim+"\n  rigor: 0.5\n", false)
	if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Check != kb.CheckSymlink {
		t.Fatalf("set-rigor over a symlinked register: %+v", res)
	}
	if data, _ := os.ReadFile(outside); string(data) != register {
		t.Error("the refused op reached the file the link names")
	}
}

// TestASymlinkErrorIsARefusal: the reader's refusal of a symbolic link is a
// check: symlink refusal carrying the link's path, not a failed write.
func TestASymlinkErrorIsARefusal(t *testing.T) {
	res := errorResult(&kb.SymlinkError{Path: "/kb/kb-root/a.md"})
	if res.Outcome != result.Refused || len(res.Refusals) != 1 {
		t.Fatalf("errorResult = %+v, want one refusal", res)
	}
	if it := res.Refusals[0]; it.Check != kb.CheckSymlink || it.Path != "/kb/kb-root/a.md" || it.Remedy == "" {
		t.Errorf("refusal = %+v, want check %q at the link's path", it, kb.CheckSymlink)
	}
}
