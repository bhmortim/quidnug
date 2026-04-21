/**
 * @quidnug/client/v1-wire — v1.0-conformant canonical form helpers
 * and typed transaction builders for the JavaScript SDK.
 *
 * Why this module exists
 * ----------------------
 * The legacy `quidnug-client.js` does:
 *   - Base64 keys (server expects hex)
 *   - SPKI-derived quid IDs (server derives from SEC1 uncompressed)
 *   - Insertion-order JSON (server expects struct-declaration order)
 *
 * None of those work against a real v1.0 node. This module
 * provides the conformant primitives without disturbing the
 * existing SDK surface. New code should import from
 * `@quidnug/client/v1-wire`; legacy code keeps working against
 * the old v1 module until it's deprecated.
 *
 * Canonical form specification:
 *   - Signatures: 64-byte IEEE-1363 raw (r||s), hex-encoded.
 *   - Canonical signable bytes: fields emitted in Go-struct
 *     declaration order via the tx-type-specific builders below.
 *   - Quid IDs: hex(sha256(SEC1-uncompressed-pubkey)[0..8]).
 *   - Public keys on the wire: hex-encoded SEC1 uncompressed
 *     (`0x04 || X || Y`, 65 bytes → 130 hex chars).
 *
 * Test vectors at `docs/test-vectors/v1.0/` lock this in.
 *
 * @module
 */

import { webcrypto } from "node:crypto";

const subtle = webcrypto.subtle;

// ---------------------------------------------------------------
// Hex + bytes helpers
// ---------------------------------------------------------------

export function bytesToHex(buf) {
  const view = buf instanceof Uint8Array ? buf : new Uint8Array(buf);
  let out = "";
  for (const b of view) out += b.toString(16).padStart(2, "0");
  return out;
}

export function hexToBytes(hex) {
  if (hex.length % 2 !== 0) throw new Error("hex has odd length");
  const out = new Uint8Array(hex.length / 2);
  for (let i = 0; i < out.length; i++) {
    out[i] = parseInt(hex.substr(i * 2, 2), 16);
  }
  return out;
}

// ---------------------------------------------------------------
// Quid (identity) primitives
// ---------------------------------------------------------------

/**
 * Extract the SEC1 uncompressed-point representation of an ECDSA
 * P-256 public key from a WebCrypto CryptoKey. WebCrypto's
 * `exportKey('raw', publicKey)` returns exactly this byte format
 * on ECDSA keys (per the WebCrypto spec + Node.js docs).
 */
async function exportSEC1Uncompressed(publicKey) {
  const raw = await subtle.exportKey("raw", publicKey);
  const bytes = new Uint8Array(raw);
  if (bytes.length !== 65 || bytes[0] !== 0x04) {
    throw new Error(
      `expected 65-byte SEC1 uncompressed (0x04||X||Y), got ${bytes.length} bytes`,
    );
  }
  return bytes;
}

/**
 * Import a SEC1 uncompressed-point hex public key into a WebCrypto
 * ECDSA P-256 verifying key.
 */
async function importVerifyingKeyFromHex(hex) {
  const bytes = hexToBytes(hex);
  return subtle.importKey(
    "raw",
    bytes,
    { name: "ECDSA", namedCurve: "P-256" },
    true,
    ["verify"],
  );
}

/**
 * Import a raw 32-byte private scalar (hex) into a WebCrypto ECDSA
 * P-256 signing key. Used by test vectors; production code should
 * manage keys via WebCrypto-native storage.
 *
 * Requires Node.js ≥ 19 or a browser that supports JWK ECDSA import
 * (all modern browsers do).
 */
async function importSigningKeyFromScalarHex(scalarHex, pubSEC1Hex) {
  // Turn scalar + pubkey into a JWK and import. JWK is the only
  // WebCrypto format that accepts raw scalar data directly.
  const scalar = hexToBytes(scalarHex.padStart(64, "0"));
  const pub = hexToBytes(pubSEC1Hex);
  if (pub.length !== 65) throw new Error("pub must be SEC1 uncompressed 65 bytes");
  const x = pub.subarray(1, 33);
  const y = pub.subarray(33);
  const b64url = (b) =>
    Buffer.from(b).toString("base64").replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
  const jwk = {
    kty: "EC",
    crv: "P-256",
    d: b64url(scalar),
    x: b64url(x),
    y: b64url(y),
    ext: true,
  };
  return subtle.importKey(
    "jwk",
    jwk,
    { name: "ECDSA", namedCurve: "P-256" },
    true,
    ["sign"],
  );
}

