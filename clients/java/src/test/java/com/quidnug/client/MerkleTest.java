package com.quidnug.client;

import org.junit.jupiter.api.Test;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.List;

import static org.junit.jupiter.api.Assertions.*;

class MerkleTest {

    private static byte[] sh(byte[] data) throws Exception {
        return MessageDigest.getInstance("SHA-256").digest(data);
    }

    @Test
    void singleSiblingRight() throws Exception {
        byte[] tx = "tx-1".getBytes(StandardCharsets.UTF_8);
        byte[] sib = sh("tx-2".getBytes(StandardCharsets.UTF_8));
        byte[] leaf = sh(tx);
        byte[] concat = new byte[64];
        System.arraycopy(leaf, 0, concat, 0, 32);
        System.arraycopy(sib,  0, concat, 32, 32);
        byte[] root = sh(concat);

        List<Merkle.Frame> frames = List.of(new Merkle.Frame(Hex.encode(sib), "right"));
        assertTrue(Merkle.verifyInclusionProof(tx, frames, Hex.encode(root)));
    }

    @Test
    void fourLeafTree() throws Exception {
        byte[][] leaves = new byte[4][];
        for (int i = 0; i < 4; i++) leaves[i] = sh(("tx-" + i).getBytes(StandardCharsets.UTF_8));
        byte[] pair0 = sh(concat(leaves[0], leaves[1]));
        byte[] pair1 = sh(concat(leaves[2], leaves[3]));
        byte[] root  = sh(concat(pair0, pair1));

        List<Merkle.Frame> frames = List.of(
                new Merkle.Frame(Hex.encode(leaves[3]), "right"),
                new Merkle.Frame(Hex.encode(pair0),     "left")
        );
        assertTrue(Merkle.verifyInclusionProof(
                "tx-2".getBytes(StandardCharsets.UTF_8), frames, Hex.encode(root)));
    }

    @Test
    void tamperedTxRejected() throws Exception {
        byte[] sib = sh("tx-2".getBytes(StandardCharsets.UTF_8));
        byte[] leaf = sh("tx-1".getBytes(StandardCharsets.UTF_8));
        byte[] root = sh(concat(leaf, sib));
        List<Merkle.Frame> frames = List.of(new Merkle.Frame(Hex.encode(sib), "right"));
        assertFalse(Merkle.verifyInclusionProof(
                "tampered".getBytes(StandardCharsets.UTF_8), frames, Hex.encode(root)));
    }

    @Test
    void malformedFrameErrors() {
        List<Merkle.Frame> frames = List.of(new Merkle.Frame("nothex", "right"));
        assertThrows(QuidnugException.ValidationException.class,
                () -> Merkle.verifyInclusionProof(
                        "x".getBytes(StandardCharsets.UTF_8), frames, "aa".repeat(32)));
    }

    @Test
    void emptyTxErrors() {
        assertThrows(QuidnugException.CryptoException.class,
                () -> Merkle.verifyInclusionProof(new byte[0], List.of(), "aa".repeat(32)));
    }

    private static byte[] concat(byte[] a, byte[] b) {
        byte[] out = new byte[a.length + b.length];
        System.arraycopy(a, 0, out, 0, a.length);
        System.arraycopy(b, 0, out, a.length, b.length);
        return out;
    }
}
