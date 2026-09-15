#!/bin/bash
set -euo pipefail
APP_DIR="$HOME/.local/share/keep-mattermost-online"
CONFIG="$APP_DIR/config"
SERVICE="keep-mattermost-online"

if [[ ! -f "$CONFIG" ]]; then
  echo "Niet geïnstalleerd. Draai eerst install.sh." >&2
  exit 1
fi
# shellcheck disable=SC1090
source "$CONFIG"
TOKEN="$(security find-generic-password -a "$USER" -s "$SERVICE" -w)"

ME_OUT="$(mktemp -t keep-mattermost-online-me)"
STATUS_OUT="$(mktemp -t keep-mattermost-online-status)"
trap 'rm -f "$ME_OUT" "$STATUS_OUT"' EXIT

printf "Test API-verbinding en bepaal gebruiker...\n"
ME_CODE="$(/usr/bin/curl --silent --show-error --output "$ME_OUT" \
  --write-out '%{http_code}' \
  --header "Authorization: Bearer $TOKEN" \
  "$MM_URL/api/v4/users/me" || true)"

if [[ "$ME_CODE" != "200" ]]; then
  echo "Fout: /api/v4/users/me gaf HTTP $ME_CODE" >&2
  cat "$ME_OUT" >&2 2>/dev/null || true
  exit 1
fi

USER_ID="$(sed -n 's/.*"id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$ME_OUT" | head -n 1)"
USERNAME="$(sed -n 's/.*"username"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$ME_OUT" | head -n 1)"

if [[ -z "$USER_ID" ]]; then
  echo "Fout: kon user ID niet bepalen." >&2
  exit 1
fi

printf "Gebruiker: %s (%s)\n" "${USERNAME:-onbekend}" "$USER_ID"
printf "Test status-update...\n"

STATUS_CODE="$(/usr/bin/curl --silent --show-error --output "$STATUS_OUT" \
  --write-out '%{http_code}' \
  --request PUT \
  --header "Authorization: Bearer $TOKEN" \
  --header 'Content-Type: application/json' \
  --data "{\"user_id\":\"$USER_ID\",\"status\":\"online\"}" \
  "$MM_URL/api/v4/users/$USER_ID/status" || true)"

if [[ "$STATUS_CODE" != "200" ]]; then
  echo "Fout: status-update gaf HTTP $STATUS_CODE" >&2
  cat "$STATUS_OUT" >&2 2>/dev/null || true
  exit 1
fi

printf "OK — verbinding en status-update werken.\n"
