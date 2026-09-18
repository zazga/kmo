# KMO v1 Technical Architecture

## Overview

KMO v1 is a single self-contained Go binary for macOS arm64. The terminal UI is built with Bubble Tea, Bubbles, and Lip Gloss. Mattermost integration uses the official Mattermost Go client and WebSocket support, kept behind a dedicated service layer.

The architecture favors a small number of clear packages, typed Bubble Tea messages, non-blocking network work, and explicit ownership of UI state.

## Runtime target

- Language: Go.
- Initial target: macOS arm64 / Apple Silicon.
- Output binary: `kmo`.
- No external runtime dependency is required after compilation.
- macOS Keychain access is performed by invoking the built-in `security` command.

## UI stack

- Bubble Tea: application/event loop.
- Bubbles: reusable terminal components such as viewport and textarea where appropriate.
- Lip Gloss: layout, borders, focus styling, truncation, and accent styling.

Primary accent:

```text
RGB: 21, 66, 115
HEX: #154273
```

Terminal default foreground/background colors remain authoritative.

## Mattermost integration

Use the official Mattermost Go client rather than hand-building the full HTTP/WebSocket protocol.

The Mattermost client must remain behind an internal service boundary so the UI does not directly depend on transport details.

Responsibilities of the Mattermost service:

- create/authenticate the REST client using the PAT;
- load the current user;
- resolve the v1 team context;
- retrieve channels, DMs, and group conversations;
- retrieve paginated post history;
- send normal posts;
- send thread replies;
- mark/read/update state needed for unread rendering;
- establish and maintain the WebSocket connection;
- expose normalized application events to the Bubble Tea layer.

Where practical, Mattermost's own structs are used directly. Small UI-specific wrappers are acceptable only when needed for presentation state that does not belong in Mattermost objects.

## Suggested package layout

```text
kmo/
├── cmd/
│   └── kmo/
│       └── main.go
├── internal/
│   ├── app/
│   │   ├── model.go
│   │   ├── update.go
│   │   ├── view.go
│   │   ├── messages.go
│   │   └── keymap.go
│   ├── ui/
│   │   ├── sidebar/
│   │   ├── chat/
│   │   ├── compose/
│   │   ├── setup/
│   │   ├── statusbar/
│   │   └── styles/
│   ├── mattermost/
│   │   ├── client.go
│   │   ├── websocket.go
│   │   ├── events.go
│   │   └── history.go
│   ├── config/
│   │   ├── config.go
│   │   └── keybindings.go
│   ├── keychain/
│   │   └── keychain.go
│   └── logging/
│       └── logging.go
├── Makefile
├── go.mod
├── go.sum
├── DESIGN.md
└── ARCHITECTURE.md
```

This layout is a starting point, not a requirement to create one file per concept before code justifies it. Avoid empty abstraction-only packages.

## Bubble Tea model hierarchy

Use one root model with focused child models.

### Root model owns

- application mode: setup / main;
- authenticated Mattermost user;
- selected team context;
- connection/WebSocket status;
- active conversation;
- global conversation collection;
- global keymap;
- focus target;
- high-level routing of typed messages;
- coordination between sidebar, chat, and compose.

### Child models

#### Sidebar

Owns:

- section rendering;
- current sidebar cursor;
- fuzzy search state/query;
- filtered view;
- sidebar-specific scrolling.

#### Chat

Owns:

- current viewport;
- selected user message;
- rendered inline thread structure;
- oldest/newest loaded positions;
- history-loading state;
- `new messages` indicator state.

#### Compose

Owns:

- textarea state;
- normal/reply mode;
- reply target context;
- draft content for the active compose session.

#### Setup

Owns:

- server URL field;
- PAT field;
- validation/error display state.

## Keybindings

All bindings are defined centrally by the root model/keymap package.

Defaults include:

- `Tab`: next focus target.
- `Cmd+Tab`: previous focus target.
- `Cmd+F`: fuzzy finder.
- `Cmd+R`: reply to selected message.
- `Cmd+Enter`: send composed message.
- `Cmd+?`: shortcut overlay.
- `Esc`: cancel transient modes such as fuzzy search or reply mode.
- Arrow keys + `j`/`k`: navigation where applicable.

