/**
 * Wire-format tests for the v1 + v2 SDK methods added to bring
 * @quidnug/client into full coverage with the server's HTTP surface
 * (see docs/client-coverage.md).
 *
 * Each test stubs fetch, calls a method, and asserts the URL +
 * method + body the client actually sent.
 */

import { test, beforeEach } from "node:test";
import assert from "node:assert/strict";

// Polyfill window.* so quidnug-client.js can be imported in Node.
const mockCrypto = {
  subtle: {
    generateKey: async () => ({ privateKey: {}, publicKey: {} }),
    exportKey: async () => new ArrayBuffer(0),
    importKey: async () => ({}),
    sign: async () => new ArrayBuffer(64),
    digest: async () => new ArrayBuffer(32),
  },
};
globalThis.window = {
  crypto: mockCrypto,
  btoa: (s) => Buffer.from(s, "binary").toString("base64"),
  atob: (s) => Buffer.from(s, "base64").toString("binary"),
};

const { default: QuidnugClient } = await import("./quidnug-client.js");
await import("./quidnug-client-v2.js"); // installs v2 prototype methods

// --- fetch stub -----------------------------------------------------------

const calls = [];
let queue = [];

function reset() {
  calls.length = 0;
  queue.length = 0;
}

function queueResp(payload, { status = 200 } = {}) {
  queue.push({
    ok: status < 400,
    status,
    json: async () => payload,
    arrayBuffer: async () => new ArrayBuffer(0),
  });
}

globalThis.fetch = async (url, opts = {}) => {
  calls.push({ url, opts });
  if (!queue.length) {
    return {
      ok: true,
      status: 200,
      json: async () => ({ success: true, data: {} }),
    };
  }
  return queue.shift();
};

function makeClient() {
  // Skip the constructor's auto health-check (which would race the
  // test's queued responses) by constructing without a default node
  // and seeding the pool directly.
  const c = new QuidnugClient({ maxRetries: 0 });
  c.nodes.push({ url: "http://node.local", status: "healthy", lastChecked: Date.now() });
  return c;
}

beforeEach(reset);

// --- v1 additions ---------------------------------------------------------

test("getHealth hits /api/health", async () => {
  queueResp({ success: true, data: { status: "ok" } });
  const c = makeClient();
  const r = await c.getHealth();
  assert.deepEqual(r, { status: "ok" });
  assert.match(calls[0].url, /\/api\/health$/);
});

test("getInfo hits /api/info", async () => {
  queueResp({ success: true, data: { version: "x" } });
  const c = makeClient();
  await c.getInfo();
  assert.match(calls[0].url, /\/api\/info$/);
});

test("getPeers hits /api/peers", async () => {
  queueResp({ success: true, data: { peers: [], count: 0 } });
  const c = makeClient();
  await c.getPeers();
  assert.match(calls[0].url, /\/api\/peers$/);
});

test("getPeer encodes nodeQuid into path; returns null on PEER_NOT_FOUND", async () => {
  queueResp(
    { success: false, error: { code: "PEER_NOT_FOUND", message: "n/a" } },
    { status: 404 },
  );
  const c = makeClient();
  const r = await c.getPeer("abc/def");
  assert.equal(r, null);
  assert.match(calls[0].url, /\/api\/peers\/abc%2Fdef$/);
});

test("getTentativeBlocks encodes domain into path", async () => {
  queueResp({ success: true, data: {} });
  const c = makeClient();
  await c.getTentativeBlocks("foo.com");
  assert.match(calls[0].url, /\/api\/blocks\/tentative\/foo\.com$/);
});

test("listDomains hits GET /api/domains", async () => {
  queueResp({ success: true, data: [] });
  const c = makeClient();
  await c.listDomains();
  assert.equal(calls[0].opts.method || "GET", "GET");
  assert.match(calls[0].url, /\/api\/domains$/);
});

