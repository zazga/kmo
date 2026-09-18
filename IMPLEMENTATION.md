# KMO v1 Implementation Plan

This document translates `DESIGN.md` and `ARCHITECTURE.md` into a build order. The goal is to keep every phase small, testable, and independently reviewable while preserving the agreed v1 scope.

No phase should start by adding deferred v1.5/v2.5 features.

## Phase 0 — Repository reset and Go bootstrap

Goal: turn the existing repository into the new Go TUI project without yet building Mattermost functionality.

Tasks:

- Preserve the existing design documents.
- Remove obsolete shell-script implementation artifacts once the Go replacement is in place.
- Initialize the Go module.
- Add the initial directory structure only where code is immediately needed.
- Add dependencies for:
  - Bubble Tea;
  - Bubbles;
  - Lip Gloss;
  - official Mattermost Go client/WebSocket support;
  - TOML parsing.
- Add `Makefile` targets:
  - `make build`;
  - `make test`;
  - `make install`;
  - `make uninstall`;
  - `make uninstall-full`.
- Produce a minimal `cmd/kmo/main.go` that can start and exit cleanly.

Exit criteria:

- `go test ./...` succeeds.
- `make build` creates an arm64 macOS `kmo` binary.
- `make install` installs to `~/bin/kmo` without `sudo`.
- uninstall targets behave exactly as specified in `ARCHITECTURE.md`.

## Phase 1 — Config, defaults, keybindings, and logging

Goal: establish all local application infrastructure before adding network or UI complexity.

Tasks:

### Config

- Implement `~/.config/kmo/config.toml` loading.
- Define the initial config schema:
  - Mattermost server URL;
  - last-opened conversation ID;
  - configurable keybindings.
- Provide hard-coded defaults.
- Persist config changes safely.
- Create parent directories as needed.

### Keybindings

- Define all application actions centrally.
- Implement built-in defaults:
  - Tab: next focus;
  - Cmd+Tab: previous focus;
  - Cmd+F: fuzzy finder;
  - Cmd+R: reply;
  - Cmd+Enter: send;
  - Cmd+?: shortcut overlay;
  - Esc: cancel transient mode;
  - arrow keys and j/k where appropriate.
- Validate configured bindings individually.
- On invalid or conflicting bindings:
  - log the issue;
  - use the default for that action;
  - continue startup.

### Logging

- Implement `log/slog` JSON handlers.
- Write normal application events to:
  - `~/.local/state/kmo/kmo.log`
- Write warnings/errors to:
  - `~/.local/state/kmo/kmo.error.log`
- Ensure secrets and composed message contents are not logged.

Tests:

- config defaults;
- config round trip;
- missing config;
- invalid keybinding fallback;
- duplicate/conflicting binding fallback;
- log path creation where practical.

Exit criteria:

- Config can be loaded with no existing files.
- A modified config can be persisted and loaded again.
- Invalid keybindings never prevent startup.
- Structured JSON log output is produced.

## Phase 2 — macOS Keychain and setup foundations

Goal: make credentials safe and make startup decisions deterministic.

Tasks:

### Keychain

- Implement a small Keychain package using the macOS `security` command.
- Support:
  - read PAT;
  - save/replace PAT;
  - detect missing PAT.
- Keep the PAT out of config and logs.

### Startup state

Implement startup routing:

1. load config;
2. read PAT;
3. if either server URL or PAT is missing, enter setup mode;
4. otherwise proceed to credential validation.

Tests:

- test command construction and error handling without requiring real credentials where possible;
- test startup routing from config/PAT presence state.

Exit criteria:

- KMO can determine whether setup is needed.
- No PAT is written to disk.

## Phase 3 — Mattermost REST service

Goal: create the service boundary between UI and Mattermost.

Tasks:

- Create the official Mattermost REST client from server URL + PAT.
- Implement credential validation/current-user lookup.
- Resolve the single v1 team context automatically.
- Load conversations required for v1:
  - channels;
  - DMs;
  - groups.
- Determine display name and searchable name/slug/username.
- Load unread/mention state needed by the sidebar.
- Fetch recent post history for a conversation.
- Fetch older pages.
- Send normal text posts.
- Send thread replies.
- Mark/read state as required for correct unread behavior.

Keep Mattermost structs as the primary data representation; use only minimal wrappers for UI-specific fields.

Tests:

- unit-test pure conversion/sectioning/sorting/history helpers;
- do not introduce fake-server integration tests yet.

