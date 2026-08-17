package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"kbase/internal/log"
)

// The two claims ARCHITECTURE.md §12 makes for `--fresh`, made falsifiable:
// it ignores all prior outputs unconditionally, and it is always sufficient —
// including over a job directory a resume refuses outright rather than
// verdicts. The crash side of the same button (fresh over the wreckage of a
// kill) lives in crash_harness_test.go, which owns the kill machinery, and the
// one incoherence --fresh cannot cure is pinned in resume_test.go beside the
// chain rule that produces it.

// TestFreshRebuildsWhatAResumeWouldReuse runs the same complete store both
// ways. The resume is the control and is what makes the fresh numbers mean
// something: over a store every unit of which is proven, a resume spends
// nothing at all, and a fresh run spends the whole corpus again.
func TestFreshRebuildsWhatAResumeWouldReuse(t *testing.T) {
	dir := t.TempDir()
	runToCompletion(t, dir, ModeResume)
	want := storeState(t, dir)

	client, res := runToCompletion(t, dir, ModeResume)
	if res.Reused != synthUnits || res.Produced != 0 {
		t.Errorf("resume over a complete store reused %d and produced %d, want %d and 0",
			res.Reused, res.Produced, synthUnits)
	}
	if client.callCount() != 0 {
		t.Errorf("resume over a complete store made %d calls, want none", client.callCount())
	}

	client, res = runToCompletion(t, dir, ModeFresh)
	if res.Reused != 0 || res.Produced != synthUnits {
		t.Errorf("fresh over the same store reused %d and produced %d, want 0 and %d",
			res.Reused, res.Produced, synthUnits)
	}
	if client.callCount() != synthCalls {
		t.Errorf("fresh made %d calls, want %d — every unit is rebuilt", client.callCount(), synthCalls)
	}
	assertSameState(t, want, storeState(t, dir), "after --fresh over a complete store")
}

// TestFreshEscapesAStoreAResumeRefuses is why --fresh discards the job
// directory rather than merely declining to read it. Each fixture below is a
// state the scan refuses outright — the store contradicting itself, which
// redoing a unit cannot fix — and each is the state the refusal points
// --fresh at. A fresh run that only overwrote the paths its chain names would
// leave the contradiction in place: the directory would still be sitting where
// the artifact belongs, and the write onto it would fail mid-run instead.
func TestFreshEscapesAStoreAResumeRefuses(t *testing.T) {
	// A leaf, so the wreckage sits behind three stages the scan proves first
	// and the refusal is reached the way a real one would be.
	const unit = "leaves/d1/two.md"

	cases := []struct {
		name  string
		wreck func(t *testing.T, dir string)
	}{
		{
			name: "a directory where an artifact belongs",
			wreck: func(t *testing.T, dir string) {
				path := filepath.Join(storeRoot(dir), filepath.FromSlash(unit))
				if err := os.Remove(path); err != nil {
					t.Fatalf("remove the artifact: %v", err)
				}
				if err := os.Mkdir(path, CreateDirMode); err != nil {
					t.Fatalf("put a directory in its place: %v", err)
				}
			},
		},
		{
			name: "a stamp from a rearranged tree",
			wreck: func(t *testing.T, dir string) {
				s := synthStore(t, dir, log.Discard())
				restamp(t, s, unit, func(st *Stamp) { st.Path = "leaves/d9/moved.md" })
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			golden := t.TempDir()
			runToCompletion(t, golden, ModeResume)
			want := storeState(t, golden)

			dir := t.TempDir()
			runToCompletion(t, dir, ModeResume)
			tc.wreck(t, dir)

			_, err := newSynthCoordinator(t, dir, echoStub(), crashWorkers, log.Discard()).
				Run(context.Background(), synthPlan(t), ModeResume)
			assertIncoherent(t, err, remedyFresh)

			runToCompletion(t, dir, ModeFresh)
			assertSameState(t, want, storeState(t, dir), "after --fresh over "+tc.name)
		})
	}
}

// TestDiscardRefusesARootThatIsNotOurs: the discard deletes a directory tree
// whole, and what makes that safe is the same fact the sweep stands on — the
// root is the temp-work directory kbase created under the output directory.
// The tripwire is one string comparison against the day someone hands this a
// root that was never ours.
func TestDiscardRefusesARootThatIsNotOurs(t *testing.T) {
	dir := t.TempDir()
	keep := filepath.Join(dir, "notes.md")
	if err := os.WriteFile(keep, []byte("mine\n"), CreateFileMode); err != nil {
		t.Fatalf("write the operator's file: %v", err)
	}

	if err := NewArtifactStore(dir, log.Discard()).discard(); err == nil {
		t.Fatal("discard emptied a directory kbase did not create")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("the refused discard removed the operator's file: %v", err)
	}
}
