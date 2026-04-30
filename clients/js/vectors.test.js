/**
 * JS SDK consumer for the v1.0 cross-SDK test vectors at
 * `docs/test-vectors/v1.0/`.
 *
 * Asserts the five conformance properties against the
 * `@quidnug/client/v1-wire` module:
 *
 *   1. Canonical signable bytes reproduce.
 *   2. Transaction ID derivation matches.
 *   3. Reference signature verifies.
 *   4. Tampered signature rejects.
 *   5. Independent sign-then-verify round-trip succeeds + produces
 *      64-byte IEEE-1363.
 *
 * Run with: `node --test vectors.test.js`.
 */

import { test } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
// Node 18 does not expose the WebCrypto API as a `crypto` global
// (that came in Node 19). Pull it in from "node:crypto" and
// expose it as `crypto` for the SubtleCrypto calls below. v20+
// already has the global; reassigning is harmless.
import { webcrypto as crypto } from "node:crypto";

import {
  Quid,
  bytesToHex,
  hexToBytes,
  buildTrustTx,
  buildIdentityTx,
  buildEventTx,
} from "./v1-wire.js";

// ---------------------------------------------------------------
// Vector loading
// ---------------------------------------------------------------

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// clients/js/ -> repo root: three levels up.
const VECTORS_ROOT = path.join(
  __dirname,
  "..",
  "..",
  "docs",
  "test-vectors",
  "v1.0",
);

function loadVectorFile(name) {
  const raw = fs.readFileSync(path.join(VECTORS_ROOT, name), "utf-8");
  return JSON.parse(raw);
}

function loadKeys() {
  const keysDir = path.join(VECTORS_ROOT, "test-keys");
  const entries = fs.readdirSync(keysDir);
  const out = {};
  for (const name of entries) {
    if (!name.endsWith(".json")) continue;
    const raw = fs.readFileSync(path.join(keysDir, name), "utf-8");
    const k = JSON.parse(raw);
    out[k.name] = k;
  }
  return out;
}

// ---------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------

function encoderUtf8(s) {
  return new TextEncoder().encode(s);
}

async function assertAllProperties(caseObj, key, signable, derivedId) {
  // Property 1: sha256 of canonical matches.
  const digest = await crypto.subtle.digest("SHA-256", signable);
  const shaHex = bytesToHex(digest);
  assert.equal(
    shaHex,
    caseObj.expected.sha256_of_canonical_hex,
    `${caseObj.name}: SHA-256 mismatch`,
  );

  // Property 2: canonical hex + utf8 match.
  assert.equal(
    bytesToHex(signable),
    caseObj.expected.canonical_signable_bytes_hex,
    `${caseObj.name}: canonical hex diverges`,
  );
  const decoded = new TextDecoder().decode(signable);
  assert.equal(
    decoded,
    caseObj.expected.canonical_signable_bytes_utf8,
    `${caseObj.name}: canonical utf8 diverges`,
  );

  // Property 3: derived ID matches.
  assert.equal(
    derivedId,
    caseObj.expected.expected_id,
    `${caseObj.name}: ID derivation mismatch`,
  );

  // Property 4: reference signature verifies via SDK Quid.verify.
  const qRO = await Quid.fromPublicHex(key.public_key_sec1_hex);
  const refOk = await qRO.verify(signable, caseObj.expected.reference_signature_hex);
  assert.ok(refOk, `${caseObj.name}: SDK Verify rejected reference signature`);
  assert.equal(
    caseObj.expected.signature_length_bytes,
    64,
    `${caseObj.name}: expected_sig_len != 64`,
  );

  // Property 5: tampered signature rejects.
  const sigBytes = hexToBytes(caseObj.expected.reference_signature_hex);
  const tampered = new Uint8Array(sigBytes);
  tampered[5] ^= 0x01;
  const tamperOk = await qRO.verify(signable, bytesToHex(tampered));
  assert.equal(tamperOk, false, `${caseObj.name}: tampered signature accepted`);

  // Property 6: SDK sign-then-verify round-trip.
  const qSign = await Quid.fromPrivateScalarHex(
    key.private_scalar_hex,
    key.public_key_sec1_hex,
  );
  const sdkSig = await qSign.sign(signable);
  assert.equal(
    hexToBytes(sdkSig).length,
    64,
    `${caseObj.name}: SDK signature not 64 bytes`,
  );
  const sdkOk = await qSign.verify(signable, sdkSig);
  assert.ok(sdkOk, `${caseObj.name}: SDK sign-verify round-trip failed`);
}

// ---------------------------------------------------------------
// Tests
// ---------------------------------------------------------------

