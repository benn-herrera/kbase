package statedir

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/result"
)

// TestDirPerKBRoot: two KB roots under one XDG state home get two directories,
// and each stands under <xdg>/kbase under its own key.
func TestDirPerKBRoot(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_STATE_HOME", xdg)
	base := t.TempDir()
	dirs := map[string]string{}
	for _, name := range []string{"one", "two"} {
		root := filepath.Join(base, name, kb.KBDir)
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		dir, err := Dir("", root)
		if err != nil {
			t.Fatalf("Dir over %s: %v", root, err)
		}
		if want := filepath.Join(xdg, "kbase"); filepath.Dir(dir) != want {
			t.Errorf("Dir over %s = %s, want one under %s", root, dir, want)
		}
		dirs[name] = dir
	}
	if dirs["one"] == dirs["two"] {
		t.Errorf("two KB roots share the state directory %s", dirs["one"])
	}
}

// TestOverrideInsideKBRootRefused: an override the user names is used as given,
// and one inside kb-root/ is refused with the item every state-store refusal
// names.
func TestOverrideInsideKBRootRefused(t *testing.T) {
	kbRoot := filepath.Join(t.TempDir(), "repo", kb.KBDir)
	if err := os.MkdirAll(kbRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	override := filepath.Join(t.TempDir(), "store")
	got, err := Dir(override, kbRoot)
	if err != nil || got != override {
		t.Errorf("Dir(%q) = %q, %v; want the override as given", override, got, err)
	}

	inside := filepath.Join(kbRoot, "store")
	_, err = Dir(inside, kbRoot)
	var ref result.Refusal
	if !errors.As(err, &ref) {
		t.Fatalf("Dir inside kb-root = %v, want a refusal", err)
	}
	detail := fmt.Sprintf("the state directory %s is inside %s; build state is never written there", inside, kbRoot)
	if len(ref) != 1 || ref[0].Check != checkUsage || ref[0].Key != "--state-dir" || ref[0].Path != inside || ref[0].Detail != detail {
		t.Errorf("the refusal = %v; want one item, check %s, key --state-dir, detail %q", ref, checkUsage, detail)
	}
}

// TestKeyResolvesTheUnbuiltTree: one KB root named through a link before
// kb-root/ exists and by its resolved path once it does is one 16-digit key.
func TestKeyResolvesTheUnbuiltTree(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	before, err := Key(filepath.Join(link, kb.KBDir))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(real, kb.KBDir), 0o755); err != nil {
		t.Fatal(err)
	}
	after, err := Key(filepath.Join(real, kb.KBDir))
	if err != nil {
		t.Fatal(err)
	}
	if before != after || len(before) != KeyHexDigits {
		t.Errorf("state keys %q (through a link, kb-root absent) and %q (resolved, present) must be one %d-digit key", before, after, KeyHexDigits)
	}
}
