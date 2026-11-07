package buildstore

import (
	"fmt"

	"kbase/internal/argmark"
	"kbase/internal/gitledger"
	"kbase/internal/result"
)

// dirtyPathsShown is how many changed paths the trail result names before it
// counts the rest.
const dirtyPathsShown = 20

// BuildTrail reads the build's commit trail and the worktree, the state store
// opened as status opens it, and returns the result naming every boundary,
// oldest first, and started-at, each with the exact command that rewinds there;
// where no build stands, none. Nothing is run: the commands are the caller's.
func BuildTrail(opts MonitorOptions) (string, []result.Field) {
	repo, kbRoot, dir, err := opts.Open()
	if err != nil {
		return MonitorEnd(nil, err)
	}
	fields := []result.Field{{Key: "kb-root", Value: kbRoot}, {Key: "state-dir", Value: dir}}
	head, err := repo.Head()
	if err != nil {
		return MonitorEnd(fields, err)
	}
	fields = append(fields, result.Field{Key: "head", Value: Nullable(head)})
	all, err := repo.Trail()
	if err != nil {
		return MonitorEnd(fields, err)
	}
	read, err := readStore(dir)
	if err != nil {
		return MonitorEnd(fields, err)
	}
	none, err := noBuild(repo, kbRoot, read.live, all)
	if err != nil {
		return MonitorEnd(fields, err)
	}
	if none {
		all = nil
	}
	trail := CurrentTrail(all)
	var recorded []string
	if read.last != nil {
		recorded = read.last[0].Resume
	}
	fields = append(fields, result.Field{Key: "boundaries", Value: trailBoundaries(trail, recorded)})
	var started any
	if _, startedAt := StartedAt(trail); startedAt != "" {
		started = result.Record{{Key: "commit", Value: startedAt}, {Key: "command", Value: []string{resetCommand(startedAt)}}}
	}
	fields = append(fields, result.Field{Key: "started-at", Value: started})
	// The trail's own reading sees only committed stages, so this names a gap
	// in walk order and nothing else.
	if items := TrailRefusal(repo.Root, RecordedStages(trail)); len(items) > 0 {
		return MonitorEnd(fields, items)
	}
	dirty, err := repo.DirtyTree()
	if err != nil {
		return MonitorEnd(fields, err)
	}
	return result.Done, append(fields, result.Field{Key: "dirty", Value: dirtyField(dirty)})
}

// resetCommand is the reset to commit: git reset --keep, which refuses rather
// than discarding uncommitted changes in the files the reset touches.
func resetCommand(commit string) string { return "git reset --keep " + commit }

// trailBoundaries is the trail's boundaries as the result lists them, oldest
// first, each with its command: the reset to it, then the build resumed from
// it as the run recording resume launched it, where resume reads as a kbase
// build invocation, else the bare resume.
func trailBoundaries(trail []gitledger.Entry, resume []string) []result.Record {
	command, confirmed := RecordedCommand(resume)
	if !confirmed {
		command = argmark.Call("build", map[string]any{"resume": true})
	}
	records := make([]result.Record, len(trail))
	for i := range trail {
		b := trail[len(trail)-1-i]
		records[i] = result.Record{
			{Key: "stage", Value: b.Stage}, {Key: "commit", Value: b.Commit}, {Key: "subject", Value: b.Subject},
			{Key: "command", Value: []string{resetCommand(b.Commit), command}},
		}
	}
	return records
}

// dirtyField is the result's dirty value: false where the worktree is clean,
// else the changed paths, the first dirtyPathsShown named and the rest counted.
func dirtyField(paths []string) any {
	if len(paths) == 0 {
		return false
	}
	shown := append([]string{}, paths...)
	if rest := len(shown) - dirtyPathsShown; rest > 0 {
		shown = append(shown[:dirtyPathsShown], fmt.Sprintf("+%d more", rest))
	}
	return shown
}