async function sha256Hex(bytes) {
  const digest = await subtle.digest("SHA-256", bytes);
  return bytesToHex(digest);
}

/**
 * Derive the v1.0 quid ID from a SEC1 uncompressed-point hex
 * public key: hex(sha256(pubkey_bytes)[0..8]).
 */
export async function deriveQuidIdFromPublicHex(pubHex) {
  const bytes = hexToBytes(pubHex);
  const hash = await sha256Hex(bytes);
  return hash.substring(0, 16);
}

/**
 * Opaque Quid handle carrying a SEC1-hex public key + (optional)
 * WebCrypto signing key. Mirrors the Go/Python/Rust Quid types.
 */
export class Quid {
  constructor({ id, publicKeyHex, signingKey, verifyingKey }) {
    this.id = id;
    this.publicKeyHex = publicKeyHex;
    this._signingKey = signingKey || null;
    this._verifyingKey = verifyingKey;
  }

  hasPrivateKey() {
    return this._signingKey != null;
  }

  /**
   * Sign data with the quid's private key. Returns a hex-encoded
   * 64-byte IEEE-1363 signature.
   *
   * WebCrypto's `subtle.sign` with ECDSA-P256 returns raw r||s
   * directly — the concatenation is the IEEE-1363 form we need.
   * This is a happy accident of the WebCrypto spec: no DER
   * conversion required.
   */
  async sign(data) {
    if (!this._signingKey) throw new Error("quid is read-only");
    const sig = await subtle.sign(
      { name: "ECDSA", hash: { name: "SHA-256" } },
      this._signingKey,
      data instanceof Uint8Array ? data : new TextEncoder().encode(data),
    );
    const bytes = new Uint8Array(sig);
    if (bytes.length !== 64) {
      throw new Error(
        `expected 64-byte IEEE-1363 signature, got ${bytes.length} bytes`,
      );
    }
    return bytesToHex(bytes);
  }

  async verify(data, sigHex) {
    if (sigHex.length !== 128) return false;
    let sigBytes;
    try {
      sigBytes = hexToBytes(sigHex);
    } catch {
      return false;
    }
    const bytes = data instanceof Uint8Array ? data : new TextEncoder().encode(data);
    try {
      return await subtle.verify(
        { name: "ECDSA", hash: { name: "SHA-256" } },
        this._verifyingKey,
        sigBytes,
        bytes,
      );
    } catch {
      return false;
    }
  }

  /**
   * Generate a fresh Quid with a new ECDSA P-256 keypair.
   */
  static async generate() {
    const kp = await subtle.generateKey(
      { name: "ECDSA", namedCurve: "P-256" },
      true,
      ["sign", "verify"],
    );
    const pubBytes = await exportSEC1Uncompressed(kp.publicKey);
    const publicKeyHex = bytesToHex(pubBytes);
    const id = await deriveQuidIdFromPublicHex(publicKeyHex);
    return new Quid({
      id,
      publicKeyHex,
      signingKey: kp.privateKey,
      verifyingKey: kp.publicKey,
    });
  }

  /**
   * Construct a read-only Quid from a SEC1-hex public key.
   */
  static async fromPublicHex(publicKeyHex) {
    const verifyingKey = await importVerifyingKeyFromHex(publicKeyHex);
    const id = await deriveQuidIdFromPublicHex(publicKeyHex);
    return new Quid({ id, publicKeyHex, verifyingKey });
  }

  /**
   * Construct a signing Quid from a raw private scalar (hex) +
   * the corresponding SEC1-hex public key. Used by test vectors
   * which check in deterministic keys as raw scalars.
   */
  static async fromPrivateScalarHex(privateScalarHex, publicKeyHex) {
    const signingKey = await importSigningKeyFromScalarHex(
      privateScalarHex,
      publicKeyHex,
    );
    const verifyingKey = await importVerifyingKeyFromHex(publicKeyHex);
    const id = await deriveQuidIdFromPublicHex(publicKeyHex);
    return new Quid({ id, publicKeyHex, signingKey, verifyingKey });
  }
}

// ---------------------------------------------------------------
// Canonical signable bytes builders
// ---------------------------------------------------------------

