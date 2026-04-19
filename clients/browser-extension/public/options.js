const $ = (id) => document.getElementById(id);

(async function init() {
    const { "quidnug:node": node, "quidnug:token": token } =
        await chrome.storage.local.get(["quidnug:node", "quidnug:token"]);
    if (node)  $("node").value = node;
    if (token) $("token").value = token;
})();

$("save").onclick = async () => {
    await chrome.storage.local.set({
        "quidnug:node": $("node").value || "http://localhost:8080",
        "quidnug:token": $("token").value,
    });
    $("status").textContent = "Saved.";
};
