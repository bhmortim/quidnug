/**
 * Quidnug Client SDK v3 extensions.
 *
 * Adds protocol coverage for endpoints introduced after the v2 release:
 *
 *   - Peer scoreboard + per-peer breakdown (QDP-0011)
 *   - Node advertisements
 *   - Domain registry: top domains, tentative blocks, domain gossip
 *   - Content moderation actions (QDP-0015)
 *   - Operator audit log (QDP-0018)
 *   - Data subject rights / consent / restrictions (QDP-0017)
 *   - Network + operator discovery (QDP-0014)
 *   - DNS domain attestation (QDP-0023)
 *
 * Usage (ES modules):
 *
 *     import QuidnugClient from "@quidnug/client";
 *     import "@quidnug/client/v2";
 *     import "@quidnug/client/v3";
 *
 *     const c = new QuidnugClient({ defaultNode: "http://localhost:8080" });
 *     const head = await c.getAuditHead();
 *     const ads = await c.getDNSAttestations("example.org");
 *
 * The mixin pattern keeps v1/v2 tests untouched — v3 adds strictly
 * new methods that share the v2 envelope-parsing helpers.
 */

import QuidnugClient from "./quidnug-client.js";

// --- Internal HTTP helpers ------------------------------------------------
//
// These mirror the helpers in quidnug-client-v2.js but accept an explicit
// API-prefix selector. Some v3 routes (discovery, DNS attestation) live
// under /api/v2/<path>; others (peers, audit, moderation, privacy,
// node-advertisements, gossip/domains) live under /api/<path>.

