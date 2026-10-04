package sheet

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestRenderDigestsTheIndexInSortedPathOrder(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{"cites.jsonl": "c\n", "claims.jsonl": "a\n", "depends-on.jsonl": "", "notes.txt": "ignored"}
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sum := sha256.Sum256([]byte("c\na\n"))
	want := "index sha256:" + hex.EncodeToString(sum[:])[:12]
	svg, err := Render(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(svg, []byte(want)) || !bytes.Contains(svg, []byte(">NYI<")) {
		t.Errorf("Render = %s, want it to show NYI and %q", svg, want)
	}
	again, err := Render(dir)
	if err != nil || !bytes.Equal(svg, again) {
		t.Errorf("Render is not deterministic: %v", err)
	}
	if !IsOwn(svg) {
		t.Error("IsOwn(placeholder) = false")
	}
	if IsOwn([]byte(`<svg><text>clm-aaaaaa</text></svg>`)) {
		t.Error("IsOwn(drawn sheet) = true")
	}
}
