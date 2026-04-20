/**
 * Service worker — Quidnug extension backing store.
 *
 * Responsibilities:
 *   1. Hold the user's Quids in chrome.storage.local, encrypted with
 *      a passphrase-derived AES-GCM key.
 *   2. Expose a message-passing API to the content script and popup
 *      for signing / identity queries.
 *   3. Cache the current node URL + bearer token from options.
 *
 * This is intentionally minimal — production extensions should
 * layer WebAuthn-backed key wrapping, audit logging, and origin
 * whitelisting on top.
 */

// ---------------------------------------------------------------------
// Storage helpers
// ---------------------------------------------------------------------

const STORAGE_KEYS = {
    ENCRYPTED_VAULT: "quidnug:vault:enc",
    SALT: "quidnug:vault:salt",
    NODE_URL: "quidnug:node",
    TOKEN: "quidnug:token",
    ORIGINS: "quidnug:allowed-origins",
};

async function getVaultEncrypted() {
    const obj = await chrome.storage.local.get([
        STORAGE_KEYS.ENCRYPTED_VAULT,
        STORAGE_KEYS.SALT,
    ]);
    return {
        enc: obj[STORAGE_KEYS.ENCRYPTED_VAULT],
        salt: obj[STORAGE_KEYS.SALT],
    };
}

async function putVaultEncrypted(enc, salt) {
    await chrome.storage.local.set({
        [STORAGE_KEYS.ENCRYPTED_VAULT]: enc,
        [STORAGE_KEYS.SALT]: salt,
    });
}

// ---------------------------------------------------------------------
// AES-GCM wrapping over passphrase-derived keys
// ---------------------------------------------------------------------

async function deriveKey(passphrase, salt) {
    const pwBytes = new TextEncoder().encode(passphrase);
    const baseKey = await crypto.subtle.importKey(
        "raw", pwBytes, "PBKDF2", false, ["deriveKey"]);
    return crypto.subtle.deriveKey(
        {
            name: "PBKDF2",
            salt,
            iterations: 310_000,
            hash: "SHA-256",
        },
        baseKey,
        { name: "AES-GCM", length: 256 },
        false,
        ["encrypt", "decrypt"]
    );
}

async function encryptVault(vault, passphrase) {
    const salt = crypto.getRandomValues(new Uint8Array(16));
    const iv = crypto.getRandomValues(new Uint8Array(12));
    const key = await deriveKey(passphrase, salt);
    const data = new TextEncoder().encode(JSON.stringify(vault));
    const cipher = await crypto.subtle.encrypt({ name: "AES-GCM", iv }, key, data);
    return {
        enc: {
            iv: Array.from(iv),
            cipher: Array.from(new Uint8Array(cipher)),
        },
        salt: Array.from(salt),
    };
}

async function decryptVault(enc, saltArr, passphrase) {
    const salt = new Uint8Array(saltArr);
    const iv = new Uint8Array(enc.iv);
    const cipher = new Uint8Array(enc.cipher);
    const key = await deriveKey(passphrase, salt);
    const plain = await crypto.subtle.decrypt({ name: "AES-GCM", iv }, key, cipher);
    return JSON.parse(new TextDecoder().decode(plain));
}

// ---------------------------------------------------------------------
// Unlock cache (session-only)
// ---------------------------------------------------------------------

/** In-memory unlocked vault; cleared on every SW restart. */
let unlocked = null; // { quids: [{ alias, id, publicKeyHex, privateKeyHex }], lockAt: number }

function lock() { unlocked = null; }

async function unlock(passphrase) {
    const { enc, salt } = await getVaultEncrypted();
    if (!enc || !salt) {
        // First-time setup: empty vault
        unlocked = { quids: [], lockAt: Date.now() + 5 * 60_000 };
        return true;
    }
    try {
        const vault = await decryptVault(enc, salt, passphrase);
        unlocked = { ...vault, lockAt: Date.now() + 5 * 60_000 };
        return true;
    } catch {
        return false;
    }
}

async function persist(passphrase) {
    if (!unlocked) throw new Error("not unlocked");
    const payload = { quids: unlocked.quids };
    const { enc, salt } = await encryptVault(payload, passphrase);
    await putVaultEncrypted(enc, salt);
}

// ---------------------------------------------------------------------
// ECDSA P-256 signing with WebCrypto (matches every other Quidnug SDK)
// ---------------------------------------------------------------------

async function signWithQuid(quid, canonicalBytes) {
    if (!quid.privateKeyHex) throw new Error("quid is read-only");
    const privDER = hexToBytes(quid.privateKeyHex);
    const key = await crypto.subtle.importKey(
        "pkcs8", privDER, { name: "ECDSA", namedCurve: "P-256" }, false, ["sign"]);
    const ieee = new Uint8Array(await crypto.subtle.sign(
        { name: "ECDSA", hash: "SHA-256" }, key, canonicalBytes));
    // v1.0 canonical form: return 64-byte IEEE-1363 raw (r||s) hex.
    // WebCrypto's ECDSA sign returns this format natively, so no
    // encoding conversion is required. The previous DER conversion
    // produced signatures incompatible with the reference node.
    return bytesToHex(ieee);
}

function hexToBytes(hex) {
    const out = new Uint8Array(hex.length / 2);
    for (let i = 0; i < out.length; i++) {
        out[i] = parseInt(hex.slice(i * 2, i * 2 + 2), 16);
    }
    return out;
}

