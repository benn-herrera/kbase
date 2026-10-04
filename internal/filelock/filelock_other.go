//go:build !unix

package filelock

import "os"

// Where the platform has no flock nothing is locked and nothing reads as
// held: two writers to one register are separated only by the store's
// read-then-compare check, and a build's liveness is not observable.
func tryExclusive(*os.File) (bool, error) { return false, nil }

func probe(*os.File) (bool, error) { return false, nil }

func unlock(*os.File) {}
