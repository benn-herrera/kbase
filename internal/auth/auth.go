// Package auth is kbase's credentials store: one file per provider, holding
// that provider's API key sealed with AES-GCM under a per-user key. The
// sealing is obfuscation, not a boundary — anyone running as the same user
// can recover the key; a reader of the file gets ciphertext. Separation
// between users is the file's mode.
package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"kbase/internal/atomicfile"
	"kbase/internal/secret"
)

// DirName is the store's directory under kbase's state base.
const DirName = "auth"

// FileSuffix ends every key file's name: <provider>-key.enc.
const FileSuffix = "-key.enc"

// FileMode is every key file's permission, on creation and on every replace.
const FileMode fs.FileMode = 0o600

// DirMode is the store directory's permission, set on every Add whether Add
// creates the directory or finds it standing.
const DirMode fs.FileMode = 0o700

// sourceKey is the secret every user's key is derived from. It is a plain
// constant in public source on purpose: a key chosen at build time would leave
// each release unable to read the last one's store, and disguising a constant
// that ships in the source hides nothing.
const sourceKey = "\x50\x4a\xe1\x01\x1b\x08\x6d\x85\x27\x5d\xc3\xa0\xb4\xb7\xdf\x90" +
	"\xbb\x61\x2f\x38\xe2\x5c\x20\x2f\x3b\x69\x54\xcb\x3b\xb2\x69\xa7"

var (
	// ErrTruncated is a key file shorter than any sealed key.
	ErrTruncated = errors.New("too short to hold a stored key: the file is truncated")
	// ErrDoesNotOpen is a key file that does not open under the current user's
	// encryption key: altered since it was written, or written for another user.
	// Nothing in the file tells the two apart.
	ErrDoesNotOpen = errors.New("does not decrypt under the current OS user's encryption key: the file was altered, or written under another user account")
	// ErrNoUsername is an OS that supplies no username to derive the key from.
	ErrNoUsername = errors.New("the OS reports no username for the current user, from which the store's encryption key is derived")
)

// FileError is a failure of one key file: the file's path and what went
// wrong, never the file's content.
type FileError struct {
	Path string
	Err  error
}

func (e *FileError) Error() string { return e.Path + ": " + e.Err.Error() }
func (e *FileError) Unwrap() error { return e.Err }

// NamePattern is what a provider name matches, and NameRule says it in words:
// the rule internal/config holds a providers.toml entry's name to, stated
// again because this package imports no config; a test pins the two equal.
// No name it admits is `.` or `..` or holds a path separator.
const (
	NamePattern = `^[A-Za-z0-9_][A-Za-z0-9_-]*$`
	NameRule    = `a provider name is one or more ASCII letters, digits, "_" or "-", not beginning with "-"`
)

var validName = regexp.MustCompile(NamePattern)

// NameError refuses a provider name that breaks the rule. Its text is the rule
// alone, never the name.
type NameError struct{ Name, Detail string }

func (e *NameError) Error() string { return e.Detail }

// CheckName refuses a provider name that breaks the rule. The name arrives
// from the user, so it is checked before any path is built from it.
func CheckName(name string) error {
	if !validName.MatchString(name) {
		return &NameError{Name: name, Detail: NameRule}
	}
	return nil
}

// Store is the key files in one directory, sealed for the current user.
type Store struct {
	dir      string
	username func() (string, error)
}

// NewStore is the store in dir, which Add creates where it does not stand.
func NewStore(dir string) Store { return Store{dir: dir, username: currentUsername} }

func currentUsername() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrNoUsername, err)
	}
	return u.Username, nil
}

// Add stores key for the provider name, replacing any key stored for it.
func (s Store) Add(name string, key secret.Key) error {
	path, err := s.path(name)
	if err != nil {
		return err
	}
	aead, err := s.sealer()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, DirMode); err != nil {
		return err
	}
	if err := os.Chmod(s.dir, DirMode); err != nil {
		return err
	}
	return atomicfile.WriteMode(path, aead.Seal(nil, nil, []byte(key.Reveal()), nil), FileMode)
}

// Read is the key stored for the provider name. A name with no file is a
// *FileError matching fs.ErrNotExist; a file that does not open is one
// wrapping ErrTruncated or ErrDoesNotOpen.
func (s Store) Read(name string) (secret.Key, error) {
	path, err := s.path(name)
	if err != nil {
		return secret.Key{}, err
	}
	aead, err := s.sealer()
	if err != nil {
		return secret.Key{}, err
	}
	sealed, err := os.ReadFile(path)
	if err != nil {
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			err = pathErr.Err
		}
		return secret.Key{}, &FileError{Path: path, Err: err}
	}
	if len(sealed) < aead.Overhead() {
		return secret.Key{}, &FileError{Path: path, Err: ErrTruncated}
	}
	key, err := aead.Open(nil, nil, sealed, nil)
	if err != nil {
		return secret.Key{}, &FileError{Path: path, Err: ErrDoesNotOpen}
	}
	return secret.NewKey(string(key)), nil
}

// Remove clears the key stored for the provider name, reporting whether one
// stood.
func (s Store) Remove(name string) (bool, error) {
	path, err := s.path(name)
	if err != nil {
		return false, err
	}
	if err := os.Remove(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// List names the providers with a key file, in lowercase and in name order. A
// directory that does not stand holds none; a file whose name no lowercase
// provider name yields is not a key file, nor is anything but a regular file
// or a symbolic link to one.
func (s Store) List() ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), FileSuffix)
		if ok && CheckName(name) == nil && name == strings.ToLower(name) && s.isRegular(e) {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names, nil
}

// isRegular reports whether the entry is a regular file, following a symbolic
// link to its target; a link whose target does not stand is not.
func (s Store) isRegular(e fs.DirEntry) bool {
	if e.Type()&fs.ModeSymlink == 0 {
		return e.Type().IsRegular()
	}
	info, err := os.Stat(filepath.Join(s.dir, e.Name()))
	return err == nil && info.Mode().IsRegular()
}

// path is the provider name's key file, once the name is checked: the name
// lowercased, so names differing only by case share one file, whatever the
// filesystem's own case rule.
func (s Store) path(name string) (string, error) {
	if err := CheckName(name); err != nil {
		return "", err
	}
	return filepath.Join(s.dir, strings.ToLower(name)+FileSuffix), nil
}

// sealer is AES-256-GCM under the current user's key, derived by HKDF from
// sourceKey with the username as its info, each seal under a fresh random
// nonce carried in the sealed bytes. The provider's name is bound nowhere, so
// a file renamed by hand opens under its new name.
func (s Store) sealer() (cipher.AEAD, error) {
	name, err := s.username()
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, ErrNoUsername
	}
	key, err := hkdf.Key(sha256.New, []byte(sourceKey), nil, name, len(sourceKey))
	if err != nil {
		return nil, fmt.Errorf("derive the store's encryption key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("build the store's cipher: %w", err)
	}
	aead, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return nil, fmt.Errorf("build the store's cipher: %w", err)
	}
	return aead, nil
}
