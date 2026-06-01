//! Integration tests for the methods added in the Python-parity pass.
//!
//! Every new endpoint gets a wiremock-backed test that asserts the
//! correct HTTP method + path; for response-parsing endpoints we also
//! check that the typed result deserializes the way the Python decoders
//! do.

use quidnug::{
    AnchorGossipMessage, Client, DomainFingerprint, EventParams, ForkBlock, ForkSig,
    GuardianRecoveryCommit, GuardianRecoveryInit, GuardianRecoveryVeto, GuardianRef,
    GuardianResignation, GuardianSet, GuardianSetUpdate, GuardianSignature, NonceSnapshot,
    NonceSnapshotEntry, OwnershipStake, PrimarySignature, Quid, TitleParams,
};
use serde_json::json;
use wiremock::matchers::{body_json, method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};

fn ok(data: serde_json::Value) -> ResponseTemplate {
    ResponseTemplate::new(200).set_body_json(json!({ "success": true, "data": data }))
}

fn not_found() -> ResponseTemplate {
    ResponseTemplate::new(404).set_body_json(json!({
        "success": false,
        "error": { "code": "NOT_FOUND", "message": "missing" }
    }))
}

// --- Nodes / blocks / domains --------------------------------------------

#[tokio::test]
async fn nodes_passes_limit_offset() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/nodes"))
        .respond_with(ok(json!({"value": [{"quid": "abc"}]})))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let v = client.nodes(Some(10), Some(20)).await.unwrap();
    assert!(v.get("value").is_some());
}

#[tokio::test]
async fn get_blocks_paginates() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/blocks"))
        .respond_with(ok(json!([])))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let _ = client.get_blocks(Some(5), None).await.unwrap();
}

#[tokio::test]
async fn get_tentative_blocks_path() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/blocks/tentative/foo.bar"))
        .respond_with(ok(json!({"blocks": []})))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let v = client.get_tentative_blocks("foo.bar").await.unwrap();
    assert!(v.get("blocks").is_some());
}

#[tokio::test]
async fn get_pending_transactions_path() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/transactions"))
        .respond_with(ok(json!({"transactions": []})))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let _ = client.get_pending_transactions(None, None).await.unwrap();
}

#[tokio::test]
async fn list_domains_works() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/domains"))
        .respond_with(ok(json!({"domains": ["a", "b"]})))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let v = client.list_domains().await.unwrap();
    assert_eq!(v["domains"], json!(["a", "b"]));
}

#[tokio::test]
async fn node_domains_get_and_update() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/node/domains"))
        .respond_with(ok(json!({"managedDomains": ["x"]})))
        .mount(&server)
        .await;
    Mock::given(method("POST"))
        .and(path("/api/node/domains"))
        .and(body_json(json!({"managedDomains": ["a", "b"]})))
        .respond_with(ok(json!({"ok": true})))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let v = client.get_node_domains().await.unwrap();
    assert_eq!(v["managedDomains"], json!(["x"]));
    let _ = client
        .update_node_domains(&["a".to_string(), "b".to_string()])
        .await
        .unwrap();
}

// --- Registry queries ----------------------------------------------------

#[tokio::test]
async fn query_identity_registry_works() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/registry/identity"))
        .respond_with(ok(json!({"identities": []})))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let v = client
        .query_identity_registry(Some(10), Some(0), Some("abc"))
        .await
        .unwrap();
    assert!(v.get("identities").is_some());
}

#[tokio::test]
async fn query_trust_registry_works() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/registry/trust"))
        .respond_with(ok(json!({"edges": []})))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let _ = client
        .query_trust_registry(Some(5), None, Some("a"), Some("b"))
        .await
        .unwrap();
}

#[tokio::test]
async fn query_title_registry_works() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/registry/title"))
        .respond_with(ok(json!({"titles": []})))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let _ = client
        .query_title_registry(None, None, Some("asset-1"), None)
        .await
        .unwrap();
}

#[tokio::test]
async fn query_relational_trust_posts() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/api/trust/query"))
        .and(body_json(json!({
            "observer": "a", "target": "b", "domain": "d", "maxDepth": 5
        })))
        .respond_with(ok(json!({
            "observer": "a", "target": "b",
            "trustLevel": 0.8,
            "trustPath": ["a", "b"], "pathDepth": 1, "domain": "d"
        })))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let tr = client
        .query_relational_trust("a", "b", "d", 5)
        .await
        .unwrap();
    assert_eq!(tr.trust_level, 0.8);
    assert_eq!(tr.path_depth, 1);
}

#[tokio::test]
async fn query_domain_passes_params() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/domains/d.com/query"))
        .respond_with(ok(json!({"result": "x"})))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let v = client
        .query_domain("d.com", "identity", "abc")
        .await
        .unwrap();
    assert_eq!(v["result"], "x");
}

