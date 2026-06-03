//! Async HTTP client (reqwest-based) mirroring Python / Go SDKs.

use crate::crypto::Quid;
use crate::error::{Error, Result};
use crate::types::{IdentityRecord, OwnershipStake, Title, TrustEdge, TrustResult};
use crate::wire::{EventTx, IdentityTx, TitleTx, TrustTx, WireOwnershipStake};
use reqwest::{Client as HttpClient, StatusCode};
use serde::de::DeserializeOwned;
use serde_json::Value;
use std::time::Duration;

/// Async Quidnug HTTP client.
#[derive(Clone, Debug)]
pub struct Client {
    http: HttpClient,
    api_base: String,
    timeout: Duration,
}

/// Parameters for [`Client::grant_trust`].
#[derive(Debug, Clone)]
pub struct TrustParams<'a> {
    /// Trustee quid ID.
    pub trustee: &'a str,
    /// Trust level in [0, 1].
    pub level: f64,
    /// Trust domain.
    pub domain: &'a str,
    /// Monotonic nonce.
    pub nonce: i64,
}

/// Parameters for [`Client::register_title`].
#[derive(Debug, Clone)]
pub struct TitleParams<'a> {
    /// Asset id (subject quid of the title).
    pub asset_id: &'a str,
    /// Ownership stakes (sum to 1.0; 100-scale also accepted).
    pub owners: Vec<OwnershipStake>,
    /// Trust domain the title lives in.
    pub domain: &'a str,
    /// Optional title-type discriminator.
    pub title_type: Option<&'a str>,
}

/// Parameters for [`Client::emit_event`].
#[derive(Debug, Clone)]
pub struct EventParams<'a> {
    /// Subject quid id (the entity emitting the event).
    pub subject_id: &'a str,
    /// `"QUID"` or `"TITLE"`.
    pub subject_type: &'a str,
    /// Event type discriminator.
    pub event_type: &'a str,
    /// Trust domain.
    pub domain: &'a str,
    /// Inline payload (mutually exclusive with `payload_cid`).
    pub payload: Option<Value>,
    /// IPFS CID for off-chain payload (mutually exclusive with `payload`).
    pub payload_cid: Option<&'a str>,
    /// Optional explicit sequence number; defaults to the next sequence
    /// inferred from the existing stream, falling back to 1.
    pub sequence: Option<i64>,
}

impl Client {
    /// Construct a new client against `base_url` (e.g. `http://localhost:8080`).
    pub fn new(base_url: &str) -> Result<Self> {
        let trimmed = base_url.trim_end_matches('/');
        let api_base = format!("{trimmed}/api");
        let http = HttpClient::builder()
            .timeout(Duration::from_secs(30))
            .build()?;
        Ok(Self {
            http,
            api_base,
            timeout: Duration::from_secs(30),
        })
    }

    /// Health check (GET /api/health).
    pub async fn health(&self) -> Result<Value> {
        self.get("health").await
    }

    /// Info (GET /api/info).
    pub async fn info(&self) -> Result<Value> {
        self.get("info").await
    }

    /// Register an identity for `signer`, optionally with `name` and `home_domain`.
    ///
    /// v1.0 conformant: builds a typed `IdentityTx` wire struct
    /// whose field order matches `core.IdentityTransaction`,
    /// derives the tx ID using the same seed the server uses,
    /// signs with IEEE-1363, submits.
    pub async fn register_identity(
        &self,
        signer: &Quid,
        name: &str,
        home_domain: &str,
    ) -> Result<Value> {
        if !signer.has_private_key() {
            return Err(Error::validation("signer must have a private key"));
        }
        let mut tx = IdentityTx {
            id: String::new(),
            tx_type: "IDENTITY",
            trust_domain: "default",
            timestamp: now_secs(),
            signature: String::new(),
            public_key: signer.public_key_hex(),
            quid_id: signer.id(),
            name,
            description: "",
            attributes: None,
            creator: signer.id(),
            update_nonce: 1,
            home_domain,
        };
        tx.id = tx.derive_id();
        let signable = serde_json::to_vec(&tx)?;
        tx.signature = signer.sign(&signable)?;
        let body = serde_json::to_value(&tx)?;
        self.post("transactions/identity", &body).await
    }

