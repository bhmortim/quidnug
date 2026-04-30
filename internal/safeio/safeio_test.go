package safeio

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestValidatePath(t *testing.T) {
	// Cases that must be rejected.
	bad := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"NUL byte", "foo\x00bar"},
		{"relative traversal", "../etc/passwd"},
		{"relative dotdot deep", "../../../../../../etc/shadow"},
		{"current dir then dotdot", "./../escape"},
	}
	for _, tc := range bad {
		tc := tc
		t.Run("reject_"+tc.name, func(t *testing.T) {
			if _, err := ValidatePath(tc.in); err == nil {
				t.Fatalf("expected error for input %q", tc.in)
			}
		})
	}

	// Cases that must be accepted (and idempotently cleaned).
	good := []struct {
		name string
		in   string
		want string
	}{
		{"absolute simple", "/tmp/foo", filepath.Clean("/tmp/foo")},
		{"absolute with dot", "/tmp/./foo", filepath.Clean("/tmp/foo")},
		{"relative simple", "config.yaml", "config.yaml"},
		{"relative subdir", "data/pending.json", filepath.Clean("data/pending.json")},
		{"relative leading dot", "./config.yaml", "config.yaml"},
	}
	for _, tc := range good {
		tc := tc
		t.Run("accept_"+tc.name, func(t *testing.T) {
			got, err := ValidatePath(tc.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestReadFileSymlinkRejected(t *testing.T) {
	// Skip on Windows where symlink creation needs admin.
	dir := t.TempDir()
	target := filepath.Join(dir, "real.txt")
	if err := os.WriteFile(target, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	if _, err := ReadFile(link); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink rejection, got %v", err)
	}
	// Direct read of the regular file must succeed.
	got, err := ReadFile(target)
	if err != nil {
		t.Fatalf("read regular file: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("read content mismatch")
	}
}

func TestWriteFileEnforces0600(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "secret.txt")
	if err := WriteFile(p, []byte("k")); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "k" {
		t.Fatal("content mismatch")
	}
	// POSIX perm bits aren't meaningful on Windows (ACLs govern
	// real access); we still pass 0600 to os.WriteFile so the
	// gosec G306 alert is satisfied at the call site.
	if runtime.GOOS != "windows" {
		if st.Mode().Perm()&0o077 != 0 {
			t.Fatalf("permissions too open: %v", st.Mode().Perm())
		}
	}
}

func TestMkdirAllEnforces0750(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "a", "b", "c")
	if err := MkdirAll(target); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if !st.IsDir() {
		t.Fatal("expected dir")
	}
	// Same Windows caveat as TestWriteFileEnforces0600.
	if runtime.GOOS != "windows" {
		if st.Mode().Perm()&0o007 != 0 {
			t.Fatalf("dir world-bits set: %v", st.Mode().Perm())
		}
	}
}
