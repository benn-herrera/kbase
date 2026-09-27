//go:build !unix

package config

import (
	"io/fs"
	"testing"
)

// setTestUmask is the no-op twin of the unix version: there is no umask to
// set, and the modes a stat reports are synthesized by the platform rather
// than stored. The tests that call it skip on those platforms; this
// declaration exists so the package still builds there.
func setTestUmask(t *testing.T) fs.FileMode {
	t.Helper()
	return 0
}
