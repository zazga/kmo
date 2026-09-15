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

printf "Test API-verbinding...\n"
/usr/bin/curl --fail-with-body --silent --show-error \
  --header "Authorization: Bearer $TOKEN" \
  "$MM_URL/api/v4/users/me"
printf "\n\nVerbinding werkt.\n"