/**
 * Build the canonical UTF-8 bytes for a field list. Fields are
 * an array of [key, value, omitempty] tuples emitted in declared
 * order. When `omitempty` is true and the value is zero/empty
 * (matching Go's `omitempty` semantics), the field is omitted.
 *
 * Nested `map[string]interface{}` values (plain JS objects used
 * as payloads / attributes) are recursively normalized with
 * keys in Unicode codepoint order — matching Go's
 * `encoding/json` default map-marshal behavior. Struct-typed
 * top-level fields are NOT sorted; their order is fixed by the
 * caller's tuple list (Go struct declaration order).
 *
 * Arrays preserve index order; their element objects are
 * recursively normalized.
 *
 * Without this normalization, payloads constructed by users via
 * object literals with non-alphabetical key insertion order
 * produce canonical bytes that differ from what a Go server
 * computes after re-marshaling from `map[string]interface{}` —
 * and signature verification fails. (Fixed 2026-04; prior
 * versions of this SDK shipped with the bug masked by test
 * vectors whose payloads happened to already be sorted.)
 */
function emitSignable(fields) {
  const obj = {};
  for (const [k, v, omitempty] of fields) {
    if (omitempty && isZero(v)) continue;
    obj[k] = goCompatValue(v);
  }
  // Stringify the top-level object in its insertion order (which
  // matches the Go struct's declaration order by construction).
  return new TextEncoder().encode(stringifyTopLevel(obj));
}

/**
 * Recursively normalize a value so nested plain-object keys
 * come out sorted when stringified. Arrays recurse but keep
 * their order. Primitives pass through.
 */
function goCompatValue(v) {
  if (v === null || v === undefined) return v;
  if (Array.isArray(v)) return v.map(goCompatValue);
  if (typeof v === "object") {
    // Plain object — sort keys.
    const out = {};
    const keys = Object.keys(v).sort();
    for (const k of keys) out[k] = goCompatValue(v[k]);
    return out;
  }
  return v;
}

/**
 * Stringify a top-level object preserving its own key insertion
 * order (caller has ordered the top-level fields via the tuple
 * list). Nested objects were already alphabetized by
 * goCompatValue before being inserted.
 */
function stringifyTopLevel(obj) {
  // JSON.stringify respects insertion order for non-integer keys
  // — which is what we want at the top level.
  return JSON.stringify(obj);
}

function isZero(v) {
  if (v === null || v === undefined) return true;
  if (v === "" || v === 0 || v === false) return true;
  if (Array.isArray(v) && v.length === 0) return true;
  if (typeof v === "object" && Object.keys(v).length === 0) return true;
  return false;
}

/**
 * JSON.stringify with Go-compatible float formatting.
 *
 * Go's encoding/json serializes `float64(1.0)` as `"1"`, not
 * `"1.0"`. JavaScript's JSON.stringify does the same by default
 * (JavaScript Number is a single type, so `1` and `1.0` round-trip
 * as the same JSON). So we don't need custom handling — just a
 * documentation note that any non-integer float will carry
 * precision through JSON.stringify unchanged.
 */
async function sha256OfBytes(bytes) {
  return subtle.digest("SHA-256", bytes);
}

// ---------------------------------------------------------------
// Node-integration helpers: ensureDomain + waitForIdentity +
// waitForTitle
//
// These are the ergonomics layer that demos and bootstrap code
// need when talking to a live v1.0 node. Without them, every
// first-time integrator hits the same two issues we surfaced
// during the POC sweep:
//
//   1. Non-default trust domains must be registered before any
//      identity/trust/title/event tx in that domain will be
//      accepted (ensureDomain, idempotent).
//
//   2. Identity and title transactions live in the node's
//      pending pool until the next block is sealed; follow-on
//      tx referencing the new quid/title fail with
//      "Subject QUID/TITLE not found" until commit
//      (waitForIdentity / waitForTitle poll until visible).
//
// These helpers do not depend on any particular HTTP client;
// each one accepts a baseUrl and uses fetch. They are designed
// to be copy-pasteable into a new app, not tied to the
// QuidnugClient class.
// ---------------------------------------------------------------

/**
 * Unwrap the standard {success, data, error} response envelope.
 * Throws an Error with .code and .message on success:false.
 */
async function parseEnvelope(resp) {
  const text = await resp.text();
  let body;
  try {
    body = text ? JSON.parse(text) : null;
  } catch {
    throw new Error(`HTTP ${resp.status}: non-JSON body: ${text.slice(0, 200)}`);
  }
  if (body && body.success && "data" in body) return body.data;
  const e = (body && body.error) || {};
  const err = new Error(e.message || `HTTP ${resp.status}`);
  err.code = e.code || "UNKNOWN";
  err.status = resp.status;
  throw err;
}

