# Ledger Nano (S/Plus/X) app for Quidnug (scaffold)

Status: **SCAFFOLD — not yet on Ledger Live.**

A Ledger device app that holds a Quidnug P-256 key on-device and
signs transactions only after on-screen confirmation. Closest
parallel: the Ledger ETH app, but the envelopes are Quidnug wire
types instead of EVM transactions.

## Architecture

```
Host app    ──(APDU)──►   Ledger device
(Python SDK                 (Ledger app, written
 via ledgercomm)             in C against Ledger BOLOS)
                                │
                                └─ BOLOS sign: P-256 key sealed in SE
```

## APDU surface

```
INS: 0x01  GetPublicKey       <bip32path>         → <SEC1 pubkey>
INS: 0x02  GetQuidID          <bip32path>         → <16 hex chars>
INS: 0x03  SignTx             <bip32path> <canonical tx bytes>
                                                   → <DER hex sig>
     (user must confirm tx type + signer + trustee + level on-device)
```

## Roadmap

1. BOLOS app C source under `integrations/ledger/app/`.
2. Host-side Python bindings shipped as `quidnug[ledger]` extra.
3. Ledger Live catalog submission.
4. End-to-end integration test with a Speculos emulator.

## Why

Most enterprise users who care about key security already own Ledger
devices. Letting them sign Quidnug transactions with the hardware
they already trust is the lowest-friction path to production for
individual signers.

## License

Apache-2.0.
