package score

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/query"
	"kbase/internal/result"
)

// keysOf is the top-level keys of one JSON object line, in the order written.
func keysOf(t *testing.T, line []byte) []string {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(line))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		t.Fatalf("%q opens no object: %v", line, err)
	}
	var keys []string
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, tok.(string))
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			t.Fatal(err)
		}
	}
	return keys
}

// rawLines is a JSONL file's lines, unparsed; a file that does not stand is none.
func rawLines(t *testing.T, path string) [][]byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return bytes.Split(bytes.TrimSuffix(b, []byte{'\n'}), []byte{'\n'})
}

// TestRunCapturesEveryCallItMakes: a landing, a parse refusal, a failed call and
// a refused write each leave one capture, its keys in the record's order, its
// reply what the seam returned and its error the attempt's reason; the
// parse-refused capture carries the reply text that its ledger line does not.
func TestRunCapturesEveryCallItMakes(t *testing.T) {
	src, ix, set := runSet(t)
	// Two lines, so the reason the parse gives — which quotes the line it
	// refused — is not the reply whole.
	const malformed = "0.5, roughly.\nThe paragraph is thin."
	callErr := errors.New("the provider closed the connection")
	writeErr := errors.New("the register changed under the write")
	identityKeys := []string{"run", "pass", "kind", "id", "class", "text", "lines", "prompt", "prompt_sha256", "reply"}
	tail := []string{"model", "backend", "started", "ended"}
	for _, tc := range []struct {
		name      string
		reply     string
		err       error
		write     error
		wantReply string
		wantError string
	}{
		{name: "a landing", reply: "none", wantReply: "none"},
		{name: "a parse refusal", reply: malformed, wantReply: malformed, wantError: "no answer"},
		{name: "a failed call", err: callErr, wantReply: "", wantError: callErr.Error()},
		{name: "a refused write", reply: "none", write: writeErr, wantReply: "none", wantError: writeErr.Error()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := kept(t, t.TempDir())
			ask, _ := asking(t, []asked{{title: "1| An Analysis", reply: tc.reply, err: tc.err}})
			write := func(query.Item, kb.Bearing) error { return tc.write }
			if _, err := (&Runner{Source: src, Index: ix, Items: set[:1], Pass: query.PassRigor, Ask: ask, Write: write,
				Ledger: l, Model: "small"}).Run(context.Background()); err != nil {
				t.Fatal(err)
			}
			captures := rawLines(t, filepath.Join(l.Dir(), capturesFile))
			if len(captures) != 1 {
				t.Fatalf("the run left %d captures, want one", len(captures))
			}
			want := slices.Clone(identityKeys)
			if tc.wantError != "" {
				want = append(want, "error")
			}
			want = append(want, tail...)
			if got := keysOf(t, captures[0]); !slices.Equal(got, want) {
				t.Errorf("the capture's keys = %q, want %q", got, want)
			}
			var rec captureRecord
			if err := json.Unmarshal(captures[0], &rec); err != nil {
				t.Fatal(err)
			}
			line := lines(t, filepath.Join(l.Dir(), linesFile))[0]
			if rec.Reply != tc.wantReply || !strings.Contains(rec.Error, tc.wantError) {
				t.Errorf("the capture records reply %q, error %q; want %q and an error naming %q", rec.Reply, rec.Error, tc.wantReply, tc.wantError)
			}
			if tc.wantError != "" && rec.Error != line["reply"] {
				t.Errorf("the capture's error %q is not the reason its line records, %q", rec.Error, line["reply"])
			}
			if rec.PromptSHA256 != line["prompt_sha256"] || rec.PromptSHA256 != digestOf(rec.Prompt) || rec.Class != methodClass.name {
				t.Errorf("the capture's digest %q, class %q; want the line's %v, its prompt's digest, and %q",
					rec.PromptSHA256, rec.Class, line["prompt_sha256"], methodClass.name)
			}
			if lineReply, _ := line["reply"].(string); tc.reply == malformed && strings.Contains(lineReply, malformed) {
				t.Errorf("the parse-refused ledger line carries the reply text: %v", line)
			}
		})
	}
}

// TestRunCapturesNoCallItDidNotFinish: a call the run's cancellation interrupts
// leaves neither a capture nor a ledger line, and a text the guard refuses makes
// no call and leaves no capture.
func TestRunCapturesNoCallItDidNotFinish(t *testing.T) {
	src, ix, set := runSet(t)
	t.Run("a cancelled call", func(t *testing.T) {
		l := kept(t, t.TempDir())
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var calls int
		ask := func(context.Context, string) (string, error) {
			calls++
			if calls == 2 {
				cancel()
				return "", context.Canceled
			}
			return "none", nil
		}
		write, _ := writing()
		res, err := (&Runner{Source: src, Index: ix, Items: set[:3], Pass: query.PassRigor, Ask: ask, Write: write,
			Ledger: l, Model: "small"}).Run(ctx)
		if err != nil || res.Outcome != result.Cancelled {
			t.Fatalf("outcome %q, %v; want cancelled", res.Outcome, err)
		}
		captures := rawLines(t, filepath.Join(l.Dir(), capturesFile))
		if len(captures) != 1 || len(lines(t, filepath.Join(l.Dir(), linesFile))) != 1 {
			t.Fatalf("the run left %d captures, want the one item that finished and nothing of the one cancelled", len(captures))
		}
		var rec captureRecord
		if err := json.Unmarshal(captures[0], &rec); err != nil {
			t.Fatal(err)
		}
		if rec.ID != set[0].Node.ID {
			t.Errorf("the capture names %s, want the finished %s", rec.ID, set[0].Node.ID)
		}
	})
	t.Run("a refused text", func(t *testing.T) {
		l := kept(t, t.TempDir())
		ask := func(context.Context, string) (string, error) {
			t.Fatal("a refused text made a call")
			return "", nil
		}
		write, _ := writing()
		items := []query.Item{node(kb.NodeKindClaim, "clm-dddddd")}
		if _, err := (&Runner{Source: src, Index: ix, Items: items, Pass: query.PassRigor, Ask: ask, Write: write,
			Ledger: l, Model: "small"}).Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(l.Dir(), capturesFile)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("a refused text left a capture file: %v", err)
		}
	})
}

// TestRunFailsWhereACaptureCannotBeWritten: a capture that cannot be written
// fails the run as a ledger line that cannot be written does, and the item's
// line, which follows its capture, is not written.
func TestRunFailsWhereACaptureCannotBeWritten(t *testing.T) {
	src, ix, set := runSet(t)
	l := kept(t, t.TempDir())
	if err := os.Mkdir(filepath.Join(l.Dir(), capturesFile), 0o777); err != nil {
		t.Fatal(err)
	}
	ask, _ := asking(t, []asked{{title: "1| An Analysis", reply: "none"}})
	write, _ := writing()
	res, err := (&Runner{Source: src, Index: ix, Items: set[:2], Pass: query.PassRigor, Ask: ask, Write: write,
		Ledger: l, Model: "small"}).Run(context.Background())
	if err == nil || res.Outcome != result.Failed {
		t.Fatalf("outcome %q, %v; want the run failed on the capture", res.Outcome, err)
	}
	if _, err := os.Stat(filepath.Join(l.Dir(), linesFile)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the item's ledger line was written behind a capture that was not: %v", err)
	}
}