test("trust vectors conform to v1.0 canonical form", async () => {
  const keys = loadKeys();
  const vf = loadVectorFile("trust-tx.json");
  assert.equal(vf.tx_type, "TRUST");
  assert.ok(vf.cases.length > 0, "no cases");

  for (const c of vf.cases) {
    const key = keys[c.signer_key_ref];
    const inp = c.input;

    const { id, signable } = await buildTrustTx({
      trustDomain: inp.trustDomain,
      timestamp: inp.timestamp,
      publicKey: inp.publicKey,
      truster: inp.truster,
      trustee: inp.trustee,
      trustLevel: inp.trustLevel,
      nonce: inp.nonce,
      description: inp.description || "",
      validUntil: inp.validUntil || 0,
    });

    await assertAllProperties(c, key, signable, id);
  }
});

test("identity vectors conform to v1.0 canonical form", async () => {
  const keys = loadKeys();
  const vf = loadVectorFile("identity-tx.json");
  assert.equal(vf.tx_type, "IDENTITY");
  assert.ok(vf.cases.length > 0, "no cases");

  for (const c of vf.cases) {
    const key = keys[c.signer_key_ref];
    const inp = c.input;

    const { id, signable } = await buildIdentityTx({
      trustDomain: inp.trustDomain,
      timestamp: inp.timestamp,
      publicKey: inp.publicKey,
      quidId: inp.quidId,
      name: inp.name,
      description: inp.description || "",
      attributes: inp.attributes || null,
      creator: inp.creator,
      updateNonce: inp.updateNonce,
      homeDomain: inp.homeDomain || "",
    });

    await assertAllProperties(c, key, signable, id);
  }
});

test("event vectors conform to v1.0 canonical form", async () => {
  const keys = loadKeys();
  const vf = loadVectorFile("event-tx.json");
  assert.equal(vf.tx_type, "EVENT");
  assert.ok(vf.cases.length > 0, "no cases");

  for (const c of vf.cases) {
    const key = keys[c.signer_key_ref];
    const inp = c.input;

    const { id, signable } = await buildEventTx({
      trustDomain: inp.trustDomain,
      timestamp: inp.timestamp,
      publicKey: inp.publicKey,
      subjectId: inp.subjectId,
      subjectType: inp.subjectType,
      sequence: inp.sequence,
      eventType: inp.eventType,
      payload: inp.payload !== undefined ? inp.payload : null,
      payloadCid: inp.payloadCid || "",
      previousEventId: inp.previousEventId || "",
    });

    await assertAllProperties(c, key, signable, id);
  }
});

test("payload with non-alphabetical insertion order is sorted in canonical form", async () => {
  // REGRESSION GUARD: this test does NOT load its payload from
  // the vector JSON file — because JSON.parse normalizes key
  // order on load, the vector-driven test can't catch the bug.
  // Instead we construct the payload as a JavaScript object
  // literal with keys in deliberately reverse-alphabetical
  // insertion order (matching how a user would build one), and
  // assert the canonical bytes come out alphabetically sorted.
  const keys = loadKeys();
  const aliceKey = keys.alice;

  // User-side: object literal, non-alphabetical keys, including
  // a nested object also in non-alphabetical order.
  const payload = {
    zebra: 1,
    apple: 2,
    mango: 3,
    nested: {
      yankee: "A",
      alpha: "B",
      mike: "C",
    },
  };

  const { signable } = await buildEventTx({
    trustDomain: "reviews.public.technology.laptops",
    timestamp: 1729641600,
    publicKey: aliceKey.public_key_sec1_hex,
    subjectId: aliceKey.quid_id,
    subjectType: "QUID",
    sequence: 3,
    eventType: "generic.test-payload",
    payload,
  });

  // Canonical bytes must contain the nested payload with keys
  // sorted alphabetically ("alpha","mike","yankee") and
  // ("apple","mango","nested","zebra") — NOT in insertion order.
  const canonical = new TextDecoder().decode(signable);
  const expectedPayloadFragment =
    '"payload":{"apple":2,"mango":3,"nested":{"alpha":"B","mike":"C","yankee":"A"},"zebra":1}';
  assert.ok(
    canonical.includes(expectedPayloadFragment),
    `expected canonical bytes to contain sorted nested payload; got:\n${canonical}`,
  );
});

test("Quid.generate produces 64-byte IEEE-1363 signatures", async () => {
  const q = await Quid.generate();
  const data = encoderUtf8("hello-quidnug");
  const sigHex = await q.sign(data);
  const sigBytes = hexToBytes(sigHex);
  assert.equal(sigBytes.length, 64, "signature must be 64 bytes (IEEE-1363)");
  const ok = await q.verify(data, sigHex);
  assert.ok(ok, "SDK sign-verify round-trip failed");
});

test("quid_id derivation matches test keys", async () => {
  const keys = loadKeys();
  for (const k of Object.values(keys)) {
    const q = await Quid.fromPublicHex(k.public_key_sec1_hex);
    assert.equal(q.id, k.quid_id, `quid_id mismatch for ${k.name}`);
  }
});