Exit criteria:

- Service API is usable independently of Bubble Tea.
- UI packages do not need to know how HTTP/authentication works.

## Phase 4 — Setup TUI

Goal: make first-run configuration fully terminal-native.

Tasks:

- Build the setup child model.
- Fields:
  - Mattermost server URL;
  - PAT.
- Submit validation asynchronously using `tea.Cmd`.
- Show the exact server/Mattermost error on failure.
- On success:
  - write server URL to config;
  - save PAT to Keychain;
  - transition to main application startup.
- If an existing saved configuration fails validation, return to this same setup view.

Exit criteria:

- A fresh machine can configure KMO entirely inside the TUI.
- Bad credentials never enter the main chat interface.

## Phase 5 — Main UI skeleton and focus model

Goal: establish the final two-column structure before filling it with live data.

Tasks:

### Root model

- Application mode.
- User/team state.
- Connection status.
- Active conversation.
- Central keymap.
- Current focus target.
- Routing of typed Bubble Tea messages.

### Child models

Create working versions of:

- sidebar;
- chat;
- compose;
- status bar;
- shortcut overlay.

### Layout

- Fixed 30-character sidebar.
- Chat pane fills remaining width.
- Compose box always visible at bottom of chat pane.
- Status bar at bottom.
- Terminal-size guard instead of responsive one-pane collapse.
- Default terminal foreground/background colors.
- Accent `#154273`.
- Product-like borders and focus state.

### Focus

- Tab cycles forward.
- Cmd+Tab cycles backward.
- Root model remains authoritative for global shortcuts.

Exit criteria:

- Main view renders cleanly at supported terminal sizes.
- Focus cycles between sidebar, chat, and compose.
- Shortcut overlay opens via Cmd+?.

## Phase 6 — Conversation sidebar

Goal: make all conversations discoverable and navigable.

Tasks:

- Render fixed sections:
  1. Channels;
  2. DMs;
  3. Groups.
- Display unread `•` and mention `@` indicators.
- Sort per section:
  1. unread/mention conversations first;
  2. alphabetically;
  3. read conversations alphabetically.
- Truncate names beyond available width and suffix with `.....`.
- Open the selected conversation directly.
- Restore last-opened conversation from config.
- Persist last-opened conversation when selection changes.

### Fuzzy finder

- Cmd+F enters dedicated fuzzy mode.
- Live filtering while typing.
- Match both display name and slug/username.
- Preserve Channels/DMs/Groups grouping in filtered output.
- Enter opens immediately.
- Esc exits fuzzy mode.

Tests:

- deterministic sorting;
- sectioning;
- unread/mention equal-priority ordering;
- fuzzy matching behavior;
- truncation helper.

Exit criteria:

- Every loaded conversation remains discoverable.
- Sorting and search behave exactly as specified.

## Phase 7 — Chat history and navigation

Goal: make the active conversation useful before adding realtime delivery.

Tasks:

- Load a recent page immediately when opening a conversation.
- Render sender + compact timestamp.
- Today: `HH:MM`.
- Older: include date.
- Render text only.
- Style own messages using accent color.
- Render system messages subtly but visibly.
- System messages are not selectable.
- User messages are selectable with:
  - arrows;
  - j/k.
- Preserve selection sensibly as history changes.
- Infinite upward pagination.
- Preserve visual scroll position when older messages are prepended.
- Continue until history is exhausted.

Tests:

- history merge/order/deduplication;
- message-selection helpers;
- system-message skipping;
- timestamp formatting.

Exit criteria:

- The user can browse as far back as Mattermost history permits without blocking initial render.

## Phase 8 — Inline threads

Goal: support thread reading before thread composition.

Tasks:

- Detect thread roots/replies from Mattermost post data.
- Render replies inline beneath their root.
- Mark thread roots clearly.
- Indent replies distinctly.
- Keep threads expanded by default.
- Ensure message navigation works over selectable user messages in the rendered thread structure.

Tests:

- thread grouping;
- chronological reply ordering;
- selection across roots/replies/system entries.

Exit criteria:

- Existing threads are readable inline without leaving the main chat view.

## Phase 9 — Compose and sending

Goal: complete the outbound chat workflow.

Tasks:

### Normal compose

- Textarea always visible.
- Enter inserts a newline.
- Configured send binding defaults to Cmd+Enter.
- Send asynchronously.
- Keep UI responsive while sending.
- Clear compose on successful send.
- Preserve content on send failure where possible.

