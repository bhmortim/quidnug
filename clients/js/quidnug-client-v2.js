/**
 * Quidnug Client SDK v2 extensions.
 *
 * Adds protocol coverage for QDPs 0002–0010 on top of the v1
 * identity/trust/title/event surface in quidnug-client.js:
 *
 *   - Guardian sets + recovery + resignation (QDP-0002, QDP-0006)
 *   - Cross-domain fingerprint & anchor gossip (QDP-0003)
 *   - Push gossip (QDP-0005)
 *   - K-of-K bootstrap nonce snapshots (QDP-0008)
 *   - Fork-block activation (QDP-0009)
 *   - Compact Merkle inclusion proofs (QDP-0010)
 *
 * Usage (ES modules):
 *
 *     import QuidnugClient from "@quidnug/client";
 *     import "@quidnug/client/v2";     // installs v2 methods on the prototype
 *
 *     const c = new QuidnugClient({ defaultNode: "http://localhost:8080" });
 *     const gs = await c.getGuardianSet("abcd1234abcd1234");
 *     const ok = QuidnugClient.verifyInclusionProof(txBytes, frames, rootHex);
 *
 * The mixin pattern keeps v1 tests untouched — v1 methods are still
 * exported from quidnug-client.js, and v2 adds strictly new methods.
 */

import QuidnugClient from "./quidnug-client.js";

// ---------------------------------------------------------------------------
// Helper: shared POST JSON / GET JSON primitives that hit a healthy node.
// ---------------------------------------------------------------------------

