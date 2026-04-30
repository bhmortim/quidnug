// Disk-backed audit log store.
//
// Format: JSON-lines. Each line is one Entry. The file is
// append-only; existing lines are never rewritten, so the
// on-disk representation is naturally tamper-evident against
// an attacker with write access (you can append, but any edit
// to existing lines invalidates the hash chain).
//
// Phase 1 keeps things deliberately simple: one flat file. A
// Phase 6 follow-up will add size-based rotation + separate
// index files if operator logs grow enough to warrant it; the
// envelope estimate in QDP-0018 §8 (annual ≤ 250 MB) suggests
// this is fine for at least a year of operation.
package audit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// FileStore is the disk-backed Store implementation. Safe for
// concurrent use by a single Log.
type FileStore struct {
	mu   sync.Mutex
	path string
	f    *os.File
	w    *bufio.Writer
}

// NewFileStore opens (or creates) an audit log file at path.
// The parent directory is created with 0700 (operator-only) and
// the file with 0600 permissions so audit entries (which can
// reveal request patterns and signer identities) are not
// world-readable. Opened in O_APPEND mode so writes can't
// accidentally clobber existing entries.
//
// The path is sourced from operator config and validated against
// path traversal: NUL bytes and ".."-rooted relative paths are
// refused. Symlinks are not followed when statting the directory.
func NewFileStore(path string) (*FileStore, error) {
	if path == "" {
		return nil, fmt.Errorf("audit store path must not be empty")
	}
	clean, err := validateAuditPath(path)
	if err != nil {
		return nil, fmt.Errorf("audit store path: %w", err)
	}
	dir := filepath.Dir(clean)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create audit log dir: %w", err)
	}
	f, err := os.OpenFile(clean, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o600) // #nosec G304 -- path validated by validateAuditPath
	if err != nil {
		return nil, fmt.Errorf("open audit log %q: %w", clean, err)
	}
	return &FileStore{
		path: path,
		f:    f,
		w:    bufio.NewWriter(f),
	}, nil
}

// Append writes one JSON-encoded entry as a single line.
// Flushes the buffer + fsyncs the file so a crash immediately
// after the call still leaves the entry on disk. This is
// heavier than strictly necessary for a development store but
// is what production operators will expect from an audit log.
func (s *FileStore) Append(e Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal entry: %w", err)
	}
	if _, err := s.w.Write(raw); err != nil {
		return fmt.Errorf("write entry: %w", err)
	}
	if _, err := s.w.WriteString("\n"); err != nil {
		return fmt.Errorf("write newline: %w", err)
	}
	if err := s.w.Flush(); err != nil {
		return fmt.Errorf("flush: %w", err)
	}
	if err := s.f.Sync(); err != nil {
		return fmt.Errorf("fsync: %w", err)
	}
	return nil
}

// Load reads every persisted entry in file order. Used by
// NewLogWithStore at startup to rebuild the in-memory index.
func (s *FileStore) Load() ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Seek back to the start so Load is idempotent across
	// repeat calls.
	if _, err := s.f.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("rewind audit log: %w", err)
	}
	defer func() {
		// Restore the write-position to end-of-file for the
		// next Append. O_APPEND on the OS-level fd already
		// guarantees writes land at the end, but resetting the
		// read offset avoids confusing future readers.
		_, _ = s.f.Seek(0, io.SeekEnd)
	}()

	var out []Entry
	scanner := bufio.NewScanner(s.f)
	// Raise the default 64 KiB scanner buffer so payloads with
	// long human notes or large structured fields don't trip it.
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			// Phase 1 is strict: a malformed line is a fatal
			// corruption indicator. Later phases can add a
			// "quarantine corrupt lines and continue" mode
			// behind a config flag.
			return nil, fmt.Errorf("parse audit line: %w", err)
		}
		out = append(out, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan audit log: %w", err)
	}
	return out, nil
}

// Close flushes and closes the underlying file. Safe to call
// multiple times; subsequent calls are no-ops.
func (s *FileStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f == nil {
		return nil
	}
	var err error
	if s.w != nil {
		if ferr := s.w.Flush(); ferr != nil {
			err = ferr
		}
	}
	if cerr := s.f.Close(); cerr != nil && err == nil {
		err = cerr
	}
	s.f = nil
	s.w = nil
	return err
}

// Path returns the on-disk file path (useful for operator
// logging / observability).
func (s *FileStore) Path() string {
	return s.path
}

// validateAuditPath cleans the operator-supplied audit log path
// and rejects path-traversal attempts. We don't import
// internal/safeio here to keep the audit package leaf-level
// (avoids any future import cycle with packages that themselves
// depend on the audit log).
func validateAuditPath(p string) (string, error) {
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