test("registerDomain POSTs name+attrs", async () => {
  queueResp({ success: true, data: { domain: "x" } });
  const c = makeClient();
  await c.registerDomain("x.example", { description: "hi" });
  assert.equal(calls[0].opts.method, "POST");
  const body = JSON.parse(calls[0].opts.body);
  assert.equal(body.name, "x.example");
  assert.equal(body.description, "hi");
});

test("getTopDomains hits /api/domains/top", async () => {
  queueResp({ success: true, data: [] });
  const c = makeClient();
  await c.getTopDomains();
  assert.match(calls[0].url, /\/api\/domains\/top$/);
});

test("getNodeDomains / updateNodeDomains hit /api/node/domains", async () => {
  queueResp({ success: true, data: { managedDomains: [] } });
  queueResp({ success: true, data: {} });
  const c = makeClient();
  await c.getNodeDomains();
  await c.updateNodeDomains(["a.x", "b.y"]);
  assert.match(calls[0].url, /\/api\/node\/domains$/);
  assert.equal(calls[1].opts.method, "POST");
  const body = JSON.parse(calls[1].opts.body);
  assert.deepEqual(body, { managedDomains: ["a.x", "b.y"] });
});

test("updateNodeDomains rejects non-array input", async () => {
  const c = makeClient();
  await assert.rejects(() => c.updateNodeDomains("not-an-array"));
});

test("getTrustEdges hits /api/trust/edges/{quid}", async () => {
  queueResp({ success: true, data: [] });
  const c = makeClient();
  await c.getTrustEdges("abc123");
  assert.match(calls[0].url, /\/api\/trust\/edges\/abc123$/);
});

test("createNodeAdvertisement POSTs body untouched", async () => {
  queueResp({ success: true, data: { id: "tx" } });
  const c = makeClient();
  const ad = { nodeQuid: "n", operatorQuid: "op", endpoints: [], capabilities: {} };
  await c.createNodeAdvertisement(ad);
  assert.match(calls[0].url, /\/api\/node-advertisements$/);
  assert.deepEqual(JSON.parse(calls[0].opts.body), ad);
});

test("sendDomainGossip POSTs to /api/gossip/domains", async () => {
  queueResp({ success: true, data: { status: "accepted" } });
  const c = makeClient();
  await c.sendDomainGossip({ domain: "x" });
  assert.match(calls[0].url, /\/api\/gossip\/domains$/);
  assert.equal(calls[0].opts.method, "POST");
});

// --- Moderation -----------------------------------------------------------

test("createModerationAction requires moderatorQuid", async () => {
  const c = makeClient();
  await assert.rejects(() => c.createModerationAction({}));
});

test("createModerationAction POSTs body to /api/moderation/actions", async () => {
  queueResp({ success: true, data: { id: "tx" } });
  const c = makeClient();
  await c.createModerationAction({
    moderatorQuid: "mod",
    targetType: "QUID",
    targetId: "t",
    scope: "hide",
    reasonCode: "spam",
    nonce: 1,
  });
  assert.match(calls[0].url, /\/api\/moderation\/actions$/);
});

test("getModerationActions builds the (type, id) path", async () => {
  queueResp({ success: true, data: { actions: [] } });
  const c = makeClient();
  await c.getModerationActions("QUID", "abc");
  assert.match(calls[0].url, /\/api\/moderation\/actions\/QUID\/abc$/);
});

// --- Audit ----------------------------------------------------------------

test("getAuditHead hits /api/audit/head", async () => {
  queueResp({ success: true, data: { height: 0 } });
  const c = makeClient();
  await c.getAuditHead();
  assert.match(calls[0].url, /\/api\/audit\/head$/);
});

test("getAuditEntries appends ?since= and ?limit=", async () => {
  queueResp({ success: true, data: { entries: [] } });
  const c = makeClient();
  await c.getAuditEntries({ since: 10, limit: 50 });
  assert.match(calls[0].url, /\/api\/audit\/entries\?since=10&limit=50$/);
});