function bytesToHex(b) {
    let s = "";
    for (const x of b) s += x.toString(16).padStart(2, "0");
    return s;
}

function ieeeToDer(ieee) {
    const half = ieee.length / 2;
    const r = ieee.slice(0, half);
    const s = ieee.slice(half);
    return derSeq([derInt(r), derInt(s)]);
}

function derInt(b) {
    // Strip leading zeros but keep one if high bit would be set.
    let i = 0;
    while (i < b.length - 1 && b[i] === 0) i++;
    let body = b.slice(i);
    if (body[0] & 0x80) {
        const pad = new Uint8Array(body.length + 1);
        pad.set(body, 1);
        body = pad;
    }
    const out = new Uint8Array(body.length + 2);
    out[0] = 0x02;
    out[1] = body.length;
    out.set(body, 2);
    return out;
}

function derSeq(parts) {
    let len = 0;
    for (const p of parts) len += p.length;
    const out = new Uint8Array(len + 2);
    out[0] = 0x30;
    out[1] = len;
    let off = 2;
    for (const p of parts) {
        out.set(p, off);
        off += p.length;
    }
    return out;
}

// ---------------------------------------------------------------------
// Message dispatcher
// ---------------------------------------------------------------------

chrome.runtime.onMessage.addListener((msg, sender, respond) => {
    (async () => {
        try {
            respond({ ok: true, data: await dispatch(msg, sender) });
        } catch (err) {
            respond({ ok: false, error: err.message });
        }
    })();
    return true; // keep the channel open for async respond
});

async function dispatch(msg, sender) {
    const { type } = msg;
    switch (type) {
        case "unlock":
            return { unlocked: await unlock(msg.passphrase) };
        case "lock":
            lock();
            return { locked: true };
        case "isUnlocked":
            return { unlocked: !!unlocked };
        case "listQuids":
            requireUnlock();
            return { quids: unlocked.quids.map(stripPrivate) };
        case "generateQuid":
            requireUnlock();
            return await handleGenerate(msg.alias ?? "default", msg.passphrase);
        case "importQuid":
            requireUnlock();
            return await handleImport(msg.alias, msg.privateKeyHex, msg.passphrase);
        case "signCanonical":
            requireUnlock();
            return await handleSign(msg.quidId, msg.canonicalHex, sender);
        case "setNode":
            await chrome.storage.local.set({ [STORAGE_KEYS.NODE_URL]: msg.url });
            return { ok: true };
        case "getNode":
            return await chrome.storage.local.get([STORAGE_KEYS.NODE_URL, STORAGE_KEYS.TOKEN]);
        default:
            throw new Error("unknown message type: " + type);
    }
}

function requireUnlock() {
    if (!unlocked) throw new Error("vault is locked");
    if (unlocked.lockAt && Date.now() > unlocked.lockAt) {
        unlocked = null;
        throw new Error("vault auto-locked after idle");
    }
    unlocked.lockAt = Date.now() + 5 * 60_000;
}

function stripPrivate(q) {
    return { alias: q.alias, id: q.id, publicKeyHex: q.publicKeyHex };
}

async function handleGenerate(alias, passphrase) {
    const kp = await crypto.subtle.generateKey(
        { name: "ECDSA", namedCurve: "P-256" }, true, ["sign", "verify"]);
    const pub = new Uint8Array(await crypto.subtle.exportKey("raw", kp.publicKey));
    const priv = new Uint8Array(await crypto.subtle.exportKey("pkcs8", kp.privateKey));
    const idBytes = new Uint8Array(await crypto.subtle.digest("SHA-256", pub));
    const id = bytesToHex(idBytes.slice(0, 8));

    const quid = {
        alias,
        id,
        publicKeyHex: bytesToHex(pub),
        privateKeyHex: bytesToHex(priv),
    };
    unlocked.quids.push(quid);
    if (passphrase) await persist(passphrase);
    return { quid: stripPrivate(quid) };
}

async function handleImport(alias, privateKeyHex, passphrase) {
    const privDER = hexToBytes(privateKeyHex);
    const key = await crypto.subtle.importKey(
        "pkcs8", privDER, { name: "ECDSA", namedCurve: "P-256" }, true, ["sign"]);
    // Re-export to get matching SPKI / raw public key.
    const jwk = await crypto.subtle.exportKey("jwk", key);
    const rawPubKey = await crypto.subtle.importKey(
        "jwk", { kty: jwk.kty, crv: jwk.crv, x: jwk.x, y: jwk.y },
        { name: "ECDSA", namedCurve: "P-256" }, true, ["verify"]);
    const pub = new Uint8Array(await crypto.subtle.exportKey("raw", rawPubKey));
    const idBytes = new Uint8Array(await crypto.subtle.digest("SHA-256", pub));
    const id = bytesToHex(idBytes.slice(0, 8));

    const quid = {
        alias, id,
        publicKeyHex: bytesToHex(pub),
        privateKeyHex,
    };
    unlocked.quids.push(quid);
    if (passphrase) await persist(passphrase);
    return { quid: stripPrivate(quid) };
}

async function handleSign(quidId, canonicalHex, sender) {
    const quid = unlocked.quids.find((q) => q.id === quidId);
    if (!quid) throw new Error("quid not found");
    // TODO: show the user a confirmation popup before signing.
    // For the scaffold we auto-approve; production must promptForApproval(sender).
    const sig = await signWithQuid(quid, hexToBytes(canonicalHex));
    return { signature: sig };
}
