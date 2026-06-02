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
// Health / info / raw
// ---------------------------------------------------------------------------

/**
 * GET /api/health — node reachability and liveness probe.
 * @returns {Promise<Object>}
 */
QuidnugClient.prototype.health = async function () {
  return _getJson(this, "health");
};

/**
 * GET /api/info — node identity, version, features, and managed domains.
 * @returns {Promise<Object>}
 */
QuidnugClient.prototype.info = async function () {
  return _getJson(this, "info");
};

/**
 * GET an arbitrary node path (relative to /api) and return the raw
 * response body as a string. The leading slash is optional. Used for
 * ad-hoc endpoints that do not yet have a typed wrapper.
 *
 * The returned bytes are the full envelope; callers parse it themselves.
 * A non-2xx response throws an error whose `httpStatus` and `body`
 * properties carry the server reply.
 *
 * @param {string} path
 * @returns {Promise<string>}
 */
QuidnugClient.prototype.rawGet = async function (path) {
  if (typeof path !== "string" || path === "") {
    throw new Error("path required");
  }
  const nodeUrl = this._getHealthyNode();
  const trimmed = path.replace(/^\/+/, "");
  const resp = await this._fetchWithRetry(`${nodeUrl}/api/${trimmed}`);
  const text = await resp.text();
  if (!resp.ok) {
    const err = new Error(`status ${resp.status}`);
    err.httpStatus = resp.status;
    err.body = text;
    throw err;
  }
  return text;
};

// ---------------------------------------------------------------------------
// QDP-0014: Discovery queries
// ---------------------------------------------------------------------------

/**
 * GET /api/v2/discovery/domain/{domain} — current consortium, endpoint
 * hints, and block tip for a domain.
 * @param {string} domain
 * @returns {Promise<Object>}
 */
QuidnugClient.prototype.discoverDomain = async function (domain) {
  if (!domain) throw new Error("domain is required");
  return _getJson(this, `v2/discovery/domain/${encodeURIComponent(domain)}`);
};

/**
 * GET /api/v2/discovery/node/{quid} — raw signed advertisement for a quid.
 * @param {string} quid
 * @returns {Promise<Object>}
 */
QuidnugClient.prototype.discoverNode = async function (quid) {
  if (!quid) throw new Error("quid is required");
  return _getJson(this, `v2/discovery/node/${encodeURIComponent(quid)}`);
};

/**
 * GET /api/v2/discovery/operator/{quid} — list all advertisements for
 * a given operator quid.
 * @param {string} operatorQuid
 * @returns {Promise<Object>}
 */
QuidnugClient.prototype.discoverOperator = async function (operatorQuid) {
  if (!operatorQuid) throw new Error("operatorQuid is required");
  return _getJson(
    this,
    `v2/discovery/operator/${encodeURIComponent(operatorQuid)}`,
  );
};

/**
 * GET /api/v2/discovery/quids — per-domain quid index.
 *
 * @param {Object} params
 * @param {string} params.domain - required
 * @param {number} [params.since] - UnixNano lower bound
 * @param {string} [params.sort] - "activity" | "last-seen" | "first-seen" | "trust-weight"
 * @param {string} [params.observer] - enables trust-weight sort
 * @param {string} [params.eventType]
 * @param {number} [params.minTrustWeight]
 * @param {string[]} [params.excludeQuids]
 * @param {number} [params.limit] - default 50, max 500
 * @param {number} [params.offset]
 * @returns {Promise<Object>}
 */
QuidnugClient.prototype.discoverQuids = async function (params = {}) {
  const {
    domain,
    since,
    sort,
    observer,
    eventType,
    minTrustWeight,
    excludeQuids,
    limit,
    offset,
  } = params;
  if (!domain) throw new Error("domain is required");
  const q = new URLSearchParams();
  q.set("domain", domain);
  if (since && since > 0) q.set("since", String(since));
  if (sort) q.set("sort", sort);
  if (observer) q.set("observer", observer);
  if (eventType) q.set("eventType", eventType);
  if (typeof minTrustWeight === "number" && minTrustWeight > 0) {
    q.set("min-trust-weight", String(minTrustWeight));
  }
  if (Array.isArray(excludeQuids) && excludeQuids.length > 0) {
    q.set("excludeQuid", excludeQuids.join(","));
  }
  if (limit && limit > 0) q.set("limit", String(limit));
  if (offset && offset > 0) q.set("offset", String(offset));
  return _getJson(this, `v2/discovery/quids?${q.toString()}`);
};