async function _postJson(client, path, body, { v2 = false } = {}) {
  const nodeUrl = client._getHealthyNode();
  const prefix = v2 ? "api/v2" : "api";
  const resp = await client._fetchWithRetry(`${nodeUrl}/${prefix}/${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  return client._parseResponse(resp);
}

async function _getJson(client, path, { v2 = false } = {}) {
  const nodeUrl = client._getHealthyNode();
  const prefix = v2 ? "api/v2" : "api";
  const resp = await client._fetchWithRetry(`${nodeUrl}/${prefix}/${path}`);
  return client._parseResponse(resp);
}

async function _getOrNull(client, path, opts) {
  try {
    return await _getJson(client, path, opts);
  } catch (err) {
    if (err && err.code === "NOT_FOUND") return null;
    throw err;
  }
}

function _qs(params) {
  if (!params) return "";
  const parts = [];
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === null) continue;
    parts.push(`${encodeURIComponent(k)}=${encodeURIComponent(v)}`);
  }
  return parts.length ? `?${parts.join("&")}` : "";
}

// ---------------------------------------------------------------------------
// Peer scoreboard + per-peer breakdown
// ---------------------------------------------------------------------------

QuidnugClient.prototype.getPeers = async function (params = {}) {
  return _getJson(this, `peers${_qs(params)}`);
};

QuidnugClient.prototype.getPeer = async function (nodeQuid) {
  if (!nodeQuid) throw new Error("nodeQuid required");
  return _getOrNull(this, `peers/${encodeURIComponent(nodeQuid)}`);
};

// ---------------------------------------------------------------------------
// Node advertisements (POST only; reads happen via discovery)
// ---------------------------------------------------------------------------

QuidnugClient.prototype.submitNodeAdvertisement = async function (ad) {
  if (!ad) throw new Error("advertisement required");
  return _postJson(this, "node-advertisements", ad);
};

// ---------------------------------------------------------------------------
// Domain registry extras
// ---------------------------------------------------------------------------

QuidnugClient.prototype.getTopDomains = async function (params = {}) {
  return _getJson(this, `domains/top${_qs(params)}`);
};

QuidnugClient.prototype.getTentativeBlocks = async function (domain) {
  if (!domain) throw new Error("domain required");
  return _getJson(this, `blocks/tentative/${encodeURIComponent(domain)}`);
};

QuidnugClient.prototype.submitDomainGossip = async function (msg) {
  if (!msg) throw new Error("message required");
  return _postJson(this, "gossip/domains", msg);
};

// ---------------------------------------------------------------------------
// Content moderation (QDP-0015)
// ---------------------------------------------------------------------------

QuidnugClient.prototype.submitModerationAction = async function (action) {
  if (!action) throw new Error("action required");
  return _postJson(this, "moderation/actions", action);
};

QuidnugClient.prototype.getModerationActions = async function (targetType, targetId) {
  if (!targetType || !targetId) throw new Error("targetType and targetId required");
  return _getJson(this,
    `moderation/actions/${encodeURIComponent(targetType)}/${encodeURIComponent(targetId)}`);
};

// ---------------------------------------------------------------------------
// Operator audit log (QDP-0018)
// ---------------------------------------------------------------------------

QuidnugClient.prototype.getAuditHead = async function () {
  return _getJson(this, "audit/head");
};

QuidnugClient.prototype.getAuditEntries = async function (params = {}) {
  return _getJson(this, `audit/entries${_qs(params)}`);
};

QuidnugClient.prototype.getAuditEntry = async function (sequence) {
  if (sequence === undefined || sequence === null) throw new Error("sequence required");
  return _getOrNull(this, `audit/entry/${encodeURIComponent(sequence)}`);
};

// ---------------------------------------------------------------------------
// Data subject rights / consent / restrictions (QDP-0017)
// ---------------------------------------------------------------------------

QuidnugClient.prototype.submitDSR = async function (request) {
  if (!request) throw new Error("request required");
  return _postJson(this, "privacy/dsr", request);
};

QuidnugClient.prototype.getDSRStatus = async function (requestTxId) {
  if (!requestTxId) throw new Error("requestTxId required");
  return _getOrNull(this, `privacy/dsr/${encodeURIComponent(requestTxId)}`);
};

QuidnugClient.prototype.grantConsent = async function (grant) {
  if (!grant) throw new Error("grant required");
  return _postJson(this, "privacy/consent/grants", grant);
};

QuidnugClient.prototype.withdrawConsent = async function (withdraw) {
  if (!withdraw) throw new Error("withdraw required");
  return _postJson(this, "privacy/consent/withdraws", withdraw);
};

QuidnugClient.prototype.getConsentHistory = async function (params = {}) {
  return _getJson(this, `privacy/consent/history${_qs(params)}`);
};

QuidnugClient.prototype.createProcessingRestriction = async function (restriction) {
  if (!restriction) throw new Error("restriction required");
  return _postJson(this, "privacy/restrictions", restriction);
};

QuidnugClient.prototype.getProcessingRestrictions = async function (subjectQuid) {
  if (!subjectQuid) throw new Error("subjectQuid required");
  return _getJson(this, `privacy/restrictions/${encodeURIComponent(subjectQuid)}`);
};

QuidnugClient.prototype.submitDSRCompliance = async function (compliance) {
  if (!compliance) throw new Error("compliance required");
  return _postJson(this, "privacy/compliance", compliance);
};

// ---------------------------------------------------------------------------
// Network + operator discovery (QDP-0014) — under /api/v2/discovery/*
// ---------------------------------------------------------------------------

QuidnugClient.prototype.getDiscoveryDomain = async function (name) {
  if (!name) throw new Error("name required");
  return _getOrNull(this, `discovery/domain/${encodeURIComponent(name)}`, { v2: true });
};

QuidnugClient.prototype.getDiscoveryNode = async function (quid) {
  if (!quid) throw new Error("quid required");
  return _getOrNull(this, `discovery/node/${encodeURIComponent(quid)}`, { v2: true });
};

QuidnugClient.prototype.getDiscoveryOperator = async function (quid) {
  if (!quid) throw new Error("quid required");
  return _getOrNull(this, `discovery/operator/${encodeURIComponent(quid)}`, { v2: true });
};

QuidnugClient.prototype.discoveryQuids = async function (params = {}) {
  return _getJson(this, `discovery/quids${_qs(params)}`, { v2: true });
};

QuidnugClient.prototype.discoveryTrustedQuids = async function (params = {}) {
  return _getJson(this, `discovery/trusted-quids${_qs(params)}`, { v2: true });
};

// ---------------------------------------------------------------------------
// DNS domain attestation (QDP-0023) — under /api/v2/dns/*
// ---------------------------------------------------------------------------

QuidnugClient.prototype.submitDNSClaim = async function (claim) {
  if (!claim) throw new Error("claim required");
  return _postJson(this, "dns/claim", claim, { v2: true });
};

QuidnugClient.prototype.submitDNSChallenge = async function (challenge) {
  if (!challenge) throw new Error("challenge required");
  return _postJson(this, "dns/challenge", challenge, { v2: true });
};

QuidnugClient.prototype.submitDNSAttestation = async function (attestation) {
  if (!attestation) throw new Error("attestation required");
  return _postJson(this, "dns/attestation", attestation, { v2: true });
};

QuidnugClient.prototype.submitDNSRenewal = async function (renewal) {
  if (!renewal) throw new Error("renewal required");
  return _postJson(this, "dns/renewal", renewal, { v2: true });
};

QuidnugClient.prototype.submitDNSRevocation = async function (revocation) {
  if (!revocation) throw new Error("revocation required");
  return _postJson(this, "dns/revocation", revocation, { v2: true });
};

QuidnugClient.prototype.submitDNSDelegate = async function (delegate) {
  if (!delegate) throw new Error("delegate required");
  return _postJson(this, "dns/delegate", delegate, { v2: true });
};

QuidnugClient.prototype.submitDNSDelegateRevocation = async function (revocation) {
  if (!revocation) throw new Error("revocation required");
  return _postJson(this, "dns/delegate-revocation", revocation, { v2: true });
};

QuidnugClient.prototype.getDNSAttestations = async function (domain) {
  if (!domain) throw new Error("domain required");
  return _getJson(this, `dns/attestations/${encodeURIComponent(domain)}`, { v2: true });
};

QuidnugClient.prototype.getDNSAttestationsWeighted = async function (domain) {
  if (!domain) throw new Error("domain required");
  return _getJson(this, `dns/attestations/${encodeURIComponent(domain)}/weighted`, { v2: true });
};

QuidnugClient.prototype.resolveDNS = async function (domain, recordType) {
  if (!domain || !recordType) throw new Error("domain and recordType required");
  return _getJson(this,
    `dns/resolve/${encodeURIComponent(domain)}/${encodeURIComponent(recordType)}`,
    { v2: true });
};

export default QuidnugClient;
export { QuidnugClient };
