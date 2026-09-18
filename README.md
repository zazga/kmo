# KMO

KMO is a keyboard-first Mattermost terminal client for macOS, built with Go and Bubble Tea.

## v1 features

- Live Mattermost messages over WebSocket.
- Send multiline text messages.
- Channels, direct messages, and group messages in a two-column TUI.
- Inline expanded threads and thread replies.
- Unread (`•`) and mention (`@`) indicators.
- Live fuzzy filtering of conversations.
- Paginated history while scrolling upward.
- Setup flow for server URL + Personal Access Token.
- PAT stored in macOS Keychain, never in the TOML config.
- JSON structured logs.

See `DESIGN.md`, `ARCHITECTURE.md`, and `IMPLEMENTATION.md` for the agreed v1 design.

## Requirements

- macOS on Apple Silicon (arm64).
- Go 1.25 or newer to build from source.
- A Mattermost Personal Access Token.
- A terminal window of at least 72×16 cells for the mandatory two-column layout.

## Build and install

```sh
make test
make build
make install
```

The default install location is `~/bin/kmo`.

Run:

```sh
kmo
```

On first run, KMO opens its setup screen. Enter your Mattermost server URL and PAT. The PAT is validated before it is stored in Keychain.

## Files

Config:

```text
~/.config/kmo/config.toml
```

Logs:

```text
~/.local/state/kmo/kmo.log
~/.local/state/kmo/kmo.error.log
```

The PAT is stored in macOS Keychain under service `kmo`.

## Default shortcuts

Terminal emulators differ in how they encode the Command key. KMO supports the requested defaults when the terminal sends those key combinations, plus portable fallbacks so the client remains usable.

| Action | Preferred | Portable fallback |
|---|---|---|
| Focus next | `Tab` | `Tab` |
| Focus previous | `Cmd+Tab` | `Shift+Tab` |
| Find conversations | `Cmd+F` | `Ctrl+F` |
| Reply in thread | `Cmd+R` | `Ctrl+R` |
| Send | `Cmd+Enter` | `Ctrl+Enter` |
| Shortcuts | `Cmd+?` | `Ctrl+?` |
| Cancel mode | `Esc` | `Esc` |
| Navigate | arrows | `j` / `k` |

All bindings can be overridden in `config.toml`. Invalid custom bindings fall back to defaults and are logged.

## Uninstall

Remove only the binary:

```sh
make uninstall
```

Remove binary, config, and logs (Keychain PAT remains):

```sh
make uninstall-full
```
