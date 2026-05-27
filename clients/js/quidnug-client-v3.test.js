/**
 * Tests for the v3 extension methods.
 *
 * Confirms URL routing: /api/ for moderation/audit/privacy/peers/etc.
 * and /api/v2/ for discovery/DNS attestation.
 *
 * Uses Node's built-in test runner + fetch mock.
 */

import { test } from "node:test";
import assert from "node:assert/strict";

import QuidnugClient from "./quidnug-client-v3.js";

function envelope(data) {
  return new Response(
    JSON.stringify({ success: true, data }),
    { status: 200, headers: { "Content-Type": "application/json" } },
  );
}

function makeClient() {
  const client = new QuidnugClient({ defaultNode: "http://n.local" });
  client.nodes[0] = { url: "http://n.local", status: "healthy" };
  return client;
}

// --- /api/* routing --------------------------------------------------------

test("getAuditHead — routes to /api/audit/head", async () => {
  let hitUrl;
  globalThis.fetch = async (url) => {
    hitUrl = url;
    return envelope({ sequence: 7 });
  };
  const c = makeClient();
  const head = await c.getAuditHead();
  assert.equal(hitUrl, "http://n.local/api/audit/head");
  assert.equal(head.sequence, 7);
});

test("submitModerationAction — routes to /api/moderation/actions", async () => {
  let hitUrl, hitBody;
  globalThis.fetch = async (url, init) => {
    hitUrl = url;
    hitBody = JSON.parse(init.body);
    return envelope({ ok: true });
  };
  const c = makeClient();
  await c.submitModerationAction({ targetType: "review", targetId: "abc", action: "hide" });
  assert.equal(hitUrl, "http://n.local/api/moderation/actions");
  assert.equal(hitBody.action, "hide");
});

test("getModerationActions — routes to /api/moderation/actions/{type}/{id}", async () => {
  let hitUrl;
  globalThis.fetch = async (url) => {
    hitUrl = url;
    return envelope({ actions: [] });
  };
  const c = makeClient();
  await c.getModerationActions("review", "abc 123");
  assert.equal(hitUrl, "http://n.local/api/moderation/actions/review/abc%20123");
});

test("submitDSR — routes to /api/privacy/dsr", async () => {
  let hitUrl;
  globalThis.fetch = async (url) => {
    hitUrl = url;
    return envelope({ txId: "tx-1" });
  };
  const c = makeClient();
  await c.submitDSR({ subjectQuid: "x", action: "delete" });
  assert.equal(hitUrl, "http://n.local/api/privacy/dsr");
});

test("getPeers — routes to /api/peers with query string", async () => {
  let hitUrl;
  globalThis.fetch = async (url) => {
    hitUrl = url;
    return envelope({ peers: [] });
  };
  const c = makeClient();
  await c.getPeers({ limit: 25, offset: 0 });
  assert.equal(hitUrl, "http://n.local/api/peers?limit=25&offset=0");
});

test("submitNodeAdvertisement — routes to /api/node-advertisements", async () => {
  let hitUrl;
  globalThis.fetch = async (url) => {
    hitUrl = url;
    return envelope({ ok: true });
  };
  const c = makeClient();
  await c.submitNodeAdvertisement({ nodeQuid: "n", endpoints: [] });
  assert.equal(hitUrl, "http://n.local/api/node-advertisements");
});

// --- /api/v2/* routing -----------------------------------------------------

test("getDiscoveryDomain — routes to /api/v2/discovery/domain/{name}", async () => {
  let hitUrl;
  globalThis.fetch = async (url) => {
    hitUrl = url;
    return envelope({ domain: "ex.org" });
  };
  const c = makeClient();
  await c.getDiscoveryDomain("ex.org");
  assert.equal(hitUrl, "http://n.local/api/v2/discovery/domain/ex.org");
});

test("submitDNSClaim — routes to /api/v2/dns/claim", async () => {
  let hitUrl, hitBody;
  globalThis.fetch = async (url, init) => {
    hitUrl = url;
    hitBody = JSON.parse(init.body);
    return envelope({ ok: true });
  };
  const c = makeClient();
  await c.submitDNSClaim({ domain: "ex.org", claimantQuid: "q" });
  assert.equal(hitUrl, "http://n.local/api/v2/dns/claim");
  assert.equal(hitBody.domain, "ex.org");
});

test("resolveDNS — routes to /api/v2/dns/resolve/{domain}/{type}", async () => {
  let hitUrl;
  globalThis.fetch = async (url) => {
    hitUrl = url;
    return envelope({ records: [] });
  };
  const c = makeClient();
  await c.resolveDNS("ex.org", "A");
  assert.equal(hitUrl, "http://n.local/api/v2/dns/resolve/ex.org/A");
});

// --- 404 → null for *OrNull endpoints --------------------------------------

test("getAuditEntry — returns null on NOT_FOUND", async () => {
  globalThis.fetch = async () =>
    new Response(
      JSON.stringify({ success: false, error: { code: "NOT_FOUND", message: "absent" } }),
      { status: 404, headers: { "Content-Type": "application/json" } },
    );
  const c = makeClient();
  assert.equal(await c.getAuditEntry(999), null);
});

test("getDiscoveryNode — returns null on NOT_FOUND", async () => {
  globalThis.fetch = async () =>
    new Response(
      JSON.stringify({ success: false, error: { code: "NOT_FOUND", message: "absent" } }),
      { status: 404, headers: { "Content-Type": "application/json" } },
    );
  const c = makeClient();
  assert.equal(await c.getDiscoveryNode("missing"), null);
});

// --- Validation -----------------------------------------------------------

test("v3 methods reject missing required args", async () => {
  const c = makeClient();
  await assert.rejects(() => c.getPeer(undefined), /nodeQuid required/);
  await assert.rejects(() => c.getDiscoveryDomain(""), /name required/);
  await assert.rejects(() => c.getModerationActions("review"), /required/);
  await assert.rejects(() => c.resolveDNS("ex.org"), /required/);
  await assert.rejects(() => c.getDNSAttestations(""), /domain required/);
});
