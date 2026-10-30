package buildstore

import (
	"strings"

	"kbase/internal/gitledger"
)

// The lines a boundary's body records the build by: every boundary ends with
// the inputs, one line per path, and the start boundary also records the
// commit the build began from.
const (
	volumeRootField   = "volume-root:"
	bibliographyField = "bibliography:"
	startedAtField    = "started-at:"
)

// Inputs is what a build is built from, each list in the order the build
// reads it. Bibliographies is nil where the build was given none, so the
// bibliographies beside the volume roots are discovered.
type Inputs struct{ VolumeRoots, Bibliographies []string }

// Body is the lines a boundary's body ends with: started-at where startedAt
// is not empty, then a volume-root line per volume root, then a bibliography
// line per bibliography.
func (in Inputs) Body(startedAt string) string {
	var lines []string
	if startedAt != "" {
		lines = append(lines, startedAtField+" "+startedAt)
	}
	for _, p := range in.VolumeRoots {
		lines = append(lines, volumeRootField+" "+p)
	}
	for _, p := range in.Bibliographies {
		lines = append(lines, bibliographyField+" "+p)
	}
	return strings.Join(lines, "\n")
}

// RecordedInputs is the inputs a boundary's body records, in the order
// recorded; ok is false where it records none, as a boundary written before
// bodies carried them does not.
func RecordedInputs(body string) (in Inputs, ok bool) {
	for _, line := range strings.Split(body, "\n") {
		if p, found := strings.CutPrefix(line, volumeRootField+" "); found {
			in.VolumeRoots = append(in.VolumeRoots, p)
		} else if p, found := strings.CutPrefix(line, bibliographyField+" "); found {
			in.Bibliographies = append(in.Bibliographies, p)
		}
	}
	return in, len(in.VolumeRoots) > 0
}

// StartedAt is the trail's newest start boundary, the zero Entry where it
// holds none, and the commit that boundary records the build began from, ""
// where it records none.
func StartedAt(trail []gitledger.Entry) (start gitledger.Entry, startedAt string) {
	for _, e := range trail {
		if e.Stage != StageStart {
			continue
		}
		for _, line := range strings.Split(e.Body, "\n") {
			if c, found := strings.CutPrefix(line, startedAtField+" "); found {
				return e, c
			}
		}
		return e, ""
	}
	return gitledger.Entry{}, ""
}
