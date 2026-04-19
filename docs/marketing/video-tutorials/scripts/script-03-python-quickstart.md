# Script 03 — Python SDK quickstart

**Length:** 5:00
**Audience:** Python developers (FastAPI / Django / data / ML).
**Goal:** Viewer pauses, runs the same code, and it works.
**Recommended stack:** Loom AI or Descript + cloned voiceover
from ElevenLabs + VS Code with large font.

---

## Pre-flight

Before recording:

1. Fresh venv, Python 3.11+.
2. Docker running, local node up:
   `cd deploy/compose && docker compose up -d`.
3. Browser tab open to **github.com/bhmortim/quidnug**.
4. Terminal + editor side by side.
5. Recording resolution: 1920×1080. Fonts: 18–20 px terminal,
   18 px code editor — viewers on phones need to read this.

---

## Script

### [0:00–0:20] Intro

> "In this video we'll set up the Python SDK for Quidnug from
> scratch, register two identities, create a trust
> relationship, and query relational trust — all in under five
> minutes."
>
> "If you want to skip ahead, the same workflow is in the
> quickstart example in the repo. I'll link it in the
> description."

*[cut to terminal]*

### [0:20–1:00] Install

> "First, fresh Python environment."

```bash
python -m venv venv
source venv/bin/activate   # or .\venv\Scripts\Activate.ps1 on Windows
pip install quidnug
```

> "Pip pulls in the Python SDK, which is built on the
> `cryptography` library and `requests`. No other dependencies."
>
> "Let's verify it installed:"

```bash
python -c "import quidnug; print(quidnug.__version__)"
```

> "Two point zero — we're ready."

### [1:00–1:40] Start a local node

> "If you already have a Quidnug node somewhere, skip this. I'll
> use the Docker Compose dev setup — three nodes, Prometheus,
> Grafana, all wired up."

```bash
git clone https://github.com/bhmortim/quidnug.git
cd quidnug/deploy/compose
docker compose up -d
curl http://localhost:8081/api/health
```

> "All three nodes are up. We'll talk to the first one on
> port 8081."

### [1:40–3:20] Write the code

> "Open a new file: `demo.py`."

```python
from quidnug import Quid, QuidnugClient

client = QuidnugClient("http://localhost:8081")

alice = Quid.generate()
bob = Quid.generate()
print(f"alice = {alice.id}")
print(f"bob   = {bob.id}")

client.register_identity(alice, name="Alice", home_domain="demo")
client.register_identity(bob, name="Bob", home_domain="demo")

client.grant_trust(alice, trustee=bob.id, level=0.9, domain="demo")

tr = client.get_trust(alice.id, bob.id, domain="demo")
print(f"trust = {tr.trust_level:.3f}")
print(f"path  = {' -> '.join(tr.path)}")
```

> "Let's walk through this."
>
> "`Quid.generate()` creates a fresh ECDSA P-256 keypair. The
> quid ID is the first 16 hex chars of the public key's SHA-256
> — the same ID every other Quidnug SDK produces for the same
> keypair."
>
> "`register_identity` submits a signed IDENTITY transaction.
> The SDK handles canonical-byte computation and signing
> internally."
>
> "`grant_trust` issues a trust edge from Alice to Bob with
> level 0.9 in the 'demo' domain."
>
> "`get_trust` runs a relational-trust query from Alice's
> perspective to Bob. Since there's a direct edge, the result
> is 0.9 via a length-1 path."

### [3:20–3:50] Run it

```bash
python demo.py
```

> "The output:"

```
alice = d1f2a3b4...
bob   = e5f6g7h8...
trust = 0.900
path  = d1f2a3b4 -> e5f6g7h8
```

> "We have a Quidnug network with two identities, a trust
> relationship, and a queryable score."

### [3:50–4:30] One more thing — transitive trust

> "Let's show why relational trust is the interesting part.
> Add a third actor."

```python
carol = Quid.generate()
client.register_identity(carol, name="Carol", home_domain="demo")

# Bob trusts Carol at 0.8
client.grant_trust(bob, trustee=carol.id, level=0.8, domain="demo")

# What's Alice's transitive trust in Carol?
# Alice has NO direct edge to Carol.
tr = client.get_trust(alice.id, carol.id, domain="demo")
print(f"alice->carol trust = {tr.trust_level:.3f}")
print(f"path = {' -> '.join(tr.path)}")
```

> "Run it again:"

*[re-run]*

```
alice->carol trust = 0.720
path = alice -> bob -> carol
```

> "Alice has never met Carol. But because she trusts Bob at
> 0.9, and Bob trusts Carol at 0.8, Alice's transitive trust in
> Carol is 0.9 × 0.8 = 0.72."
>
> "This is what makes Quidnug different from a certificate
> authority or a blockchain reputation system. Trust composes
> along the graph, per-observer."

### [4:30–5:00] Where to go next

> "That's the core. From here, check out:"
>
> "- The async client if you're in FastAPI or asyncio."
>
> "- The full protocol surface — guardians, event streams,
>   cross-domain gossip, Merkle proofs."
>
> "- The AI-agents example if you're working with LLM
>   attribution."
>
> "- The comparison docs if you're weighing Quidnug against
>   DIDs + Verifiable Credentials, PGP, or blockchain
>   reputation."
>
> "All linked in the description. Apache-2.0. See you in the
> next video."

---

## Editing checklist

- [ ] Cut ANY um / uh / "so let's see" — Descript filler-word
      removal.
- [ ] Add chapter markers at 0:20, 1:00, 1:40, 3:20, 4:30.
- [ ] Burn-in captions.
- [ ] Zoom in on the terminal at the "output" moment (3:30).
- [ ] Export one horizontal 1080p + one vertical 60-second cut
      showing just the `pip install` → output moment.

## Template for other SDKs

Swap the install commands and code; the structure is identical.
- Go: `go get github.com/quidnug/quidnug/pkg/client`, Go script.
- Rust: `cargo add quidnug`, Rust script.
- JS: `npm install @quidnug/client`, JS script.
- Java: Maven dep snippet + Java script.
- .NET: `dotnet add package Quidnug.Client`.
- Swift: SPM dep + Swift file.

## License

Apache-2.0.