async function _postJson(client, path, body) {
  const nodeUrl = client._getHealthyNode();
  const resp = await client._fetchWithRetry(`${nodeUrl}/api/${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  return client._parseResponse(resp);
}

async function _getJson(client, path) {
  const nodeUrl = client._getHealthyNode();
  const resp = await client._fetchWithRetry(`${nodeUrl}/api/${path}`);
  return client._parseResponse(resp);
}

async function _getOrNull(client, path) {
  try {
    return await _getJson(client, path);
  } catch (err) {
    if (err && err.code === "NOT_FOUND") return null;
    throw err;
  }
}

// ---------------------------------------------------------------------------
// Guardian sets (QDP-0002, QDP-0006)
// ---------------------------------------------------------------------------

/**
 * Install or rotate a guardian set for a subject quid.
 * @param {Object} update - Fully-signed GuardianSetUpdate envelope
 * @returns {Promise<Object>} Server receipt
 */
QuidnugClient.prototype.submitGuardianSetUpdate = async function (update) {
  if (!update || !update.subjectQuid) throw new Error("subjectQuid required");
  return _postJson(this, "guardian/set-update", update);
};

QuidnugClient.prototype.submitRecoveryInit = async function (init) {
  return _postJson(this, "guardian/recovery/init", init);
};

QuidnugClient.prototype.submitRecoveryVeto = async function (veto) {
  return _postJson(this, "guardian/recovery/veto", veto);
};

QuidnugClient.prototype.submitRecoveryCommit = async function (commit) {
  return _postJson(this, "guardian/recovery/commit", commit);
};

QuidnugClient.prototype.submitGuardianResignation = async function (resignation) {
  return _postJson(this, "guardian/resign", resignation);
};

QuidnugClient.prototype.getGuardianSet = async function (quidId) {
  if (!quidId) throw new Error("quidId required");
  return _getOrNull(this, `guardian/set/${encodeURIComponent(quidId)}`);
};

QuidnugClient.prototype.getPendingRecovery = async function (quidId) {
  return _getOrNull(this, `guardian/pending-recovery/${encodeURIComponent(quidId)}`);
};

QuidnugClient.prototype.getGuardianResignations = async function (quidId) {
  const data = await _getJson(this, `guardian/resignations/${encodeURIComponent(quidId)}`);
  return data.data || data.resignations || [];
};

// ---------------------------------------------------------------------------
// Cross-domain gossip (QDP-0003, QDP-0005)
// ---------------------------------------------------------------------------

QuidnugClient.prototype.submitDomainFingerprint = async function (fingerprint) {
  return _postJson(this, "domain-fingerprints", fingerprint);
};

QuidnugClient.prototype.getLatestDomainFingerprint = async function (domain) {
  if (!domain) throw new Error("domain required");
  return _getOrNull(this, `domain-fingerprints/${encodeURIComponent(domain)}/latest`);
};

QuidnugClient.prototype.submitAnchorGossip = async function (message) {
  return _postJson(this, "anchor-gossip", message);
};

QuidnugClient.prototype.pushAnchor = async function (message) {
  return _postJson(this, "gossip/push-anchor", message);
};

QuidnugClient.prototype.pushFingerprint = async function (fingerprint) {
  return _postJson(this, "gossip/push-fingerprint", fingerprint);
};

// ---------------------------------------------------------------------------
// K-of-K bootstrap (QDP-0008)
// ---------------------------------------------------------------------------

QuidnugClient.prototype.submitNonceSnapshot = async function (snapshot) {
  return _postJson(this, "nonce-snapshots", snapshot);
};

QuidnugClient.prototype.getLatestNonceSnapshot = async function (domain) {
  if (!domain) throw new Error("domain required");
  return _getOrNull(this, `nonce-snapshots/${encodeURIComponent(domain)}/latest`);
};

QuidnugClient.prototype.getBootstrapStatus = async function () {
  return _getJson(this, "bootstrap/status");
};

// ---------------------------------------------------------------------------
// Fork-block (QDP-0009)
// ---------------------------------------------------------------------------

QuidnugClient.prototype.submitForkBlock = async function (forkBlock) {
  return _postJson(this, "fork-block", forkBlock);
};

QuidnugClient.prototype.getForkBlockStatus = async function () {
  return _getJson(this, "fork-block/status");
};

// ---------------------------------------------------------------------------
// Compact Merkle inclusion proofs (QDP-0010)
// ---------------------------------------------------------------------------

/**
 * Hash-concat two buffers. SHA-256 available everywhere (browser via
 * SubtleCrypto, Node via node:crypto).
 */
async function _sha256(buf) {
  if (typeof globalThis.crypto !== "undefined" && globalThis.crypto.subtle) {
    const h = await globalThis.crypto.subtle.digest("SHA-256", buf);
    return new Uint8Array(h);
  }
  // Node fallback
  const { createHash } = await import("node:crypto");
  return new Uint8Array(createHash("sha256").update(Buffer.from(buf)).digest());
}

function _hexToBytes(hex) {
  if (typeof hex !== "string" || hex.length % 2 !== 0) {
    throw new Error("invalid hex string");
  }
  const out = new Uint8Array(hex.length / 2);
  for (let i = 0; i < out.length; i++) {
    out[i] = parseInt(hex.slice(i * 2, i * 2 + 2), 16);
    if (Number.isNaN(out[i])) throw new Error("invalid hex character");
  }
  return out;
}

function _bytesToHex(buf) {
  const arr = buf instanceof Uint8Array ? buf : new Uint8Array(buf);
  let out = "";
  for (const b of arr) out += b.toString(16).padStart(2, "0");
  return out;
}

function _concat(a, b) {
  const out = new Uint8Array(a.length + b.length);
  out.set(a, 0);
  out.set(b, a.length);
  return out;
}

function _bytesEqual(a, b) {
  if (a.length !== b.length) return false;
  let r = 0;
  for (let i = 0; i < a.length; i++) r |= a[i] ^ b[i];
  return r === 0;
}

/**
 * Verify a QDP-0010 compact Merkle inclusion proof.
 *
 * @param {Uint8Array|ArrayBuffer|string} txBytes - canonical signable
 *   encoding of the transaction (string is interpreted as UTF-8).
 * @param {Array<{hash: string, side: "left"|"right"}>} frames - proof frames.
 * @param {string} expectedRootHex - hex string of Block.transactions_root.
 * @returns {Promise<boolean>} true if the proof reconstructs the root.
 */
QuidnugClient.verifyInclusionProof = async function (txBytes, frames, expectedRootHex) {
  if (!txBytes) throw new Error("txBytes must be non-empty");
  if (!Array.isArray(frames)) throw new Error("frames must be an array");
  const expected = _hexToBytes(expectedRootHex);
  if (expected.length !== 32) throw new Error("expectedRoot must be 32 bytes");

  let buf;
  if (typeof txBytes === "string") {
    buf = new TextEncoder().encode(txBytes);
  } else if (txBytes instanceof Uint8Array) {
    buf = txBytes;
  } else {
    buf = new Uint8Array(txBytes);
  }

  let current = await _sha256(buf);
  for (let i = 0; i < frames.length; i++) {
    const f = frames[i];
    if (!f || (f.side !== "left" && f.side !== "right")) {
      throw new Error(`frame ${i}: side must be "left" or "right"`);
    }
    const sib = _hexToBytes(f.hash);
    if (sib.length !== 32) throw new Error(`frame ${i} hash must be 32 bytes`);
    const concat = f.side === "left" ? _concat(sib, current) : _concat(current, sib);
    current = await _sha256(concat);
  }
  return _bytesEqual(current, expected);
};

// Expose hex/bytes helpers for test + power-user code.
QuidnugClient.bytesToHex = _bytesToHex;
QuidnugClient.hexToBytes = _hexToBytes;

// ---------------------------------------------------------------------------
// Canonical signable bytes — matches Python/Go SDKs byte-for-byte.
// ---------------------------------------------------------------------------

/**
 * Return the canonical UTF-8 bytes for signing.
 *
 * Matches Go's json.Marshal → unmarshal(map[string]any) → json.Marshal
 * pattern: the second marshal alphabetizes keys. Excludes named
 * top-level fields (typically "signature", "txId", "publicKey").
 *
 * @param {object} obj
 * @param {string[]} excludeFields
 * @returns {Uint8Array}
 */
QuidnugClient.canonicalBytes = function (obj, excludeFields = []) {
  if (!obj || typeof obj !== "object") throw new Error("obj must be an object");
  const shallow = { ...obj };
  for (const f of excludeFields) delete shallow[f];
  // Sort keys recursively to mimic Go map serialization.
  const sorted = _sortKeysDeep(shallow);
  return new TextEncoder().encode(JSON.stringify(sorted));
};

function _sortKeysDeep(v) {
  if (Array.isArray(v)) return v.map(_sortKeysDeep);
  if (v && typeof v === "object") {
    const out = {};
    for (const k of Object.keys(v).sort()) {
      out[k] = _sortKeysDeep(v[k]);
    }
    return out;
  }
  return v;
}

export default QuidnugClient;
export { QuidnugClient };
