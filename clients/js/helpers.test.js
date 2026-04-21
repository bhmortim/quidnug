/**
 * Live-node integration tests for the v1-wire ergonomics
 * helpers: ensureDomain, waitForIdentity, waitForIdentities,
 * waitForTitle.
 *
 * Skipped unless QUIDNUG_NODE is set and reachable. To run:
 *
 *     BLOCK_INTERVAL=2s ./bin/quidnug &
 *     QUIDNUG_NODE=http://localhost:8080 node --test helpers.test.js
 */

import { test, before } from "node:test";
import assert from "node:assert/strict";

import {
  Quid,
  buildIdentityTx,
  ensureDomain,
  registerDomain,
  waitForIdentity,
  waitForIdentities,
} from "./v1-wire.js";

const NODE = process.env.QUIDNUG_NODE || "http://localhost:8080";
let nodeReachable = false;

before(async () => {
  try {
    const resp = await fetch(`${NODE}/api/health`, {
      signal: AbortSignal.timeout(1000),
    });
    nodeReachable = resp.ok;
  } catch {
    nodeReachable = false;
  }
});

test("ensureDomain registers a fresh domain", async (t) => {
  if (!nodeReachable) {
    t.skip(`no node at ${NODE}; set QUIDNUG_NODE`);
    return;
  }
  const domain = `test.js-helpers.fresh.${Date.now()}`;
  const out = await ensureDomain(NODE, domain);
  assert.ok(out);
});

test("ensureDomain is idempotent on already-exists", async (t) => {
  if (!nodeReachable) {
    t.skip(`no node at ${NODE}; set QUIDNUG_NODE`);
    return;
  }
  const domain = `test.js-helpers.idempotent.${Date.now()}`;
  await ensureDomain(NODE, domain);
  // Second call must not throw.
  const out = await ensureDomain(NODE, domain);
  assert.ok(out);
  assert.equal(out.domain, domain);
});

test("registerDomain rejects on already-exists; ensureDomain swallows it", async (t) => {
  if (!nodeReachable) {
    t.skip(`no node at ${NODE}; set QUIDNUG_NODE`);
    return;
  }
  const domain = `test.js-helpers.reject.${Date.now()}`;
  await registerDomain(NODE, domain);
  // Second registerDomain must throw "already exists".
  let thrown;
  try {
    await registerDomain(NODE, domain);
  } catch (e) {
    thrown = e;
  }
  assert.ok(thrown, "expected registerDomain to throw on duplicate");
  assert.match(
    String(thrown.message).toLowerCase(),
    /already exists/,
    "error must mention 'already exists'",
  );
  // But ensureDomain must not throw.
  await ensureDomain(NODE, domain);
});

test("waitForIdentity returns once committed", async (t) => {
  if (!nodeReachable) {
    t.skip(`no node at ${NODE}; set QUIDNUG_NODE`);
    return;
  }
  const domain = `test.js-helpers.wait.${Date.now()}`;
  await ensureDomain(NODE, domain);

  const q = await Quid.generate();
  const { id, signable } = await buildIdentityTx({
    trustDomain: domain,
    timestamp: Math.floor(Date.now() / 1000),
    publicKey: q.publicKeyHex,
    quidId: q.id,
    name: "js-helper-test",
    // Self-registration: creator is the same quid that's being
    // registered. Required so the node's trust filter doesn't
    // skip the tx during block assembly.
    creator: q.id,
    updateNonce: 1,
    homeDomain: domain,
  });
  const sig = await q.sign(signable);
  const wire = JSON.parse(new TextDecoder().decode(signable));
  wire.signature = sig;
  wire.id = id;

  const resp = await fetch(`${NODE}/api/v1/transactions/identity`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(wire),
  });
  assert.equal(resp.status, 200, await resp.text());

  const rec = await waitForIdentity(NODE, q.id, {
    domain,
    timeoutMs: 30_000,
    pollMs: 500,
  });
  assert.equal(rec.quidId, q.id);
});

test("waitForIdentity times out on unknown quid", async (t) => {
  if (!nodeReachable) {
    t.skip(`no node at ${NODE}; set QUIDNUG_NODE`);
    return;
  }
  let thrown;
  try {
    await waitForIdentity(NODE, "0000000000000000", {
      timeoutMs: 500,
      pollMs: 200,
    });
  } catch (e) {
    thrown = e;
  }
  assert.ok(thrown);
  assert.match(String(thrown.message), /did not commit within/);
});
