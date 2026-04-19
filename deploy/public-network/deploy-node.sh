#!/usr/bin/env bash
# Deploy a Quidnug seed node to Fly.io.
#
# Required env:
#   NODE_NAME      e.g. quidnug-seed-1
#   NODE_REGION    e.g. iad, lhr, sin
#   NODE_KEY_FILE  path to the seed node's private key JSON
#   NODE_AUTH_SECRET  shared HMAC secret (hex)
#
# Optional:
#   GRAFANA_REMOTE_WRITE_URL
#   GRAFANA_REMOTE_WRITE_TOKEN
#
# Example:
#   NODE_NAME=quidnug-seed-1 NODE_REGION=iad \
#     NODE_KEY_FILE=./secrets/seed-1.key.json \
#     NODE_AUTH_SECRET=$(cat ./secrets/node-auth.hex) \
#     ./deploy-node.sh

set -euo pipefail

: "${NODE_NAME:?}"
: "${NODE_REGION:?}"
: "${NODE_KEY_FILE:?}"
: "${NODE_AUTH_SECRET:?}"

if ! command -v flyctl >/dev/null 2>&1; then
  echo "flyctl not found. Install from https://fly.io/docs/hands-on/install-flyctl/" >&2
  exit 1
fi

if [[ ! -f "$NODE_KEY_FILE" ]]; then
  echo "$NODE_KEY_FILE not found." >&2
  exit 1
fi

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT="$HERE/fly.${NODE_NAME}.toml"

echo "==> Generating $OUT"
sed -e "s/\${NODE_NAME}/$NODE_NAME/g" \
    -e "s/\${NODE_REGION}/$NODE_REGION/g" \
    "$HERE/fly.toml.template" > "$OUT"

cd "$HERE"

if ! flyctl apps list 2>/dev/null | grep -qE "^\s*$NODE_NAME\s"; then
  echo "==> Creating app $NODE_NAME"
  flyctl apps create "$NODE_NAME" --org personal || true

  echo "==> Creating 10 GB volume in $NODE_REGION"
  flyctl volumes create quidnug_data --app "$NODE_NAME" \
    --region "$NODE_REGION" --size 10 --yes
fi

echo "==> Setting secrets (no-op if unchanged)"
NODE_KEY_B64=$(base64 < "$NODE_KEY_FILE" | tr -d '\n')
flyctl secrets set --app "$NODE_NAME" --stage \
  NODE_KEY="$NODE_KEY_B64" \
  NODE_AUTH_SECRET="$NODE_AUTH_SECRET" \
  ${GRAFANA_REMOTE_WRITE_URL:+GRAFANA_REMOTE_WRITE_URL="$GRAFANA_REMOTE_WRITE_URL"} \
  ${GRAFANA_REMOTE_WRITE_TOKEN:+GRAFANA_REMOTE_WRITE_TOKEN="$GRAFANA_REMOTE_WRITE_TOKEN"}

echo "==> Deploying"
flyctl deploy --app "$NODE_NAME" --config "$OUT" --strategy rolling

echo "==> Status"
flyctl status --app "$NODE_NAME"

PUBLIC_URL="https://${NODE_NAME}.fly.dev"
echo
echo "==> Deployed: $PUBLIC_URL"
echo "    Smoke test:   curl -sS $PUBLIC_URL/api/health"
echo "    Custom DNS:   CNAME node-?.quidnug.com -> ${NODE_NAME}.fly.dev"
