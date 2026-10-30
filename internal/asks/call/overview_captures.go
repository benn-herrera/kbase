//go:build kbasedev

package call

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"kbase/internal/asks"
)

// OverviewCapture is one overview call as its capture file recorded it: the
// inputs its prompt was rendered from, the digest of that prompt, and how its
// last request ended.
type OverviewCapture struct {
	Path     string
	Attempt  int
	Excerpts string
	Rejected []string
	// HasInputs is false for a capture that recorded no excerpts.
	HasInputs    bool
	PromptSHA256 string
	// Outcome and Reply are the last attempt line's; Outcome is "" where the
	// file has no attempt line.
	Outcome string
	Reply   string
}

// ReadOverviewCaptures reads the overview captures of the trail directory
// trail, ordered by ask attempt, then by path. A trail with no captures
// directory has none.
func ReadOverviewCaptures(trail string) ([]OverviewCapture, error) {
	paths, err := filepath.Glob(filepath.Join(trail, asks.CapturesDir, overviewName+"-*.capture.jsonl"))
	if err != nil {
		return nil, err
	}
	caps := make([]OverviewCapture, 0, len(paths))
	for _, path := range paths {
		c, err := readOverviewCapture(path)
		if err != nil {
			return nil, err
		}
		caps = append(caps, c)
	}
	slices.SortFunc(caps, func(a, b OverviewCapture) int {
		return cmp.Or(cmp.Compare(a.Attempt, b.Attempt), cmp.Compare(a.Path, b.Path))
	})
	return caps, nil
}

func readOverviewCapture(path string) (OverviewCapture, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return OverviewCapture{}, err
	}
	c := OverviewCapture{Path: path}
	for i, line := range bytes.Split(b, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if i == 0 {
			var req captureRequest
			if err := json.Unmarshal(line, &req); err != nil {
				return OverviewCapture{}, fmt.Errorf("%s:%d: %w", path, i+1, err)
			}
			c.Attempt, c.PromptSHA256, c.Rejected = req.Attempt, req.PromptSHA256, req.Rejected
			if req.Excerpts != nil {
				c.Excerpts, c.HasInputs = *req.Excerpts, true
			}
			continue
		}
		var att captureAttempt
		if err := json.Unmarshal(line, &att); err != nil {
			return OverviewCapture{}, fmt.Errorf("%s:%d: %w", path, i+1, err)
		}
		if att.Type == "attempt" {
			c.Outcome, c.Reply = att.Outcome, att.Reply
		}
	}
	return c, nil
}
