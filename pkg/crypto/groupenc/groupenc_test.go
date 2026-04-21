package groupenc

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestGenerateKeyPair(t *testing.T) {
	kp, err := GenerateKeyPair(rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	if len(kp.PublicKeyBytes()) != 32 {
		t.Errorf("pub len %d != 32", len(kp.PublicKeyBytes()))
	}
	if len(kp.PrivateKeyBytes()) != 32 {
		t.Errorf("priv len %d != 32", len(kp.PrivateKeyBytes()))
	}
}

func TestWrapUnwrapRoundtrip(t *testing.T) {
	alice, _ := GenerateKeyPair(rand.Reader)
	bob, _ := GenerateKeyPair(rand.Reader)

	secret, err := NewEpochSecret(rand.Reader)
	if err != nil {
		t.Fatalf("secret: %v", err)
	}
	if len(secret) != EpochSecretLength {
		t.Errorf("secret length: %d", len(secret))
	}

	// Alice wraps the secret for Bob.
	wrapped, err := WrapEpochKey(rand.Reader, bob.Public, secret)
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}

	// Bob unwraps with his private key.
	recovered, err := UnwrapEpochKey(bob.Private, wrapped)
	if err != nil {
		t.Fatalf("unwrap: %v", err)
	}
	if !bytes.Equal(recovered, secret) {
		t.Errorf("unwrapped secret differs from original")
	}

	// Alice (non-recipient) cannot unwrap.
	if _, err := UnwrapEpochKey(alice.Private, wrapped); err == nil {
		t.Error("alice should not be able to unwrap Bob's wrapped secret")
	}
}

func TestWrappedSecret_MarshalRoundtrip(t *testing.T) {
	bob, _ := GenerateKeyPair(rand.Reader)
	secret, _ := NewEpochSecret(rand.Reader)
	wrapped, _ := WrapEpochKey(rand.Reader, bob.Public, secret)

	bytes1 := wrapped.Marshal()
	expectedSize := 32 + 12 + (32 + 16)
	if len(bytes1) != expectedSize {
		t.Errorf("marshaled size %d != %d", len(bytes1), expectedSize)
	}

	restored, err := UnmarshalWrappedSecret(bytes1)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !bytes.Equal(restored.Marshal(), bytes1) {
		t.Errorf("unmarshal-remarshal diverged")
	}

	// And it should still unwrap.
	rec, err := UnwrapEpochKey(bob.Private, restored)
	if err != nil {
		t.Fatalf("unwrap after unmarshal: %v", err)
	}
	if !bytes.Equal(rec, secret) {
		t.Error("secret recovered through marshal roundtrip differs")
	}
}

func TestEncryptDecryptRecord(t *testing.T) {
	secret, _ := NewEpochSecret(rand.Reader)
	plaintext := []byte("super-secret-employee-directory-json")

	nonce, ct, err := EncryptRecord(rand.Reader, secret, plaintext, nil)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if len(nonce) != 12 {
		t.Errorf("nonce length: %d", len(nonce))
	}
	if len(ct) != len(plaintext)+16 {
		t.Errorf("ct length: %d (expected %d)", len(ct), len(plaintext)+16)
	}

	recovered, err := DecryptRecord(secret, nonce, ct, nil)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(recovered, plaintext) {
		t.Errorf("plaintext mismatch")
	}
}

func TestDecryptRecord_WrongKey(t *testing.T) {
	secretA, _ := NewEpochSecret(rand.Reader)
	secretB, _ := NewEpochSecret(rand.Reader)
	plaintext := []byte("for alice")

	nonce, ct, _ := EncryptRecord(rand.Reader, secretA, plaintext, nil)
	if _, err := DecryptRecord(secretB, nonce, ct, nil); err == nil {
		t.Error("decryption with wrong key succeeded")
	}
}

func TestDecryptRecord_TamperedCiphertext(t *testing.T) {
	secret, _ := NewEpochSecret(rand.Reader)
	nonce, ct, _ := EncryptRecord(rand.Reader, secret, []byte("hello"), nil)
	ct[0] ^= 0x01
	if _, err := DecryptRecord(secret, nonce, ct, nil); err == nil {
		t.Error("decryption of tampered ciphertext succeeded")
	}
}

