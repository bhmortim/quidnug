# Quidnug Postman collection

`quidnug.postman_collection.json` — importable Postman collection
covering every endpoint on the Quidnug v2 API surface.

## Usage

1. In Postman: **File → Import → Upload Files** → select
   `quidnug.postman_collection.json`.
2. Set the `baseUrl` variable to your node (default
   `http://localhost:8080`).
3. Populate `quidId`, `observer`, `target`, and `domain` on the
   request variables tab.
4. Optional: set `bearerToken` if your node requires auth.

## What's in it

Each folder corresponds to an API area:

- **Health** — `/health`, `/info`, `/nodes`
- **Identity** — register / fetch / registry
- **Trust** — grant / relational-trust query / edges
- **Title** — register / fetch / registry
- **Events + streams** — emit / stream metadata / stream events
- **Guardians** — set-update / recovery init-veto-commit / lookup
- **Gossip** — domain fingerprints, anchor gossip
- **Bootstrap** — nonce snapshots, bootstrap status
- **Fork-block** — submit + status
- **Blocks** — paginated block + tx listing

Transaction POST bodies ship as templates with placeholder
`<hex-DER-sig>` — replace with a real signature from the Python /
Go / JS SDK before sending. For ad-hoc testing, generate the
signature with `quidnug-cli` and paste in.

## Generating a signature for a test transaction

```bash
# Sign a TRUST transaction with quidnug-cli
quidnug-cli trust grant --signer alice.quid.json \
    --trustee bob-id --level 0.9 --domain dev.local --json
# Copy the "signature" field from the returned envelope.
```

## Tests / assertions

Not included in this drop — the collection is for manual exploration.
Automated API validation lives in the `tests/` directories of each
client SDK (`clients/python/tests/`, `clients/rust/tests/`,
`pkg/client/*_test.go`).

## License

Apache-2.0.
