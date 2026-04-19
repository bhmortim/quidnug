package com.quidnug.client;

import org.junit.jupiter.api.Test;

import java.nio.charset.StandardCharsets;

import static org.junit.jupiter.api.Assertions.*;

class QuidTest {

    @Test
    void generateProducesExpectedIdFormat() {
        Quid q = Quid.generate();
        assertEquals(16, q.id().length(), "quid id should be 16 hex chars");
        assertTrue(q.hasPrivateKey());
        assertNotNull(q.publicKeyHex());
        assertNotNull(q.privateKeyHex());
    }

    @Test
    void signVerifyRoundtrip() throws Exception {
        Quid q = Quid.generate();
        String sig = q.sign("hello".getBytes(StandardCharsets.UTF_8));
        assertTrue(q.verify("hello".getBytes(StandardCharsets.UTF_8), sig));
        assertFalse(q.verify("tampered".getBytes(StandardCharsets.UTF_8), sig));
    }

    @Test
    void privateHexRoundtrip() {
        Quid q = Quid.generate();
        Quid r = Quid.fromPrivateHex(q.privateKeyHex());
        assertEquals(q.id(), r.id());
        assertEquals(q.publicKeyHex(), r.publicKeyHex());
        assertTrue(r.hasPrivateKey());
    }

    @Test
    void readOnlyQuidCannotSign() {
        Quid q = Quid.generate();
        Quid ro = Quid.fromPublicHex(q.publicKeyHex());
        assertFalse(ro.hasPrivateKey());
        assertEquals(q.id(), ro.id());
        assertThrows(IllegalStateException.class,
                () -> ro.sign("x".getBytes(StandardCharsets.UTF_8)));
    }

    @Test
    void signatureDoesNotVerifyForDifferentQuid() throws Exception {
        Quid a = Quid.generate();
        Quid b = Quid.generate();
        String sig = a.sign("shared".getBytes(StandardCharsets.UTF_8));
        assertFalse(b.verify("shared".getBytes(StandardCharsets.UTF_8), sig));
    }
}
