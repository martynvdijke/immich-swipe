# Immich Swipe

Swipe-review your Immich library: right = keep, left = trash. Like a dating app, but for photos (and videos).

![Vue 3](https://img.shields.io/badge/Vue-3.x-4FC08D?logo=vue.js)
![TypeScript](https://img.shields.io/badge/TypeScript-5.x-3178C6?logo=typescript)
![Tailwind CSS](https://img.shields.io/badge/Tailwind-4.x-06B6D4?logo=tailwindcss)

<p align="center">
  <img src="docs/screenshots/home.png" width="960" alt="Immich Swipe home screen (sanitized demo)" />
</p>

<p align="center">
  <img src="docs/screenshots/mobile.png" width="260" alt="Immich Swipe mobile view (sanitized demo)" />
</p>

<p align="center">
  <img src="docs/screenshots/album-picker.png" width="960" alt="Album picker + hotkey mapping (sanitized demo)" />
</p>

> Screenshots are sanitized (no real photos or API keys).

## Features

- Swipe (touch/mouse) or use keyboard/buttons
- Random or chronological review (oldest/newest first)
- Skip videos toggle
- Favorite toggle (press `F`)
- Add-to-album (+ configurable `0–9` hotkeys)
- Undo (Ctrl/⌘+Z or ↑)
- Reviewed cache + stats persisted per server/user
- Preloads the next asset

## Controls

| Action | Gesture / Key | Button |
|---|---|---|
| Keep | Swipe right / `→` | ✓ |
| Delete (moves to trash) | Swipe left / `←` | ✕ |
| Undo | `Ctrl/⌘+Z` or `↑` | ↶ |
| Favorite | `F` | ♡ |
| Add to album | `0–9` (configured) | Album icon |

## Quickstart

### Local development

```bash
npm install
npm run dev
```

Open `http://localhost:5173`.

### Docker (recommended)

```bash
cp env.example .env
# edit .env (set your Immich server URL and API key)
docker compose up --build
```

Open `http://localhost:2293`.

All configuration is read at **runtime** by the Go backend — no rebuild needed for `.env` changes. Just restart the container.

### GitHub Pages / SPA-only mode

The app can also run as a pure SPA (no Go backend) behind an Nginx reverse proxy. This repo includes a GitHub Actions workflow (`deploy-gh-pages.yml`) that deploys to GitHub Pages on every push to `main`.

In SPA-only mode, API keys are stored in `localStorage` and the browser calls Immich directly.

## Configuration

### Option A: `.env` with Go backend (Docker)

The Go backend reads runtime environment variables (no rebuild needed):

```bash
IMMICH_SERVER_URL=https://immich.example.com
IMMICH_API_KEY_1_NAME=Alice
IMMICH_API_KEY_1_KEY=your-api-key-here
```

Behavior:
- 1 user configured: auto-login
- >1 users configured: user selection screen (`/select-user`)
- no API keys configured: manual login (`/login`), enter server URL + API key in the UI

### Option B: manual login (SPA-only or fallback)

If no API keys are configured in the environment, the app falls back to manual login:
- Enter your Immich Server URL and API key in the login screen
- These are kept in `sessionStorage` (cleared on tab close) — or `localStorage` in SPA-only mode

## Architecture

The app uses a **Go backend** that serves static files and proxies all Immich API requests:

```
Browser → Go backend (port 8080) → Immich server
         ↕
    sessionStorage (session token)
```

- Immich API keys stay **server-side** — never in the browser
- No CORS configuration needed
- Session tokens with 24h sliding expiry

The frontend (Vue 3 SPA) authenticates via the backend and all API calls go through the reverse proxy.

## Stored data (localStorage / sessionStorage)

- `immich-swipe-session` (sessionStorage — session token from Go backend)
- `immich-swipe-theme`
- `immich-swipe-skip-videos`
- `immich-swipe-stats:<server>:<user>` (keep/delete counters)
- `immich-swipe-reviewed:<server>:<user>` (already reviewed IDs + decision)
- `immich-swipe-preferences:<server>:<user>` (order mode + album hotkeys)

## Immich API key permissions

Minimum:
- `asset.read`
- `asset.delete`

If you want albums and favorites, grant the corresponding read/update permissions as well.

## Development scripts

- `npm run dev` (Vite, `5173`, `--host`)
- `npm run build`
- `npm run preview`
- `npm run type-check`
- `npm test`
