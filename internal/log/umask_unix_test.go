//go:build unix

package log

import (
	"io/fs"
	"syscall"
	"testing"
)

// testUmask is the mask the mode tests run under. It is deliberately not a
// common value: under it the ruled creation modes land on 0640 and 0750, so
// neither a hard-coded 0644/0755 nor a hard-coded 0600/0700 can pass by
// coincidence.
const testUmask fs.FileMode = 0o027

// setTestUmask installs testUmask for the duration of the calling test and
// returns it, so an expectation reads `mode &^ mask` — the same arithmetic
// the kernel does at create time.
//
// Call it BEFORE the code under test creates anything: a umask applies when
// a file is made and nothing revisits it afterwards. The process umask is
// global state, which is safe here only because no test in this package runs
// in parallel with another.
func setTestUmask(t *testing.T) fs.FileMode {
	t.Helper()
	prev := syscall.Umask(int(testUmask))
	t.Cleanup(func() { syscall.Umask(prev) })
	return testUmask
}