func TestEncryptRecord_AADBinding(t *testing.T) {
	secret, _ := NewEpochSecret(rand.Reader)
	plaintext := []byte("msg")
	aadA := []byte("group-a")
	aadB := []byte("group-b")

	nonce, ct, _ := EncryptRecord(rand.Reader, secret, plaintext, aadA)
	// Correct AAD → decrypts.
	if _, err := DecryptRecord(secret, nonce, ct, aadA); err != nil {
		t.Errorf("valid AAD rejected: %v", err)
	}
	// Wrong AAD → rejects.
	if _, err := DecryptRecord(secret, nonce, ct, aadB); err == nil {
		t.Error("wrong AAD accepted")
	}
}

func TestMultiMemberFlow(t *testing.T) {
	// Simulate a 3-member group: alice, bob, carol share an
	// epoch secret; each receives a WrappedSecret encrypted
	// to their own public key; all three should recover the
	// same secret.
	alice, _ := GenerateKeyPair(rand.Reader)
	bob, _ := GenerateKeyPair(rand.Reader)
	carol, _ := GenerateKeyPair(rand.Reader)

	secret, _ := NewEpochSecret(rand.Reader)

	wrappedA, _ := WrapEpochKey(rand.Reader, alice.Public, secret)
	wrappedB, _ := WrapEpochKey(rand.Reader, bob.Public, secret)
	wrappedC, _ := WrapEpochKey(rand.Reader, carol.Public, secret)

	recA, err := UnwrapEpochKey(alice.Private, wrappedA)
	if err != nil || !bytes.Equal(recA, secret) {
		t.Errorf("alice unwrap failed: err=%v", err)
	}
	recB, err := UnwrapEpochKey(bob.Private, wrappedB)
	if err != nil || !bytes.Equal(recB, secret) {
		t.Errorf("bob unwrap failed: err=%v", err)
	}
	recC, err := UnwrapEpochKey(carol.Private, wrappedC)
	if err != nil || !bytes.Equal(recC, secret) {
		t.Errorf("carol unwrap failed: err=%v", err)
	}

	// Cross-check: encrypt a record under the epoch secret;
	// all three decrypt it; an outsider cannot.
	plaintext := []byte("record payload")
	nonce, ct, _ := EncryptRecord(rand.Reader, secret, plaintext, nil)

	for _, who := range []struct {
		name string
		sec  []byte
	}{{"alice", recA}, {"bob", recB}, {"carol", recC}} {
		got, err := DecryptRecord(who.sec, nonce, ct, nil)
		if err != nil {
			t.Errorf("%s decrypt: %v", who.name, err)
		}
		if !bytes.Equal(got, plaintext) {
			t.Errorf("%s got wrong plaintext", who.name)
		}
	}

	// Outsider.
	dave, _ := GenerateKeyPair(rand.Reader)
	// Outsider tries to decrypt wrappedA — fails (not
	// recipient).
	if _, err := UnwrapEpochKey(dave.Private, wrappedA); err == nil {
		t.Error("outsider unwrapped alice's wrapped secret")
	}
}

func TestKeyPairFromPrivate(t *testing.T) {
	kp1, _ := GenerateKeyPair(rand.Reader)
	kp2, err := KeyPairFromPrivate(kp1.PrivateKeyBytes())
	if err != nil {
		t.Fatalf("reconstruct: %v", err)
	}
	if !bytes.Equal(kp1.PublicKeyBytes(), kp2.PublicKeyBytes()) {
		t.Error("reconstructed public key diverged")
	}
}

func TestEpochSecretFingerprint_Stable(t *testing.T) {
	secret, _ := NewEpochSecret(rand.Reader)
	fp1 := EpochSecretFingerprint(secret)
	fp2 := EpochSecretFingerprint(secret)
	if fp1 != fp2 {
		t.Errorf("unstable: %s vs %s", fp1, fp2)
	}
	if len(fp1) != 16 {
		t.Errorf("fingerprint length %d != 16", len(fp1))
	}
}
