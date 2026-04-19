# Terraform provider for Quidnug (scaffold)

Status: **SCAFFOLD — not yet on Terraform Registry.**

`terraform-provider-quidnug` will let operators manage domains,
guardian sets, and fork-block activations declaratively.

## Planned resources

```hcl
terraform {
    required_providers {
        quidnug = {
            source  = "quidnug/quidnug"
            version = "~> 2.0"
        }
    }
}

provider "quidnug" {
    node_url = "https://quidnug.example.com"
    token    = var.quidnug_token
}

resource "quidnug_domain" "supplychain" {
    name       = "supplychain.example.com"
    attributes = {
        owner = "trust-ops@example.com"
    }
}

resource "quidnug_guardian_set" "ceo" {
    subject_quid             = data.quidnug_identity.ceo.quid_id
    threshold                = 3
    recovery_delay_seconds   = 86400 * 3
    require_guardian_rotation = true

    guardian {
        quid   = data.quidnug_identity.cfo.quid_id
        weight = 1
    }
    guardian {
        quid   = data.quidnug_identity.cto.quid_id
        weight = 1
    }
    # ... etc
}

resource "quidnug_fork_block" "qdp_0011" {
    trust_domain = quidnug_domain.supplychain.name
    feature      = "QDP-0011"
    fork_height  = 10_000
}
```

## Data sources

- `quidnug_identity` — fetch a quid by ID
- `quidnug_trust` — compute relational trust
- `quidnug_title` — fetch ownership
- `quidnug_guardian_set` — introspect current guardian set

## Roadmap

1. Scaffold the provider using the Terraform Plugin Framework.
2. Implement resources above.
3. Publish to the Terraform Registry as `quidnug/quidnug`.

## License

Apache-2.0.
