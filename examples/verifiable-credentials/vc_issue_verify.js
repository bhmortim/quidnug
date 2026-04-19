/**
 * Verifiable Credentials on Quidnug — end-to-end issue + verify.
 *
 * Run:
 *   npm install @quidnug/client
 *   node examples/verifiable-credentials/vc_issue_verify.js
 *
 * Assumes a local Quidnug node at http://localhost:8080.
 * Start one via `cd deploy/compose && docker compose up -d`.
 */

import QuidnugClient from "@quidnug/client";
import "@quidnug/client/v2";
import { randomUUID, createHash } from "node:crypto";

const NODE_URL = process.env.QUIDNUG_NODE ?? "http://localhost:8080";
const DOMAIN   = "vc.education";

async function main() {
    const client = new QuidnugClient({ defaultNode: NODE_URL });
    await new Promise((r) => setTimeout(r, 500)); // let health check settle

    // --- 1. Actors -------------------------------------------------------
    const university = await client.generateQuid({ name: "University of Example" });
    const accreditor = await client.generateQuid({ name: "HigherEd Accreditation" });
    const student    = await client.generateQuid({ name: "Alice Student" });
    const employer   = await client.generateQuid({ name: "Acme HR" });

    console.log(`university = ${university.id}`);
    console.log(`accreditor = ${accreditor.id}`);
    console.log(`student    = ${student.id}`);
    console.log(`employer   = ${employer.id}\n`);

    // --- 2. Identity registration ---------------------------------------
    for (const [q, name] of [
        [university, "University of Example"],
        [accreditor, "HigherEd Accreditation"],
        [student,    "Alice Student"],
        [employer,   "Acme HR"],
    ]) {
        const tx = await client.createIdentityTransaction({
            subjectQuid: q.id, domain: DOMAIN, name,
        }, q);
        await client.submitTransaction(tx);
    }

    // --- 3. Trust chain: accreditor --> university; employer --> accreditor
    // This is the chain that makes transitive trust work even when the
    // employer has never heard of the university directly.
    const accreditorVouches = await client.createTrustTransaction({
        trustee: university.id, trustLevel: 0.95,
        domain: DOMAIN, nonce: 1,
        description: "accredited institution",
    }, accreditor);
    await client.submitTransaction(accreditorVouches);

    const employerTrustsAccreditor = await client.createTrustTransaction({
        trustee: accreditor.id, trustLevel: 0.9,
        domain: DOMAIN, nonce: 1,
        description: "standards body we rely on",
    }, employer);
    await client.submitTransaction(employerTrustsAccreditor);
    console.log("✓ trust chain installed\n");

    // --- 4. University issues a degree credential ------------------------
    const credentialId = `urn:uuid:${randomUUID()}`;
    const vcJsonLd = {
        "@context": [
            "https://www.w3.org/2018/credentials/v1",
            "https://www.w3.org/2018/credentials/examples/v1",
        ],
        id: credentialId,
        type: ["VerifiableCredential", "UniversityDegreeCredential"],
        issuer: `quidnug://${university.id}`,
        issuanceDate: new Date().toISOString(),
        credentialSubject: {
            id: `quidnug://${student.id}`,
            degree: {
                type: "BachelorDegree",
                name: "Bachelor of Science in Computer Science",
                gpa: 3.8,
                graduationDate: "2026-05-15",
            },
        },
        proof: {
            // In a real flow, the VC's own proof block would be an
            // Ed25519Signature2020 over the canonicalized document.
            // We leave it abbreviated here to keep the example focused
            // on the Quidnug trust layer.
            type: "Ed25519Signature2020",
            created: new Date().toISOString(),
            verificationMethod: `quidnug://${university.id}#keys-1`,
            proofPurpose: "assertionMethod",
            proofValue: "PLACEHOLDER_VC_PROOF",
        },
    };

    // Record the issuance as an EVENT on the student's stream, signed
    // by the university.
    await client.createEventTransaction({
        subjectId: student.id,
        subjectType: "QUID",
        eventType: "VC_ISSUED",
        domain: DOMAIN,
        payload: {
            credentialId,
            credentialHash: sha256Hex(JSON.stringify(vcJsonLd)),
            vc: vcJsonLd,
        },
    }, university);
    console.log(`✓ credential ${credentialId} issued\n`);

    // --- 5. Employer verifies the credential -----------------------------
    const stream = await client.getStreamEvents(student.id, { limit: 50, domain: DOMAIN });
    const issuances   = stream.events.filter((e) => e.eventType === "VC_ISSUED");
    const revocations = stream.events.filter((e) => e.eventType === "VC_REVOKED");

    const active = issuances.filter((ev) =>
        !revocations.some((rv) => rv.payload?.credentialId === ev.payload?.credentialId)
    );

    console.log(`Found ${issuances.length} issuance(s), ${revocations.length} revocation(s)`);
    console.log(`${active.length} active credential(s) on student's stream\n`);

    for (const ev of active) {
        const vc = ev.payload.vc;
        const issuerQuid = vc.issuer.replace("quidnug://", "");

        const tr = await client.getTrustLevel(employer.id, issuerQuid, DOMAIN, { maxDepth: 5 });
        const verdict = tr.trustLevel >= 0.7 ? "✓ ACCEPT" : "✗ REJECT";

        console.log(`${verdict} credential ${ev.payload.credentialId}`);
        console.log(`  issuer: ${issuerQuid}`);
        console.log(`  degree: ${vc.credentialSubject.degree.name}`);
        console.log(`  employer trust in issuer: ${tr.trustLevel.toFixed(3)}`);
        console.log(`  trust path: ${(tr.trustPath ?? []).join(" -> ") || "direct"}`);
        console.log();
    }
}

function sha256Hex(s) {
    return "sha256:" + createHash("sha256").update(s, "utf8").digest("hex");
}

main().catch((e) => { console.error(e); process.exit(1); });
