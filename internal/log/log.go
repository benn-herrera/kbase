// Package log is the appliance's logging seam.
//
// ARCHITECTURE.md depends on logging in several places — monotone-safety
// fallbacks (§3), verification fallback (§5.3), the layer-3 churn tripwire
// (§7), usage-driven calibration (§8) — all of which say "and log" without
// naming a destination. This package is that destination.
//
// It is a four-method interface over log/slog rather than *slog.Logger
// itself. That is the project's one build-ahead-of-need abstraction, and it
// earns the exception: every future call site names the logger's type, so a
// backend swap that has to touch every call site is a backend that never
// gets swapped. The interface is deliberately the smallest thing that can
// carry a leveled, structured record.
//
// The logger is built once at the composition root (cmd/main.go) and passed
// to what needs it. There is no package-level default and no setter: a
// component that logs says so in its constructor signature.
package log

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Logger is what the rest of the appliance logs through.
//
// Arguments after msg are alternating key/value pairs, the log/slog
// convention:
//
//	lg.Warn("falling back to mechanical list", "stage", 3, "reason", err)
//
// An odd trailing argument is recorded as a malformed entry rather than
// dropped — slog's behaviour, kept deliberately, because a broken log call
// that is visible in the output gets fixed and one that silently loses its
// value does not.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// Level is the minimum severity a Logger emits.
//
// It is a string type so that the --log-level flag value, the parse error
// text, and this constant set are all one vocabulary; the mapping onto the
// backend's own level numbers stays private to this file.
type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// DefaultLevel is what a zero Options selects and what the --log-level flag
// defaults to. kbase is a batch appliance whose verbs already report their
// own results on stderr; anything below a warning would be diagnostics
// talking over that output. So the quiet default is the useful one, and
// detail is opt-in.
const DefaultLevel = LevelWarn

// logFileMode is the mode New ASKS FOR when it creates the log file: the
// ordinary permissive creation mode, masked by the user's umask. A log is a
// transcript the user asked for by name and will hand to someone else when
// something went wrong; kbase is a documentation tool, not a keystore (ruled
// 2026-08-14), and it never writes key material here (see config's providers
// loader, which keeps key bytes out of every record). Same number as
// atomicfile.CreateMode, declared here because log imports nothing of kbase.
const logFileMode = 0o666

// ParseLevel converts a --log-level value to a Level. Surrounding space is
// tolerated and case is not significant; empty selects DefaultLevel.
//
// Anything else is an error naming the accepted set rather than a silent
// fallback: a misspelled level that quietly reverted to the default would
// suppress exactly the output it was asked to produce.
func ParseLevel(s string) (Level, error) {
	switch l := Level(strings.ToLower(strings.TrimSpace(s))); l {
	case "":
		return DefaultLevel, nil
	case LevelDebug, LevelInfo, LevelWarn, LevelError:
		return l, nil
	default:
		return "", fmt.Errorf("unknown log level %q (want %s, %s, %s, or %s)",
			s, LevelDebug, LevelInfo, LevelWarn, LevelError)
	}
}

// slogLevel maps a validated Level onto the backend's severity scale. It is
// unexported because slog's levels are an implementation detail of the
// default backend, not part of this package's contract.
func (l Level) slogLevel() slog.Level {
	switch l {
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelWarn
	}
}

// Options configures a Logger. The zero value is valid: warnings and above,
// on stderr, no file.
type Options struct {
	// Level is the minimum severity emitted. Empty selects DefaultLevel;
	// an unrecognized value is an error from New, not a fallback.
	Level Level

	// Console is the always-on destination. Nil selects os.Stderr —
	// diagnostics never share stdout, which carries verb output a caller
	// may be parsing.
	Console io.Writer

	// FilePath, when non-empty, tees every record to that file as well as
	// to Console. The file is appended to, created if absent (at the user's
	// umask — see logFileMode). There
	// is no rotation and no size cap: a run is bounded, and a transcript
	// the user asked for by name is one they can delete.
	FilePath string
}

// New builds a Logger over log/slog and returns it with the closer owning
// whatever file it opened. On success the closer is never nil, so a caller's
// deferred Close needs no guard; with no FilePath it is a no-op.
func New(opts Options) (Logger, io.Closer, error) {
	level, err := ParseLevel(string(opts.Level))
	if err != nil {
		return nil, nil, err
	}

	console := opts.Console
	if console == nil {
		console = os.Stderr
	}

	dest, closer := console, io.Closer(nopCloser{})
	if opts.FilePath != "" {
		f, err := os.OpenFile(opts.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, logFileMode)
		if err != nil {
			return nil, nil, fmt.Errorf("open log file %s: %w", opts.FilePath, err)
		}
		dest, closer = io.MultiWriter(console, f), f
	}

	handler := slog.NewTextHandler(dest, &slog.HandlerOptions{Level: level.slogLevel()})
	return &slogLogger{l: slog.New(handler)}, closer, nil
}

// Discard returns a Logger that drops every record. It exists so a component
// that takes a Logger can be built in a test, or anywhere there is no logger
// to hand it, without a nil check in front of every call.
func Discard() Logger { return &slogLogger{l: slog.New(slog.DiscardHandler)} }

// slogLogger is the default backend: log/slog, one text handler, one writer.
//
// Console and file share the handler — and therefore the format — because a
// second handler would be a second output format to keep in sync for no
// gain. The file is a transcript of the console, not a different view of it.
type slogLogger struct{ l *slog.Logger }

func (s *slogLogger) Debug(msg string, args ...any) { s.l.Debug(msg, args...) }
func (s *slogLogger) Info(msg string, args ...any)  { s.l.Info(msg, args...) }
func (s *slogLogger) Warn(msg string, args ...any)  { s.l.Warn(msg, args...) }
func (s *slogLogger) Error(msg string, args ...any) { s.l.Error(msg, args...) }

// nopCloser is the closer New returns when it opened no file, so that the
// "never nil" contract holds without the caller branching on it.
type nopCloser struct{}

func (nopCloser) Close() error { return nil }