/**
 * GET /api/v2/discovery/trusted-quids — quids directly TRUSTed by the
 * consortium above a threshold.
 *
 * @param {string} domain - required
 * @param {number} [minTrust]
 * @param {number} [limit]
 * @returns {Promise<Object>}
 */
QuidnugClient.prototype.discoverTrustedQuids = async function (
  domain,
  minTrust,
  limit,
) {
  if (!domain) throw new Error("domain is required");
  const q = new URLSearchParams();
  q.set("domain", domain);
  if (typeof minTrust === "number" && minTrust > 0) {
    q.set("min-trust", String(minTrust));
  }
  if (typeof limit === "number" && limit > 0) {
    q.set("limit", String(limit));
  }
  return _getJson(this, `v2/discovery/trusted-quids?${q.toString()}`);
};

// ---------------------------------------------------------------------------
// Domain registry
// ---------------------------------------------------------------------------

/**
 * GET /api/domains — list all known trust domains.
 * @returns {Promise<Object>}
 */
QuidnugClient.prototype.listDomains = async function () {
  return _getJson(this, "domains");
};

/**
 * POST /api/domains — register a new trust domain.
 *
 * Fails with an "already exists" error if the domain is already
 * registered; see ensureDomain for an idempotent variant. Extra
 * attributes beyond the name are merged into the POST body alongside
 * `{ name: domain }`.
 *
 * @param {string} domain
 * @param {Object} [attrs]
 * @returns {Promise<Object>}
 */
QuidnugClient.prototype.registerDomain = async function (domain, attrs) {
  if (!domain) throw new Error("domain is required");
  const body = { name: domain };
  if (attrs && typeof attrs === "object") {
    for (const k of Object.keys(attrs)) body[k] = attrs[k];
  }
  return _postJson(this, "domains", body);
};

/**
 * Idempotent wrapper around registerDomain: registers the domain if it
 * does not already exist, otherwise returns a success envelope. Safe to
 * call from demo and bootstrap scripts.
 *
 * @param {string} domain
 * @param {Object} [attrs]
 * @returns {Promise<Object>}
 */
QuidnugClient.prototype.ensureDomain = async function (domain, attrs) {
  try {
    return await this.registerDomain(domain, attrs);
  } catch (err) {
    const msg = String((err && err.message) || "").toLowerCase();
    if (msg.includes("already exists")) {
      return {
        status: "success",
        domain,
        message: "trust domain already exists",
      };
    }
    throw err;
  }
};

// ---------------------------------------------------------------------------
// Polling helpers — wait for commit
// ---------------------------------------------------------------------------

const DEFAULT_WAIT_TIMEOUT_MS = 30000;
const DEFAULT_WAIT_POLL_INTERVAL_MS = 500;

function _sleepAbortable(ms, signal) {
  return new Promise((resolve, reject) => {
    if (signal && signal.aborted) {
      reject(signal.reason || new Error("aborted"));
      return;
    }
    let timer;
    const onAbort = () => {
      clearTimeout(timer);
      reject(signal.reason || new Error("aborted"));
    };
    timer = setTimeout(() => {
      if (signal) signal.removeEventListener("abort", onAbort);
      resolve();
    }, ms);
    if (signal) signal.addEventListener("abort", onAbort, { once: true });
  });
}

async function _pollUntil(fetcher, opts = {}) {
  const timeoutMs =
    typeof opts.timeoutMs === "number" && opts.timeoutMs > 0
      ? opts.timeoutMs
      : DEFAULT_WAIT_TIMEOUT_MS;
  const pollIntervalMs =
    typeof opts.pollIntervalMs === "number" && opts.pollIntervalMs > 0
      ? opts.pollIntervalMs
      : DEFAULT_WAIT_POLL_INTERVAL_MS;
  const signal = opts.signal;
  const deadline = Date.now() + timeoutMs;
  for (;;) {
    if (signal && signal.aborted) {
      throw signal.reason || new Error("aborted");
    }
    const rec = await fetcher();
    if (rec) return rec;
    const remaining = deadline - Date.now();
    if (remaining <= 0) {
      const err = new Error("wait timeout exceeded");
      err.code = "TIMEOUT";
      throw err;
    }
    const wait = Math.min(pollIntervalMs, remaining);
    await _sleepAbortable(wait, signal);
  }
}

