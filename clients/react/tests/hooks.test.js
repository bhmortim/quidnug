/**
 * Smoke tests validating the package exports + source integrity.
 *
 * We deliberately do NOT render React components here — doing so
 * would pull in react-dom + a test renderer, inflating the dev-
 * dependency tree. Rendering tests live under the host app's test
 * suite; these smoke tests validate the module surface.
 */

import { test } from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

test("package.json declares peer deps correctly", async () => {
    const pkg = JSON.parse(await readFile("./package.json", "utf8"));
    assert.equal(pkg.name, "@quidnug/react");
    assert.ok(pkg.peerDependencies["@quidnug/client"]);
    assert.ok(pkg.peerDependencies.react);
});

test("src/index.js re-exports every public symbol", async () => {
    const src = await readFile("./src/index.js", "utf8");
    const expected = [
        "QuidnugProvider", "useQuidnug",
        "useQuid", "useTrust", "useStream", "useIdentity", "useGuardianSet",
        "useRegisterIdentity", "useGrantTrust", "useEmitEvent",
        "TrustBadge", "TrustPath", "GuardianSetCard",
    ];
    for (const sym of expected) {
        assert.ok(src.includes(sym),
            `src/index.js must re-export ${sym}, but substring is missing`);
    }
});

test("every hook module references useQuidnug", async () => {
    for (const f of ["useTrust", "useStream", "useIdentity", "useGuardianSet"]) {
        const src = await readFile(`./src/hooks/${f}.js`, "utf8");
        assert.ok(src.includes("useQuidnug"), `${f} must call useQuidnug`);
    }
});

test("every mutation module references useMutation", async () => {
    for (const f of ["useRegisterIdentity", "useGrantTrust", "useEmitEvent"]) {
        const src = await readFile(`./src/mutations/${f}.js`, "utf8");
        assert.ok(src.includes("useMutation"),
            `${f} must use the useMutation primitive`);
    }
});
