// Package safeio provides path-traversal defenses for file paths
// sourced from operator configuration, CLI arguments, or other
// untrusted-by-default inputs. The helpers wrap the standard
// library so call sites stay one-line and security scanners
// (gosec G304, CodeQL path-injection) can verify that the
// dangerous sink is gated by validation.
//
// Policy enforced by ValidatePath:
//
//   - Reject paths containing a NUL byte (defense against any
//     cgo/syscall path that doesn't honor the Go string length).
//   - Clean the path to collapse "." and ".." segments.
//   - Reject relative paths whose head is ".." (would escape the
//     working directory). Absolute paths are allowed; the caller
//     chose the root.
//
// ReadFile additionally rejects symlinks and non-regular files
// (sockets, pipes, devices) so an attacker who can plant entries
// in a known directory can't redirect a config-file read into
// /dev/random or a process pipe.
package safeio

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidatePath canonicalizes a user-supplied path and rejects
// inputs that try to traverse out of the working directory.
// Returns the cleaned path or an error explaining the rejection.
func ValidatePath(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("path is empty")
	}
	if strings.ContainsRune(p, 0) {
		return "", fmt.Errorf("path contains NUL byte")
	}
	clean := filepath.Clean(p)
	if !filepath.IsAbs(clean) {
		first := strings.SplitN(clean, string(filepath.Separator), 2)[0]
		if first == ".." {
			return "", fmt.Errorf("path escapes working directory: %s", p)
		}
	}
	return clean, nil
}

// ReadFile is a drop-in replacement for os.ReadFile that runs the
// input through ValidatePath first and rejects symlinks and
// non-regular files.
func ReadFile(p string) ([]byte, error) {
	clean, err := ValidatePath(p)
	if err != nil {
		return nil, err
	}
	st, err := os.Lstat(clean)
	if err != nil {
		return nil, err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("refusing to read symlink: %s", clean)
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("refusing to read non-regular file: %s", clean)
	}
	return os.ReadFile(clean) // #nosec G304 -- path validated by ValidatePath
}

// WriteFile is a drop-in replacement for os.WriteFile that runs
// the input through ValidatePath first and writes with 0600
// permissions by default. Callers that need a specific mode can
// use WriteFileMode.
func WriteFile(p string, data []byte) error {
	return WriteFileMode(p, data, 0o600)
}

// WriteFileMode is like WriteFile but allows the caller to specify
// the permission bits. Callers should pass 0600 or stricter for
// any file that may contain secrets.
func WriteFileMode(p string, data []byte, perm os.FileMode) error {
	clean, err := ValidatePath(p)
	if err != nil {
		return err
	}
	return os.WriteFile(clean, data, perm) // #nosec G304 -- path validated by ValidatePath
}

// MkdirAll is a drop-in replacement for os.MkdirAll that runs the
// input through ValidatePath first and uses 0750 by default. Use
// MkdirAllMode for other modes.
func MkdirAll(p string) error {
	return MkdirAllMode(p, 0o750)
}

// MkdirAllMode is like MkdirAll but allows the caller to specify
// the permission bits. Pass 0750 or stricter for directories that
// may hold sensitive files.
func MkdirAllMode(p string, perm os.FileMode) error {
	clean, err := ValidatePath(p)
	if err != nil {
		return err
	}
	return os.MkdirAll(clean, perm)
}
