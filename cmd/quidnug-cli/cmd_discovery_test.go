package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestWellKnownGenerate builds the CLI, generates a signed
// well-known file with a throwaway key, and verifies the
// resulting JSON is well-formed and carries the expected
// fields. No node contact required — this exercises the
// generator in isolation.
func TestWellKnownGenerate(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("no go toolchain: %v", err)
	}
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, exeName())
	if out, err := exec.Command("go", "build", "-o", binPath, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	// Generate an operator quid to sign with.
	keyPath := filepath.Join(tmp, "operator.key.json")
	if out, err := exec.Command(binPath, "quid", "generate", "--out", keyPath).CombinedOutput(); err != nil {
		t.Fatalf("keygen: %v\n%s", err, out)
	}

	seedsJSON := `[{"nodeQuid":"5f8a9b0000000001","url":"https://node1.example.com","region":"iad","capabilities":["validator","archive"]}]`
	domainsJSON := `[{"name":"reviews.public","description":"demo","tree":"reviews.public.*"}]`
	outPath := filepath.Join(tmp, "quidnug-network.json")

	cmd := exec.Command(binPath, "well-known", "generate",
		"--operator-key", keyPath,
		"--api-gateway", "https://api.example.com",
		"--seeds-json", seedsJSON,
		"--domains-json", domainsJSON,
		"--operator-name", "test-op",
		"--out", outPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate: %v\n%s", err, out)
	}

	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read %s: %v", outPath, err)
	}

	var doc WellKnownDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse: %v\nraw=%s", err, raw)
	}

	if doc.Version != 1 {
		t.Errorf("version: got %d want 1", doc.Version)
	}
	if doc.APIGateway != "https://api.example.com" {
		t.Errorf("apiGateway mismatch: %q", doc.APIGateway)
	}
	if len(doc.Seeds) != 1 || doc.Seeds[0].URL != "https://node1.example.com" {
		t.Errorf("seeds mismatch: %+v", doc.Seeds)
	}
	if len(doc.Domains) != 1 || doc.Domains[0].Name != "reviews.public" {
		t.Errorf("domains mismatch: %+v", doc.Domains)
	}
	if doc.Operator.Name != "test-op" {
		t.Errorf("operator name mismatch: %q", doc.Operator.Name)
	}
	if doc.Operator.PublicKey == "" || len(doc.Operator.PublicKey) < 128 {
		t.Errorf("operator publicKey looks wrong: %q", doc.Operator.PublicKey)
	}
	if doc.Signature == "" {
		t.Error("signature missing")
	}
	if doc.LastUpdated == 0 {
		t.Error("lastUpdated missing")
	}
}

// TestNodeAdvertiseUsageError confirms the CLI catches missing
// required flags before touching the network.
func TestNodeAdvertiseUsageError(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("no go toolchain: %v", err)
	}
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, exeName())
	if out, err := exec.Command("go", "build", "-o", binPath, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	cmd := exec.Command(binPath, "node", "advertise", "--signer", "/does/not/exist")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit for missing flags, got: %s", out)
	}
	// Check the error mentions a required flag.
	want := "required"
	if !bytes.Contains(bytes.ToLower(out), []byte(want)) {
		t.Errorf("expected error about required flags, got: %s", out)
	}
}

// TestDiscoverRequiresDomain confirms discover subcommands
// reject missing --domain at arg-parse time.
func TestDiscoverRequiresDomain(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("no go toolchain: %v", err)
	}
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, exeName())
	if out, err := exec.Command("go", "build", "-o", binPath, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	for _, sub := range []string{"domain", "quids", "trusted-quids"} {
		out, err := exec.Command(binPath, "discover", sub).CombinedOutput()
		if err == nil {
			t.Errorf("%s: expected non-zero exit, got %s", sub, out)
		}
		if !strings.Contains(string(out), "--domain") {
			t.Errorf("%s: expected error mentioning --domain, got %s", sub, out)
		}
	}
}
