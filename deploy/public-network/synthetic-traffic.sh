#!/usr/bin/env bash
# Emit realistic baseline traffic against the public network. Runs as a
# cron on exactly one seed node (don't double-run it or charts will
# overstate activity).
#
# Env:
#   NODE_URL         e.g. https://node-1.quidnug.com
#   TRAFFIC_KEY_DIR  dir of throwaway quid key JSONs — generated on
#                    first run if empty
#   BUDGET_PER_RUN   max transactions this invocation (default 12)
#
# Cron schedule: every 5 minutes from crontab inside the seed's VM, e.g.
#   */5 * * * * /app/synthetic-traffic.sh >> /var/log/synthetic.log 2>&1
#
# Design:
#   - Transactions are scoped to `examples.public.quidnug.com` so they
#     never pollute real app domains.
#   - Each run picks a random mix of: new identity, trust edge, event
#     append, optional rotation.
#   - Keys are reused across runs for the event-stream pattern; new
#     identities slowly accrete so the identity-registry chart grows.

set -euo pipefail

: "${NODE_URL:?}"
: "${TRAFFIC_KEY_DIR:=/data/synthetic-keys}"
BUDGET="${BUDGET_PER_RUN:-12}"

mkdir -p "$TRAFFIC_KEY_DIR"

DOMAIN="examples.public.quidnug.com"

# Ensure we have at least 10 throwaway identities.
existing=$(ls "$TRAFFIC_KEY_DIR"/*.json 2>/dev/null | wc -l)
need=$(( 10 - existing ))
if (( need > 0 )); then
  echo "[$(date -Is)] seeding $need throwaway keys"
  for i in $(seq 1 "$need"); do
    name="demo-$(head -c 6 /dev/urandom | od -An -tx1 | tr -d ' \n')"
    quidnug-cli keygen --out "$TRAFFIC_KEY_DIR/$name.json" --name "$name" --node "$NODE_URL"
  done
fi

ALL=($(ls "$TRAFFIC_KEY_DIR"/*.json))
random_key() { echo "${ALL[$(( RANDOM % ${#ALL[@]} ))]}"; }

EVENT_TYPES=(
  "profile.updated" "order.placed" "shipment.received"
  "reputation.attested" "credential.issued" "audit.logged"
)

posted=0
while (( posted < BUDGET )); do
  case $(( RANDOM % 5 )) in
    0)  # trust edge
      a=$(random_key); b=$(random_key)
      [[ "$a" == "$b" ]] && continue
      level=$(printf "0.%d" $(( 30 + RANDOM % 60 )))
      quidnug-cli grant-trust --node "$NODE_URL" --key "$a" \
        --trustee-key "$b" --domain "$DOMAIN" --trust-level "$level" \
        >/dev/null 2>&1 && echo "[$(date -Is)] TRUST  $(basename $a .json) -> $(basename $b .json) @ $level" || true
      ;;
    1|2)  # event append
      k=$(random_key)
      et="${EVENT_TYPES[$(( RANDOM % ${#EVENT_TYPES[@]} ))]}"
      quidnug-cli append-event --node "$NODE_URL" --key "$k" \
        --event-type "$et" --payload '{"seq":'$RANDOM'}' \
        >/dev/null 2>&1 && echo "[$(date -Is)] EVENT  $(basename $k .json) $et" || true
      ;;
    3)  # identity metadata update
      k=$(random_key)
      quidnug-cli update-identity --node "$NODE_URL" --key "$k" \
        --meta "updatedAt=$(date -Is)" \
        >/dev/null 2>&1 && echo "[$(date -Is)] IDENT  $(basename $k .json)" || true
      ;;
    4)  # occasional new identity
      if (( RANDOM % 5 == 0 )); then
        name="demo-$(head -c 6 /dev/urandom | od -An -tx1 | tr -d ' \n')"
        quidnug-cli keygen --out "$TRAFFIC_KEY_DIR/$name.json" --name "$name" --node "$NODE_URL" \
          >/dev/null 2>&1 && echo "[$(date -Is)] IDENT+ $name" || true
        ALL+=("$TRAFFIC_KEY_DIR/$name.json")
      fi
      ;;
  esac
  posted=$(( posted + 1 ))
  sleep $(( 1 + RANDOM % 3 ))
done

echo "[$(date -Is)] done ($posted attempted)"
