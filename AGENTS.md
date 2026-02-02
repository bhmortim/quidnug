This style guide is derived from the Quidnug protocol implementation, focusing on its specific cryptographic, concurrent, and hierarchical domain conventions.

### 1. Cryptographic Naming & Types
*   **Quid as First-Class Entity:** Use the term `Quid` (not User or Account) for identities. IDs should be 16-character hex strings derived from the SHA-256 hash of the public key.
*   **Transaction Typing:** Strictly use `TransactionType` enums (`TRUST`, `IDENTITY`, `TITLE`).
*   **Signatures:** Signatures must be Base64 encoded strings in JSON/JS and raw byte arrays in Go.

### 2. Go: Concurrency & Registry Patterns
*   **Granular Mutexes:** Avoid a single global lock. Use specific `RWMutex` instances for each state registry (e.g., `BlockchainMutex`, `TrustRegistryMutex`, `IdentityRegistryMutex`).
*   **Registry-State Pattern:** Maintain separate "Registries" (Go maps) that act as a materialized view of the blockchain. Update these registries only after a block is validated.
*   **Safe Map Initialization:** Use the "initialize on demand" pattern inside locks:
    ```go
    if _, exists := node.TrustRegistry[tx.Truster]; !exists {
        node.TrustRegistry[tx.Truster] = make(map[string]float64)
    }
    ```
*   **Background Domain Workers:** Use `time.Sleep` loops in goroutines to manage periodic domain tasks like block generation or node discovery.

### 3. JavaScript: Web Crypto & Node Pooling
*   **Web Crypto API:** Favor `window.crypto.subtle` for all ECDSA (P-256) and SHA-256 operations. Do not use external crypto libraries for core primitives.
*   **Healthy Node Selection:** Implement a "Node Pool" that tracks health status. Requests must only be routed to nodes with a `healthy` status, selected via a random load-balancing strategy.
*   **Private Method Convention:** Prefix internal helper methods with an underscore (e.g., `_checkNodeHealth`, `_signData`) even if not enforced by the runtime.

### 4. Domain & Data Conventions
*   **Hierarchical Domains:** Use dot-notation for trust domains (e.g., `sub.domain.com`). Implement logic to "walk up" the domain tree to find parent authorities if a specific subdomain node is unavailable.
*   **Ownership Maps:** Ownership in `TITLE` transactions must always be an array of objects containing `ownerId` and a float `percentage`. Validation must enforce that percentages sum exactly to `100.0`.
*   **Unix Timestamps:** All time-based fields (`timestamp`, `validUntil`, `lastSeen`) must use Unix seconds (int64), not ISO strings.

### 5. API & Error Handling
*   **Transaction Nonces:** For `IDENTITY` updates, use an `updateNonce` that must be strictly greater than the current registry value.
*   **Result Persistence:** When creating local entities (like a Quid), always attempt to register with a node but fall back to "local-only" mode if the network is unreachable, providing a warning instead of a hard failure.