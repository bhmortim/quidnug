package com.quidnug.client;

import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.List;

/**
 * QDP-0010 compact Merkle inclusion-proof verifier.
 *
 * <pre>
 *   leaf   = sha256(txBytes)
 *   parent = sha256(sibling || self) if side == "left"
 *            sha256(self || sibling) if side == "right"
 *   root   = parent after walking every frame
 * </pre>
 *
 * Returns {@code true} when the reconstructed root matches the expected
 * root; {@code false} for a clean mismatch; throws
 * {@link QuidnugException.ValidationException} for malformed input.
 */
public final class Merkle {

    private Merkle() {}

    /** One sibling hash + side in a proof. */
    public static final class Frame {
        public final String hash;
        public final String side;   // "left" or "right"
        public Frame(String hash, String side) {
            this.hash = hash;
            this.side = side;
        }
    }

    public static boolean verifyInclusionProof(byte[] txBytes, List<Frame> frames, String expectedRootHex) {
        if (txBytes == null || txBytes.length == 0) {
            throw new QuidnugException.CryptoException("txBytes is empty");
        }
        byte[] root;
        try {
            root = Hex.decode(expectedRootHex);
        } catch (Exception e) {
            throw new QuidnugException.ValidationException("expectedRoot is not valid hex");
        }
        if (root.length != 32) {
            throw new QuidnugException.ValidationException(
                    "expectedRoot must be 32 bytes (got " + root.length + ")");
        }

        byte[] current = sha256(txBytes);
        int i = 0;
        for (Frame f : frames) {
            byte[] sib;
            try {
                sib = Hex.decode(f.hash);
            } catch (Exception e) {
                throw new QuidnugException.ValidationException(
                        "frame " + i + " hash is not valid hex");
            }
            if (sib.length != 32) {
                throw new QuidnugException.ValidationException(
                        "frame " + i + " hash must be 32 bytes (got " + sib.length + ")");
            }
            byte[] concat;
            if ("left".equals(f.side)) {
                concat = new byte[64];
                System.arraycopy(sib, 0, concat, 0, 32);
                System.arraycopy(current, 0, concat, 32, 32);
            } else if ("right".equals(f.side)) {
                concat = new byte[64];
                System.arraycopy(current, 0, concat, 0, 32);
                System.arraycopy(sib, 0, concat, 32, 32);
            } else {
                throw new QuidnugException.ValidationException(
                        "frame " + i + " side must be 'left' or 'right', got '" + f.side + "'");
            }
            current = sha256(concat);
            i++;
        }
        // constant-time compare
        int diff = 0;
        for (int j = 0; j < 32; j++) diff |= (current[j] ^ root[j]) & 0xff;
        return diff == 0;
    }

    private static byte[] sha256(byte[] data) {
        try {
            return MessageDigest.getInstance("SHA-256").digest(data);
        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException(e);
        }
    }
}