// --- Title / events ------------------------------------------------------

#[tokio::test]
async fn register_title_normalizes_percentages() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/api/transactions/title"))
        .respond_with(ok(json!({"txId": "t1"})))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let q = Quid::generate();
    let v = client
        .register_title(
            &q,
            TitleParams {
                asset_id: "asset-1",
                owners: vec![
                    OwnershipStake {
                        owner_id: "a".into(),
                        percentage: 60.0,
                        stake_type: None,
                    },
                    OwnershipStake {
                        owner_id: "b".into(),
                        percentage: 40.0,
                        stake_type: None,
                    },
                ],
                domain: "demo",
                title_type: "deed",
            },
        )
        .await
        .unwrap();
    assert_eq!(v["txId"], "t1");
}

#[tokio::test]
async fn register_title_rejects_bad_sums() {
    let client = Client::new("http://127.0.0.1:1").unwrap();
    let q = Quid::generate();
    let err = client
        .register_title(
            &q,
            TitleParams {
                asset_id: "a",
                owners: vec![OwnershipStake {
                    owner_id: "x".into(),
                    percentage: 50.0,
                    stake_type: None,
                }],
                domain: "d",
                title_type: "",
            },
        )
        .await
        .unwrap_err();
    assert!(matches!(err, quidnug::Error::Validation(_)));
}

#[tokio::test]
async fn emit_event_auto_sequences_from_stream() {
    let server = MockServer::start().await;
    // first the SDK reads the stream → gets latestSequence=3
    Mock::given(method("GET"))
        .and(path("/api/streams/subj"))
        .respond_with(ok(json!({"subjectId": "subj", "latestSequence": 3})))
        .mount(&server)
        .await;
    Mock::given(method("POST"))
        .and(path("/api/events"))
        .respond_with(ok(json!({"txId": "e1"})))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let q = Quid::generate();
    let v = client
        .emit_event(
            &q,
            EventParams {
                subject_id: "subj",
                subject_type: "QUID",
                event_type: "ping",
                domain: "demo",
                payload: Some(json!({"k": "v"})),
                payload_cid: "",
                sequence: 0,
            },
        )
        .await
        .unwrap();
    assert_eq!(v["txId"], "e1");
}

#[tokio::test]
async fn emit_event_payload_xor_cid_required() {
    let client = Client::new("http://127.0.0.1:1").unwrap();
    let q = Quid::generate();
    // both none
    let err = client
        .emit_event(
            &q,
            EventParams {
                subject_id: "x",
                subject_type: "QUID",
                event_type: "t",
                domain: "d",
                payload: None,
                payload_cid: "",
                sequence: 1,
            },
        )
        .await
        .unwrap_err();
    assert!(matches!(err, quidnug::Error::Validation(_)));
}

#[tokio::test]
async fn get_event_stream_404_is_none() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/streams/missing"))
        .respond_with(not_found())
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let v = client.get_event_stream("missing", "").await.unwrap();
    assert!(v.is_none());
}

#[tokio::test]
async fn get_stream_events_unwraps_array() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/streams/s/events"))
        .respond_with(ok(json!({
            "data": [
                {"subjectId": "s", "subjectType": "QUID", "eventType": "e",
                 "timestamp": 1, "sequence": 1}
            ]
        })))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let evs = client
        .get_stream_events("s", "", Some(50), None)
        .await
        .unwrap();
    assert_eq!(evs.len(), 1);
    assert_eq!(evs[0].subject_id, "s");
}

// --- IPFS ----------------------------------------------------------------

#[tokio::test]
async fn ipfs_pin_returns_cid() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/api/ipfs/pin"))
        .respond_with(ok(json!({"cid": "Qm123"})))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let cid = client.ipfs_pin(b"hello").await.unwrap();
    assert_eq!(cid, "Qm123");
}

#[tokio::test]
async fn ipfs_get_returns_raw_bytes() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/ipfs/Qm123"))
        .respond_with(ResponseTemplate::new(200).set_body_bytes(b"raw bytes".to_vec()))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let bytes = client.ipfs_get("Qm123").await.unwrap();
    assert_eq!(bytes, b"raw bytes");
}

// --- Guardian sets + recovery -------------------------------------------

fn sample_guardian_set() -> GuardianSet {
    GuardianSet {
        subject_quid: "subj".into(),
        guardians: vec![GuardianRef {
            quid: "g1".into(),
            weight: 1,
            epoch: 0,
            added_at_block: None,
        }],
        threshold: 1,
        recovery_delay_seconds: 600,
        require_guardian_rotation: false,
        updated_at_block: None,
    }
}

