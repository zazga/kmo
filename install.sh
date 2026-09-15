#!/bin/bash
set -euo pipefail

LABEL="com.local.keep-mattermost-online"
APP_DIR="$HOME/.local/share/keep-mattermost-online"
BIN="$APP_DIR/keep-online.sh"
PLIST="$HOME/Library/LaunchAgents/$LABEL.plist"
SERVICE="keep-mattermost-online"
INTERVAL="240"

printf "Keep Mattermost Online installer\n\n"
read -r -p "Mattermost URL (bijv. https://mattermost.bedrijf.nl): " MM_URL
MM_URL="${MM_URL%/}"

printf "Personal Access Token (wordt veilig opgeslagen in macOS Keychain): "
stty -echo
IFS= read -r TOKEN
stty echo
printf "\n"

if [[ -z "$MM_URL" || -z "$TOKEN" ]]; then
  echo "Fout: URL en token zijn verplicht." >&2
  exit 1
fi

if [[ ! "$MM_URL" =~ ^https?:// ]]; then
  echo "Fout: Mattermost URL moet beginnen met http:// of https://" >&2
  exit 1
fi

TMP_ME="$(mktemp -t keep-mattermost-online-me)"
trap 'rm -f "$TMP_ME"' EXIT

HTTP_CODE="$(/usr/bin/curl --silent --show-error --output "$TMP_ME" \
  --write-out '%{http_code}' \
  --connect-timeout 10 \
  --max-time 20 \
  --header "Authorization: Bearer $TOKEN" \
  "$MM_URL/api/v4/users/me" || true)"

if [[ "$HTTP_CODE" != "200" ]]; then
  echo "Fout: Mattermost kon de gebruiker achter dit token niet ophalen (HTTP ${HTTP_CODE:-curl_error})." >&2
  cat "$TMP_ME" >&2 2>/dev/null || true
  exit 1
fi

USER_ID="$(sed -n 's/.*"id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$TMP_ME" | head -n 1)"
USERNAME="$(sed -n 's/.*"username"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$TMP_ME" | head -n 1)"

if [[ -z "$USER_ID" ]]; then
  echo "Fout: kon user ID niet uit /api/v4/users/me halen." >&2
  exit 1
fi

printf "Mattermost gebruiker gevonden: %s (%s)\n" "${USERNAME:-onbekend}" "$USER_ID"

mkdir -p "$APP_DIR" "$HOME/Library/LaunchAgents" "$HOME/Library/Logs"

# Store/replace token in the user's login Keychain.
security delete-generic-password -a "$USER" -s "$SERVICE" >/dev/null 2>&1 || true
security add-generic-password -a "$USER" -s "$SERVICE" -w "$TOKEN" -U >/dev/null
unset TOKEN

cat >"$APP_DIR/config" <<CFG
MM_URL='$MM_URL'
CFG
chmod 600 "$APP_DIR/config"

cat >"$BIN" <<'SCRIPT'
#!/bin/bash
set -u

APP_DIR="$HOME/.local/share/keep-mattermost-online"
CONFIG="$APP_DIR/config"
SERVICE="keep-mattermost-online"

[[ -f "$CONFIG" ]] || exit 2
# shellcheck disable=SC1090
source "$CONFIG"

TOKEN="$(security find-generic-password -a "$USER" -s "$SERVICE" -w 2>/dev/null)" || exit 3

ME_OUT="$(mktemp -t keep-mattermost-online-me)"
STATUS_OUT="$(mktemp -t keep-mattermost-online-status)"
ERR_OUT="$(mktemp -t keep-mattermost-online-err)"
trap 'rm -f "$ME_OUT" "$STATUS_OUT" "$ERR_OUT"' EXIT

ME_CODE="$(/usr/bin/curl --silent --show-error --output "$ME_OUT" \
  --write-out '%{http_code}' \
  --connect-timeout 10 \
  --max-time 20 \
  --header "Authorization: Bearer $TOKEN" \
  "$MM_URL/api/v4/users/me" 2>"$ERR_OUT" || true)"

if [[ "$ME_CODE" != "200" ]]; then
  printf '%s GET /users/me HTTP %s: ' "$(date '+%Y-%m-%d %H:%M:%S')" "${ME_CODE:-curl_error}" >&2
  cat "$ERR_OUT" >&2 2>/dev/null || true
  cat "$ME_OUT" >&2 2>/dev/null || true
  printf '\n' >&2
  exit 4
fi

USER_ID="$(sed -n 's/.*"id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$ME_OUT" | head -n 1)"
if [[ -z "$USER_ID" ]]; then
  printf '%s Kon user ID niet uit /api/v4/users/me halen.\n' "$(date '+%Y-%m-%d %H:%M:%S')" >&2
  exit 5
fi

: >"$ERR_OUT"
HTTP_CODE="$(/usr/bin/curl --silent --show-error --output "$STATUS_OUT" \
  --write-out '%{http_code}' \
  --request PUT \
  --connect-timeout 10 \
  --max-time 20 \
  --header "Authorization: Bearer $TOKEN" \
  --header 'Content-Type: application/json' \
  --data "{\"user_id\":\"$USER_ID\",\"status\":\"online\"}" \
  "$MM_URL/api/v4/users/$USER_ID/status" 2>"$ERR_OUT" || true)"

if [[ "$HTTP_CODE" != "200" ]]; then
  printf '%s PUT /users/%s/status HTTP %s: ' "$(date '+%Y-%m-%d %H:%M:%S')" "$USER_ID" "${HTTP_CODE:-curl_error}" >&2
  cat "$ERR_OUT" >&2 2>/dev/null || true
  cat "$STATUS_OUT" >&2 2>/dev/null || true
  printf '\n' >&2
  exit 6
fi
SCRIPT
chmod 700 "$BIN"

cat >"$PLIST" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>$LABEL</string>
  <key>ProgramArguments</key>
  <array>
    <string>$BIN</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>StartInterval</key>
  <integer>$INTERVAL</integer>
  <key>StandardOutPath</key>
  <string>$HOME/Library/Logs/keep-mattermost-online.log</string>
  <key>StandardErrorPath</key>
  <string>$HOME/Library/Logs/keep-mattermost-online.err.log</string>
  <key>ProcessType</key>
  <string>Background</string>
</dict>
</plist>
PLIST

plutil -lint "$PLIST" >/dev/null

launchctl bootout "gui/$(id -u)" "$PLIST" >/dev/null 2>&1 || true
launchctl bootstrap "gui/$(id -u)" "$PLIST"
launchctl kickstart -k "gui/$(id -u)/$LABEL" || true

echo
echo "Geïnstalleerd. Je Mattermost-status wordt iedere $INTERVAL seconden op online gezet."
echo "Je Mattermost user ID wordt automatisch via /api/v4/users/me bepaald."
echo "Logs: ~/Library/Logs/keep-mattermost-online.err.log"
echo "Verwijderen: voer uninstall.sh uit uit dit pakket."