`Cmd+Q` and `Cmd+W` are deliberately not application bindings.

All application bindings are configurable through TOML.

If a configured binding is invalid or conflicts in a way KMO cannot safely resolve:

1. log the problem to `kmo.error.log`;
2. fall back to the built-in default for that action;
3. continue startup.

A bad optional binding must not make the entire application unusable.

## Bubble Tea message flow

Use typed Bubble Tea messages rather than a generic result wrapper.

Representative message types:

```go
type connectedMsg struct { /* ... */ }
type connectionLostMsg struct { /* ... */ }
type reconnectingMsg struct { /* ... */ }
type conversationsLoadedMsg struct { /* ... */ }
type messagesLoadedMsg struct { /* ... */ }
type olderMessagesLoadedMsg struct { /* ... */ }
type newPostMsg struct { /* ... */ }
type postSentMsg struct { /* ... */ }
type setupValidatedMsg struct { /* ... */ }
type errMsg struct { /* ... */ }
```

Names may evolve with implementation, but messages should remain specific enough that `Update()` can handle explicit state transitions.

## Non-blocking work

Never perform blocking network I/O directly inside the synchronous Bubble Tea `Update()` path.

REST operations are issued through `tea.Cmd` functions or controlled goroutines that return typed messages.

WebSocket reading runs independently and feeds events into the Bubble Tea program.

The UI must remain responsive during:

- initial REST loading;
- history pagination;
- sends;
- WebSocket reconnects;
- setup validation.

## WebSocket architecture

Use one central WebSocket event loop.

Conceptually:

```text
Mattermost WebSocket
        |
        v
central websocket event loop
        |
        +-- posted
        +-- channel/unread events
        +-- reconnect state
        +-- future edit/delete events
        |
        v
small event handlers
        |
        v
typed Bubble Tea messages
```

The central event loop owns connection lifecycle and event ordering concerns. Event-specific handlers remain small and translate Mattermost events into application-level messages.

### Reconnect behavior

- Do not replace/reset the active UI when the socket disconnects.
- Update root connection state to `reconnecting` / `offline` as appropriate.
- Retry automatically in the background.
- On successful reconnect, return to `connected`.
- Where necessary, refresh server state after reconnect to cover events potentially missed while disconnected.

Reconnect policy should use backoff rather than a tight retry loop.

## Conversation model and ordering

Use Mattermost structs for server data, with a minimal UI wrapper if needed to track:

- conversation type: channel / DM / group;
- display name;
- searchable slug/username;
- unread flag;
- mention flag;
- selected/filter state.

Display groups are always rendered in this order:

1. Channels
2. DMs
3. Groups

Within each section:

1. unread conversations first (mention and non-mention equal priority);
2. alphabetical by display name;
3. read conversations alphabetically.

Sorting must be deterministic and covered by unit tests.

## Message/history strategy

The goal is to make full available history reachable without making startup wait for all of it.

### Initial open

- Fetch a useful recent page immediately.
- Render as soon as that page is available.
- Track whether older pages remain.

### Upward scrolling

- When the viewport approaches the oldest loaded message, request the next older page.
- Preserve visual scroll position when older messages are inserted above the viewport.
- Continue until Mattermost reports no older history.

### Incoming posts

If the user is at/near the bottom:

- append the new post;
- remain pinned to the bottom.

If the user has scrolled upward:

- append the post to application state;
- do not move the viewport;
- show `↓ new messages`.

## Threads

Inline thread rendering is owned by the chat child model.

- Root posts appear in chronological flow.
- A post with thread replies receives a thread marker.
- Replies render beneath the root with indentation.
- Threads are expanded by default.
- Selection skips system messages but may select root/reply user posts.
- `Cmd+R` sets the compose model into reply mode against the selected Mattermost post.
- Sending in reply mode creates a Mattermost post with the appropriate root/thread reference.

## Setup and authentication

### Config location

```text
~/.config/kmo/config.toml
```

The config contains non-sensitive values only.

Expected categories include:

- Mattermost server URL;
- configurable keybindings;
- future non-sensitive UI preferences.

### PAT storage

The PAT is stored in macOS Keychain.

