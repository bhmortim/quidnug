//! Integration tests for the HTTP client against wiremock.

use quidnug::{Client, Error, Quid, TrustParams};
use serde_json::json;
use wiremock::matchers::{method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};

#[tokio::test]
async fn grant_trust_posts_correct_envelope() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/api/transactions/trust"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "success": true, "data": { "txId": "abc" }
        })))
        .mount(&server)
        .await;

    let client = Client::new(&server.uri()).unwrap();
    let q = Quid::generate();
    let res = client
        .grant_trust(
            &q,
            TrustParams {
                trustee: "bob",
                level: 0.9,
                domain: "demo.home",
                nonce: 1,
            },
        )
        .await
        .unwrap();
    assert_eq!(res.get("txId").unwrap().as_str().unwrap(), "abc");
}

#[tokio::test]
async fn error_envelope_409_raises_conflict() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/api/transactions/trust"))
        .respond_with(ResponseTemplate::new(409).set_body_json(json!({
            "success": false,
            "error": { "code": "NONCE_REPLAY", "message": "stale nonce" }
        })))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let q = Quid::generate();
    let err = client
        .grant_trust(
            &q,
            TrustParams {
                trustee: "bob",
                level: 0.9,
                domain: "demo.home",
                nonce: 1,
            },
        )
        .await
        .unwrap_err();
    match err {
        Error::Conflict { code, .. } => assert_eq!(code, "NONCE_REPLAY"),
        _ => panic!("expected Conflict, got {err:?}"),
    }
}

#[tokio::test]
async fn unavailable_on_503() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/api/health"))
        .respond_with(ResponseTemplate::new(503).set_body_json(json!({
            "success": false,
            "error": { "code": "BOOTSTRAPPING", "message": "warming up" }
        })))
        .mount(&server)
        .await;
    let client = Client::new(&server.uri()).unwrap();
    let err = client.health().await.unwrap_err();
    assert!(matches!(err, Error::Unavailable { .. }));
}

#[tokio::test]
async fn level_validation_fails_before_network() {
    let client = Client::new("http://127.0.0.1:1").unwrap();
    let q = Quid::generate();
    let err = client
        .grant_trust(
            &q,
            TrustParams {
                trustee: "bob",
                level: 1.5,
                domain: "x",
                nonce: 1,
            },
        )
        .await
        .unwrap_err();
    assert!(matches!(err, Error::Validation(_)));
}
