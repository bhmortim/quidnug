package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func exeName() string {
	if runtime.GOOS == "windows" {
		return "quidnug-cli.exe"
	}
	return "quidnug-cli"
}

// End-to-end smoke test for `quidnug-cli merkle verify`. Does not need
// a running node — proves that the CLI wires up correctly through the
// Go client's VerifyInclusionProof.

func TestMerkleVerifyEndToEnd(t *testing.T) {
	// Build the CLI into a temp dir so we can exec it.
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, exeName())
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("go toolchain not available: %v", err)
	}
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = "." // package dir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	// Tree:  leaf[0] + leaf[1] = root.
	tx := []byte("tx-1")
	sibling := sha256.Sum256([]byte("tx-2"))
	leaf := sha256.Sum256(tx)
	rootArr := sha256.Sum256(append(append([]byte{}, leaf[:]...), sibling[:]...))

	txFile := filepath.Join(tmp, "tx.bin")
	if err := os.WriteFile(txFile, tx, 0o600); err != nil {
		t.Fatalf("write tx: %v", err)
	}
	proofFile := filepath.Join(tmp, "proof.json")
	frames := []map[string]string{{"hash": hex.EncodeToString(sibling[:]), "side": "right"}}
	pb, _ := json.Marshal(frames)
	if err := os.WriteFile(proofFile, pb, 0o600); err != nil {
		t.Fatalf("write proof: %v", err)
	}

	// Happy path.
	cmd := exec.Command(binPath, "merkle", "verify",
		"--tx", txFile,
		"--proof", proofFile,
		"--root", hex.EncodeToString(rootArr[:]),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("merkle verify pass: %v\n%s", err, out)
	}
	if !bytes.Contains(out, []byte("PASS")) {
		t.Fatalf("expected PASS, got: %s", out)
	}

	// Tamper the tx and expect non-zero exit code 6.
	if err := os.WriteFile(txFile, []byte("tampered"), 0o600); err != nil {
		t.Fatalf("rewrite tx: %v", err)
	}
	cmd = exec.Command(binPath, "merkle", "verify",
		"--tx", txFile,
		"--proof", proofFile,
		"--root", hex.EncodeToString(rootArr[:]),
	)
	out, err = cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit on tampered tx; output: %s", out)
	}
	if !bytes.Contains(out, []byte("FAIL")) {
		t.Fatalf("expected FAIL line, got: %s", out)
	}
	// ExitError.ExitCode() should be 6 per our exit-code contract.
	if ee, ok := err.(*exec.ExitError); ok {
		if ee.ExitCode() != 6 {
			t.Fatalf("expected exit code 6, got %d", ee.ExitCode())
		}
	}
}

func TestVersionFlag(t *testing.T) {
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, exeName())
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("no go toolchain: %v", err)
	}
	if out, err := exec.Command("go", "build", "-o", binPath, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	out, err := exec.Command(binPath, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("--version: %v\n%s", err, out)
	}
	if !bytes.Contains(out, []byte(version)) {
		t.Fatalf("expected %s in output, got %s", version, out)
	}
}
