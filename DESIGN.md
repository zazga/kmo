# KMO v1 Product Design

## Purpose

KMO is a minimal, keyboard-first Mattermost TUI for macOS. Its primary purpose is to receive new messages in real time, send messages, and follow conversations across channels, direct messages, and group messages.

The v1 experience is intentionally focused. It is not a full Mattermost replacement yet; it concentrates on a fast, polished text-chat workflow in the terminal.

## Core v1 scope

### Conversations

- Support one Mattermost server, one user account, and one team context.
- Show all accessible conversations in a fixed-width sidebar.
- Organize conversations into three sections:
  - Channels
  - DMs
  - Groups
- Everything must remain discoverable.
- Sort within each section in two stages:
  1. Conversations with unread activity first.
  2. Alphabetically within unread/read groups.
- Mentions and ordinary unread conversations have equal sort priority.
- Unread indicator: `•`.
- Mention indicator: `@`.
- Sidebar width: 30 characters.
- Long names are truncated and end with `.....`.

### Fuzzy finder

- `Cmd+F` enters a dedicated fuzzy-find mode.
- Search updates live while typing.
- Search matches both:
  - visible conversation name;
  - channel slug / username.
- Search results remain grouped into Channels, DMs, and Groups.
- `Enter` immediately opens the selected conversation.
- `Esc` exits fuzzy-find mode and returns to normal navigation.

### Chat view

- Mandatory two-column layout at all times.
- If the terminal is too small, show a clear minimum-size message rather than collapsing to one pane.
- The chat header contains only conversation type + name, for example:
  - `# development`
  - `@ alice`
  - `group project-x`
- No presence information in v1.
- No channel topic/header text in v1.

### Messages

- Text-only rendering in v1.
- Show sender name and timestamp compactly.
- Messages from today use `HH:MM`.
- Older messages also show a date.
- The user's own messages receive subtle styling using the primary accent color.
- System messages are shown, but are visually secondary and not selectable.
- URLs remain plain text.
- Reactions, attachments, rich media, code-specific rendering, and other rich content are deferred.

### Message navigation

- User messages are selectable.
- Navigate selectable messages with:
  - arrow keys;
  - `j` / `k`.
- System messages remain visible but are skipped by selection/navigation.
- While at the bottom of the chat, incoming messages auto-scroll into view.
- When the user scrolls upward, the current viewport is preserved.
- If messages arrive while scrolled up, show a subtle `↓ new messages` indicator.

### History loading

- Load as much available history as practical.
- Use pagination rather than blocking startup on the entire history.
- Automatically load older messages when the user scrolls upward.
- Continue until the available history is exhausted.
- Local message/conversation caching is deferred to v1.5.

### Threads

- Threads are part of v1.
- Threads are rendered inline in the main chat view.
- Thread roots receive a clear marker.
- Replies are visually indented and distinct from normal timeline messages.
- Threads are expanded by default.
- `Cmd+R` starts a thread reply to the selected user message.
- When replying, the compose area shows contextual text such as:
  `Replying to Alice: "..."`
- `Esc` cancels reply mode and returns to normal compose mode.

### Compose

- The compose box is always visible at the bottom of the chat pane.
- Enter inserts a newline.
- Default send shortcut: `Cmd+Enter`.
- Send shortcut is configurable.
- All keybindings are configurable in `config.toml`.

### Focus and keyboard interaction

- The three primary focus targets are:
  1. conversation sidebar;
  2. message list;
  3. compose box.
- `Tab` cycles focus forward.
- `Cmd+Tab` cycles focus backward.
- Vim-style navigation and standard arrow-key navigation are both supported.
- `Cmd+?` opens the shortcut overlay.
- `Cmd+Q` and `Cmd+W` are not intercepted; the terminal/macOS keeps control of them.
- No explicit in-app quit shortcut in v1.

### Status bar

The bottom status bar contains only compact status information plus a shortcut hint.

At minimum it shows:

- connection state, such as:
  - `connected`
  - `reconnecting`
  - `offline`
- `Cmd+? shortcuts`

Reconnect information remains subtle and stays in the status bar; no toast/banner is needed.

### Realtime behavior

- New messages arrive live via Mattermost WebSocket events.
- WebSocket disconnects do not remove the user from the current screen.
- The application automatically reconnects in the background.
- Connection changes are reflected in the status bar.

## Startup and setup

### Authentication

- Authentication uses a Mattermost Personal Access Token (PAT).
- The PAT is never stored in `config.toml`.
- The PAT is stored in macOS Keychain using the macOS `security` command.

### Setup flow

At startup:

1. Read `~/.config/kmo/config.toml`.
2. Read the PAT from macOS Keychain.
3. If configuration is complete, connect immediately.
4. If required configuration is missing, enter the TUI setup flow.
5. The setup flow asks for server URL and PAT.
6. Validate both against Mattermost before opening the main interface.
7. On success, store the PAT in Keychain and non-sensitive settings in the config file.
8. On failure, show the exact Mattermost/server error and remain in setup.

The app automatically selects the available team context for v1; there is no team picker.

If an existing saved configuration cannot connect, return to setup rather than leaving the user in a broken chat view.

## Visual design

- Product-like polish rather than a bare utility aesthetic.
- Primary accent color: `RGB(21, 66, 115)` / `#154273`.
- Respect the terminal's default foreground and background colors.
- Use the accent color selectively for:
  - focus state;
  - selected elements;
  - the user's own messages;
  - important UI accents.
- Use subtle borders and clear focus states.
- Preserve a compact terminal-first feel.

## Explicitly out of scope for v1

### v1.5 candidates

- macOS desktop notifications.
- Local message/conversation cache.
- Integration tests with a fake Mattermost backend.

### v2.5 candidates

- Edit messages.
- Delete messages.
- Search message history.

### Later candidates

- Reactions.
- Attachments.
- Rich media.
- Improved code-block rendering.
- URL opening from the TUI.
- Presence/status display.
- Multiple accounts/servers.
- Multiple-team navigation.

## v1 success criteria

A v1 build is successful when a user can:

1. configure a Mattermost server and PAT entirely from the terminal;
2. start KMO and connect automatically on subsequent launches;
3. see all channels, DMs, and group messages in a structured sidebar;
4. fuzzy-find and open any conversation;
5. receive new messages in real time;
6. navigate and read paginated message history;
7. send multiline text messages with an explicit send shortcut;
8. read inline threads and send thread replies;
9. see unread/mention state update in the sidebar;
10. survive WebSocket reconnects without losing the current UI context.
