/**
 * Tests for the v2 extension methods.
 *
 * Uses Node's built-in test runner + fetch mock. Matches the style of
 * quidnug-client.test.js.
 */

import { test } from "node:test";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";

import QuidnugClient from "./quidnug-client-v2.js";

function sh(data) {
  return new Uint8Array(createHash("sha256").update(data).digest());
}

function hex(buf) {
  let s = "";
  for (const b of buf) s += b.toString(16).padStart(2, "0");
  return s;
}

// --- Inclusion proof -------------------------------------------------------

test("verifyInclusionProof — single sibling right", async () => {
  const tx = new TextEncoder().encode("tx-1");
  const sibling = sh("tx-2");
  const leaf = sh(Buffer.from(tx));
  const root = sh(Buffer.concat([Buffer.from(leaf), Buffer.from(sibling)]));
  const frames = [{ hash: hex(sibling), side: "right" }];
  const ok = await QuidnugClient.verifyInclusionProof(tx, frames, hex(root));
  assert.equal(ok, true);
});

test("verifyInclusionProof — single sibling left", async () => {
  const tx = new TextEncoder().encode("tx-2");
  const sibling = sh("tx-1");
  const leaf = sh(Buffer.from(tx));
  const root = sh(Buffer.concat([Buffer.from(sibling), Buffer.from(leaf)]));
  const frames = [{ hash: hex(sibling), side: "left" }];
  const ok = await QuidnugClient.verifyInclusionProof(tx, frames, hex(root));
  assert.equal(ok, true);
});

test("verifyInclusionProof — tampered tx rejected", async () => {
  const tx = new TextEncoder().encode("tx-1");
  const sibling = sh("tx-2");
  const leaf = sh(Buffer.from(tx));
  const root = sh(Buffer.concat([Buffer.from(leaf), Buffer.from(sibling)]));
  const frames = [{ hash: hex(sibling), side: "right" }];
  const ok = await QuidnugClient.verifyInclusionProof(
    new TextEncoder().encode("tampered"),
    frames,
    hex(root),
  );
  assert.equal(ok, false);
});

test("verifyInclusionProof — malformed frame rejected", async () => {
  await assert.rejects(
    () => QuidnugClient.verifyInclusionProof(
      new TextEncoder().encode("x"),
      [{ hash: "nothex", side: "right" }],
      "aa".repeat(32),
    ),
  );
  await assert.rejects(
    () => QuidnugClient.verifyInclusionProof(
      new TextEncoder().encode("x"),
      [{ hash: "aa".repeat(32), side: "middle" }],
      "aa".repeat(32),
    ),
  );
});

test("verifyInclusionProof — accepts string txBytes as UTF-8", async () => {
  const tx = "tx-1";
  const sibling = sh("tx-2");
  const leaf = sh(Buffer.from(tx, "utf8"));
  const root = sh(Buffer.concat([Buffer.from(leaf), Buffer.from(sibling)]));
  const frames = [{ hash: hex(sibling), side: "right" }];
  const ok = await QuidnugClient.verifyInclusionProof(tx, frames, hex(root));
  assert.equal(ok, true);
});

// --- Canonical bytes -------------------------------------------------------

test("canonicalBytes — stable across key order", () => {
  const a = QuidnugClient.canonicalBytes({ b: 1, a: 2 });
  const b = QuidnugClient.canonicalBytes({ a: 2, b: 1 });
  assert.equal(Buffer.from(a).toString(), Buffer.from(b).toString());
});

test("canonicalBytes — excludes named fields", () => {
  const out = QuidnugClient.canonicalBytes(
    { type: "TRUST", signature: "abc", level: 0.9 },
    ["signature"],
  );
  const s = Buffer.from(out).toString();
  assert.ok(!s.includes("signature"));
  assert.ok(s.includes("level"));
});

test("canonicalBytes — sorts nested keys", () => {
  const out = QuidnugClient.canonicalBytes({
    nested: { z: 1, a: 2 },
    outer: "x",
  });
  assert.equal(Buffer.from(out).toString(), '{"nested":{"a":2,"z":1},"outer":"x"}');
});

// --- Guardian method routing (happy path, smoke test) ---------------------