test("getAuditEntry returns null on NOT_FOUND", async () => {
  queueResp(
    { success: false, error: { code: "NOT_FOUND", message: "n/a" } },
    { status: 404 },
  );
  const c = makeClient();
  assert.equal(await c.getAuditEntry(999), null);
});

// --- Privacy --------------------------------------------------------------

test("createDSR / getDSRStatus path round-trip", async () => {
  queueResp({ success: true, data: { id: "tx" } });
  queueResp({ success: true, data: { request: {} } });
  const c = makeClient();
  await c.createDSR({ subjectQuid: "s", requestType: "ERASURE", nonce: 1 });
  await c.getDSRStatus("tx");
  assert.match(calls[0].url, /\/api\/privacy\/dsr$/);
  assert.match(calls[1].url, /\/api\/privacy\/dsr\/tx$/);
});

test("createConsentGrant / Withdraw paths", async () => {
  queueResp({ success: true, data: {} });
  queueResp({ success: true, data: {} });
  const c = makeClient();
  await c.createConsentGrant({ subjectQuid: "s", controllerQuid: "c", scope: [], nonce: 1 });
  await c.createConsentWithdraw({
    subjectQuid: "s",
    withdrawsGrantTxId: "g",
    nonce: 2,
  });
  assert.match(calls[0].url, /\/api\/privacy\/consent\/grants$/);
  assert.match(calls[1].url, /\/api\/privacy\/consent\/withdraws$/);
});

test("getConsentHistory uses subject query param", async () => {
  queueResp({ success: true, data: { entries: [] } });
  const c = makeClient();
  await c.getConsentHistory("subq");
  assert.match(calls[0].url, /\/api\/privacy\/consent\/history\?subject=subq$/);
});

test("createProcessingRestriction / getRestrictionsForSubject", async () => {
  queueResp({ success: true, data: {} });
  queueResp({ success: true, data: { restrictedUses: [] } });
  const c = makeClient();
  await c.createProcessingRestriction({ subjectQuid: "s", restrictedUses: [], nonce: 1 });
  await c.getRestrictionsForSubject("s");
  assert.match(calls[0].url, /\/api\/privacy\/restrictions$/);
  assert.match(calls[1].url, /\/api\/privacy\/restrictions\/s$/);
});

test("createDSRCompliance POSTs to /api/privacy/compliance", async () => {
  queueResp({ success: true, data: { id: "tx" } });
  const c = makeClient();
  await c.createDSRCompliance({
    requestTxId: "req",
    requestType: "ERASURE",
    operatorQuid: "op",
    completedAt: 1,
    actionsCategory: "manifest-generated",
    nonce: 1,
  });
  assert.match(calls[0].url, /\/api\/privacy\/compliance$/);
});

// --- v2 discovery + DNS ---------------------------------------------------

test("discoverDomain hits /api/v2/discovery/domain/{name}", async () => {
  queueResp({ success: true, data: {} });
  const c = makeClient();
  await c.discoverDomain("foo");
  assert.match(calls[0].url, /\/api\/v2\/discovery\/domain\/foo$/);
});

test("discoverNode / Operator / Quids / TrustedQuids paths", async () => {
  for (let i = 0; i < 4; i++) queueResp({ success: true, data: {} });
  const c = makeClient();
  await c.discoverNode("n");
  await c.discoverOperator("op");
  await c.discoverQuids();
  await c.discoverTrustedQuids();
  assert.match(calls[0].url, /\/api\/v2\/discovery\/node\/n$/);
  assert.match(calls[1].url, /\/api\/v2\/discovery\/operator\/op$/);
  assert.match(calls[2].url, /\/api\/v2\/discovery\/quids$/);
  assert.match(calls[3].url, /\/api\/v2\/discovery\/trusted-quids$/);
});