The Go application invokes macOS `security` for add/find operations, consistent with the previous KMO approach. Do not write the PAT to disk or logs.

### Startup sequence

```text
start
  |
  +--> load TOML config
  +--> read PAT from Keychain
  |
  +--> incomplete? --> setup model
  |
  +--> complete --> validate Mattermost credentials
                    |
                    +--> failure --> setup model + exact error
                    |
                    +--> success --> load user/team/conversations
                                      |
                                      +--> start WebSocket
                                      +--> open last conversation
```

The selected/last-opened conversation identifier may be persisted in the TOML config because it is non-sensitive.

## Configuration parsing

Use a TOML library appropriate for Go.

Config loading should:

- create sensible defaults when the file is missing;
- validate individual settings;
- preserve hard-coded default keybindings;
- fall back per invalid key rather than rejecting the whole config;
- create parent directories as needed when persisting configuration.

## Logging

Use Go standard-library `log/slog`.

Log format: JSON Lines / JSON handler.

Locations:

```text
~/.local/state/kmo/kmo.log
~/.local/state/kmo/kmo.error.log
```

### `kmo.log`

Normal lifecycle and diagnostic events, for example:

- startup;
- configuration loaded;
- authenticated user/team resolved;
- WebSocket connected/reconnected;
- conversation opened;
- history page loaded.

### `kmo.error.log`

Warnings/errors requiring attention, for example:

- invalid keybinding fallback;
- REST request failures;
- WebSocket failures;
- malformed server events;
- setup validation errors.

Structured fields should include where relevant:

- timestamp;
- level;
- component;
- message;
- operation;
- conversation/channel ID;
- Mattermost request/error identifiers.

Never log:

- PAT values;
- authorization headers;
- message compose contents unless explicitly needed for future opt-in diagnostics.

## Error handling

Distinguish errors by where they belong:

### Setup errors

- Render the exact server/Mattermost error in setup.
- Keep the user in setup so they can correct server URL/PAT.

### Runtime recoverable errors

- Log detailed error.
- Keep the current UI state where possible.
- Surface only minimal state through the status bar if relevant.

### Fatal initialization errors

Examples could include inability to initialize mandatory terminal state or filesystem paths. These may terminate with a clear stderr message after logging when possible.

## Installation

### Build

The initial Makefile targets macOS arm64.

Expected targets:

```text
make build
make test
make install
make uninstall
make uninstall-full
```

### `make install`

- Build `kmo` for macOS arm64.
- Install to:

```text
~/bin/kmo
```

- No `sudo` required.

### `make uninstall`

Remove only:

```text
~/bin/kmo
```

Do not remove configuration, logs, or Keychain entries.

### `make uninstall-full`

Remove:

```text
~/bin/kmo
~/.config/kmo/
~/.local/state/kmo/
```

Do not remove the PAT from macOS Keychain.

## Testing strategy for v1

Unit tests are part of v1 from the start.

Minimum areas:

- TOML config loading/defaults;
- invalid keybinding fallback;
- keybinding conflict handling;
- conversation sectioning/sorting;
- fuzzy-match result behavior where custom logic exists;
- typed event -> state transitions;
- chat message selection skipping system messages;
- unread/mention state transitions;
- history merge/deduplication/order;
- reply-mode state transitions;
- reconnect state transitions.

Integration tests with a fake Mattermost service are deferred to v1.5.

No GitHub Actions/CI pipeline is required for v1.

## Dependency policy

Prefer:

1. Go standard library;
2. official Mattermost Go packages;
3. Charmbracelet libraries required for the TUI;
4. one well-maintained TOML library.

Avoid adding dependencies for functionality readily available in the standard library or the selected core packages.

## Implementation principles

- Keep transport/network details out of UI models.
- Keep blocking operations out of Bubble Tea `Update()`.
- Prefer explicit typed messages over generic event/result maps.
- Keep the root model responsible for coordination rather than every rendering detail.
- Let child models own local cursor/view state.
- Use Mattermost structs directly until an application-specific abstraction solves a concrete problem.
- Keep v1 scope strict; deferred features should not leak into the core design prematurely.
- Make reconnect/error states recoverable whenever possible.
- Optimize for a polished keyboard-first Mattermost chat experience, not feature parity with the desktop client.
