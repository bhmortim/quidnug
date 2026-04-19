//! Two-party trust quickstart.
//!
//! ```bash
//! cd clients/rust
//! cargo run --example quickstart
//! ```

use quidnug::{Client, Quid, TrustParams};

#[tokio::main]
async fn main() -> Result<(), quidnug::Error> {
    let client = Client::new("http://localhost:8080")?;
    let info = client.info().await?;
    println!(
        "connected to {} v{}",
        info.get("quidId").and_then(|v| v.as_str()).unwrap_or("?"),
        info.get("version").and_then(|v| v.as_str()).unwrap_or("?")
    );

    let alice = Quid::generate();
    let bob = Quid::generate();
    println!("alice={} bob={}", alice.id(), bob.id());

    client.register_identity(&alice, "Alice", "demo.home").await?;
    client.register_identity(&bob, "Bob", "demo.home").await?;

    client
        .grant_trust(
            &alice,
            TrustParams {
                trustee: bob.id(),
                level: 0.9,
                domain: "demo.home",
                nonce: 1,
            },
        )
        .await?;

    let tr = client.get_trust(alice.id(), bob.id(), "demo.home", 5).await?;
    println!(
        "relational trust = {:.3} via {}",
        tr.trust_level,
        tr.path.join(" -> ")
    );
    Ok(())
}