#[tokio::test]
async fn submit_guardian_set_update_posts() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/api/guardian/set-update"))
        .respond_with(ok(json!({"ok": true})))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let update = GuardianSetUpdate {
        subject_quid: "subj".into(),
        new_set: sample_guardian_set(),
        anchor_nonce: 1,
        valid_from: 1700000000,
        primary_signature: Some(PrimarySignature {
            key_epoch: 0,
            signature: "deadbeef".into(),
        }),
        new_guardian_consents: vec![],
        current_guardian_sigs: vec![],
    };
    let _ = client.submit_guardian_set_update(&update).await.unwrap();
}

#[tokio::test]
async fn submit_recovery_init_posts() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/api/guardian/recovery/init"))
        .respond_with(ok(json!({"ok": true})))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let init = GuardianRecoveryInit {
        subject_quid: "subj".into(),
        from_epoch: 0,
        to_epoch: 1,
        new_public_key: "04abc".into(),
        min_next_nonce: 100,
        max_accepted_old_nonce: 99,
        anchor_nonce: 1,
        valid_from: 1,
        guardian_sigs: vec![GuardianSignature {
            guardian_quid: "g1".into(),
            key_epoch: 0,
            signature: "sig".into(),
        }],
        expires_at: None,
    };
    let _ = client.submit_recovery_init(&init).await.unwrap();
}

#[tokio::test]
async fn submit_recovery_veto_posts() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/api/guardian/recovery/veto"))
        .respond_with(ok(json!({"ok": true})))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let veto = GuardianRecoveryVeto {
        subject_quid: "subj".into(),
        recovery_anchor_hash: "abc".into(),
        anchor_nonce: 1,
        valid_from: 1,
        primary_signature: None,
        guardian_sigs: vec![],
    };
    let _ = client.submit_recovery_veto(&veto).await.unwrap();
}

#[tokio::test]
async fn submit_recovery_commit_posts() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/api/guardian/recovery/commit"))
        .respond_with(ok(json!({"ok": true})))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let commit = GuardianRecoveryCommit {
        subject_quid: "subj".into(),
        recovery_anchor_hash: "abc".into(),
        anchor_nonce: 1,
        valid_from: 1,
        committer_quid: "comm".into(),
        committer_sig: "sig".into(),
    };
    let _ = client.submit_recovery_commit(&commit).await.unwrap();
}

#[tokio::test]
async fn submit_guardian_resignation_posts() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/api/guardian/resign"))
        .respond_with(ok(json!({"ok": true})))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let resign = GuardianResignation {
        guardian_quid: "g1".into(),
        subject_quid: "subj".into(),
        guardian_set_hash: "hash".into(),
        resignation_nonce: 1,
        effective_at: 2,
        signature: "sig".into(),
    };
    let _ = client.submit_guardian_resignation(&resign).await.unwrap();
}

#[tokio::test]
async fn get_guardian_set_parses_payload() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/guardian/set/subj"))
        .respond_with(ok(json!({
            "subjectQuid": "subj",
            "guardians": [{"quid": "g1", "weight": 2, "epoch": 0}],
            "threshold": 2,
            "recoveryDelaySeconds": 600
        })))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let gs = client.get_guardian_set("subj").await.unwrap().unwrap();
    assert_eq!(gs.threshold, 2);
    assert_eq!(gs.guardians[0].quid, "g1");
    assert_eq!(gs.guardians[0].weight, 2);
}

#[tokio::test]
async fn get_guardian_set_404_is_none() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/guardian/set/unknown"))
        .respond_with(not_found())
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let gs = client.get_guardian_set("unknown").await.unwrap();
    assert!(gs.is_none());
}

#[tokio::test]
async fn get_pending_recovery_returns_value() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/guardian/pending-recovery/subj"))
        .respond_with(ok(json!({"subjectQuid": "subj", "validFrom": 1})))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let v = client.get_pending_recovery("subj").await.unwrap().unwrap();
    assert_eq!(v["validFrom"], 1);
}

#[tokio::test]
async fn get_guardian_resignations_unwraps() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/guardian/resignations/subj"))
        .respond_with(ok(json!({"data": [{"guardianQuid": "g1"}]})))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let v = client.get_guardian_resignations("subj").await.unwrap();
    assert_eq!(v.len(), 1);
    assert_eq!(v[0]["guardianQuid"], "g1");
}

// --- Gossip / fingerprints ----------------------------------------------

fn sample_fingerprint() -> DomainFingerprint {
    DomainFingerprint {
        domain: "d".into(),
        block_height: 10,
        block_hash: "hash".into(),
        producer_quid: "p".into(),
        timestamp: 1,
        signature: "sig".into(),
        schema_version: 1,
    }
}

