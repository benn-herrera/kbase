package build

import (
	"fmt"
	"slices"

	"kbase/internal/ledger"
	"kbase/internal/result"
)

// dirtyPathsShown is how many changed paths the trail document names before it
// counts the rest.
const dirtyPathsShown = 20

// unresolvedCommit stands in the reset command where no target names the
// commit to reset to.
const unresolvedCommit = "<sha>"

// RewindOptions is one read of the build's commit trail: the boundaries a
// build recorded and, for the boundary a target names, the commands that reset
// to it and continue the build.
type RewindOptions struct {
	// RepoRoot is where the search for the repository root starts.
	RepoRoot string
	// StateDir overrides the state store's location.
	StateDir string
	// Target is the boundary to reset to, by stage id or display name.
	Target string
}

// Rewind reads the commit trail, the worktree and the state store, and returns
// the document naming every boundary and the commands that resume from one.
// Nothing is written: the reset and the re-issue are the caller's to run.
func Rewind(opts RewindOptions) (string, []result.Field) {
	repo, kbRoot, dir, err := MonitorOptions{StateDir: opts.StateDir, WorkDir: opts.RepoRoot}.open()
	if err != nil {
		return monitorEnd(nil, err)
	}
	fields := []result.Field{{Key: "kb-root", Value: kbRoot}, {Key: "state-dir", Value: dir}}
	head, err := repo.Head()
	if err != nil {
		return monitorEnd(fields, err)
	}
	fields = append(fields, result.Field{Key: "head", Value: nullable(head)})
	trail, err := repo.Trail()
	if err != nil {
		return monitorEnd(fields, err)
	}
	fields = append(fields, result.Field{Key: "boundaries", Value: trailBoundaries(trail)})
	// An out-of-order trail is refused as build refuses it. The trail's own
	// reading sees only committed stages, so this names a gap in walk order and
	// nothing else.
	if items := trailRefusal(repo.Root, (&walk{trail: trail}).recorded()); len(items) > 0 {
		return monitorEnd(fields, items)
	}
	dirty, err := repo.DirtyTree()
	if err != nil {
		return monitorEnd(fields, err)
	}
	fields = append(fields, result.Field{Key: "dirty", Value: dirtyField(dirty)})
	at := -1
	if opts.Target != "" {
		if at = targetBoundary(opts.Target, trail); at < 0 {
			return monitorEnd(fields, targetRefusal(opts.Target, trail))
		}
	}
	run, ended, err := standingRun(dir)
	if err != nil {
		return monitorEnd(fields, err)
	}
	reset := "git reset --hard " + unresolvedCommit
	if at >= 0 {
		reset = "git reset --hard " + trail[at].Commit
	}
	commands := []string{reset}
	command, confirmed := recordedCommand(run.Resume)
	if confirmed {
		commands = append(commands, command)
	}
	fields = append(fields, result.Field{Key: "commands", Value: commands})
	if at >= 0 {
		fields = append(fields, result.Field{Key: "target", Value: result.Record{
			{Key: "stage", Value: trail[at].Stage}, {Key: "commit", Value: trail[at].Commit},
			{Key: "discards", Value: at},
		}})
	}
	if note := resumeNote(dir, run.Resume, ended); note != "" {
		fields = append(fields, result.Field{Key: "resume-note", Value: note})
	}
	return result.Done, fields
}

// trailBoundaries is the trail's boundaries as the document lists them, oldest
// first.
func trailBoundaries(trail []ledger.Entry) []result.Record {
	records := make([]result.Record, len(trail))
	for i := range trail {
		b := trail[len(trail)-1-i]
		records[i] = result.Record{
			{Key: "stage", Value: b.Stage}, {Key: "commit", Value: b.Commit}, {Key: "subject", Value: b.Subject},
		}
	}
	return records
}

// targetBoundary is the trail's index of the boundary a target names, -1 where
// it names none: a target resolves as --through resolves it, and names the
// newest boundary of that stage.
func targetBoundary(target string, trail []ledger.Entry) int {
	s, ok := ResolveStage(target)
	if !ok {
		return -1
	}
	return slices.IndexFunc(trail, func(e ledger.Entry) bool { return e.Stage == s.ID })
}

// targetRefusal is the usage refusal naming a target no boundary carries, the
// boundary ids the trail holds as the closed set it comes from.
func targetRefusal(target string, trail []ledger.Entry) result.Refusal {
	ids := make([]string, len(trail))
	for i, e := range trail {
		ids[len(trail)-1-i] = e.Stage
	}
	return result.Refusal{{Check: checkUsage, Key: "target", Allowed: ids,
		Detail: fmt.Sprintf("target %q names no boundary the trail holds, by stage id or display name", target)}}
}

// dirtyField is the document's dirty value: false where the worktree is clean,
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

// standingRun is the newest invocation the store at dir holds that the build
// stands at, and the outcome it ended with: a run that refused or asked to be
// retried without recording a boundary left nothing to continue, so the run
// before it is the one read.
func standingRun(dir string) (event, string, error) {
	events, err := readProgress(dir)
	if err != nil {
		return event{}, "", err
	}
	all := invocations(events)
	for i := len(all) - 1; i >= 0; i-- {
		if all[i].stands() {
			return all[i][0], all[i].end().Outcome, nil
		}
	}
	return event{}, "", nil
}

// resumeNote is the document's one sentence where its commands need
// explaining: why no resume line is among them, or why a bounded run's is.
func resumeNote(dir string, argv []string, ended string) string {
	_, confirmed := recordedCommand(argv)
	switch {
	case argv == nil:
		return fmt.Sprintf("the state store %s records no build a re-issue continues, so there is no resume command to print", dir)
	case !confirmed:
		return fmt.Sprintf("the state store %s records a command that could not be confirmed as a kbase build invocation, so it is not printed", dir)
	case ended == result.Bounded:
		return "this build ended bounded; its recorded command is printed here because after a reset to an earlier boundary there are stages left to walk"
	}
	return ""
}
