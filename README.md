# Knitnox

> A suite of **offline-first Progressive Web Apps** built with **Svelte 5**, **SvelteKit**, and a custom **Neumorphic design system**. Deployed automatically to GitHub Pages via GitHub Actions.

No cloud. No accounts. No tracking. Just fast, installable tools that work entirely in your browser.

---

## Table of Contents

- [Architecture](#architecture)
- [Project Structure](#project-structure)
- [Apps](#apps)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Adding a New App](#adding-a-new-app)
- [Deployment](#deployment)
- [Design System](#design-system)
- [Tech Stack](#tech-stack)

---

## Architecture

Knitnox is a **static monorepo** — multiple independent SvelteKit apps that build into a single `dist/` folder, served from one GitHub Pages site.

```
knitnox/ (monorepo root)
│
├── launcher/              ← SvelteKit app   → builds to dist/
│   └── src/routes/        ← /  /apps  /app/[id]
│
├── apps/calculator/       ← SvelteKit app   → builds to dist/apps/calculator/
├── apps/color-calculator/ ← SvelteKit app   → builds to dist/apps/color-calculator/
├── apps/temp-converter/   ← SvelteKit app   → builds to dist/apps/temp-converter/
│
└── dist/                  ← merged build output → GitHub Pages
    ├── index.html          ← launcher
    ├── 404.html            ← SPA fallback for all launcher routes
    ├── sw.js               ← launcher service worker (PWA)
    └── apps/
        ├── calculator/
        ├── color-calculator/
        └── temp-converter/
```

### How the pieces connect

**Launcher** is the app store at `/`. It reads `/apps.json` (a static JSON file committed in `launcher/public/`) to render the app grid. When a user clicks an app card, the launcher navigates to that app's URL (e.g. `/apps/calculator/`), which is a completely separate SvelteKit app served from a sub-directory.

**Sub-apps** are fully independent. They have their own `package.json`, `svelte.config.js`, service worker, and web manifest. They know their base path (`/apps/<name>/`) via `paths.base` in `svelte.config.js`, which SvelteKit uses to prefix all asset and navigation URLs automatically.

**Build merging** works because each app's `adapter-static` is configured to output into a different sub-directory of the shared `dist/` folder:

| App | `adapter-static` output |
|---|---|
| Launcher | `dist/` |
| Calculator | `dist/apps/calculator/` |
| Color Calculator | `dist/apps/color-calculator/` |
| Temp Converter | `dist/apps/temp-converter/` |

**Dev server** — the launcher's `vite.config.js` includes a custom `routeAppsPlugin`. In dev mode it reads pre-built sub-apps from `dist/` using `fs.readFileSync` and serves them at `/apps/*`, so you can test the full linked flow without running all four dev servers simultaneously.

**PWA** — every app (including the launcher) registers its own service worker via `vite-plugin-pwa`. The launcher's service worker excludes `/apps/*` routes via `navigateFallbackDenylist` so it never intercepts requests meant for sub-app service workers.

**Install detection** — when a sub-app launches in `standalone` display mode (i.e. it was installed to the home screen), its `+layout.svelte` writes a flag to `localStorage` (`knitnox-app-<id>`). The launcher reads this key on the app detail page to show a green **✓ Installed** badge.

**Launcher splash screen** — the launcher's `app.html` contains a pure-CSS splash (`#splash`) with the animated Knitnox wordmark. It is painted by the browser immediately — before any JavaScript downloads — then slides off-screen via a CSS `animation` after 1.2 s. No JS is involved; the `animation-fill-mode: forwards` keeps it hidden permanently once dismissed.

---

## Project Structure

```
knitnox/
├── .github/
│   └── workflows/
│       └── deploy.yml           ← CI/CD: install → build → deploy on push to main
│
├── launcher/                    ← App Store SPA
│   ├── svelte.config.js         ← adapter-static → ../dist, fallback: 404.html
│   ├── vite.config.js           ← sveltekit() + VitePWA + routeAppsPlugin
│   ├── package.json
│   ├── public/
│   │   ├── apps.json            ← App registry (add entries here to add apps)
│   │   ├── icons/               ← App icons (.svg) referenced by apps.json
│   │   ├── screenshots/         ← App screenshots referenced by apps.json
│   │   ├── CNAME                ← Custom domain
│   │   └── 404.html             ← GitHub Pages SPA fallback
│   └── src/
│       ├── app.html             ← SvelteKit HTML shell (meta tags, fonts, splash)
│       ├── routes/
│       │   ├── +layout.js       ← ssr=false, prerender=false (SPA mode)
│       │   ├── +layout.svelte   ← imports CSS, wraps in .neumorph-body
│       │   ├── +page.svelte     ← Home / landing page
│       │   ├── apps/
│       │   │   ├── +page.js     ← loads apps.json via fetch
│       │   │   └── +page.svelte ← App grid with search + infinite scroll
│       │   └── app/
│       │       ├── +page.js     ← backwards-compat: /app?id=x → /app/x
│       │       └── [id]/
│       │           ├── +page.js     ← loads app by id from apps.json
│       │           └── +page.svelte ← App detail: description, screenshots, install
│       └── lib/
│           ├── neumorphic.css   ← Shared design tokens + components
│           ├── styles.css       ← Launcher-specific styles
│           └── components/
│               └── AppCard.svelte
│
├── apps/
│   ├── calculator/
│   │   ├── svelte.config.js     ← paths.base: /apps/calculator/, output → dist/apps/calculator/
│   │   ├── vite.config.js       ← sveltekit() + VitePWA
│   │   ├── package.json
│   │   └── src/
│   │       ├── app.html         ← HTML shell with splash screen
│   │       ├── routes/
│   │       │   ├── +layout.js   ← ssr=false, prerender=false
│   │       │   ├── +layout.svelte ← imports neumorphic.css, handles splash + install flag
│   │       │   └── +page.svelte ← full calculator UI + keyboard handler + PWA prompt
│   │       ├── components/
│   │       │   ├── Display.svelte
│   │       │   ├── ButtonGrid.svelte
│   │       │   └── CalcButton.svelte
│   │       └── lib/
│   │           └── neumorphic.css
│   │
│   ├── color-calculator/        ← same structure as calculator
│   │   └── src/
│   │       ├── components/
│   │       │   ├── ColorPreview.svelte
│   │       │   ├── HexInput.svelte
│   │       │   ├── RgbInputs.svelte
│   │       │   └── HslInputs.svelte
│   │       └── routes/
│   │           └── +page.svelte ← real-time HEX ↔ RGB ↔ HSL conversion
│   │
│   └── temp-converter/          ← same structure as calculator
│       └── src/
│           └── routes/
│               └── +page.svelte ← real-time °C ↔ °F ↔ K conversion
│
├── dist/                        ← build output (gitignored — never edit directly)
├── package.json                 ← root scripts: build:all, install:all, dev:*
└── .gitignore
```

---

## Apps

| App | Path | Description |
|---|---|---|
| **Launcher** | `/` | App store — browse, search, open, and install apps |
| **Calculator** | `/apps/calculator/` | Neumorphic calculator with full keyboard support |
| **Color Calculator** | `/apps/color-calculator/` | Real-time HEX ↔ RGB ↔ HSL converter |
| **Temp Converter** | `/apps/temp-converter/` | Real-time °C ↔ °F ↔ K converter |

---

## Getting Started

### Prerequisites

You need **Node.js v18+** and **npm v9+**. Download the LTS version from [nodejs.org](https://nodejs.org/).

```bash
node --version   # v18 or higher
npm --version    # v9 or higher
```

### 1. Clone the repo

```bash
git clone https://github.com/knitnox/knitnox.git
cd knitnox
```

### 2. Install all dependencies

```bash
npm run install:all
```

This installs `node_modules` for the launcher and every sub-app.

### 3. Build everything

```bash
npm run build
```

All four SvelteKit apps build in sequence and their outputs merge into `dist/`.

### 4. Preview the complete site locally

```bash
cd launcher
npm run preview
```

Open `http://localhost:4173`. You'll see the full app store with all apps working and linkable.

---

## Development Workflow

### Launcher (app store)

```bash
cd launcher
npm run dev
# → http://localhost:5173
```

The launcher dev server serves itself via SvelteKit's dev mode. Clicking an app card navigates to `/apps/<name>/` — the `routeAppsPlugin` reads the pre-built sub-apps from `dist/apps/` and serves them directly. **You must build sub-apps at least once before they'll appear in launcher dev mode:**

```bash
# From the repo root, one-time setup:
npm run build:calculator
npm run build:color-calculator
npm run build:temp-converter
```

After that, run `npm run dev --prefix launcher` and all links work.

### Individual sub-apps

Each sub-app runs on its own dev server with full hot-reload:

```bash
# From the repo root:
npm run dev:calculator         # → http://localhost:5173/apps/calculator/
npm run dev:color-calculator   # → http://localhost:5173/apps/color-calculator/
npm run dev:temp-converter     # → http://localhost:5173/apps/temp-converter/

# Or from inside the app folder:
cd apps/calculator
npm run dev
```

### Building individual apps

```bash
npm run build:launcher
npm run build:calculator
npm run build:color-calculator
npm run build:temp-converter
```

---

## Adding a New App

Here is the complete, step-by-step process. Use `apps/calculator` as the reference template.

### Step 1 — Copy the template

```powershell
# PowerShell
Copy-Item apps/calculator apps/my-app -Recurse

# macOS / Linux
cp -r apps/calculator apps/my-app
```

### Step 2 — Update `svelte.config.js`

Open `apps/my-app/svelte.config.js` and change the base path and output directory:

```js
import adapter from '@sveltejs/adapter-static';

export default {
  kit: {
    adapter: adapter({
      pages: '../../dist/apps/my-app',
      assets: '../../dist/apps/my-app',
      fallback: 'index.html',
      precompress: false,
      strict: false,
    }),
    paths: {
      base: '/apps/my-app',   // ← must match the URL path exactly
    },
  },
};
```

### Step 3 — Update `vite.config.js`

Open `apps/my-app/vite.config.js` and update the PWA manifest fields:

```js
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import { VitePWA } from 'vite-plugin-pwa';

export default defineConfig({
  plugins: [
    sveltekit(),
    VitePWA({
      registerType: 'autoUpdate',
      manifest: {
        name: 'Knitnox My App',
        short_name: 'My App',
        description: 'What your app does.',
        start_url: '/apps/my-app/',
        scope: '/apps/my-app/',
        display: 'standalone',
        background_color: '#e6e9ef',
        theme_color: '#e6e9ef',
        orientation: 'portrait',
        icons: [
          {
            src: 'icon.svg',
            sizes: 'any',
            type: 'image/svg+xml',
            purpose: 'any maskable',
          },
        ],
      },
      workbox: {
        globPatterns: ['**/*.{js,css,html,svg,ico,woff,woff2}'],
        navigateFallback: 'index.html',
      },
    }),
  ],
});
```

### Step 4 — Update `src/app.html`

Open `apps/my-app/src/app.html` and update the `<title>`, meta tags, and splash screen name:

```html
<title>My App — Knitnox</title>
<meta name="description" content="What your app does." />
<meta property="og:title" content="My App — Knitnox" />
<!-- ... other meta tags ... -->

<!-- splash screen name -->
<p class="splash-name">My App</p>
```

The `%sveltekit.assets%` placeholder in the splash icon `src` is automatically replaced by SvelteKit with the correct base-path-aware asset URL — **do not change it**:

```html
<img src="%sveltekit.assets%/icon.svg" alt="" class="splash-icon" />
```

### Step 5 — Write the app UI

Edit `apps/my-app/src/routes/+page.svelte`. This file is the entire app — import components from `../components/`, use Svelte 5 runes, and keep the "Back to Apps" footer link pointing to `/` (absolute URL, not base-path-relative):

```svelte
<script>
  // Svelte 5 runes — reactive by default
  let value = $state('');
  let result = $derived(value.toUpperCase());

  $effect(() => {
    localStorage.setItem('knitnox-my-app', value);
  });
</script>

<div class="page">
  <div class="card">
    <h1>My App</h1>
    <input bind:value />
    <p>{result}</p>
  </div>
  <div class="footer">
    <a href="/">← Back to Apps</a>   <!-- always use absolute "/" -->
  </div>
</div>
```

The `+layout.svelte` (already in the template) imports `neumorphic.css`, handles the splash screen fade-out, and writes the install detection flag to `localStorage` — **you don't need to touch it**.

### Step 6 — Add an icon to the launcher

Place your app's icon SVG in:

```
launcher/public/icons/my-app.svg
```

Optionally add screenshots:

```
launcher/public/screenshots/my-app/
  screenshot1.png
  screenshot2.png
```

The icon is also required inside the sub-app's `public/` folder for the PWA manifest (the service worker caches it):

```
apps/my-app/public/
  icon.svg        ← required by vite-plugin-pwa
  favicon.svg     ← optional, falls back to site-wide /favicon.svg
```

### Step 7 — Register in `apps.json`

Open `launcher/public/apps.json` and add an entry. The `fullDescription` field supports **Markdown** — use it to write real documentation for your app:

```json
{
  "id": "my-app",
  "name": "My App",
  "category": "Utilities",
  "icon": "/icons/my-app.svg",
  "shortDescription": "One line shown on the app card.",
  "fullDescription": "## Overview\n\nDescribe your app here. Markdown is supported.\n\n## Features\n\n- Feature one\n- Feature two\n\n## Privacy\n\nNo data leaves your device.",
  "folder": "apps/my-app",
  "screenshots": ["/screenshots/my-app/screenshot1.png"],
  "manifestUrl": "/apps/my-app/manifest.webmanifest",
  "url": "/apps/my-app/"
}
```

### Step 8 — Add scripts to the root `package.json`

Open the root `package.json` and add your app to the build and install scripts:

```json
{
  "scripts": {
    "build": "npm run build:launcher && npm run build:calculator && npm run build:color-calculator && npm run build:temp-converter && npm run build:my-app",
    "build:my-app": "npm run build --prefix apps/my-app",
    "install:all": "npm install --prefix launcher && npm install --prefix apps/calculator && npm install --prefix apps/color-calculator && npm install --prefix apps/temp-converter && npm install --prefix apps/my-app",
    "dev:my-app": "npm run dev --prefix apps/my-app"
  }
}
```

> **Note:** the template `apps/calculator/package.json` already contains the required `overrides` block that pins `@sveltejs/vite-plugin-svelte` to a Vite-7-compatible version. Keep it when copying the template — removing it will cause peer-dependency conflicts on install.

### Step 9 — Add CI steps to `deploy.yml`

Open `.github/workflows/deploy.yml` and add an install step for your new app. Without this the CI build will fail since `node_modules` is never committed:

```yaml
      - name: Install my-app dependencies
        run: npm install --no-package-lock
        working-directory: apps/my-app
```

Add it after the existing install steps, before the "Build all projects" step.

> **Note:** The workflow uses `npm install --no-package-lock` so that npm ignores the Windows-generated lock file and re-resolves all dependencies (including platform-specific native binaries like the Rollup Linux module) fresh on the CI runner.

### Step 10 — Install, build, and verify

```bash
# From the repo root:
npm install --no-package-lock --prefix apps/my-app
npm run build
cd launcher && npm run preview
```

Open `http://localhost:4173` — your app should appear in the launcher grid. Click it, open the detail page, and verify the "Open App" button navigates correctly.

---

## Deployment

The site deploys **automatically** to GitHub Pages on every push to `main`.

### How it works

```
git push origin main
        │
        ▼
GitHub Actions (.github/workflows/deploy.yml)
        ├── Checkout repo
        ├── Setup Node 20
        ├── npm install --no-package-lock   (launcher)
        ├── npm install --no-package-lock   (apps/calculator)
        ├── npm install --no-package-lock   (apps/color-calculator)
        ├── npm install --no-package-lock   (apps/temp-converter)
        ├── npm run build   (builds all 4 apps → dist/)
        ├── Upload dist/ as Pages artifact
        └── Deploy artifact → knitnox.github.io
```

> **Why `--no-package-lock`?** The `package-lock.json` files are generated on Windows and only record platform-specific native binaries for Windows (e.g. `@rollup/rollup-win32-x64-msvc`). Even plain `npm install` consults the lock file for optional dep resolution and skips the Linux binary. `--no-package-lock` forces npm to ignore the lock file entirely and re-resolve all deps for the current platform, installing `@rollup/rollup-linux-x64-gnu` correctly.

### First-time GitHub Pages setup

1. Go to your repo → **Settings → Pages**
2. Under **Source**, select **"GitHub Actions"**
3. Push anything to `main` — the workflow runs automatically

### Manual build for other hosts

```bash
npm run build
# Upload the dist/ folder to any static host:
# Netlify, Vercel, Cloudflare Pages, S3, etc.
```

No server-side rendering, no server required — `dist/` is 100% static HTML/CSS/JS.

### Custom domain

The file `launcher/public/CNAME` contains the custom domain. Edit it if you're deploying to your own domain. GitHub Pages reads this file automatically from the deployed artifact.

---

## Design System

All apps share `neumorphic.css` — a soft UI library using CSS custom properties. The neumorphic effect creates depth using two shadows: one dark (indent) and one light (highlight).

### CSS variables

```css
:root {
  --bg:     #e6e9ef;  /* page background — also the widget color */
  --light:  #ffffff;  /* highlight shadow */
  --dark:   #c2c8d0;  /* depth shadow */
  --text:   #2a2f3a;
  --accent: #5a8dee;
}
```

### Shadow patterns

| Class / usage | Effect | Box-shadow value |
|---|---|---|
| Card / raised button | Raised | `6px 6px 12px var(--dark), -6px -6px 12px var(--light)` |
| Active / pressed button | Pressed (inset) | `inset 4px 4px 8px var(--dark), inset -4px -4px 8px var(--light)` |
| Hero card | Large raised | `10px 10px 20px var(--dark), -10px -10px 20px var(--light)` |
| Text input | Recessed | `inset 4px 4px 8px var(--dark), inset -4px -4px 8px var(--light)` |

### Utility classes

```html
<div class="neumorph-card">…</div>
<button class="neumorph-btn">Default</button>
<button class="neumorph-btn neumorph-btn-accent">Accent</button>
<input class="neumorph-input" />
<div class="neumorph-container">…</div>    <!-- centered max-width wrapper -->
<div class="neumorph-grid-2">…</div>       <!-- 2-column grid -->
<div class="neumorph-grid-responsive">…</div> <!-- 2-col → 4-col at 1024px -->
```

### PWA install FAB

`neumorphic.css` includes a `.pwa-install-fab` class for the floating "Install App" button. It's already wired up in every app's `+page.svelte` — just conditionally render it when `deferredPrompt` is non-null:

```svelte
{#if !isStandalone && deferredPrompt}
  <button class="pwa-install-fab" onclick={installApp}>⊕ Install App</button>
{/if}
```

---

## Tech Stack

| Layer | Technology | Details |
|---|---|---|
| **UI Framework** | [Svelte 5](https://svelte.dev) | Runes: `$state`, `$derived`, `$effect`, `$props` |
| **App Framework** | [SvelteKit 2](https://kit.svelte.dev) | File-based routing, `adapter-static` for static output |
| **Build Tool** | [Vite 7](https://vitejs.dev) | One independent config per app |
| **Static Adapter** | `@sveltejs/adapter-static` | Outputs each app into its own `dist/` sub-directory |
| **PWA** | [vite-plugin-pwa](https://vite-pwa-org.netlify.app) v1.2 + Workbox | Service worker + offline caching + web manifest |
| **Markdown** | [marked](https://marked.js.org) v12 | Renders `fullDescription` on the app detail page |
| **Styling** | Custom CSS (neumorphic system) | No CSS framework — pure CSS custom properties |
| **Language** | JavaScript (no TypeScript) | |
| **Fonts** | [Orbitron](https://fonts.google.com/specimen/Orbitron) + Segoe UI | Headings and body text |
| **Deployment** | GitHub Pages + GitHub Actions | Push to `main` → auto build and deploy |

---

## Dependencies

Every package used in this project, where it lives, and exactly why it is needed.

### Root workspace (`package.json`)

The root `package.json` holds only tooling shared across the monorepo. It has no runtime code of its own.

| Package | Version | Why it's here |
|---|---|---|
| `svelte` | `^5.55.1` | Provides Svelte language types for the VS Code language server. Not bundled into any app — each app installs its own copy. |
| `@sveltejs/kit` | `^2.56.1` | Same reason — type definitions for the monorepo editor experience. |
| `@sveltejs/adapter-static` | `^3.0.10` | Same reason — adapter types available workspace-wide. |
| `svelte-language-server` | `^0.17.30` | Powers Svelte IntelliSense in VS Code (syntax highlighting, hover docs, error underlining). Dev-only, never shipped. |

---

### Launcher (`launcher/package.json`)

| Package | Version | Type | Why it's here |
|---|---|---|---|
| `marked` | `^12.0.2` | runtime dependency | Parses the `fullDescription` Markdown field in `apps.json` and converts it to HTML for display on the app detail page. The only runtime library in the entire project. |
| `svelte` | `^5.0.0` | devDependency | The Svelte compiler. Transforms `.svelte` files into JavaScript at build time. Not shipped to the browser as a library — compiled away entirely. |
| `@sveltejs/kit` | `^2.0.0` | devDependency | The SvelteKit framework: file-based router, SSR/prerender pipeline, `vite`/`svelte` integration. Drives `vite build` and `vite dev`. |
| `@sveltejs/adapter-static` | `^3.0.0` | devDependency | Tells SvelteKit to output a fully static site (`dist/`) instead of a Node.js server. Writes `index.html` + `404.html` + all assets. Required for GitHub Pages. |
| `vite` | `^7.0.0` | devDependency | The build tool and dev server. Bundles and tree-shakes all JS/CSS, serves HMR in dev mode, drives SSR build pipeline for SvelteKit. Vite 7 uses Rollup (not Rolldown), which is required for `vite-plugin-pwa` compatibility. |
| `vite-plugin-pwa` | `^1.2.0` | devDependency | Generates `sw.js` (service worker) and `manifest.webmanifest` at build time using Workbox. Enables offline caching, "Add to Home Screen" install prompts, and PWA badge in browser address bars. |

#### Launcher overrides

| Override | Pinned to | Why |
|---|---|---|
| `@sveltejs/vite-plugin-svelte` | `^6.2.4` | npm's peer-resolution algorithm would otherwise pick `v7.0.0`, which requires Vite 8. Pinning to v6 keeps everything on Vite 7 cleanly, with no `--legacy-peer-deps`. |
| `serialize-javascript` | `^6.0.2` | Patches a known prototype-pollution vulnerability (CVE) in older transitive versions pulled in by Workbox. |

---

### Sub-apps (`apps/*/package.json`)

All three sub-apps (`calculator`, `color-calculator`, `temp-converter`) have identical dependency sets. None have any runtime dependencies — they are pure compile-time builds.

| Package | Version | Type | Why it's here |
|---|---|---|---|
| `svelte` | `^5.0.0` | devDependency | Svelte compiler — same role as in the launcher. |
| `@sveltejs/kit` | `^2.0.0` | devDependency | SvelteKit framework — file-based routing, build pipeline. Each sub-app is a fully independent SvelteKit project. |
| `@sveltejs/adapter-static` | `^3.0.0` | devDependency | Outputs the sub-app into `dist/apps/<name>/` as a static SPA with `index.html` as the SPA fallback. |
| `vite` | `^7.0.0` | devDependency | Build tool — same role as in the launcher. |
| `vite-plugin-pwa` | `^1.2.0` | devDependency | Generates a per-app service worker and web manifest so each sub-app is independently installable as a PWA. |

#### Sub-app overrides (same as launcher)

| Override | Pinned to | Why |
|---|---|---|
| `@sveltejs/vite-plugin-svelte` | `^6.2.4` | Same Vite 7 compatibility pin as the launcher. |
| `serialize-javascript` | `^6.0.2` | Same security patch as the launcher. |

---

### Transitive dependencies (auto-resolved, not directly declared)

These are installed automatically by npm as dependencies of the packages above. You never declare them — but it's useful to know they exist.

| Package | Pulled in by | Role |
|---|---|---|
| `@sveltejs/vite-plugin-svelte` | `@sveltejs/kit` | The Vite plugin that connects the Svelte compiler to Vite's transform pipeline. Pinned to `^6.2.4` via `overrides`. |
| `workbox-build` | `vite-plugin-pwa` | Generates the Workbox-powered service worker (`sw.js`) and its precache manifest at build time. |
| `workbox-window` | `vite-plugin-pwa` | Tiny browser-side library (inlined into the service worker registration script) that manages SW lifecycle events — update detection, `skipWaiting`, etc. |
| `rollup` | `vite` | The bundler Vite 7 uses under the hood (replaced by Rolldown in Vite 8, which is why we stay on Vite 7). |