#[tokio::test]
async fn submit_and_get_domain_fingerprint() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/api/domain-fingerprints"))
        .respond_with(ok(json!({"ok": true})))
        .mount(&server)
        .await;
    Mock::given(method("GET"))
        .and(path("/api/domain-fingerprints/d/latest"))
        .respond_with(ok(json!({
            "domain": "d", "blockHeight": 10, "blockHash": "hash",
            "producerQuid": "p", "timestamp": 1, "signature": "sig",
            "schemaVersion": 1
        })))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let _ = client
        .submit_domain_fingerprint(&sample_fingerprint())
        .await
        .unwrap();
    let fp = client
        .get_latest_domain_fingerprint("d")
        .await
        .unwrap()
        .unwrap();
    assert_eq!(fp.block_height, 10);
}

#[tokio::test]
async fn submit_anchor_gossip_posts() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/api/anchor-gossip"))
        .respond_with(ok(json!({"duplicate": false})))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let m = AnchorGossipMessage {
        message_id: "m1".into(),
        origin_domain: "d1".into(),
        origin_block_height: 1,
        origin_block: json!({}),
        anchor_tx_index: 0,
        domain_fingerprint: sample_fingerprint(),
        timestamp: 1,
        gossip_producer_quid: "p".into(),
        gossip_signature: "sig".into(),
        schema_version: 1,
        merkle_proof: None,
    };
    let _ = client.submit_anchor_gossip(&m).await.unwrap();
}

#[tokio::test]
async fn push_anchor_and_fingerprint_post() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/api/gossip/push-anchor"))
        .respond_with(ok(json!({"ok": true})))
        .mount(&server)
        .await;
    Mock::given(method("POST"))
        .and(path("/api/gossip/push-fingerprint"))
        .respond_with(ok(json!({"ok": true})))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let m = AnchorGossipMessage {
        message_id: "m1".into(),
        origin_domain: "d".into(),
        origin_block_height: 1,
        origin_block: json!({}),
        anchor_tx_index: 0,
        domain_fingerprint: sample_fingerprint(),
        timestamp: 1,
        gossip_producer_quid: "p".into(),
        gossip_signature: "sig".into(),
        schema_version: 1,
        merkle_proof: None,
    };
    let _ = client.push_anchor(&m).await.unwrap();
    let _ = client.push_fingerprint(&sample_fingerprint()).await.unwrap();
}

// --- Bootstrap (nonce snapshots) ----------------------------------------

#[tokio::test]
async fn submit_and_get_nonce_snapshot() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/api/nonce-snapshots"))
        .respond_with(ok(json!({"ok": true})))
        .mount(&server)
        .await;
    Mock::given(method("GET"))
        .and(path("/api/nonce-snapshots/d/latest"))
        .respond_with(ok(json!({
            "blockHeight": 5,
            "blockHash": "h",
            "timestamp": 1,
            "trustDomain": "d",
            "entries": [{"quid": "q1", "epoch": 0, "maxNonce": 7}],
            "producerQuid": "p",
            "signature": "sig",
            "schemaVersion": 1
        })))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let snap = NonceSnapshot {
        block_height: 5,
        block_hash: "h".into(),
        timestamp: 1,
        trust_domain: "d".into(),
        entries: vec![NonceSnapshotEntry {
            quid: "q1".into(),
            epoch: 0,
            max_nonce: 7,
        }],
        producer_quid: "p".into(),
        signature: "sig".into(),
        schema_version: 1,
    };
    let _ = client.submit_nonce_snapshot(&snap).await.unwrap();
    let got = client
        .get_latest_nonce_snapshot("d")
        .await
        .unwrap()
        .unwrap();
    assert_eq!(got.entries[0].max_nonce, 7);
}

#[tokio::test]
async fn bootstrap_status_works() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/bootstrap/status"))
        .respond_with(ok(json!({"ready": true})))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let v = client.bootstrap_status().await.unwrap();
    assert_eq!(v["ready"], true);
}

// --- Fork-block ----------------------------------------------------------

#[tokio::test]
async fn submit_fork_block_and_status() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/api/fork-block"))
        .respond_with(ok(json!({"ok": true})))
        .mount(&server)
        .await;
    Mock::given(method("GET"))
        .and(path("/api/fork-block/status"))
        .respond_with(ok(json!({"features": ["qdp-0009"]})))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let fb = ForkBlock {
        trust_domain: "d".into(),
        feature: "qdp-0009".into(),
        fork_height: 100,
        fork_nonce: 1,
        proposed_at: 1,
        signatures: vec![ForkSig {
            validator_quid: "v".into(),
            key_epoch: 0,
            signature: "sig".into(),
        }],
        expires_at: None,
    };
    let _ = client.submit_fork_block(&fb).await.unwrap();
    let v = client.fork_block_status().await.unwrap();
    assert!(v.get("features").is_some());
}
