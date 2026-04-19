# Quidnug × ISO 20022

The working Go integration for ISO 20022 lives at
[`integrations/iso20022/`](../../integrations/iso20022/).

This directory previously held a design scaffold that has since been
superseded by the real package. See the integration README for:

- [Usage + quickstart](../../integrations/iso20022/README.md)
- [Cross-border payment example](../../integrations/iso20022/examples/cross_border_payment.go)
- [Tests](../../integrations/iso20022/iso20022_test.go)

Companion SDKs in other languages will wrap that Go package via the
same HTTP/JSON event surface used by every Quidnug SDK — no
language-specific ISO 20022 port required.

## License

Apache-2.0.
