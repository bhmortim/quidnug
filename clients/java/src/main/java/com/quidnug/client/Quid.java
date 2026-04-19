package com.quidnug.client;

import org.bouncycastle.jce.provider.BouncyCastleProvider;

import java.security.*;
import java.security.interfaces.ECPrivateKey;
import java.security.interfaces.ECPublicKey;
import java.security.spec.*;
import java.util.Arrays;

/**
 * A cryptographic identity (ECDSA P-256 keypair).
 *
 * <p>The quid ID is sha256(publicKey)[0..8] in hex — 16 hex chars —
 * matching the Go/Python/Rust SDKs byte-for-byte.
 */
public final class Quid {

    static {
        if (Security.getProvider("BC") == null) {
            Security.addProvider(new BouncyCastleProvider());
        }
    }

    private final String id;
    private final String publicKeyHex;
    private final String privateKeyHex;
    private final PrivateKey privateKey;  // null for read-only quids
    private final PublicKey publicKey;

    private Quid(String id, String pubHex, String privHex, PrivateKey sk, PublicKey pk) {
        this.id = id;
        this.publicKeyHex = pubHex;
        this.privateKeyHex = privHex;
        this.privateKey = sk;
        this.publicKey = pk;
    }

    /** Generate a fresh P-256 keypair. */
    public static Quid generate() {
        try {
            KeyPairGenerator g = KeyPairGenerator.getInstance("EC");
            g.initialize(new ECGenParameterSpec("secp256r1"));
            KeyPair kp = g.generateKeyPair();
            return fromKeyPair(kp.getPrivate(), kp.getPublic());
        } catch (GeneralSecurityException e) {
            throw new RuntimeException("keygen: " + e.getMessage(), e);
        }
    }

    /** Reconstruct from a PKCS#8 DER hex-encoded private key. */
    public static Quid fromPrivateHex(String privHex) {
        try {
            byte[] der = Hex.decode(privHex);
            KeyFactory kf = KeyFactory.getInstance("EC");
            PrivateKey sk = kf.generatePrivate(new PKCS8EncodedKeySpec(der));
            // Derive the public key from the private key.
            ECPrivateKey ecPriv = (ECPrivateKey) sk;
            ECPoint q = ecPriv.getParams().getGenerator();
            // Use BC KeyFactory to derive a matching public key.
            PublicKey pk = derivePublicKey(ecPriv);
            return fromKeyPair(sk, pk);
        } catch (Exception e) {
            throw new RuntimeException("fromPrivateHex: " + e.getMessage(), e);
        }
    }

    /** Build a read-only Quid from a SEC1 uncompressed hex public key. */
    public static Quid fromPublicHex(String pubHex) {
        try {
            byte[] sec1 = Hex.decode(pubHex);
            if (sec1.length != 65 || sec1[0] != 0x04) {
                throw new IllegalArgumentException("expected 65-byte SEC1 uncompressed point");
            }
            ECParameterSpec spec = ecParameterSpec();
            int byteLen = 32;
            BigInteger x = new BigInteger(1, Arrays.copyOfRange(sec1, 1, 1 + byteLen));
            BigInteger y = new BigInteger(1, Arrays.copyOfRange(sec1, 1 + byteLen, sec1.length));
            ECPoint point = new ECPoint(x, y);
            KeyFactory kf = KeyFactory.getInstance("EC");
            PublicKey pk = kf.generatePublic(new ECPublicKeySpec(point, spec));
            byte[] pub = sec1;
            String id = quidIdOf(pub);
            return new Quid(id, Hex.encode(pub), null, null, pk);
        } catch (Exception e) {
            throw new RuntimeException("fromPublicHex: " + e.getMessage(), e);
        }
    }

    private static Quid fromKeyPair(PrivateKey sk, PublicKey pk) {
        byte[] pub = sec1Encode(pk);
        byte[] priv = sk.getEncoded();
        String id = quidIdOf(pub);
        return new Quid(id, Hex.encode(pub), Hex.encode(priv), sk, pk);
    }

    public String id() { return id; }
    public String publicKeyHex() { return publicKeyHex; }
    public String privateKeyHex() { return privateKeyHex; }
    public boolean hasPrivateKey() { return privateKey != null; }

    /** Sign data. Returns hex-encoded DER ECDSA signature. */
    public String sign(byte[] data) throws GeneralSecurityException {
        if (privateKey == null) throw new IllegalStateException("quid is read-only");
        Signature s = Signature.getInstance("SHA256withECDSA");
        s.initSign(privateKey);
        s.update(data);
        return Hex.encode(s.sign());
    }

    /** Verify a hex-encoded DER signature. */
    public boolean verify(byte[] data, String sigHex) {
        try {
            Signature s = Signature.getInstance("SHA256withECDSA");
            s.initVerify(publicKey);
            s.update(data);
            return s.verify(Hex.decode(sigHex));
        } catch (Exception e) {
            return false;
        }
    }

    // --- Helpers ----------------------------------------------------

    private static byte[] sec1Encode(PublicKey pk) {
        ECPublicKey ec = (ECPublicKey) pk;
        byte[] x = bigIntToFixed(ec.getW().getAffineX(), 32);
        byte[] y = bigIntToFixed(ec.getW().getAffineY(), 32);
        byte[] out = new byte[65];
        out[0] = 0x04;
        System.arraycopy(x, 0, out, 1, 32);
        System.arraycopy(y, 0, out, 33, 32);
        return out;
    }

    private static byte[] bigIntToFixed(BigInteger v, int len) {
        byte[] b = v.toByteArray();
        if (b.length == len) return b;
        if (b.length == len + 1 && b[0] == 0) return Arrays.copyOfRange(b, 1, b.length);
        byte[] out = new byte[len];
        System.arraycopy(b, 0, out, len - b.length, b.length);
        return out;
    }

    private static String quidIdOf(byte[] pub) {
        try {
            MessageDigest md = MessageDigest.getInstance("SHA-256");
            byte[] d = md.digest(pub);
            return Hex.encode(Arrays.copyOf(d, 8));
        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException(e);
        }
    }

    private static ECParameterSpec ecParameterSpec() throws GeneralSecurityException {
        AlgorithmParameters params = AlgorithmParameters.getInstance("EC");
        params.init(new ECGenParameterSpec("secp256r1"));
        return params.getParameterSpec(ECParameterSpec.class);
    }

    private static PublicKey derivePublicKey(ECPrivateKey sk) throws GeneralSecurityException {
        // Use BC's private-to-public derivation.
        java.security.interfaces.ECPublicKey pk;
        KeyFactory kf = KeyFactory.getInstance("EC", "BC");
        var s = sk.getS();
        var params = sk.getParams();
        ECPoint qGen = params.getGenerator();
        // Scalar multiplication via BC
        var bcSpec = org.bouncycastle.jce.ECNamedCurveTable.getParameterSpec("P-256");
        var qPoint = bcSpec.getG().multiply(s).normalize();
        org.bouncycastle.jce.spec.ECPublicKeySpec pubSpec =
            new org.bouncycastle.jce.spec.ECPublicKeySpec(qPoint, bcSpec);
        pk = (java.security.interfaces.ECPublicKey) kf.generatePublic(pubSpec);
        return pk;
    }

    /** minimal BigInteger import helper so we avoid an extra import at top. */
    private static final class BigInteger {
        private BigInteger() {}
        static java.math.BigInteger of(byte[] sign, byte[] b) {
            return new java.math.BigInteger(sign[0], b);
        }
    }
}
