/**
 * Tests for v1 parity additions: health/info/getTentativeBlocks,
 * getTrustEdges, domain management (list/register/ensure/getNode/
 * updateNode), and the commit-wait helpers (waitForIdentity,
 * waitForIdentities, waitForTitle).
 *
 * Uses Node's built-in test runner + a swappable `globalThis.fetch`,
 * matching quidnug-client-v2.test.js. We don't go through the
 * window.crypto mock path because these methods don't touch crypto.
 */

import { test } from "node:test";
import assert from "node:assert/strict";

import QuidnugClient from "./quidnug-client.js";

/**
 * Build a JSON Response from a (success, data) or (error) shape.
 */
function jsonResponse(body, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

/**
 * Spin up a client with a single pre-healthy node, bypassing the
 * async _checkNodeHealth path entirely. Constructing with
 * `defaultNode` would race the test body's fetch mock — `addNode`
 * fires a health probe whose pending response can revert
 * `nodes[0].status` after the test has reassigned it.
 */
function clientWithHealthyNode(url = "http://n.local") {
  const c = new QuidnugClient({});
  c.nodes.push({ url, status: "healthy", lastChecked: Date.now() });
  return c;
}

// ---------------------------------------------------------------------------
// health / info / getTentativeBlocks
// ---------------------------------------------------------------------------

test("health — GET /api/health, returns envelope data", async () => {
  let hitUrl;
  globalThis.fetch = async (url) => {
    hitUrl = url;
    return jsonResponse({ success: true, data: { status: "ok", quidId: "abc" } });
  };
  const c = clientWithHealthyNode();
  const out = await c.health();
  assert.equal(hitUrl, "http://n.local/api/health");
  assert.deepEqual(out, { status: "ok", quidId: "abc" });
});

test("info — GET /api/info, returns envelope data", async () => {
  let hitUrl;
  globalThis.fetch = async (url) => {
    hitUrl = url;
    return jsonResponse({
      success: true,
      data: { version: "1.2.3", features: ["events"] },
    });
  };
  const c = clientWithHealthyNode();
  const out = await c.info();
  assert.equal(hitUrl, "http://n.local/api/info");
  assert.equal(out.version, "1.2.3");
});

test("getTentativeBlocks — URL-encodes domain", async () => {
  let hitUrl;
  globalThis.fetch = async (url) => {
    hitUrl = url;
    return jsonResponse({ success: true, data: { blocks: [] } });
  };
  const c = clientWithHealthyNode();
  await c.getTentativeBlocks("contractors.home/q");
  assert.equal(
    hitUrl,
    "http://n.local/api/blocks/tentative/contractors.home%2Fq",
  );
});

test("getTentativeBlocks — throws on missing domain", async () => {
  const c = clientWithHealthyNode();
  await assert.rejects(
    () => c.getTentativeBlocks(""),
    /Missing required parameter: domain/,
  );
});

// ---------------------------------------------------------------------------
// getTrustEdges
// ---------------------------------------------------------------------------

test("getTrustEdges — unwraps `edges` envelope", async () => {
  globalThis.fetch = async () =>
    jsonResponse({
      success: true,
      data: {
        edges: [
          { truster: "a", trustee: "b", trustLevel: 0.9 },
          { truster: "a", trustee: "c", trustLevel: 0.5 },
        ],
      },
    });
  const c = clientWithHealthyNode();
  const edges = await c.getTrustEdges("a");
  assert.equal(edges.length, 2);
  assert.equal(edges[0].trustee, "b");
});

test("getTrustEdges — falls back to `data` envelope", async () => {
  globalThis.fetch = async () =>
    jsonResponse({
      success: true,
      data: {
        data: [{ truster: "a", trustee: "b", trustLevel: 0.9 }],
      },
    });
  const c = clientWithHealthyNode();
  const edges = await c.getTrustEdges("a");
  assert.equal(edges.length, 1);
  assert.equal(edges[0].trustee, "b");
});

test("getTrustEdges — returns [] when neither field present", async () => {
  globalThis.fetch = async () => jsonResponse({ success: true, data: {} });
  const c = clientWithHealthyNode();
  const edges = await c.getTrustEdges("a");
  assert.deepEqual(edges, []);
});

test("getTrustEdges — throws on missing quidId", async () => {
  const c = clientWithHealthyNode();
  await assert.rejects(
    () => c.getTrustEdges(""),
    /Missing required parameter: quidId/,
  );
});

// ---------------------------------------------------------------------------
// Domain management
// ---------------------------------------------------------------------------

test("listDomains — GET /api/domains", async () => {
  let hitUrl;
  globalThis.fetch = async (url) => {
    hitUrl = url;
    return jsonResponse({
      success: true,
      data: { domains: ["default", "contractors.home"] },
    });
  };
  const c = clientWithHealthyNode();
  const out = await c.listDomains();
  assert.equal(hitUrl, "http://n.local/api/domains");
  assert.deepEqual(out.domains, ["default", "contractors.home"]);
});

test("registerDomain — POST /api/domains with { name } body", async () => {
  let hitUrl, hitBody, hitMethod;
  globalThis.fetch = async (url, init) => {
    hitUrl = url;
    hitMethod = init.method;
    hitBody = JSON.parse(init.body);
    return jsonResponse({ success: true, data: { name: "foo" } });
  };
  const c = clientWithHealthyNode();
  const out = await c.registerDomain("foo");
  assert.equal(hitUrl, "http://n.local/api/domains");
  assert.equal(hitMethod, "POST");
  assert.deepEqual(hitBody, { name: "foo" });
  assert.equal(out.name, "foo");
});

test("registerDomain — throws on missing name", async () => {
  const c = clientWithHealthyNode();
  await assert.rejects(
    () => c.registerDomain(""),
    /Missing required parameter: name/,
  );
});

test("ensureDomain — delegates to registerDomain on success", async () => {
  let hitBody;
  globalThis.fetch = async (url, init) => {
    hitBody = JSON.parse(init.body);
    return jsonResponse({ success: true, data: { name: "bar" } });
  };
  const c = clientWithHealthyNode();
  const out = await c.ensureDomain("bar");
  assert.equal(hitBody.name, "bar");
  assert.equal(out.name, "bar");
});

test("ensureDomain — returns synthetic envelope on `already exists` message", async () => {
  globalThis.fetch = async () =>
    jsonResponse(
      {
        success: false,
        error: { code: "INVALID_REQUEST", message: "domain already exists" },
      },
      400,
    );
  const c = clientWithHealthyNode();
  const out = await c.ensureDomain("bar");
  assert.deepEqual(out, {
    status: "success",
    domain: "bar",
    message: "trust domain already exists",
  });
});

test("ensureDomain — returns synthetic envelope on ALREADY_EXISTS code", async () => {
  globalThis.fetch = async () =>
    jsonResponse(
      {
        success: false,
        error: { code: "ALREADY_EXISTS", message: "exists" },
      },
      409,
    );
  const c = clientWithHealthyNode();
  const out = await c.ensureDomain("bar");
  assert.equal(out.status, "success");
  assert.equal(out.domain, "bar");
});

test("ensureDomain — rethrows non-conflict errors", async () => {
  globalThis.fetch = async () =>
    jsonResponse(
      {
        success: false,
        error: { code: "UNAUTHORIZED", message: "nope" },
      },
      403,
    );
  const c = clientWithHealthyNode();
  await assert.rejects(() => c.ensureDomain("bar"), /nope/);
});

test("getNodeDomains — GET /api/node/domains", async () => {
  let hitUrl;
  globalThis.fetch = async (url) => {
    hitUrl = url;
    return jsonResponse({
      success: true,
      data: { managedDomains: ["default"] },
    });
  };
  const c = clientWithHealthyNode();
  const out = await c.getNodeDomains();
  assert.equal(hitUrl, "http://n.local/api/node/domains");
  assert.deepEqual(out.managedDomains, ["default"]);
});

test("updateNodeDomains — POST /api/node/domains with managedDomains body", async () => {
  let hitUrl, hitBody, hitMethod;
  globalThis.fetch = async (url, init) => {
    hitUrl = url;
    hitMethod = init.method;
    hitBody = JSON.parse(init.body);
    return jsonResponse({ success: true, data: { ok: true } });
  };
  const c = clientWithHealthyNode();
  await c.updateNodeDomains(["a.example", "b.example"]);
  assert.equal(hitUrl, "http://n.local/api/node/domains");
  assert.equal(hitMethod, "POST");
  assert.deepEqual(hitBody, { managedDomains: ["a.example", "b.example"] });
});

test("updateNodeDomains — throws on non-array input", async () => {
  const c = clientWithHealthyNode();
  await assert.rejects(
    () => c.updateNodeDomains("not-an-array"),
    /domains must be an array/,
  );
});

// ---------------------------------------------------------------------------
// Commit-wait helpers
// ---------------------------------------------------------------------------

test("waitForIdentity — returns immediately when identity present", async () => {
  globalThis.fetch = async () =>
    jsonResponse({
      success: true,
      data: { quidId: "abc", name: "Alice" },
    });
  const c = clientWithHealthyNode();
  const rec = await c.waitForIdentity("abc", { timeoutMs: 100, pollIntervalMs: 10 });
  assert.equal(rec.quidId, "abc");
});

test("waitForIdentity — polls until identity appears", async () => {
  let calls = 0;
  globalThis.fetch = async () => {
    calls++;
    if (calls < 3) {
      return jsonResponse(
        { success: false, error: { code: "NOT_FOUND", message: "absent" } },
        404,
      );
    }
    return jsonResponse({ success: true, data: { quidId: "abc" } });
  };
  const c = clientWithHealthyNode();
  const rec = await c.waitForIdentity("abc", { timeoutMs: 1000, pollIntervalMs: 5 });
  assert.equal(rec.quidId, "abc");
  assert.ok(calls >= 3);
});

test("waitForIdentity — throws on timeout", async () => {
  globalThis.fetch = async () =>
    jsonResponse(
      { success: false, error: { code: "NOT_FOUND", message: "absent" } },
      404,
    );
  const c = clientWithHealthyNode();
  await assert.rejects(
    () => c.waitForIdentity("abc", { timeoutMs: 30, pollIntervalMs: 5 }),
    /did not commit within 30ms/,
  );
});

test("waitForIdentity — throws on missing quidId", async () => {
  const c = clientWithHealthyNode();
  await assert.rejects(
    () => c.waitForIdentity(""),
    /Missing required parameter: quidId/,
  );
});

test("waitForIdentities — succeeds for all committed quids", async () => {
  globalThis.fetch = async () =>
    jsonResponse({ success: true, data: { quidId: "x" } });
  const c = clientWithHealthyNode();
  await c.waitForIdentities(["a", "b", "c"], { timeoutMs: 200, pollIntervalMs: 5 });
});

test("waitForIdentities — throws on non-array input", async () => {
  const c = clientWithHealthyNode();
  await assert.rejects(
    () => c.waitForIdentities("nope"),
    /quidIds must be an array/,
  );
});

test("waitForIdentities — shares a single deadline across batch", async () => {
  // Always 404 so every wait blocks until timeout. With a 40ms shared
  // budget the first id should consume most of it and we should be
  // told the batch failed.
  globalThis.fetch = async () =>
    jsonResponse(
      { success: false, error: { code: "NOT_FOUND", message: "absent" } },
      404,
    );
  const c = clientWithHealthyNode();
  await assert.rejects(
    () =>
      c.waitForIdentities(["a", "b"], { timeoutMs: 40, pollIntervalMs: 5 }),
    /did not commit within|not all committed/,
  );
});

test("waitForTitle — returns immediately when title present", async () => {
  globalThis.fetch = async () =>
    jsonResponse({
      success: true,
      data: { assetId: "asset-1", ownershipMap: [] },
    });
  const c = clientWithHealthyNode();
  const rec = await c.waitForTitle("asset-1", { timeoutMs: 100, pollIntervalMs: 5 });
  assert.equal(rec.assetId, "asset-1");
});

test("waitForTitle — throws on timeout", async () => {
  globalThis.fetch = async () =>
    jsonResponse(
      { success: false, error: { code: "NOT_FOUND", message: "absent" } },
      404,
    );
  const c = clientWithHealthyNode();
  await assert.rejects(
    () => c.waitForTitle("asset-1", { timeoutMs: 25, pollIntervalMs: 5 }),
    /title asset-1 did not commit within 25ms/,
  );
});

test("waitForTitle — throws on missing assetId", async () => {
  const c = clientWithHealthyNode();
  await assert.rejects(
    () => c.waitForTitle(""),
    /Missing required parameter: assetId/,
  );
});
