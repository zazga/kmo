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

read -r -p "Mattermost user ID: " USER_ID

printf "Personal Access Token (wordt veilig opgeslagen in macOS Keychain): "
stty -echo
IFS= read -r TOKEN
stty echo
printf "\n"

if [[ -z "$MM_URL" || -z "$USER_ID" || -z "$TOKEN" ]]; then
  echo "Fout: URL, user ID en token zijn verplicht." >&2
  exit 1
fi

if [[ ! "$MM_URL" =~ ^https?:// ]]; then
  echo "Fout: Mattermost URL moet beginnen met http:// of https://" >&2
  exit 1
fi

mkdir -p "$APP_DIR" "$HOME/Library/LaunchAgents" "$HOME/Library/Logs"

# Store/replace token in the user's login Keychain.
security delete-generic-password -a "$USER" -s "$SERVICE" >/dev/null 2>&1 || true
security add-generic-password -a "$USER" -s "$SERVICE" -w "$TOKEN" -U >/dev/null
unset TOKEN

cat >"$APP_DIR/config" <<CFG
MM_URL='$MM_URL'
USER_ID='$USER_ID'
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

# Set online status. We intentionally do one API call per launchd run.
HTTP_CODE="$(/usr/bin/curl --silent --show-error --output /tmp/keep-mattermost-online.$$.out \
  --write-out '%{http_code}' \
  --request PUT \
  --connect-timeout 10 \
  --max-time 20 \
  --header "Authorization: Bearer $TOKEN" \
  --header 'Content-Type: application/json' \
  --data "{\"user_id\":\"$USER_ID\",\"status\":\"online\"}" \
  "$MM_URL/api/v4/users/$USER_ID/status" 2>/tmp/keep-mattermost-online.$$.err || true)"

if [[ "$HTTP_CODE" != "200" ]]; then
  printf '%s HTTP %s: ' "$(date '+%Y-%m-%d %H:%M:%S')" "${HTTP_CODE:-curl_error}" >&2
  cat /tmp/keep-mattermost-online.$$.err >&2 2>/dev/null || true
  cat /tmp/keep-mattermost-online.$$.out >&2 2>/dev/null || true
  printf '\n' >&2
fi

rm -f /tmp/keep-mattermost-online.$$.out /tmp/keep-mattermost-online.$$.err
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
echo "Logs: ~/Library/Logs/keep-mattermost-online.err.log"
echo "Verwijderen: voer uninstall.sh uit uit dit pakket."
