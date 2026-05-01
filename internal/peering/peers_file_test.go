package peering

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadPeersFile_BasicYAML(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "peers.yaml")
	body := `peers:
  - address: "node2.example.com:8080"
    operator_quid: "034bc467852ffa94"
  - address: "192.168.1.50:8080"
    operator_quid: "feedfacedeadbeef"
    allow_private: true
`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadPeersFile(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got)=%d want 2", len(got))
	}
	if got[0].Address != "node2.example.com:8080" || got[0].AllowPrivate {
		t.Fatalf("entry 0 unexpected: %+v", got[0])
	}
	if got[1].Address != "192.168.1.50:8080" || !got[1].AllowPrivate {
		t.Fatalf("entry 1 unexpected: %+v", got[1])
	}
}

func TestLoadPeersFile_RejectsBadAddress(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "peers.yaml")
	body := `peers:
  - address: "missing-port"
`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadPeersFile(p)
	if err == nil || !strings.Contains(err.Error(), "host:port") {
		t.Fatalf("expected host:port rejection, got %v", err)
	}
}

func TestLoadPeersFile_RejectsBadOperatorQuid(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "peers.yaml")
	body := `peers:
  - address: "node:8080"
    operator_quid: "TOOSHORT"
`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadPeersFile(p)
	if err == nil || !strings.Contains(err.Error(), "operator_quid") {
		t.Fatalf("expected operator_quid rejection, got %v", err)
	}
}

func TestLoadPeersFile_RejectsDuplicateAddress(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "peers.yaml")
	body := `peers:
  - address: "node:8080"
  - address: "node:8080"
`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadPeersFile(p)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate rejection, got %v", err)
	}
}

func TestLoadPeersFile_BracketedIPv6(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "peers.yaml")
	body := `peers:
  - address: "[2001:db8::1]:8080"
`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadPeersFile(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 1 || got[0].Address != "[2001:db8::1]:8080" {
		t.Fatalf("entry 0 unexpected: %+v", got)
	}
}

func TestLoadPeersFile_JSON(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "peers.json")
	body := `{"peers":[{"address":"node:8080","operatorQuid":"034bc467852ffa94"}]}`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadPeersFile(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 1 || got[0].Address != "node:8080" {
		t.Fatalf("got %+v", got)
	}
}

func TestLoadPeersFile_PathTraversalRejected(t *testing.T) {
	_, err := LoadPeersFile("../../../etc/passwd")
	if err == nil || !strings.Contains(err.Error(), "escapes working directory") {
		t.Fatalf("expected traversal rejection, got %v", err)
	}
}

func TestWatcher_InitialDeliveryAndReload(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "peers.yaml")
	if err := os.WriteFile(p, []byte("peers:\n  - address: \"a:8080\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	w := NewWatcher(p)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer w.Stop()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	// Initial delivery.
	select {
	case got := <-w.Events():
		if len(got) != 1 || got[0].Address != "a:8080" {
			t.Fatalf("initial got %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no initial event")
	}
	// Edit the file.
	if err := os.WriteFile(p, []byte("peers:\n  - address: \"b:9090\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-w.Events():
		if len(got) != 1 || got[0].Address != "b:9090" {
			t.Fatalf("reload got %+v", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no reload event")
	}
}
