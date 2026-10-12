# Phase 0 Audit — Workspace, Prototype and Brand Library

Status: Phase 0 deliverable · Audit date: 2026-10-12

This document records what existed before production development began, so later phases can trace every design decision back to evidence. It covers Task 0.1 (workspace), Task 0.2 (MateCollab-UI prototype) and summarises Task 0.3 (the full brand inventory lives in [../BRANDING-ASSETS.md](../BRANDING-ASSETS.md)).

---

## 1. Production workspace (`MateCollab/`)

| Item | Finding |
|---|---|
| Git | Repository initialised on branch `main`. **No commits, no remotes, no tags.** Local config only (`core.ignorecase=true`, `core.symlinks=false`, `core.filemode=false` — normal for Windows). |
| Tracked files | None. `MateCollab-Logo-Library/` is untracked. |
| Code | None. No `package.json`, `go.mod`, Docker files, CI, `.gitignore`, `.gitattributes` or `.editorconfig`. |
| Contents | Only `MateCollab-Logo-Library/` (320 files, 6.9 MB). Note the folder is **double-nested**: `MateCollab-Logo-Library/MateCollab-Logo-Library/…` (likely from unzipping). It is left untouched; see [BRANDING-ASSETS.md](../BRANDING-ASSETS.md#2-library-location). |

### Developer machine toolchain (observed)

| Tool | Version | Implication |
|---|---|---|
| OS | Windows 11 | Scripts must be cross-platform; line endings must be pinned (`.gitattributes`). |
| Node.js | 24.18.1 | Meets Next.js 16 requirements. |
| pnpm | 11.18.0 | Use pnpm workspaces for the JS side. |
| Go | 1.25.14 | **Out of security support since Go 1.27 shipped (August 2026).** Target Go 1.27; upgrade the machine before Phase 1. |
| Docker / Compose | 29.7.2 / v5.5.1 | Docker Desktop available for local infrastructure. |
| Python | 3.13 | Not part of the product stack. |
| `make`, `just`, `task`, `psql` | **Not installed** | Root task runner must not depend on `make`; we use `pnpm` scripts (see [LOCAL-DEVELOPMENT.md](../development/LOCAL-DEVELOPMENT.md)). Database access goes through containers or `mcctl`. |

---

## 2. MateCollab-UI prototype (`Desktop/MateCollab-UI`, `github.com/rizwansammo/MateCollab-UI`)

### 2.1 What it is

* A single commit (`a6aff79 "MateCollab Prototype"`), clean working tree.
* A **frontend-only, single-user simulation**. All data is fixture data (`lib/matecollab/data.ts`) mutated in React state and persisted to `localStorage` under `matecollab-*` keys.
* Generated on an **OpenAI Sites / Codex scaffold**: `vinext` (Vite re-implementation of Next.js), Cloudflare Workers (`wrangler`, D1 and R2 bindings), a ChatGPT header-based auth stub (`app/chatgpt-auth.ts`), "connector preview" tooling, and a vendored `@openai/sites-vite-plugin`. `vite.config.ts` imports `.openai/hosting.json`, which is not in the repository.
* Distributed as a standalone file, `MateCollab-Prototype.html` (900 KB), built by `vite.prototype.config.ts` + `scripts/package-prototype.mjs`, which **inlines the HemiHead font as base64**.

### 2.2 Stack observed

| Concern | Prototype choice | Production stance |
|---|---|---|
| UI runtime | React 19.2, TypeScript 5.9 (strict) | Keep |
| Framework | Next.js 16.3 API via `vinext` on Cloudflare | Replace with real Next.js 16 (Node runtime) — see [FRONTEND.md](FRONTEND.md) |
| Styling | Tailwind CSS 4.2 + one 73 KB hand-written `globals.css` | Keep Tailwind 4; move values into design tokens |
| Primitives | shadcn/ui "new-york" (61 vendored files in `components/ui`) on Radix + Base UI | Keep the pattern; own the primitives in `packages/ui` |
| Icons | lucide-react | Keep |
| Toasts / command palette | sonner / cmdk | Keep |
| Forms | react-hook-form + zod (installed, unused by MateCollab screens) | Use for real forms |
| State | One React context (`store.tsx`) holding every entity + `useStored` → `localStorage` | Replace with TanStack Query (server state) + small UI stores |
| Routing | Hash routes (`#/spaces/eng-general`) | Replace with App Router paths scoped by organisation |
| Data | Fixtures; Drizzle/D1 schema is empty | Discard |

### 2.3 Source map

| Module (≈226 KB, dense single-line TSX) | Responsibility | Production destination |
|---|---|---|
| `components/matecollab/app.tsx` | Shell: rail, resizable sidebar, global bar, right context panel, login, brand board, prototype-states page | `apps/web` AppShell + feature routes; brand board and states page → Storybook |
| `components/matecollab/store.tsx` | All state, persistence, routing, call lifecycle, demo role switch | Discard; replaced by API + query cache + realtime |
| `components/matecollab/ui.tsx` | Logo (prototype mark), Avatar, Btn, IconBtn, Badge, Field, Pick, Toggle, TabBar, Menu, Modal, Blank | `packages/ui` primitives (logo replaced by Library assets) |
| `components/matecollab/messaging.tsx` | Conversation view, message row + toolbar, composer, file tile, regex markup renderer | `features/messaging` |
| `components/matecollab/pages.tsx` | Home, Meetings list, Files, Actions, Support, Activity, Spaces, Saved | Feature routes |
| `components/matecollab/overlays.tsx` | Search modal, file viewer, meeting detail, support form, action forms, schedule, new chat, create space/channel, invite, rename, notification prefs, password, device test, image pickers, help | Feature dialogs |
| `components/matecollab/settings.tsx` | Personal settings (7 sections) and organisation administration (12 sections) incl. SMTP/HelpDesk forms | `features/settings`, `features/admin` |
| `components/matecollab/meeting.tsx` | Pre-call, live stage (grid/speaker/share), participants/chat panels, floating minimised call | `features/meetings` on LiveKit |
| `components/matecollab/webmcp.ts` | Feature-detected WebMCP tools (navigate, open support form) | Out of V1 scope |
| `lib/matecollab/data.ts` | Typed fixtures | Informs the data model only |

### 2.4 Screen and flow inventory

| Area | Prototype coverage | Notes for production |
|---|---|---|
| Home | Greeting, "Pick up where you left off", "Your attention, here", recent files, calendar card with Join, action items, support nudge | Aggregation of real data; completed in Phase 9–11 |
| Chats | DM and group list with favorites/unread/archived filters, presence avatars, new conversation (DM or named group) | Phase 4–5 |
| Spaces | Browse/join, create (Team/Project/Private), space tree with channels, overview tabs (Overview, Channels, Files, Meetings, Members), archive | Phase 3 |
| Channels | Messages/Files/Pinned tabs, topic, huddle button, details panel, notification prefs, favorite, archive | Phase 3–5 |
| Messages | Hover toolbar (react, thread, save, more), quote reply, pin, copy link, edit, delete (confirm), create action, create support request, reactions with counts, "Seen" receipt, "New messages" divider, link preview card, attachments | Phase 4–6 |
| Threads | Right panel (380 px), replies, "Notify me about replies" toggle, thread composer | Phase 5 |
| Files | Recent / Shared with me / Space files, list and grid views, upload (25 MB limit), preview modal, rename, download, remove | Phase 6 |
| Meetings | Upcoming/Today/Past, schedule form (title, agenda, date, time, duration, timezone, space, channel, invitees, guest access, waiting room), detail and recap (notes, actions, chat; recording/AI summary marked future) | Phase 7–8 |
| Calls | Pre-join device pickers, grid/speaker views, screen share sample, participants and chat panels, hand raise, reactions, minimise to floating card, connection-issue banner, end for everyone | Phase 7–8 |
| Huddles | Channel huddle banner, minimised floating huddle | Phase 8 |
| Activity | Filters (All, Mentions, Replies, Meetings, Actions, Support, Spaces), unread-only, mark all read | Phase 9 |
| Search | ⌘/Ctrl+K palette, type filters (All, Messages, People, Spaces, Channels, Files, Meetings), recent searches, jump-to | Phase 9 |
| Actions | Assigned to me / Created by me / Completed, create from message, detail with source link | Phase 10 |
| Support | My requests (All/Open/Pending/Resolved), create with **context selector (Selected message only / Selected thread / No conversation context) and live preview**, "Include selected attachment" toggle, ticket detail with shared context, replies | Phase 11 |
| Personal settings | Profile, Appearance (Light/Dark/System, density), Notifications (incl. quiet hours), Audio & Video, Privacy (presence, read receipts, guest DMs), Security (2FA, password, sessions), Accessibility (reduced motion, captions concept, always-show message actions, text size 16/18/20) | Across phases |
| Org administration | General, Members (role change, suspend, deactivate, remove), Roles & Permissions matrix, Spaces, Messaging policy, Meetings policy, Files policy and storage, Integrations (Overview/SMTP/HelpDesk), Security policy and sessions, Notifications, Audit Logs (export CSV), Billing & Plan (future placeholder) | Across phases |
| Auth | Sign in → workspace selection; password reset preview | Phase 2 (real auth + TOTP) |
| States | 15 documented states: empty chat, no Spaces, no meetings, no files, no search results, loading messages, uploading file, upload failure, disconnected, call connection issue, permission denied, integration error, expired invitation, guest-restricted content, offline | Rebuilt as Storybook stories and real conditions |

### 2.5 Design language extracted (from `app/globals.css` and rendered screenshots)

Screens were rendered headlessly in light, dark, tablet (1024 px) and mobile (390 px) widths to confirm the visual language rather than infer it from CSS alone.

| Token / rule | Prototype value |
|---|---|
| Layout | Rail **70 px** (navy), sidebar **274 px** resizable **240–320 px** (keyboard ±10 px), global bar **56 px**, right panel **380 px** |
| Breakpoints | ≥1650 wide page padding · ≥1400 right panel pushes content · ≤1320 compact desktop · ≤1100 sidebar becomes a drawer · ≤760 mobile with **62 px bottom navigation** |
| Radii | Inputs 6 px, cards 6 px, menus 6 px, modals 8 px, badges 4 px, **buttons 5 px**, **tooltips 6 px** (shadcn default) |
| Type | `Inter, 'Segoe UI', Arial, sans-serif`, base 14 px (`.875rem`), headings weight 650, h1 28 px, eyebrow 11 px tracking 1.7 px |
| Light colours | background `#FFFFFF`, foreground `#253248`, muted `#F3F5F8`, muted-foreground `#778294`, border `#E5E9EF`, sidebar `#F7F8FA`, rail `#142236`, accent `#EAF0FD`, primary `#2463EB` |
| Dark colours | background `#171E29`, card `#1D2633`, foreground `#DDE4EF`, muted-foreground `#95A2B5`, border `#303A49`, sidebar `#1B2330`, rail `#101722`, primary `#6A99FF` |
| Presence | available `#2E9775`, busy `#CF615C`, away `#C89639`, DND `#CC5656`, offline `#9CA7B6` |
| Elevation | `0 12px 40px #15223517` (menus/modals), subtle toolbar shadow |
| Motion | 140 ms colour transitions; `prefers-reduced-motion` respected |

### 2.6 Production vs prototype-only classification

**Keep (production intent)**
* Information architecture: six rail modules (Home, Chats, Spaces, Meetings, Files, Activity) with Actions, Saved and Support reached from Home and context menus.
* Three-panel adaptive shell and all dimensions above; drawer and bottom-navigation behaviour.
* Message interaction model (hover/focus toolbar, thread panel, quote, pin, save, react, create action, create support request).
* Support request UX: explicit context selector with a locked preview stating exactly what will be shared.
* Settings and administration information architecture, including write-only secret fields ("Saved · enter to replace").
* Restrained radii, subtle borders, disciplined spacing, light/dark parity, keyboard shortcuts (⌘/Ctrl+K, Esc, Enter, Shift+Enter), accessible labels on icon buttons, confirmation for destructive actions, toast acknowledgements, the 15 empty/loading/error states.

**Adapt**
* Colours → production tokens using **Logo Library Violet/Navy** (prototype blue conflicts with the Library; the Library wins).
* Radii normalised to the brief (buttons 6 px, tooltips 4 px).
* Typography → self-hosted Inter (the prototype declares Inter but never loads it, so macOS renders Arial).
* Copy that references the simulation ("Prototype", "Demo", "Simulated") removed.

**Prototype-only — do not carry into production**
* `store.tsx` global state, `useStored`/`localStorage` persistence of business data, hash routing.
* Simulated auth (`Login` accepts any password), the role-preview switcher, client-side guest filtering (explicitly *not* a security boundary).
* Timer-based SMTP/HelpDesk "tests" and the "Simulate connection failure" / "Simulate ticket status" controls.
* Simulated media (no `getUserMedia`), sample shared screen, fake participants and captions.
* Fixed clock (7 Oct 2026) and hard-coded `Asia/Dhaka`.
* OpenAI Sites/Cloudflare scaffold: `vinext`, `wrangler`, D1/R2 bindings, Drizzle starter, `chatgpt-auth.ts`, connector preview tooling, `build/`, `scripts/`, `examples/`, `.openai` references.
* `/brand` and `/prototype` routes (become design-system documentation).
* The prototype logo SVGs in `public/brand/`, `public/favicon.svg`, and `public/fonts/HemiHead-Bold.otf`.

### 2.7 Defects and risks found in the prototype (to correct, not copy)

| # | Finding | Production correction |
|---|---|---|
| 1 | Prototype mark (two forward-leaning forms, blue `#2463EB`, navy `#152235`) differs from the approved Library mark (four sharp L corners + centred square, Violet `#7047D1`, Navy `#101A2D`) | Use Library assets only ([BRANDING-ASSETS.md](../BRANDING-ASSETS.md)) |
| 2 | HemiHead (Typodermic Fonts Inc., `fsType=4` restricted embedding) is served via `@font-face` and base64-embedded in the standalone HTML | Never ship the font; the Library lockups have outlined lettering |
| 3 | 253 distinct hard-coded hex colours (294 uses), `#2463EB` repeated 12× outside tokens | Semantic tokens only; lint against raw colours |
| 4 | Muted text `#778294` on white is **3.89:1** (fails WCAG AA for small text) and is used for most secondary text | Darken muted foreground (validated values in [FRONTEND.md](FRONTEND.md#colour-tokens)) |
| 5 | 10 px text is used widely (timestamps, rail labels, counters) and mobile meeting/sharing views go down to 6–9 px | Minimum 11 px rendered text |
| 6 | Input borders (`#E5E9EF`) are 1.22:1 against white | Decide in Phase 1 whether to meet 3:1 for form-field boundaries |
| 7 | Markup rendered by a regex that matches only `@Rizwan Sammo`; mentions stored as display text | Mentions stored as ID tokens (`<@user-id>`), rendered client-side |
| 8 | "Seen" receipt shown on every own message; unread divider hard-coded at index 3 | Derive from real read cursors |
| 9 | Quote reply copies the quoted text into the new message | Store a reference to the quoted message |
| 10 | Message deletion removes replies; no edit window | Tombstones; org policy for edit/delete windows |
| 11 | Uploads kept as `blob:` object URLs | Presigned uploads to object storage with scanning |
| 12 | Single context store re-renders the whole tree on any change | Query cache + selective subscriptions |

### 2.8 Data model hints from the fixtures

The fixture types (`Person`, `Message`, `Conversation`, `Space`, `FileRecord`, `Meeting`, `Action`, `Ticket`, activity items) confirm the production entity set and several modelling choices:

* DMs, group chats and channels share one shape (`Conversation.kind = dm | group | channel`) → production uses one `conversations` table.
* Threads are replies nested under a root message → production stores replies as messages with `thread_root_id`.
* Activity items reference a source (conversation, meeting, ticket) → production derives Activity from the `notifications` table.
* Support tickets store a `context` string and a `source` → production stores an immutable snapshot of explicitly shared items plus a source reference.
* Guests carry a `spaces` allow-list → production uses explicit space grants.

Details: [DATABASE.md](DATABASE.md).

---

## 3. MateCollab-Logo-Library (summary)

* 320 files: 246 PNG, 46 JPG, 22 SVG, 4 ICO, 1 web manifest, 1 README.
* 15 icon variants, 6 horizontal lockups, a favicon set and a brand preview.
* Brand colours: **Violet `#7047D1`**, **Navy `#101A2D`**, **White `#FFFFFF`**.
* **No font files.** Lockup lettering is outlined HemiHead Bold, so no font is required.
* **No wordmark-only asset** (wordmark exists only inside lockups) and **no maskable PWA icons**.

Full inventory and usage rules: [../BRANDING-ASSETS.md](../BRANDING-ASSETS.md).
