#!/bin/bash
set -euo pipefail
LABEL="com.local.mattermost-keep-online"
PLIST="$HOME/Library/LaunchAgents/$LABEL.plist"
APP_DIR="$HOME/.local/share/mattermost-keep-online"
SERVICE="mattermost-keep-online"

launchctl bootout "gui/$(id -u)" "$PLIST" >/dev/null 2>&1 || true
rm -f "$PLIST"
rm -rf "$APP_DIR"
security delete-generic-password -a "$USER" -s "$SERVICE" >/dev/null 2>&1 || true
rm -f "$HOME/Library/Logs/mattermost-keep-online.log" "$HOME/Library/Logs/mattermost-keep-online.err.log"
echo "Mattermost Keep Online is verwijderd."