test("submitDNSClaim posts to /api/v2/dns/claim", async () => {
  queueResp({ success: true, data: { id: "tx" } });
  const c = makeClient();
  await c.submitDNSClaim({
    domain: "example.com",
    ownerQuid: "own",
    rootQuid: "root",
    nonce: 1,
  });
  assert.match(calls[0].url, /\/api\/v2\/dns\/claim$/);
});

test("dns: challenge/attestation/renewal/revocation paths", async () => {
  for (let i = 0; i < 4; i++) queueResp({ success: true, data: { id: "tx" } });
  const c = makeClient();
  await c.submitDNSChallenge({ claimRef: "ref" });
  await c.submitDNSAttestation({ domain: "x", claimRef: "ref" });
  await c.submitDNSRenewal({ attestationRef: "att", nonce: 1 });
  await c.submitDNSRevocation({ attestationRef: "att", reason: "test", nonce: 1 });
  assert.match(calls[0].url, /\/api\/v2\/dns\/challenge$/);
  assert.match(calls[1].url, /\/api\/v2\/dns\/attestation$/);
  assert.match(calls[2].url, /\/api\/v2\/dns\/renewal$/);
  assert.match(calls[3].url, /\/api\/v2\/dns\/revocation$/);
});

test("dns: delegate/delegate-revocation paths", async () => {
  for (let i = 0; i < 2; i++) queueResp({ success: true, data: { id: "tx" } });
  const c = makeClient();
  await c.submitAuthorityDelegate({ rootQuid: "r", delegateQuid: "d", domainScope: "x", nonce: 1 });
  await c.submitAuthorityDelegateRevocation({ delegateRef: "del", reason: "t", nonce: 1 });
  assert.match(calls[0].url, /\/api\/v2\/dns\/delegate$/);
  assert.match(calls[1].url, /\/api\/v2\/dns\/delegate-revocation$/);
});

test("getDNSAttestations + weighted + resolve paths", async () => {
  for (let i = 0; i < 3; i++) queueResp({ success: true, data: {} });
  const c = makeClient();
  await c.getDNSAttestations("example.com");
  await c.getDNSAttestationsWeighted("example.com");
  await c.resolveDNSRecord("example.com", "A");
  assert.match(calls[0].url, /\/api\/v2\/dns\/attestations\/example\.com$/);
  assert.match(calls[1].url, /\/api\/v2\/dns\/attestations\/example\.com\/weighted$/);
  assert.match(calls[2].url, /\/api\/v2\/dns\/resolve\/example\.com\/A$/);
});

test("input validation throws on missing required args", async () => {
  const c = makeClient();
  await assert.rejects(() => c.getPeer(""));
  await assert.rejects(() => c.getTentativeBlocks(""));
  await assert.rejects(() => c.registerDomain(""));
  await assert.rejects(() => c.getTrustEdges(""));
  await assert.rejects(() => c.sendDomainGossip(null));
  await assert.rejects(() => c.createNodeAdvertisement(null));
  await assert.rejects(() => c.createDSR({}));
  await assert.rejects(() => c.getDSRStatus(""));
  await assert.rejects(() => c.createConsentGrant({}));
  await assert.rejects(() => c.createConsentWithdraw({}));
  await assert.rejects(() => c.getConsentHistory(""));
  await assert.rejects(() => c.createProcessingRestriction({}));
  await assert.rejects(() => c.getRestrictionsForSubject(""));
  await assert.rejects(() => c.createDSRCompliance({}));
  await assert.rejects(() => c.getAuditEntry(-1));
  await assert.rejects(() => c.getModerationActions("", "x"));
  await assert.rejects(() => c.discoverDomain(""));
  await assert.rejects(() => c.discoverNode(""));
  await assert.rejects(() => c.discoverOperator(""));
  await assert.rejects(() => c.submitDNSClaim({}));
  await assert.rejects(() => c.submitDNSChallenge({}));
  await assert.rejects(() => c.submitDNSAttestation({}));
  await assert.rejects(() => c.getDNSAttestations(""));
  await assert.rejects(() => c.resolveDNSRecord("", "A"));
});