/**
 * Register a trust domain. Rejects with "already exists" when
 * the domain is already known; prefer `ensureDomain` for
 * idempotent bootstrap code.
 *
 * @param {string} baseUrl  e.g. "http://localhost:8080"
 * @param {string} domain   e.g. "payments.merchants"
 * @param {object} [attrs]  extra fields merged into the POST body
 */
export async function registerDomain(baseUrl, domain, attrs = {}) {
  const body = { name: domain, ...attrs };
  const resp = await fetch(`${baseUrl}/api/v1/domains`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  return parseEnvelope(resp);
}

/**
 * Idempotent domain registration. Returns normally on both
 * fresh-register and already-exists; propagates any other error.
 */
export async function ensureDomain(baseUrl, domain, attrs = {}) {
  try {
    return await registerDomain(baseUrl, domain, attrs);
  } catch (err) {
    const msg = String(err.message || "").toLowerCase();
    if (msg.includes("already exists")) {
      return {
        status: "success",
        domain,
        message: "trust domain already exists",
      };
    }
    throw err;
  }
}

/**
 * Fetch an identity record by quid ID. Returns null on 404.
 */
export async function getIdentity(baseUrl, quidId, domain) {
  const params = domain ? `?domain=${encodeURIComponent(domain)}` : "";
  const resp = await fetch(
    `${baseUrl}/api/v1/identity/${encodeURIComponent(quidId)}${params}`,
  );
  if (resp.status === 404) return null;
  try {
    return await parseEnvelope(resp);
  } catch (err) {
    if (err.code === "NOT_FOUND") return null;
    throw err;
  }
}

/**
 * Fetch a title by asset ID. Returns null on 404.
 */
export async function getTitle(baseUrl, assetId, domain) {
  const params = domain ? `?domain=${encodeURIComponent(domain)}` : "";
  const resp = await fetch(
    `${baseUrl}/api/v1/title/${encodeURIComponent(assetId)}${params}`,
  );
  if (resp.status === 404) return null;
  try {
    return await parseEnvelope(resp);
  } catch (err) {
    if (err.code === "NOT_FOUND") return null;
    throw err;
  }
}

/**
 * Block until the identity with `quidId` is visible in the
 * committed registry, or throw a timeout error.
 *
 * @param {string} baseUrl
 * @param {string} quidId
 * @param {object} [opts] { domain, timeoutMs=30000, pollMs=500 }
 */
export async function waitForIdentity(baseUrl, quidId, opts = {}) {
  const { domain, timeoutMs = 30000, pollMs = 500 } = opts;
  const deadline = Date.now() + timeoutMs;
  for (;;) {
    const rec = await getIdentity(baseUrl, quidId, domain);
    if (rec) return rec;
    if (Date.now() > deadline) {
      throw new Error(`identity ${quidId} did not commit within ${timeoutMs}ms`);
    }
    await new Promise((r) => setTimeout(r, pollMs));
  }
}

/**
 * Block until every quid ID is committed; shares one deadline
 * across the whole list.
 */
export async function waitForIdentities(baseUrl, quidIds, opts = {}) {
  const { domain, timeoutMs = 30000, pollMs = 500 } = opts;
  const deadline = Date.now() + timeoutMs;
  for (const id of quidIds) {
    const remaining = Math.max(0, deadline - Date.now());
    if (remaining <= 0) {
      throw new Error(`identities not all committed within ${timeoutMs}ms (blocked on ${id})`);
    }
    await waitForIdentity(baseUrl, id, {
      domain,
      timeoutMs: remaining,
      pollMs,
    });
  }
}

/**
 * Block until the title with `assetId` is visible in the
 * committed registry, or throw a timeout error.
 */
export async function waitForTitle(baseUrl, assetId, opts = {}) {
  const { domain, timeoutMs = 30000, pollMs = 500 } = opts;
  const deadline = Date.now() + timeoutMs;
  for (;;) {
    const t = await getTitle(baseUrl, assetId, domain);
    if (t) return t;
    if (Date.now() > deadline) {
      throw new Error(`title ${assetId} did not commit within ${timeoutMs}ms`);
    }
    await new Promise((r) => setTimeout(r, pollMs));
  }
}

async function _seedId(fields) {
  const obj = {};
  for (const [k, v] of fields) obj[k] = v;
  const bytes = new TextEncoder().encode(JSON.stringify(obj));
  const digest = await sha256OfBytes(bytes);
  return bytesToHex(digest);
}

// ---------------------------------------------------------------
// TRUST tx
// ---------------------------------------------------------------

/**
 * Build a signable TRUST transaction. Returns both the signable
 * bytes (for signing) and the derived transaction ID.
 */
export async function buildTrustTx(params) {
  const {
    trustDomain,
    timestamp,
    publicKey,
    truster,
    trustee,
    trustLevel,
    nonce,
    description = "",
    validUntil = 0,
  } = params;

  const id = await _seedId([
    ["Truster", truster],
    ["Trustee", trustee],
    ["TrustLevel", trustLevel],
    ["TrustDomain", trustDomain],
    ["Timestamp", timestamp],
  ]);

  const signable = emitSignable([
    ["id", id, false],
    ["type", "TRUST", false],
    ["trustDomain", trustDomain, false],
    ["timestamp", timestamp, false],
    ["signature", "", false],
    ["publicKey", publicKey, false],
    ["truster", truster, false],
    ["trustee", trustee, false],
    ["trustLevel", trustLevel, false],
    ["nonce", nonce, false],
    ["description", description, true],
    ["validUntil", validUntil, true],
  ]);

  return { id, signable };
}

// ---------------------------------------------------------------
// IDENTITY tx
// ---------------------------------------------------------------

export async function buildIdentityTx(params) {
  const {
    trustDomain,
    timestamp,
    publicKey,
    quidId,
    name,
    description = "",
    attributes = null,
    // Defaults matter: these fields are NOT `omitempty` on the
    // Go struct, so a re-marshaled wire at the server will emit
    // `"creator":""` and `"updateNonce":0` even when the client
    // supplied nothing. Without matching defaults here the
    // canonical bytes would diverge and signature verification
    // would fail.
    creator = "",
    updateNonce = 0,
    homeDomain = "",
  } = params;

  const id = await _seedId([
    ["QuidID", quidId],
    ["Name", name],
    ["Creator", creator],
    ["TrustDomain", trustDomain],
    ["UpdateNonce", updateNonce],
    ["Timestamp", timestamp],
  ]);

  const fields = [
    ["id", id, false],
    ["type", "IDENTITY", false],
    ["trustDomain", trustDomain, false],
    ["timestamp", timestamp, false],
    ["signature", "", false],
    ["publicKey", publicKey, false],
    ["quidId", quidId, false],
    ["name", name, false],
    ["description", description, true],
    ["attributes", attributes, true],
    ["creator", creator, false],
    ["updateNonce", updateNonce, false],
    ["homeDomain", homeDomain, true],
  ];
  const signable = emitSignable(fields);
  return { id, signable };
}

// ---------------------------------------------------------------
// EVENT tx
// ---------------------------------------------------------------

export async function buildEventTx(params) {
  const {
    trustDomain,
    timestamp,
    publicKey,
    subjectId,
    subjectType,
    sequence,
    eventType,
    payload = null,
    payloadCid = "",
    previousEventId = "",
  } = params;

  const id = await _seedId([
    ["SubjectID", subjectId],
    ["EventType", eventType],
    ["Sequence", sequence],
    ["TrustDomain", trustDomain],
    ["Timestamp", timestamp],
  ]);

  const fields = [
    ["id", id, false],
    ["type", "EVENT", false],
    ["trustDomain", trustDomain, false],
    ["timestamp", timestamp, false],
    ["signature", "", false],
    ["publicKey", publicKey, false],
    ["subjectId", subjectId, false],
    ["subjectType", subjectType, false],
    ["sequence", sequence, false],
    ["eventType", eventType, false],
  ];
  if (payload !== null && payload !== undefined) {
    fields.push(["payload", payload, true]);
  }
  if (payloadCid) {
    fields.push(["payloadCid", payloadCid, true]);
  }
  if (previousEventId) {
    fields.push(["previousEventId", previousEventId, true]);
  }
  const signable = emitSignable(fields);
  return { id, signable };
}

export default {
  Quid,
  bytesToHex,
  hexToBytes,
  deriveQuidIdFromPublicHex,
  buildTrustTx,
  buildIdentityTx,
  buildEventTx,
};
