/**
 * Popup UI — lock/unlock vault, list quids, quick-generate.
 *
 * Intentionally zero-framework. The popup loads in ~50ms and drops
 * out of memory the moment the user clicks away.
 */

const $ = (id) => document.getElementById(id);

function send(msg) {
    return new Promise((resolve, reject) => {
        chrome.runtime.sendMessage(msg, (reply) => {
            if (!reply?.ok) reject(new Error(reply?.error || "no reply"));
            else resolve(reply.data);
        });
    });
}

async function main() {
    const app = $("app");
    const { unlocked } = await send({ type: "isUnlocked" });

    if (!unlocked) {
        renderUnlock(app);
    } else {
        await renderVault(app);
    }
}

function renderUnlock(app) {
    app.innerHTML = `
        <p>Enter your vault passphrase to unlock.</p>
        <input id="pw" type="password" placeholder="passphrase">
        <div class="row">
            <button id="unlock-btn">Unlock</button>
        </div>
        <p id="err" style="color: #b00; font-size: 12px"></p>
    `;
    $("unlock-btn").onclick = async () => {
        const pw = $("pw").value;
        try {
            const { unlocked } = await send({ type: "unlock", passphrase: pw });
            if (unlocked) await renderVault(app);
            else $("err").textContent = "wrong passphrase";
        } catch (e) {
            $("err").textContent = e.message;
        }
    };
}

async function renderVault(app) {
    const { quids } = await send({ type: "listQuids" });
    app.innerHTML = `
        <div id="list"></div>
        <div class="row">
            <input id="alias" type="text" placeholder="alias (e.g. 'personal')">
            <button id="gen-btn">Generate</button>
        </div>
        <div class="row">
            <button id="lock-btn">Lock</button>
        </div>
    `;
    const list = $("list");
    list.innerHTML = quids.length === 0
        ? "<em>no quids yet</em>"
        : quids.map((q) => `
            <div class="quid">
                <div class="alias">${escapeHtml(q.alias)}</div>
                <div class="id">${q.id}</div>
            </div>
        `).join("");

    $("gen-btn").onclick = async () => {
        const alias = $("alias").value || "default";
        try {
            await send({ type: "generateQuid", alias });
            await renderVault(app);
        } catch (e) {
            alert(e.message);
        }
    };

    $("lock-btn").onclick = async () => {
        await send({ type: "lock" });
        renderUnlock(app);
    };
}

function escapeHtml(s) {
    return String(s).replace(/[&<>"']/g, (c) => ({
        "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
    }[c]));
}

main().catch((e) => {
    document.getElementById("app").textContent = String(e);
});
