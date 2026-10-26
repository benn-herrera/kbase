package buildstore

import (
	"os"
	"path/filepath"
	"testing"

	"kbase/internal/asks"
	"kbase/internal/buildstore/storetest"
)

// TestAnswerCacheCountsTheCache: the answer cache's entries and bytes are its
// regular files'; a directory under it is no answer, and a store with no cache
// holds none.
func TestAnswerCacheCountsTheCache(t *testing.T) {
	dir := t.TempDir()
	if entries, size, err := answerCache(dir); err != nil || entries != 0 || size != 0 {
		t.Errorf("answerCache with no cache = %d entries, %d bytes, %v; want none", entries, size, err)
	}
	answers := filepath.Join(dir, ScratchDir, asks.AnswersDir)
	storetest.WriteFile(t, filepath.Join(answers, "a.json"), "xy")
	storetest.WriteFile(t, filepath.Join(answers, "b.json"), "xyz")
	if err := os.Mkdir(filepath.Join(answers, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if entries, size, err := answerCache(dir); err != nil || entries != 2 || size != 5 {
		t.Errorf("answerCache = %d entries, %d bytes, %v; want 2, 5", entries, size, err)
	}
}
