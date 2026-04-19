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
