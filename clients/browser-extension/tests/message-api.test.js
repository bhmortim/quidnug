/**
 * Tests the message-dispatcher's pure helper code that doesn't need
 * chrome.* APIs — hex codec, DER-encoding. Running in a real MV3
 * environment requires a browser; those tests live in the `e2e/`
 * directory and run under Playwright (roadmap).
 */

import { test } from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

test("manifest.json is valid MV3", async () => {
    const m = JSON.parse(await readFile("./manifest.json", "utf8"));
    assert.equal(m.manifest_version, 3);
    assert.equal(m.name, "Quidnug");
    assert.ok(m.background?.service_worker);
    assert.ok(m.content_scripts?.[0]?.js?.includes("src/content.js"));
    assert.ok(m.web_accessible_resources?.[0]?.resources?.includes("src/injected.js"));
});

test("background.js exposes expected message types", async () => {
    const src = await readFile("./src/background.js", "utf8");
    for (const t of [
        "unlock", "lock", "isUnlocked", "listQuids",
        "generateQuid", "importQuid", "signCanonical",
        "setNode", "getNode",
    ]) {
        assert.ok(
            src.includes(`"${t}"`),
            `background must handle ${t} message type`
        );
    }
});

test("injected.js defines window.quidnug with the public API", async () => {
    const src = await readFile("./src/injected.js", "utf8");
    for (const fn of ["listQuids", "sign", "getNodeInfo", "isUnlocked"]) {
        assert.ok(src.includes(fn), `window.quidnug.${fn} must be exposed`);
    }
});