/**
 * Block until the identity for `quidId` is visible in the committed
 * registry, or reject on timeout / abort.
 *
 * Just-submitted identity transactions live in the pending pool until
 * the next block is sealed; callers that immediately reference the
 * subject must await commit first.
 *
 * @param {string} quidId
 * @param {string} [domain]
 * @param {Object} [opts]
 * @param {number} [opts.timeoutMs=30000]
 * @param {number} [opts.pollIntervalMs=500]
 * @param {AbortSignal} [opts.signal]
 * @returns {Promise<Object>} The identity record.
 */
QuidnugClient.prototype.waitForIdentity = async function (
  quidId,
  domain,
  opts,
) {
  if (!quidId) throw new Error("quidId required");
  return _pollUntil(() => this.getIdentity(quidId, domain), opts);
};

/**
 * Block until every listed quid is committed. Shares one timeout
 * across all ids.
 *
 * @param {string[]} quidIds
 * @param {string} [domain]
 * @param {Object} [opts]
 * @returns {Promise<void>}
 */
QuidnugClient.prototype.waitForIdentities = async function (
  quidIds,
  domain,
  opts,
) {
  if (!Array.isArray(quidIds)) throw new Error("quidIds must be an array");
  const timeoutMs =
    opts && typeof opts.timeoutMs === "number" && opts.timeoutMs > 0
      ? opts.timeoutMs
      : DEFAULT_WAIT_TIMEOUT_MS;
  const pollIntervalMs =
    opts && typeof opts.pollIntervalMs === "number" && opts.pollIntervalMs > 0
      ? opts.pollIntervalMs
      : DEFAULT_WAIT_POLL_INTERVAL_MS;
  const signal = opts && opts.signal;
  const deadline = Date.now() + timeoutMs;
  for (const id of quidIds) {
    const remaining = deadline - Date.now();
    if (remaining <= 0) {
      const err = new Error(`wait for identity ${id}: wait timeout exceeded`);
      err.code = "TIMEOUT";
      throw err;
    }
    try {
      await this.waitForIdentity(id, domain, {
        timeoutMs: remaining,
        pollIntervalMs,
        signal,
      });
    } catch (err) {
      throw new Error(`wait for identity ${id}: ${err.message}`);
    }
  }
};

/**
 * Block until the title with the given asset ID is visible in the
 * committed registry. Same rationale as waitForIdentity.
 *
 * @param {string} assetId
 * @param {string} [domain]
 * @param {Object} [opts]
 * @returns {Promise<Object>} The title record.
 */
QuidnugClient.prototype.waitForTitle = async function (assetId, domain, opts) {
  if (!assetId) throw new Error("assetId required");
  return _pollUntil(() => this.getAssetOwnership(assetId, domain), opts);
};

// ---------------------------------------------------------------------------
// QDP-0014: Node advertisement (signed) -- TODO
// ---------------------------------------------------------------------------

/**
 * Build, sign, and submit a QDP-0014 NodeAdvertisementTransaction.
 *
 * Not yet implemented in the JS SDK. Use the Go SDK
 * (Client.PublishNodeAdvertisement) until the corresponding signing
 * path is wired up here. Tracking the canonical wire format and the
 * derived ID assignment to match the Go reference byte-for-byte.
 *
 * @param {Object} quid - signer with a private key
 * @param {Object} params - NodeAdvertisementParams
 */
QuidnugClient.prototype.publishNodeAdvertisement = async function (
  // eslint-disable-next-line no-unused-vars
  quid,
  // eslint-disable-next-line no-unused-vars
  params,
) {
  const err = new Error(
    "publishNodeAdvertisement not yet implemented in the JS SDK; " +
      "use the Go SDK until the signing path is ported.",
  );
  err.code = "NOT_IMPLEMENTED";
  throw err;
};

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
