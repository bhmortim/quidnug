/**
 * Browser extension v1.0 canonical-form conformance tests.
 *
 * The extension's `signWithQuid` function in `src/background.js`
 * uses WebCrypto to sign. WebCrypto returns IEEE-1363 raw bytes
 * natively, so the conformant implementation just returns those
 * bytes directly (no DER conversion).
 *
 * These tests do two things:
 *
 * 1. Static source check: the sign path in background.js no
 *    longer calls `ieeeToDer`. Regression detector if somebody
 *    re-adds the conversion.
 *
 * 2. Logic check: replicate the signing approach with the test
 *    keys from `docs/test-vectors/v1.0/` and verify the resulting
 *    signatures verify via an independent IEEE-1363 verifier.
 *
 * The extension's `signWithQuid` is not exported (it's a service
 * worker function), so we replicate its logic here rather than
 * import it. The logic is: PKCS8 import -> subtle.sign -> hex.
 * Our replica should match the extension byte-for-byte once the
 * service worker is loaded in a real MV3 environment (tested via
 * Playwright, roadmap).
 *
 * Run with: `node --test vectors.test.js`.
 */

import { test } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { webcrypto } from "node:crypto";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const subtle = webcrypto.subtle;

// clients/browser-extension/ -> repo root: three levels up.
const VECTORS_ROOT = path.join(
    __dirname, "..", "..", "..",
    "docs", "test-vectors", "v1.0",
);

function hexToBytes(hex) {
    const out = new Uint8Array(hex.length / 2);
    for (let i = 0; i < out.length; i++) {
        out[i] = parseInt(hex.slice(i * 2, i * 2 + 2), 16);
    }
    return out;
}

function bytesToHex(b) {
    let s = "";
    for (const x of b) s += x.toString(16).padStart(2, "0");
    return s;
}

// Replica of the extension's signWithQuid. Once the service
// worker is exportable, this becomes `import { signWithQuid }`.
async function signLikeExtension(privateKeyHex, data) {
    const privDER = hexToBytes(privateKeyHex);
    const key = await subtle.importKey(
        "pkcs8", privDER, { name: "ECDSA", namedCurve: "P-256" }, false, ["sign"]);
    const sig = new Uint8Array(await subtle.sign(
        { name: "ECDSA", hash: "SHA-256" }, key, data));
    return bytesToHex(sig);
}

// Replica of an authoritative verifier (64-byte IEEE-1363).
async function verifyLikeServer(publicKeySEC1Hex, data, sigHex) {
    const sig = hexToBytes(sigHex);
    if (sig.length !== 64) return false;
    const pub = hexToBytes(publicKeySEC1Hex);
    const key = await subtle.importKey(
        "raw", pub, { name: "ECDSA", namedCurve: "P-256" }, true, ["verify"]);
    return subtle.verify(
        { name: "ECDSA", hash: "SHA-256" }, key, sig, data);
}

// Import a raw scalar as a PKCS8-ish PrivateKey via WebCrypto JWK.
async function scalarHexToPKCS8(scalarHex, pubSEC1Hex) {
    const pub = hexToBytes(pubSEC1Hex);
    const x = pub.slice(1, 33);
    const y = pub.slice(33);
    const scalar = hexToBytes(scalarHex.padStart(64, "0"));
    const b64url = (b) =>
        Buffer.from(b).toString("base64")
            .replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
    const jwk = {
        kty: "EC", crv: "P-256",
        d: b64url(scalar),
        x: b64url(x),
        y: b64url(y),
        ext: true,
    };
    const key = await subtle.importKey(
        "jwk", jwk,
        { name: "ECDSA", namedCurve: "P-256" },
        true, ["sign"]);
    const pkcs8 = new Uint8Array(await subtle.exportKey("pkcs8", key));
    return bytesToHex(pkcs8);
}

test("static: background.js sign path no longer DER-encodes", async () => {
    const src = fs.readFileSync(
        path.join(__dirname, "..", "src", "background.js"), "utf-8");
    // The v1.0 conformant implementation returns bytesToHex(ieee)
    // directly from signWithQuid. If the function still contains
    // ieeeToDer on the return statement, the fix was reverted.
    const signFn = src.match(/async function signWithQuid[\s\S]*?\n\}/);
    assert.ok(signFn, "signWithQuid function not found");
    assert.ok(
        !signFn[0].includes("bytesToHex(ieeeToDer("),
        "sign path still contains DER conversion; revert detected",
    );
    assert.ok(
        signFn[0].includes("return bytesToHex(ieee);"),
        "sign path missing expected IEEE-1363 return",
    );
});

test("logic: extension-style signing produces 64-byte IEEE-1363", async () => {
    const keyPath = path.join(VECTORS_ROOT, "test-keys", "key-alice.json");
    const key = JSON.parse(fs.readFileSync(keyPath, "utf-8"));
    const pkcs8Hex = await scalarHexToPKCS8(
        key.private_scalar_hex, key.public_key_sec1_hex);

    const data = new TextEncoder().encode("test-browser-extension-signing");
    const sigHex = await signLikeExtension(pkcs8Hex, data);
    const sigBytes = hexToBytes(sigHex);
    assert.equal(sigBytes.length, 64,
        "signature must be 64 bytes IEEE-1363");

    const verified = await verifyLikeServer(
        key.public_key_sec1_hex, data, sigHex);
    assert.ok(verified, "authoritative verifier rejected the signature");
});

test("vector-driven: extension sign + server verify round-trips", async () => {
    const vectorsTrust = JSON.parse(
        fs.readFileSync(path.join(VECTORS_ROOT, "trust-tx.json"), "utf-8"));
    const keys = Object.fromEntries(
        fs.readdirSync(path.join(VECTORS_ROOT, "test-keys"))
            .filter(n => n.endsWith(".json"))
            .map(n => {
                const k = JSON.parse(fs.readFileSync(
                    path.join(VECTORS_ROOT, "test-keys", n), "utf-8"));
                return [k.name, k];
            })
    );

    for (const c of vectorsTrust.cases) {
        const key = keys[c.signer_key_ref];
        const signable = new TextEncoder().encode(
            c.expected.canonical_signable_bytes_utf8);

        // Reference signature must verify.
        const refOk = await verifyLikeServer(
            key.public_key_sec1_hex, signable,
            c.expected.reference_signature_hex);
        assert.ok(refOk, `${c.name}: reference sig rejected`);

        // Extension-style sign + server verify round-trip.
        const pkcs8Hex = await scalarHexToPKCS8(
            key.private_scalar_hex, key.public_key_sec1_hex);
        const sig = await signLikeExtension(pkcs8Hex, signable);
        assert.equal(hexToBytes(sig).length, 64);
        const sdkOk = await verifyLikeServer(
            key.public_key_sec1_hex, signable, sig);
        assert.ok(sdkOk,
            `${c.name}: extension sign -> server verify failed`);
    }
});