### Thread reply

- Cmd+R on selected user message enters reply mode.
- Show `Replying to <name>: "<preview>"` above compose.
- Esc cancels reply mode.
- Send as a Mattermost thread reply using correct root ID.

Tests:

- compose/reply state transitions;
- cancel behavior;
- send-success/send-failure state transitions.

Exit criteria:

- Normal and thread text messages can be sent reliably.

## Phase 10 — WebSocket realtime pipeline

Goal: turn the client into a live Mattermost experience.

Tasks:

- Create one central WebSocket event loop.
- Keep event reading outside Bubble Tea `Update()`.
- Translate relevant server events into typed Bubble Tea messages.
- Handle new posts in:
  - active conversation;
  - inactive conversations;
  - threads.
- Update unread and mention indicators.
- When chat is at bottom:
  - append incoming posts;
  - remain pinned to bottom.
- When user is scrolled upward:
  - append to state;
  - preserve viewport position;
  - show `↓ new messages`.
- Connection states:
  - connected;
  - reconnecting;
  - offline.
- Status bar only; no toast/banner.

Tests:

- typed event -> state transitions;
- unread state transitions;
- mention state transitions;
- new-message indicator behavior;
- reconnect state transitions.

Exit criteria:

- New posts appear without refresh.
- Inactive conversations receive correct unread/mention indication.
- Current UI context survives a socket disconnect.

## Phase 11 — WebSocket reconnect and resynchronization

Goal: make realtime behavior robust enough for daily use.

Tasks:

- Implement reconnect with backoff.
- Avoid tight retry loops.
- On successful reconnect, refresh enough REST state to cover potentially missed events.
- Do not reset active conversation, scroll position, compose text, or focus solely because the socket disconnected.
- Log reconnect lifecycle to structured logs.

Exit criteria:

- Network interruption and restoration does not require restarting KMO.
- Reconnect status remains subtle and accurate.

## Phase 12 — Polish and v1 acceptance pass

Goal: close gaps against `DESIGN.md` without adding new scope.

Tasks:

- Verify minimum terminal-size behavior.
- Verify every default shortcut.
- Verify configurable shortcut fallback behavior.
- Verify long-name truncation.
- Verify status bar wording.
- Verify setup errors preserve exact Mattermost/server text.
- Verify last-opened conversation restoration.
- Verify own-message styling.
- Verify system-message rendering.
- Verify thread rendering and reply context.
- Verify log output and absence of secrets.
- Run `go test ./...`.
- Run `go vet ./...` manually even though no CI is required.
- Build/install on macOS arm64 using `make install`.

## v1 acceptance checklist

v1 is complete only when all of the following work in a real Mattermost environment:

- [ ] first-run setup through TUI;
- [ ] PAT stored in Keychain;
- [ ] subsequent automatic connection;
- [ ] automatic single-team selection;
- [ ] all channels/DMs/groups visible and discoverable;
- [ ] unread and mention indicators;
- [ ] live fuzzy find;
- [ ] last conversation restoration;
- [ ] text history loading;
- [ ] infinite upward pagination;
- [ ] user-message selection;
- [ ] inline expanded threads;
- [ ] multiline compose;
- [ ] normal message sending;
- [ ] thread reply sending;
- [ ] live incoming posts;
- [ ] background WebSocket reconnect;
- [ ] scroll preservation while reading old messages;
- [ ] `↓ new messages` indicator;
- [ ] configurable keybindings;
- [ ] shortcut overlay;
- [ ] structured normal/error logs;
- [ ] arm64 build and `make install`;
- [ ] unit test suite passing.

## Explicitly deferred

Do not add these while implementing v1 unless the design is intentionally revised first.

### v1.5

- macOS desktop notifications;
- local message/conversation cache;
- fake Mattermost integration-test backend.

### v2.5

- edit messages;
- delete messages;
- search message history.

### Later

- reactions;
- attachments/rich media;
- special code-block renderer;
- URL opening;
- presence status;
- multiple servers/accounts;
- multiple-team navigation.

## Development rule

Implement one phase at a time. Before moving to the next phase:

1. the current phase must build;
2. its relevant unit tests must pass;
3. its exit criteria must be demonstrably satisfied;
4. scope discovered during implementation should be compared against `DESIGN.md` and `ARCHITECTURE.md` rather than silently expanding v1.