test("submitGuardianSetUpdate — routes to /guardian/set-update", async () => {
  let hitUrl, hitBody;
  globalThis.fetch = async (url, init) => {
    hitUrl = url;
    hitBody = JSON.parse(init.body);
    return new Response(
      JSON.stringify({ success: true, data: { ok: true } }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    );
  };
  const client = new QuidnugClient({ defaultNode: "http://n.local" });
  // _checkNodeHealth runs async; mark node healthy directly so we don't
  // have to wait for it.
  client.nodes[0] = { url: "http://n.local", status: "healthy" };

  await client.submitGuardianSetUpdate({
    subjectQuid: "abc",
    newSet: { subjectQuid: "abc", guardians: [], threshold: 1, recoveryDelaySeconds: 60 },
    anchorNonce: 1,
    validFrom: 0,
  });

  assert.equal(hitUrl, "http://n.local/api/guardian/set-update");
  assert.equal(hitBody.subjectQuid, "abc");
});

test("getGuardianSet — returns null on 404", async () => {
  globalThis.fetch = async () =>
    new Response(
      JSON.stringify({ success: false, error: { code: "NOT_FOUND", message: "absent" } }),
      { status: 404, headers: { "Content-Type": "application/json" } },
    );
  const client = new QuidnugClient({ defaultNode: "http://n.local" });
  client.nodes[0] = { url: "http://n.local", status: "healthy" };

  const set = await client.getGuardianSet("missing");
  assert.equal(set, null);
});

// --- Moderation / audit / privacy routing (smoke tests) -------------------

function _stubOnce(body = { success: true, data: { ok: true } }, status = 200) {
  const calls = { url: null, method: null, body: null };
  globalThis.fetch = async (url, init = {}) => {
    calls.url = url;
    calls.method = init.method ?? "GET";
    calls.body = init.body ? JSON.parse(init.body) : null;
    return new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    });
  };
  return calls;
}

function _readyClient() {
  const client = new QuidnugClient({ defaultNode: "http://n.local" });
  client.nodes[0] = { url: "http://n.local", status: "healthy" };
  return client;
}

test("submitModerationAction — routes to /moderation/actions", async () => {
  const calls = _stubOnce();
  await _readyClient().submitModerationAction({
    targetType: "EVENT", targetId: "ev-1",
    actionType: "TAKEDOWN", moderatorQuid: "mod", reason: "spam",
  });
  assert.equal(calls.url, "http://n.local/api/moderation/actions");
  assert.equal(calls.method, "POST");
  assert.equal(calls.body.targetId, "ev-1");
});

test("submitModerationAction — rejects missing targetId", async () => {
  await assert.rejects(
    () => _readyClient().submitModerationAction({ actionType: "TAKEDOWN" }),
    /targetId required/,
  );
});

test("getModerationActions — encodes targetType/targetId into URL", async () => {
  const calls = _stubOnce({ success: true, data: { data: [{ id: "a1" }] } });
  const actions = await _readyClient().getModerationActions("QUID", "user with space");
  assert.equal(
    calls.url,
    "http://n.local/api/moderation/actions/QUID/user%20with%20space",
  );
  assert.deepEqual(actions, [{ id: "a1" }]);
});

test("getModerationActions — rejects invalid targetType", async () => {
  await assert.rejects(
    () => _readyClient().getModerationActions("REVIEW", "id"),
    /targetType must be/,
  );
});

test("getAuditHead — routes to /audit/head", async () => {
  const calls = _stubOnce({ success: true, data: { sequence: 42, hash: "deadbeef" } });
  const head = await _readyClient().getAuditHead();
  assert.equal(calls.url, "http://n.local/api/audit/head");
  assert.equal(head.sequence, 42);
});

test("getAuditEntries — passes since/limit", async () => {
  const calls = _stubOnce({ success: true, data: { data: [], pagination: {} } });
  await _readyClient().getAuditEntries({ since: 100, limit: 10 });
  assert.equal(calls.url, "http://n.local/api/audit/entries?since=100&limit=10");
});

test("getAuditEntry — encodes sequence as number", async () => {
  const calls = _stubOnce({ success: true, data: { sequence: 7 } });
  await _readyClient().getAuditEntry(7);
  assert.equal(calls.url, "http://n.local/api/audit/entry/7");
});

test("submitDSR — routes to /privacy/dsr", async () => {
  const calls = _stubOnce({ success: true, data: { requestTxId: "tx-1" } });
  const out = await _readyClient().submitDSR({
    subjectQuid: "alice", requestType: "ACCESS",
  });
  assert.equal(calls.url, "http://n.local/api/privacy/dsr");
  assert.equal(out.requestTxId, "tx-1");
});

test("getDSRStatus — encodes requestTxId", async () => {
  const calls = _stubOnce({ success: true, data: { status: "COMPLETED" } });
  await _readyClient().getDSRStatus("tx-1");
  assert.equal(calls.url, "http://n.local/api/privacy/dsr/tx-1");
});

test("getConsentHistory — passes both query params", async () => {
  const calls = _stubOnce({ success: true, data: { events: [] } });
  await _readyClient().getConsentHistory({ subjectQuid: "alice", processorQuid: "acme" });
  assert.equal(
    calls.url,
    "http://n.local/api/privacy/consent/history?subjectQuid=alice&processorQuid=acme",
  );
});

test("getRestrictionsForSubject — returns array unwrapped", async () => {
  _stubOnce({ success: true, data: { data: [{ id: "r1" }, { id: "r2" }] } });
  const out = await _readyClient().getRestrictionsForSubject("alice");
  assert.deepEqual(out, [{ id: "r1" }, { id: "r2" }]);
});
