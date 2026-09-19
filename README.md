# KMO

KMO is a keyboard-first Mattermost terminal client for macOS, built with Go and Bubble Tea.

## Features

- Live Mattermost messages over WebSocket.
- Send multiline text messages.
- Channels, direct messages, and group messages in a two-column TUI.
- Inline expanded threads and thread replies.
- Unread (`•`) and mention (`@`) indicators.
- Live fuzzy filtering of conversations.
- Paginated history while scrolling upward.
- Optional Telegram forwarding for incoming Mattermost DMs.
- Reply from Telegram by replying to a forwarded KMO message.
- Persistent Telegram reply routing for 30 days.
- Background `kmo daemon` managed by macOS `launchd`.
- Mattermost status is refreshed to `online` every 240 seconds by the daemon.
- PAT and Telegram bot token stored in macOS Keychain.
- JSON structured logs.

## Requirements

- macOS on Apple Silicon (arm64).
- Go 1.25 or newer to build from source.
- A Mattermost Personal Access Token.
- A terminal window of at least 73×16 cells for the mandatory two-column layout.
- A Telegram bot token only when Telegram forwarding is enabled.

## Build and install

```sh
make test
make build
make install
```

The default install location is `~/bin/kmo`.

`make install` also installs and starts the `com.local.kmo-daemon` LaunchAgent. The daemon always runs so the Mattermost always-online behavior continues even when Telegram forwarding is disabled.

Run the TUI with:

```sh
kmo
```

The same binary is used by launchd as:

```sh
kmo daemon
```

## Setup

KMO keeps already-known setup values filled. Mattermost and Telegram secrets are shown masked and remain stored in macOS Keychain.

Telegram forwarding is optional. In setup:

1. Press `Ctrl+T` to enable Telegram forwarding.
2. Enter the Telegram bot token.
3. Press Enter to validate the setup.
4. When KMO asks, send a new `/start` message to the bot.
5. KMO accepts the pairing only when exactly one Telegram chat is detected and sends `KMO Telegram forwarding is actief.` as a test message.

Old `/start` updates are ignored during pairing.

## Telegram forwarding

Only incoming Mattermost DMs from the other person are forwarded. KMO ignores your own posts and attachment-only messages. If a DM contains text and attachments, only the text is forwarded.

Forwarded messages use a compact header such as:

```text
Kevin · 09:42
Can you check this?
```

Mattermost thread replies are marked with `· reply`. Long messages are split and numbered `(1/3)`, `(2/3)`, and so on; every part can be replied to.

To send a message back to Mattermost, use Telegram's Reply action on a forwarded KMO message. The Telegram reply is sent as a new message in the corresponding Mattermost DM. Standalone messages to the bot are ignored. Successful sends are silent; errors include the technical Mattermost error text.

Reply mappings expire after 30 days. Replies to expired mappings return `Deze reply is verlopen.`

## Status

The TUI status line shows the existing TUI Mattermost connection and the daemon's Telegram connection:

```text
MM: connected · TG: connected
```

Telegram states include `disabled`, `connecting`, `connected`, `reconnecting`, and `unavailable`. The TUI receives daemon status through a local Unix socket with heartbeat monitoring.

## Files

Config:

```text
~/.config/kmo/config.toml
```

Persistent Telegram routing state:

```text
~/.local/state/kmo/telegram.json
```

Daemon status socket:

```text
~/.local/state/kmo/kmo.sock
```

Logs:

```text
~/.local/state/kmo/kmo.log
~/.local/state/kmo/kmo.error.log
```

Mattermost PAT is stored in macOS Keychain under service `kmo`. The Telegram bot token is stored under service `kmo-telegram`.

## Default shortcuts

| Action | Binding |
|---|---|
| Focus next | `Tab` |
| Focus previous | `Ctrl+Tab` |
| Find conversations | `Ctrl+F` |
| Reply in thread | `Ctrl+R` |
| Send | `Ctrl+Enter` |
| Shortcuts | `Ctrl+H` |
| Quit | `Ctrl+Q` |
| Cancel mode | `Esc` |
| Navigate | arrows or `j` / `k` |

All bindings can be overridden in `config.toml`. Invalid custom bindings fall back to defaults and are logged.

## Uninstall

Remove the daemon LaunchAgent and binary:

```sh
make uninstall
```

Remove the daemon, binary, config, logs, and local state while intentionally keeping Keychain secrets:

```sh
make uninstall-full
```