    /// Fetch an identity record or `None` on 404.
    pub async fn get_identity(
        &self,
        quid_id: &str,
        domain: &str,
    ) -> Result<Option<IdentityRecord>> {
        let mut path = format!("identity/{}", urlencoding(quid_id));
        if !domain.is_empty() {
            path.push_str(&format!("?domain={}", urlencoding(domain)));
        }
        match self.get_typed::<IdentityRecord>(&path).await {
            Ok(r) => Ok(Some(r)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// Submit a signed TRUST transaction.
    ///
    /// v1.0 conformant: typed `TrustTx` wire struct + IEEE-1363
    /// signature + server-compatible ID derivation.
    pub async fn grant_trust<'a>(&self, signer: &Quid, p: TrustParams<'a>) -> Result<Value> {
        if !signer.has_private_key() {
            return Err(Error::validation("signer must have a private key"));
        }
        if !(0.0..=1.0).contains(&p.level) {
            return Err(Error::validation("level must be in [0, 1]"));
        }
        let mut tx = TrustTx {
            id: String::new(),
            tx_type: "TRUST",
            trust_domain: p.domain,
            timestamp: now_secs(),
            signature: String::new(),
            public_key: signer.public_key_hex(),
            truster: signer.id(),
            trustee: p.trustee,
            trust_level: p.level,
            nonce: p.nonce,
            description: "",
            valid_until: 0,
        };
        tx.id = tx.derive_id();
        let signable = serde_json::to_vec(&tx)?;
        tx.signature = signer.sign(&signable)?;
        let body = serde_json::to_value(&tx)?;
        self.post("transactions/trust", &body).await
    }

    /// Query relational trust.
    pub async fn get_trust(
        &self,
        observer: &str,
        target: &str,
        domain: &str,
        max_depth: u32,
    ) -> Result<TrustResult> {
        let path = format!(
            "trust/{}/{}?domain={}&maxDepth={}",
            urlencoding(observer),
            urlencoding(target),
            urlencoding(domain),
            max_depth
        );
        self.get_typed(&path).await
    }

    /// Fetch a title or `None` on 404.
    pub async fn get_title(&self, asset_id: &str, domain: &str) -> Result<Option<Title>> {
        let mut path = format!("title/{}", urlencoding(asset_id));
        if !domain.is_empty() {
            path.push_str(&format!("?domain={}", urlencoding(domain)));
        }
        match self.get_typed::<Title>(&path).await {
            Ok(r) => Ok(Some(r)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// Get a quid's direct outbound trust edges.
    // --- Domain helpers -----------------------------------------------
    //
    // Every non-default trust domain must be registered with the
    // node before any identity / trust / title / event tx in that
    // domain will be accepted. `ensure_domain` is the idempotent
    // bootstrap helper to call once during app startup.

    /// Register a new trust domain. Fails with an "already exists"
    /// error if the domain is already known; see [`ensure_domain`]
    /// for an idempotent wrapper.
    pub async fn register_domain(&self, domain: &str) -> Result<Value> {
        let body = serde_json::json!({ "name": domain });
        self.post("domains", &body).await
    }

    /// Idempotent domain registration. Returns normally on both
    /// fresh-register and already-exists; propagates any other
    /// error.
    pub async fn ensure_domain(&self, domain: &str) -> Result<Value> {
        match self.register_domain(domain).await {
            Ok(v) => Ok(v),
            Err(e) => {
                let msg = format!("{}", e).to_lowercase();
                if msg.contains("already exists") {
                    Ok(serde_json::json!({
                        "status": "success",
                        "domain": domain,
                        "message": "trust domain already exists",
                    }))
                } else {
                    Err(e)
                }
            }
        }
    }

    // --- Commit-wait helpers ------------------------------------------
    //
    // Identity and title transactions live in the node's pending
    // pool until the next block is sealed. Code that immediately
    // emits events or title transactions referencing the new quid
    // must wait for commit first; these helpers poll until the
    // record is visible in the committed registry.

    /// Block until the identity with `quid_id` is visible in the
    /// committed registry, or return `Error::Timeout` when the
    /// caller-supplied deadline expires.
    pub async fn wait_for_identity(
        &self,
        quid_id: &str,
        domain: &str,
        timeout: std::time::Duration,
        poll_interval: std::time::Duration,
    ) -> Result<IdentityRecord> {
        let deadline = std::time::Instant::now() + timeout;
        loop {
            if let Some(rec) = self.get_identity(quid_id, domain).await? {
                return Ok(rec);
            }
            if std::time::Instant::now() >= deadline {
                return Err(Error::validation(format!(
                    "identity {} did not commit within {:?}",
                    quid_id, timeout
                )));
            }
            tokio::time::sleep(poll_interval).await;
        }
    }

    /// Block until every listed quid_id is committed, sharing one
    /// total deadline across the whole batch.
    pub async fn wait_for_identities(
        &self,
        quid_ids: &[&str],
        domain: &str,
        timeout: std::time::Duration,
        poll_interval: std::time::Duration,
    ) -> Result<()> {
        let deadline = std::time::Instant::now() + timeout;
        for qid in quid_ids {
            let remaining = deadline.saturating_duration_since(std::time::Instant::now());
            if remaining.is_zero() {
                return Err(Error::validation(format!(
                    "identities not all committed within {:?} (blocked on {})",
                    timeout, qid
                )));
            }
            self.wait_for_identity(qid, domain, remaining, poll_interval)
                .await?;
        }
        Ok(())
    }

    /// Block until the title with `asset_id` is visible in the
    /// committed registry. Analogous to
    /// [`wait_for_identity`][Self::wait_for_identity]; required
    /// before emitting events on a freshly-registered title.
    pub async fn wait_for_title(
        &self,
        asset_id: &str,
        domain: &str,
        timeout: std::time::Duration,
        poll_interval: std::time::Duration,
    ) -> Result<Title> {
        let deadline = std::time::Instant::now() + timeout;
        loop {
            if let Some(t) = self.get_title(asset_id, domain).await? {
                return Ok(t);
            }
            if std::time::Instant::now() >= deadline {
                return Err(Error::validation(format!(
                    "title {} did not commit within {:?}",
                    asset_id, timeout
                )));
            }
            tokio::time::sleep(poll_interval).await;
        }
    }

    /// Returns the outbound trust edges (subject → object) anchored
    /// at `quid_id`. The node may envelope the result either at the
    /// canonical `edges` field or inside the generic `data` field
    /// of the response envelope; both shapes are tolerated for
    /// forward compatibility with intermediate node releases.
    pub async fn get_trust_edges(&self, quid_id: &str) -> Result<Vec<TrustEdge>> {
        #[derive(serde::Deserialize)]
        struct Wrap {
            #[serde(default)]
            edges: Vec<TrustEdge>,
            #[serde(default)]
            data: Vec<TrustEdge>,
        }
        let w: Wrap = self
            .get_typed(&format!("trust/edges/{}", urlencoding(quid_id)))
            .await?;
        if !w.edges.is_empty() {
            Ok(w.edges)
        } else {
            Ok(w.data)
        }
    }

    // --- Nodes / blocks / pending transactions -------------------------

    /// GET /api/nodes — list known peers.
    pub async fn nodes(&self) -> Result<Value> {
        self.get("nodes").await
    }

    /// GET /api/blocks — paginated committed-block listing.
    pub async fn get_blocks(
        &self,
        limit: Option<u64>,
        offset: Option<u64>,
    ) -> Result<Value> {
        let qs = build_query(&[("limit", limit_str(limit)), ("offset", limit_str(offset))]);
        self.get(&format!("blocks{qs}")).await
    }

    /// GET /api/blocks/tentative/{domain} — tentative (pre-commit) blocks
    /// for a domain.
    pub async fn get_tentative_blocks(&self, domain: &str) -> Result<Value> {
        self.get(&format!("blocks/tentative/{}", urlencoding(domain)))
            .await
    }

    /// GET /api/transactions — pending (mempool) transactions.
    pub async fn get_pending_transactions(&self) -> Result<Value> {
        self.get("transactions").await
    }

    // --- Domains -------------------------------------------------------

    /// GET /api/domains — list registered trust domains.
    pub async fn list_domains(&self) -> Result<Value> {
        self.get("domains").await
    }

    /// GET /api/node/domains — list domains the node is configured to
    /// manage.
    pub async fn get_node_domains(&self) -> Result<Value> {
        self.get("node/domains").await
    }

    /// POST /api/node/domains — replace the node's managed-domain list.
    pub async fn update_node_domains(&self, domains: &[&str]) -> Result<Value> {
        let body = serde_json::json!({ "managedDomains": domains });
        self.post("node/domains", &body).await
    }

    // --- Registry queries ----------------------------------------------

    /// GET /api/registry/identity — paginated identity-registry dump
    /// (or single-quid lookup when `quid_id` is supplied).
    pub async fn query_identity_registry(
        &self,
        quid_id: Option<&str>,
        limit: Option<u64>,
        offset: Option<u64>,
    ) -> Result<Value> {
        let qs = build_query(&[
            ("quid_id", quid_id.map(|s| s.to_string())),
            ("limit", limit_str(limit)),
            ("offset", limit_str(offset)),
        ]);
        self.get(&format!("registry/identity{qs}")).await
    }

    /// GET /api/registry/trust — paginated trust-edge listing, optionally
    /// filtered by truster and/or trustee.
    pub async fn query_trust_registry(
        &self,
        truster: Option<&str>,
        trustee: Option<&str>,
        limit: Option<u64>,
        offset: Option<u64>,
    ) -> Result<Value> {
        let qs = build_query(&[
            ("truster", truster.map(|s| s.to_string())),
            ("trustee", trustee.map(|s| s.to_string())),
            ("limit", limit_str(limit)),
            ("offset", limit_str(offset)),
        ]);
        self.get(&format!("registry/trust{qs}")).await
    }

    /// GET /api/registry/title — paginated title-registry listing,
    /// optionally filtered by asset and/or owner.
    pub async fn query_title_registry(
        &self,
        asset_id: Option<&str>,
        owner_id: Option<&str>,
        limit: Option<u64>,
        offset: Option<u64>,
    ) -> Result<Value> {
        let qs = build_query(&[
            ("asset_id", asset_id.map(|s| s.to_string())),
            ("owner_id", owner_id.map(|s| s.to_string())),
            ("limit", limit_str(limit)),
            ("offset", limit_str(offset)),
        ]);
        self.get(&format!("registry/title{qs}")).await
    }

    /// POST /api/trust/query — structured relational-trust query.
    pub async fn query_relational_trust(
        &self,
        observer: &str,
        target: &str,
        domain: &str,
        max_depth: u32,
    ) -> Result<TrustResult> {
        let body = serde_json::json!({
            "observer": observer,
            "target": target,
            "domain": domain,
            "maxDepth": max_depth,
        });
        let url = format!("{}/{}", self.api_base, "trust/query");
        let resp = self
            .http
            .post(&url)
            .timeout(self.timeout)
            .json(&body)
            .send()
            .await?;
        parse_envelope_typed(resp).await
    }

    // --- Title (write) -------------------------------------------------

    /// Submit a TITLE transaction.
    ///
    /// Mirrors Python's `register_title`. Ownership percentages may be
    /// supplied on either the 1.0 (fraction) or 100.0 (percent) scale;
    /// they're normalized to the fraction scale for the wire, matching
    /// the server invariant `sum(percentage) == 1.0`.
    pub async fn register_title<'a>(
        &self,
        signer: &Quid,
        p: TitleParams<'a>,
    ) -> Result<Value> {
        if !signer.has_private_key() {
            return Err(Error::validation("signer must have a private key"));
        }
        if p.asset_id.is_empty() {
            return Err(Error::validation("asset_id is required"));
        }
        if p.owners.is_empty() {
            return Err(Error::validation("owners is required"));
        }
        let total: f64 = p.owners.iter().map(|s| s.percentage).sum();
        let scale = if (total - 1.0).abs() < 0.001 {
            1.0
        } else if (total - 100.0).abs() < 0.001 {
            0.01
        } else {
            return Err(Error::validation(format!(
                "ownership percentages must sum to 1.0 (or 100.0 for percent); got {total}"
            )));
        };
        let wire_owners: Vec<WireOwnershipStake> = p
            .owners
            .iter()
            .map(|s| WireOwnershipStake {
                owner_id: s.owner_id.as_str(),
                percentage: s.percentage * scale,
                stake_type: s.stake_type.as_deref().unwrap_or(""),
            })
            .collect();
        let mut tx = TitleTx {
            id: String::new(),
            tx_type: "TITLE",
            trust_domain: p.domain,
            timestamp: now_secs(),
            signature: String::new(),
            public_key: signer.public_key_hex(),
            asset_id: p.asset_id,
            owners: wire_owners,
            previous_owners: Vec::new(),
            signatures: std::collections::BTreeMap::new(),
            expiry_date: 0,
            title_type: p.title_type.unwrap_or(""),
        };
        tx.id = tx.derive_id();
        let signable = serde_json::to_vec(&tx)?;
        tx.signature = signer.sign(&signable)?;
        let body = serde_json::to_value(&tx)?;
        self.post("transactions/title", &body).await
    }

    // --- Events --------------------------------------------------------

    /// POST /api/events — submit a signed EVENT transaction.
    ///
    /// Exactly one of `payload` or `payload_cid` must be set. If
    /// `sequence` is not supplied, the next sequence is inferred from
    /// the existing event stream (falling back to 1).
    pub async fn emit_event<'a>(
        &self,
        signer: &Quid,
        p: EventParams<'a>,
    ) -> Result<Value> {
        if !signer.has_private_key() {
            return Err(Error::validation("signer must have a private key"));
        }
        if p.subject_type != "QUID" && p.subject_type != "TITLE" {
            return Err(Error::validation(
                "subject_type must be 'QUID' or 'TITLE'",
            ));
        }
        if p.event_type.is_empty() {
            return Err(Error::validation("event_type is required"));
        }
        if p.payload.is_some() == p.payload_cid.is_some() {
            return Err(Error::validation(
                "exactly one of payload or payload_cid is required",
            ));
        }

        let sequence = match p.sequence {
            Some(s) => s,
            None => match self.get_event_stream(p.subject_id, p.domain).await {
                Ok(Some(stream)) => stream
                    .get("latestSequence")
                    .and_then(|v| v.as_i64())
                    .unwrap_or(0)
                    + 1,
                _ => 1,
            },
        };

        let mut tx = EventTx {
            id: String::new(),
            tx_type: "EVENT",
            trust_domain: p.domain,
            timestamp: now_secs(),
            signature: String::new(),
            public_key: signer.public_key_hex(),
            subject_id: p.subject_id,
            subject_type: p.subject_type,
            sequence,
            event_type: p.event_type,
            payload: p.payload.clone(),
            payload_cid: p.payload_cid.unwrap_or(""),
            previous_event_id: "",
        };
        tx.id = tx.derive_id();
        let signable = serde_json::to_vec(&tx)?;
        tx.signature = signer.sign(&signable)?;
        let body = serde_json::to_value(&tx)?;
        self.post("events", &body).await
    }

    /// GET /api/streams/{subject_id} — event-stream metadata, or `None`
    /// on 404.
    pub async fn get_event_stream(
        &self,
        subject_id: &str,
        domain: &str,
    ) -> Result<Option<Value>> {
        let mut path = format!("streams/{}", urlencoding(subject_id));
        if !domain.is_empty() {
            path.push_str(&format!("?domain={}", urlencoding(domain)));
        }
        match self.get(&path).await {
            Ok(v) => Ok(Some(v)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// GET /api/streams/{subject_id}/events — paginated events for a
    /// subject's stream.
    pub async fn get_stream_events(
        &self,
        subject_id: &str,
        domain: &str,
        limit: Option<u64>,
        offset: Option<u64>,
    ) -> Result<Value> {
        let qs = build_query(&[
            ("domain", if domain.is_empty() { None } else { Some(domain.to_string()) }),
            ("limit", limit_str(limit)),
            ("offset", limit_str(offset)),
        ]);
        self.get(&format!(
            "streams/{}/events{qs}",
            urlencoding(subject_id)
        ))
        .await
    }

    // --- IPFS / large-payload storage ----------------------------------

    /// POST /api/ipfs/pin — pin raw bytes to IPFS and return the CID.
    pub async fn ipfs_pin(&self, content: &[u8]) -> Result<String> {
        let url = format!("{}/ipfs/pin", self.api_base);
        let resp = self
            .http
            .post(&url)
            .timeout(self.timeout)
            .header("Content-Type", "application/octet-stream")
            .body(content.to_vec())
            .send()
            .await?;
        let data = parse_envelope(resp).await?;
        let cid = data
            .get("cid")
            .or_else(|| data.get("value"))
            .and_then(|v| v.as_str())
            .map(|s| s.to_string());
        cid.ok_or_else(|| Error::UnexpectedResponse("IPFS pin response missing cid".into()))
    }

    /// GET /api/ipfs/{cid} — fetch the pinned content as raw bytes.
    ///
    /// Unlike most endpoints this returns the body verbatim — there is
    /// no `{success,data,error}` envelope.
    pub async fn ipfs_get(&self, cid: &str) -> Result<Vec<u8>> {
        let url = format!("{}/ipfs/{}", self.api_base, urlencoding(cid));
        let resp = self.http.get(&url).timeout(self.timeout).send().await?;
        let status = resp.status();
        if !status.is_success() {
            let body = resp.text().await.unwrap_or_default();
            return Err(Error::Node {
                status: status.as_u16(),
                message: format!("IPFS fetch failed: {}", truncate(&body, 200)),
            });
        }
        Ok(resp.bytes().await?.to_vec())
    }

    // --- Guardian sets + recovery (QDP-0002 / QDP-0006) ----------------

    /// POST /api/guardian/set-update — install or rotate guardians.
    pub async fn submit_guardian_set_update(&self, update: &Value) -> Result<Value> {
        self.post("guardian/set-update", update).await
    }

    /// POST /api/guardian/recovery/init — start the M-of-N recovery delay.
    pub async fn submit_recovery_init(&self, init: &Value) -> Result<Value> {
        self.post("guardian/recovery/init", init).await
    }

    /// POST /api/guardian/recovery/veto — owner or guardian aborts recovery.
    pub async fn submit_recovery_veto(&self, veto: &Value) -> Result<Value> {
        self.post("guardian/recovery/veto", veto).await
    }

    /// POST /api/guardian/recovery/commit — finalize the delayed recovery.
    pub async fn submit_recovery_commit(&self, commit: &Value) -> Result<Value> {
        self.post("guardian/recovery/commit", commit).await
    }

    /// POST /api/guardian/resign — guardian leaves the set.
    pub async fn submit_guardian_resignation(&self, resig: &Value) -> Result<Value> {
        self.post("guardian/resign", resig).await
    }

    /// GET /api/guardian/set/{quid} — current guardian set or `None`.
    pub async fn get_guardian_set(&self, quid_id: &str) -> Result<Option<Value>> {
        match self.get(&format!("guardian/set/{}", urlencoding(quid_id))).await {
            Ok(v) => Ok(Some(v)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// GET /api/guardian/pending-recovery/{quid} — pending recovery or `None`.
    pub async fn get_pending_recovery(&self, quid_id: &str) -> Result<Option<Value>> {
        match self
            .get(&format!("guardian/pending-recovery/{}", urlencoding(quid_id)))
            .await
        {
            Ok(v) => Ok(Some(v)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// GET /api/guardian/resignations/{quid} — list of recorded resignations.
    pub async fn get_guardian_resignations(&self, quid_id: &str) -> Result<Vec<Value>> {
        let data = self
            .get(&format!("guardian/resignations/{}", urlencoding(quid_id)))
            .await?;
        let raw = data
            .get("data")
            .or_else(|| data.get("resignations"))
            .cloned()
            .unwrap_or_else(|| {
                if data.is_array() {
                    data.clone()
                } else {
                    Value::Array(vec![])
                }
            });
        match raw {
            Value::Array(v) => Ok(v),
            _ => Ok(vec![]),
        }
    }

    // --- Cross-domain gossip + fingerprints (QDP-0003 / QDP-0005) ------

    /// POST /api/domain-fingerprints — publish a signed fingerprint.
    pub async fn submit_domain_fingerprint(&self, fp: &Value) -> Result<Value> {
        self.post("domain-fingerprints", fp).await
    }

    /// GET /api/domain-fingerprints/{domain}/latest — latest fingerprint
    /// or `None` on 404.
    pub async fn get_latest_domain_fingerprint(
        &self,
        domain: &str,
    ) -> Result<Option<Value>> {
        match self
            .get(&format!(
                "domain-fingerprints/{}/latest",
                urlencoding(domain)
            ))
            .await
        {
            Ok(v) => Ok(Some(v)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// POST /api/anchor-gossip — deliver a cross-domain anchor message.
    /// Idempotent: re-receipt returns `data.duplicate=true`.
    pub async fn submit_anchor_gossip(&self, msg: &Value) -> Result<Value> {
        self.post("anchor-gossip", msg).await
    }

    /// POST /api/gossip/push-anchor — push-gossip variant (QDP-0005).
    pub async fn push_anchor(&self, msg: &Value) -> Result<Value> {
        self.post("gossip/push-anchor", msg).await
    }

    /// POST /api/gossip/push-fingerprint — push-gossip variant (QDP-0005).
    pub async fn push_fingerprint(&self, fp: &Value) -> Result<Value> {
        self.post("gossip/push-fingerprint", fp).await
    }

    // --- Bootstrap + nonce snapshots (QDP-0008) ------------------------

    /// POST /api/nonce-snapshots — publish a K-of-K bootstrap snapshot.
    pub async fn submit_nonce_snapshot(&self, snapshot: &Value) -> Result<Value> {
        self.post("nonce-snapshots", snapshot).await
    }

    /// GET /api/nonce-snapshots/{domain}/latest — latest snapshot or `None`.
    pub async fn get_latest_nonce_snapshot(
        &self,
        domain: &str,
    ) -> Result<Option<Value>> {
        match self
            .get(&format!("nonce-snapshots/{}/latest", urlencoding(domain)))
            .await
        {
            Ok(v) => Ok(Some(v)),
            Err(Error::Validation(m)) if m.contains("NOT_FOUND") => Ok(None),
            Err(Error::Conflict { code, .. }) if code == "NOT_FOUND" => Ok(None),
            Err(e) => Err(e),
        }
    }

    /// GET /api/bootstrap/status — current bootstrap state.
    pub async fn bootstrap_status(&self) -> Result<Value> {
        self.get("bootstrap/status").await
    }

    // --- Fork-block (QDP-0009) -----------------------------------------

    /// POST /api/fork-block — submit a signed fork-activation block.
    pub async fn submit_fork_block(&self, fb: &Value) -> Result<Value> {
        self.post("fork-block", fb).await
    }

    /// GET /api/fork-block/status — activation status across features.
    pub async fn fork_block_status(&self) -> Result<Value> {
        self.get("fork-block/status").await
    }

    // --- Plumbing ------------------------------------------------------

    async fn get(&self, path: &str) -> Result<Value> {
        let url = format!("{}/{}", self.api_base, path.trim_start_matches('/'));
        let resp = self.http.get(&url).timeout(self.timeout).send().await?;
        parse_envelope(resp).await
    }

    async fn get_typed<T: DeserializeOwned>(&self, path: &str) -> Result<T> {
        let url = format!("{}/{}", self.api_base, path.trim_start_matches('/'));
        let resp = self.http.get(&url).timeout(self.timeout).send().await?;
        parse_envelope_typed(resp).await
    }

    async fn post(&self, path: &str, body: &Value) -> Result<Value> {
        let url = format!("{}/{}", self.api_base, path.trim_start_matches('/'));
        let resp = self
            .http
            .post(&url)
            .timeout(self.timeout)
            .json(body)
            .send()
            .await?;
        parse_envelope(resp).await
    }
}

async fn parse_envelope(resp: reqwest::Response) -> Result<Value> {
    let status = resp.status();
    let body = resp.text().await?;
    let env: Value = match serde_json::from_str(&body) {
        Ok(v) => v,
        Err(_) => {
            return Err(Error::Node {
                status: status.as_u16(),
                message: format!("non-JSON response: {}", truncate(&body, 200)),
            });
        }
    };
    if env.get("success").and_then(|v| v.as_bool()).unwrap_or(false) {
        return Ok(env.get("data").cloned().unwrap_or(Value::Null));
    }
    Err(envelope_to_error(status, &env))
}

async fn parse_envelope_typed<T: DeserializeOwned>(resp: reqwest::Response) -> Result<T> {
    let value = parse_envelope(resp).await?;
    serde_json::from_value(value).map_err(Error::from)
}

fn envelope_to_error(status: StatusCode, env: &Value) -> Error {
    let code = env
        .pointer("/error/code")
        .and_then(|v| v.as_str())
        .unwrap_or("UNKNOWN_ERROR")
        .to_string();
    let message = env
        .pointer("/error/message")
        .and_then(|v| v.as_str())
        .unwrap_or("")
        .to_string();
    match status.as_u16() {
        503 => Error::Unavailable { code, message },
        409 => Error::Conflict { code, message },
        s if (400..500).contains(&s) => {
            if code == "NOT_FOUND" {
                Error::Validation(format!("{}: {}", code, message))
            } else if matches!(
                code.as_str(),
                "FEATURE_NOT_ACTIVE" | "NOT_READY" | "BOOTSTRAPPING"
            ) {
                Error::Unavailable { code, message }
            } else if is_conflict_code(&code) {
                Error::Conflict { code, message }
            } else {
                Error::Validation(message)
            }
        }
        s => Error::Node {
            status: s,
            message,
        },
    }
}

fn is_conflict_code(code: &str) -> bool {
    matches!(
        code,
        "NONCE_REPLAY"
            | "GUARDIAN_SET_MISMATCH"
            | "QUORUM_NOT_MET"
            | "VETOED"
            | "INVALID_SIGNATURE"
            | "FORK_ALREADY_ACTIVE"
            | "DUPLICATE"
            | "ALREADY_EXISTS"
            | "INVALID_STATE_TRANSITION"
    )
}

fn now_secs() -> i64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|d| d.as_secs() as i64)
        .unwrap_or(0)
}

fn urlencoding(s: &str) -> String {
    // Minimal percent-encoding for path/query segments. Avoids pulling
    // in a whole crate just for this.
    let mut out = String::with_capacity(s.len());
    for b in s.bytes() {
        match b {
            b'A'..=b'Z' | b'a'..=b'z' | b'0'..=b'9' | b'-' | b'_' | b'.' | b'~' => {
                out.push(b as char)
            }
            _ => out.push_str(&format!("%{:02X}", b)),
        }
    }
    out
}

fn truncate(s: &str, n: usize) -> String {
    if s.len() <= n {
        s.to_string()
    } else {
        format!("{}...", &s[..n])
    }
}

/// Format an optional integer as its decimal string for inclusion in a
/// query string; `None` omits the parameter.
fn limit_str(v: Option<u64>) -> Option<String> {
    v.map(|n| n.to_string())
}

/// Build a `?k=v&k=v` query suffix from name/value pairs, omitting any
/// pair whose value is `None`. URL-encodes both names (verbatim) and
/// values. Returns the empty string if no pairs are present.
fn build_query(pairs: &[(&str, Option<String>)]) -> String {
    let mut parts: Vec<String> = Vec::new();
    for (k, v) in pairs {
        if let Some(val) = v {
            parts.push(format!("{}={}", k, urlencoding(val)));
        }
    }
    if parts.is_empty() {
        String::new()
    } else {
        format!("?{}", parts.join("&"))
    }
}
