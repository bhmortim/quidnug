// Regression coverage for ENG-79: per-peer allow_private was set
// at admission but never consulted by sync-loop dial paths
// (block_sync, gossip, query, broadcast). The fix introduces a
// node-method validatePeerAddress that consults the per-node
// PrivateAddrAllowList before falling through to the global
// blocked-range gate. These tests pin that contract.
//
// Note on test environment: under `go test`, allowPrivatePeers()
// returns true, so the global gate accepts 192.168.x.x and
// loopback unconditionally. We can't observe the
// "allow-list bypasses an otherwise-blocked private IP"
// behavior directly under that mode. Instead, we verify the
// allow-list short-circuit by using an unresolvable hostname:
// the allow-list path returns sanitized without DNS lookup,
// while the fallthrough free function fails on LookupIP. The
// distinction proves the short-circuit fires.
package core

import (
	"strings"
	"testing"
)

// TestNodeValidatePeerAddress_AllowListShortCircuit_SkipsDNSLookup:
// an unresolvable hostname in the allow-list passes without
// hitting LookupIP, while the same hostname through the free
// function fails. This proves the short-circuit short-circuits.
func TestNodeValidatePeerAddress_AllowListShortCircuit_SkipsDNSLookup(t *testing.T) {
	const unresolvable = "ENG79-test-host.invalid:8080"

	// Precondition: the free function fails on this hostname
	// because .invalid never resolves.
	if _, err := ValidatePeerAddress(unresolvable); err == nil {
		t.Fatal("precondition: unresolvable .invalid hostname should fail free-function ValidatePeerAddress")
	}

	node := newTestNode()
	node.PrivateAddrAllowList.Set([]string{unresolvable})

	// With the address allow-listed, the node-method should
	// short-circuit and return success without the DNS lookup.
	safe, err := node.validatePeerAddress(unresolvable)
	if err != nil {
		t.Fatalf("allow-listed unresolvable address should short-circuit and succeed: %v", err)
	}
	if safe.String() != unresolvable {
		t.Fatalf("expected sanitized %q, got %q", unresolvable, safe.String())
	}
}

// TestNodeValidatePeerAddress_HostOnlyInAllowList: bare-host
// allow-list entry matches "host:port" inputs. This is the
// case where peers_file is keyed on hostname only.
func TestNodeValidatePeerAddress_HostOnlyInAllowList(t *testing.T) {
	node := newTestNode()
	node.PrivateAddrAllowList.Set([]string{"ENG79-host-only.invalid"})

	safe, err := node.validatePeerAddress("ENG79-host-only.invalid:9000")
	if err != nil {
		t.Fatalf("host-only allow-list should match host:port input: %v", err)
	}
	if safe.String() != "ENG79-host-only.invalid:9000" {
		t.Fatalf("expected ENG79-host-only.invalid:9000, got %q", safe.String())
	}
}

// TestNodeValidatePeerAddress_NotAllowed_FallsThrough: an
// address NOT in the allow-list falls through to the global
// check. We use an unresolvable hostname to observe the
// fallthrough via the LookupIP error.
func TestNodeValidatePeerAddress_NotAllowed_FallsThrough(t *testing.T) {
	node := newTestNode()
	node.PrivateAddrAllowList.Set([]string{"ENG79-something-else.invalid"})

	// Different unresolvable hostname — not in the allow-list.
	_, err := node.validatePeerAddress("ENG79-not-allowed.invalid:8080")
	if err == nil {
		t.Fatal("non-allow-listed unresolvable hostname should fail via fallthrough")
	}
	if !strings.Contains(err.Error(), "resolve") && !strings.Contains(err.Error(), "no such host") {
		t.Fatalf("expected resolve error from fallthrough path, got: %v", err)
	}
}

// TestNodeValidatePeerAddress_ControlCharRejectedEvenIfAllowed:
// even with the allow-list short-circuit, syntactic rejection
// (control characters) still applies. A tampered peers_file
// can't smuggle CR/LF into a peer address.
func TestNodeValidatePeerAddress_ControlCharRejectedEvenIfAllowed(t *testing.T) {
	node := newTestNode()
	bad := "192.168.1.50\r\n:8080"
	node.PrivateAddrAllowList.Set([]string{bad})

	if _, err := node.validatePeerAddress(bad); err == nil {
		t.Fatal("control-character address should be rejected even when allow-listed")
	}
}

// TestNodeValidatePeerAddress_NilAllowList: when the allow-list
// is nil (defensive: shouldn't happen in production, but the
// method must not panic), behavior matches the free function
// for the fallthrough path.
func TestNodeValidatePeerAddress_NilAllowList(t *testing.T) {
	node := newTestNode()
	node.PrivateAddrAllowList = nil

	// Public address: free function passes regardless of
	// allowPrivatePeers() short-circuit, so this should pass.
	if _, err := node.validatePeerAddress("8.8.8.8:53"); err != nil {
		t.Fatalf("public address should pass with nil allow-list: %v", err)
	}
	// Unresolvable: fallthrough to free function, which fails
	// on LookupIP. This confirms we don't accidentally swallow
	// the error or short-circuit when the list is nil.
	if _, err := node.validatePeerAddress("ENG79-nilcheck.invalid:8080"); err == nil {
		t.Fatal("unresolvable address should fail when allow-list is nil")
	}
}

// TestNodeValidatePeerAddress_NilNode: defensive — the method
// receiver is nil-checked so a nil node doesn't panic on the
// allow-list lookup. Falls through to the free function.
func TestNodeValidatePeerAddress_NilNode(t *testing.T) {
	var node *QuidnugNode
	if _, err := node.validatePeerAddress("8.8.8.8:53"); err != nil {
		t.Fatalf("public address with nil node should not panic: %v", err)
	}
}
