#!/usr/bin/env bash
# Pairwise-peer the three seed nodes so each accepts the others' blocks
# as Trusted. Idempotent: repeat calls don't produce duplicate edges.
#
# Required env:
#   SEEDS           space-separated URLs, e.g. "https://node-1.quidnug.com https://node-2.quidnug.com https://node-3.quidnug.com"
#   SEED_QUIDS      space-separated quid IDs in the same order as SEEDS
#   NODE_KEY_FILES  space-separated key JSON paths in the same order
#   NODE_AUTH_SECRET  shared HMAC secret (hex)
#
# Posts TRUST edges at level 0.95 in:
#   - peering.network.quidnug.com
#   - validators.network.quidnug.com

set -euo pipefail

: "${SEEDS:?}"
: "${SEED_QUIDS:?}"
: "${NODE_KEY_FILES:?}"
: "${NODE_AUTH_SECRET:?}"

read -ra SEED_ARR  <<< "$SEEDS"
read -ra QUID_ARR  <<< "$SEED_QUIDS"
read -ra KEY_ARR   <<< "$NODE_KEY_FILES"

if (( ${#SEED_ARR[@]} != ${#QUID_ARR[@]} || ${#SEED_ARR[@]} != ${#KEY_ARR[@]} )); then
  echo "SEEDS, SEED_QUIDS, NODE_KEY_FILES must have the same length" >&2
  exit 1
fi

DOMAINS=(
  "peering.network.quidnug.com"
  "validators.network.quidnug.com"
)

for ((i=0; i<${#SEED_ARR[@]}; i++)); do
  TRUSTER_URL="${SEED_ARR[$i]}"
  TRUSTER_QUID="${QUID_ARR[$i]}"
  TRUSTER_KEY="${KEY_ARR[$i]}"

  for ((j=0; j<${#SEED_ARR[@]}; j++)); do
    [[ $i == $j ]] && continue
    TRUSTEE_QUID="${QUID_ARR[$j]}"

    for DOMAIN in "${DOMAINS[@]}"; do
      echo "==> $TRUSTER_QUID → $TRUSTEE_QUID  [$DOMAIN]"
      quidnug-cli grant-trust \
        --node "$TRUSTER_URL" \
        --key  "$TRUSTER_KEY" \
        --trustee "$TRUSTEE_QUID" \
        --domain "$DOMAIN" \
        --trust-level 0.95 \
        --hmac-secret "$NODE_AUTH_SECRET" \
        || echo "    (already exists or error — continuing)"
    done
  done
done

echo
echo "==> Verifying tiers"
for SEED in "${SEED_ARR[@]}"; do
  echo "$SEED:"
  curl -sS "$SEED/api/info" | jq '{name, chainTip, peerCount}'
done
