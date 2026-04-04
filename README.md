# Knitnox

> A **browser-native IoT control platform** for ESP32 & Arduino — plus offline-first utility and productivity apps. Built with **Svelte 5**, **Vite**, and a custom **Neumorphic design system**. Deployed automatically to GitHub Pages via GitHub Actions.

No cloud. No accounts. No drivers. Just your browser talking directly to your hardware.

---

## Table of Contents

- [Overview](#overview)
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

## Overview

Knitnox is a suite of small, focused **Progressive Web Apps (PWAs)** wrapped in a shared app-store launcher. It has two main categories:

### 🔧 Hardware Control Apps
Connect to **ESP32**, **Arduino**, and other microcontrollers directly from the browser using modern Web APIs — no drivers, no middleware, no cloud relay:

| Browser API | What it does |
|---|---|
| **Web Bluetooth** | Pair with BLE peripherals (ESP32, Arduino Nano 33 BLE) |
| **Web Serial** | Open UART/COM ports — send commands, stream sensor data |
| **WebUSB** | Talk to USB devices (CDC, HID, vendor-class) directly |
| **WebSocket / HTTP** | Control Wi-Fi devices over your Local Area Network |

### 🔒 Privacy-First Utility Apps
Everyday tools (calculators, converters, etc.) that run entirely on your device:

- Works fully **offline** via service workers
- **Installable** to the home screen on any device (PWA)
- **Zero telemetry** — no analytics, no accounts, no data ever leaves your browser
- Shares a consistent **neumorphic (soft UI)** visual language

---

## Architecture

```
┌───────────────────────────────────────────────────────────┐
│                     knitnox.com  (/)                      │
│                                                           │
│   ┌─────────────────────────────────────────────────┐    │
│   │             Launcher (Svelte 5 SPA)             │    │
│   │   App Store UI — search, browse, install apps   │    │
│   │         Reads: /apps.json  (flat data file)     │    │
│   └────────────┬───────────────────────┬────────────┘    │
│                │   opens in same tab    │                  │
│       ┌────────▼────────┐   ┌──────────▼──────────┐      │
│       │   Calculator    │   │   Color Calculator   │      │
│       │  /apps/calc…/   │   │  /apps/color-calc…/  │      │
│       │  Svelte5 + PWA  │   │   Svelte5 + PWA      │      │
│       └─────────────────┘   └──────────────────────┘      │
└───────────────────────────────────────────────────────────┘

Build output: everything merges into a single dist/ folder
Hosting: GitHub Pages (static, no server needed)
CI/CD: GitHub Actions — push to main → auto build → deploy
```

---

## Project Structure

```
knitnox/
├── .github/
│   └── workflows/
│       └── deploy.yml          ← GitHub Actions: build + deploy on push to main
│
├── launcher/                   ← App Store SPA (the home screen / app browser)
│   ├── src/
│   │   ├── main.js             ← Svelte 5 mount point
│   │   ├── App.svelte          ← View router ($state: home / apps / app detail)
│   │   ├── lib/
│   │   │   ├── neumorphic.css  ← Shared design system tokens + components
│   │   │   └── styles.css      ← Launcher-specific styles (landing page, cards, etc.)
│   │   └── components/
│   │       ├── HomeView.svelte       ← Landing page (hero, stats, how it works)
│   │       ├── AppsView.svelte       ← App grid with search and infinite scroll
│   │       ├── AppDetailView.svelte  ← App detail page (Markdown description + install)
│   │       └── AppCard.svelte        ← Single app card in the grid
│   ├── public/
│   │   ├── apps.json           ← App registry (edit this to add/remove apps)
│   │   ├── icons/              ← SVG app icons served by the launcher
│   │   ├── screenshots/        ← App screenshots served by the launcher
│   │   ├── CNAME               ← Custom domain: knitnox.com
│   │   ├── .nojekyll           ← Prevents GitHub Pages from ignoring _assets/
│   │   └── 404.html            ← GitHub Pages SPA fallback (redirects to index.html)
│   ├── index.html
│   ├── vite.config.js          ← base: '/', outDir: '../dist'
│   └── package.json            ← Dependencies include: svelte, marked (Markdown parser)
│
├── apps/
│   ├── calculator/             ← Installable PWA
│   │   ├── src/
│   │   │   ├── main.js         ← Detects standalone mode → writes install flag to localStorage
│   │   │   ├── App.svelte
│   │   │   ├── lib/
│   │   │   │   └── neumorphic.css
│   │   │   └── components/
│   │   │       ├── Display.svelte
│   │   │       ├── ButtonGrid.svelte
│   │   │       └── CalcButton.svelte
│   │   ├── index.html
│   │   ├── vite.config.js      ← base: '/apps/calculator/', PWA manifest
│   │   └── package.json
│   │
│   ├── color-calculator/       ← Installable PWA
│   │   ├── src/
│   │   │   ├── main.js
│   │   │   ├── App.svelte
│   │   │   ├── lib/
│   │   │   │   └── neumorphic.css
│   │   │   └── components/
│   │   │       ├── ColorPreview.svelte
│   │   │       ├── HexInput.svelte
│   │   │       ├── RgbInputs.svelte
│   │   │       └── HslInputs.svelte
│   │   ├── index.html
│   │   ├── vite.config.js      ← base: '/apps/color-calculator/', PWA manifest
│   │   └── package.json
│   │
│   └── temp-converter/         ← Installable PWA
│       ├── src/
│       │   ├── main.js
│       │   ├── App.svelte      ← °C ↔ °F ↔ K real-time conversion ($state)
│       │   └── lib/
│       │       └── neumorphic.css
│       ├── index.html
│       ├── vite.config.js      ← base: '/apps/temp-converter/', PWA manifest
│       └── package.json
│
├── dist/                       ← Build output (gitignored — never edit directly)
├── .gitignore
└── package.json                ← Root: runs all 4 builds sequentially
```

---

## Apps

| App | URL | Description |
|---|---|---|
| **Launcher** | `/` | App store — browse, search, open, and install apps |
| **Calculator** | `/apps/calculator/` | Neumorphic calculator with keyboard support |
| **Color Calculator** | `/apps/color-calculator/` | Real-time HEX ↔ RGB ↔ HSL converter |
| **Temp Converter** | `/apps/temp-converter/` | Real-time °C ↔ °F ↔ K temperature converter |

---

## Getting Started

### Prerequisites

You need **Node.js** and **npm** installed on your computer. If you haven't installed them yet, download from [nodejs.org](https://nodejs.org/) — choose the **LTS** version.

Check your versions:

```bash
node --version   # should be v18 or higher
npm --version    # should be v9 or higher
```

### 1. Clone the repo

```bash
git clone https://github.com/your-username/knitnox.git
cd knitnox
```

### 2. Install dependencies for all projects

```bash
npm run install:all
```

> This installs `node_modules` for the launcher and every app in a single command. It may take a minute.

### 3. Build everything

```bash
npm run build
```

This runs all builds in sequence and merges the output into `dist/`.

### 4. Preview the full site locally

```bash
npm run preview --prefix launcher
```

Open `http://localhost:4173` — you'll see the complete app store with all apps working.

---

## Development Workflow

Run any project in **hot-reload dev mode** (changes reflect instantly in the browser):

```bash
# Launcher (app store UI)
npm run dev --prefix launcher
# → http://localhost:5173

# Calculator
npm run dev --prefix apps/calculator
# → http://localhost:5173/apps/calculator/

# Color Calculator
npm run dev --prefix apps/color-calculator
# → http://localhost:5173/apps/color-calculator/

# Temp Converter
npm run dev --prefix apps/temp-converter
# → http://localhost:5173/apps/temp-converter/
```

> **Tip:** When working on the launcher, use `npm run build` + `npm run preview` to test the full linked flow between the launcher and apps. In `dev` mode, clicking "Open App" navigates to the deployed URL.

---

## Adding a New App

Follow these steps to add a brand new app to Knitnox:

### Step 1 — Copy an existing app as a template

```bash
# Windows (PowerShell)
Copy-Item apps/calculator apps/my-new-app -Recurse

# macOS / Linux
cp -r apps/calculator apps/my-new-app
```

### Step 2 — Update the Vite config

Edit `apps/my-new-app/vite.config.js`. Change the `base` and `outDir` paths, and update the PWA manifest fields:

```js
export default defineConfig({
  base: '/apps/my-new-app/',
  build: {
    outDir: '../../dist/apps/my-new-app',
    emptyOutDir: true,
  },
  plugins: [
    svelte(),
    VitePWA({
      manifest: {
        name: 'Knitnox My New App',
        short_name: 'My App',
        start_url: '/apps/my-new-app/',
        scope: '/apps/my-new-app/',
        // ...
      },
    }),
  ],
});
```

### Step 3 — Update the splash screen in index.html

Open `apps/my-new-app/index.html` and update the app name shown while loading:

```html
<div id="splash" aria-hidden="true">
  <div class="splash-card">
    <img src="icon.svg" alt="" class="splash-icon" />
    <p class="splash-name">My New App</p>  <!-- ← change this -->
    <p class="splash-brand">Knitnox</p>
  </div>
</div>
```

### Step 4 — Build the app UI

Edit `apps/my-new-app/src/App.svelte` using Svelte 5 runes:

```svelte
<script>
  let count = $state(0);          // reactive variable
  let doubled = $derived(count * 2); // computed value

  $effect(() => {
    document.title = `Count: ${count}`; // runs whenever count changes
  });
</script>

<button onclick={() => count++}>
  Clicked {count} times (doubled: {doubled})
</button>
```

### Step 5 — Add icon and screenshots

Place the app icon and any screenshots here:

```
launcher/public/icons/my-new-app.svg          ← app icon (shown on app card)
launcher/public/screenshots/my-new-app/       ← optional screenshots
  screenshot1.png
```

### Step 6 — Register in apps.json

Open `launcher/public/apps.json` and add an entry. The `fullDescription` field supports **Markdown** — use headings, lists, code blocks, and tables to write proper documentation:

```json
{
  "id": "my-new-app",
  "name": "My New App",
  "category": "Utilities",
  "icon": "/icons/my-new-app.svg",
  "shortDescription": "One-line description shown on the app card",
  "fullDescription": "## Overview\n\nDescribe your app here. Supports **Markdown**.\n\n## Features\n\n- Feature one\n- Feature two\n\n## Privacy\n\nNo data leaves your device.",
  "folder": "apps/my-new-app",
  "screenshots": ["/screenshots/my-new-app/screenshot1.png"],
  "manifestUrl": "/apps/my-new-app/manifest.webmanifest",
  "url": "/apps/my-new-app/"
}
```

### Step 7 — Add build scripts to root package.json

```json
{
  "scripts": {
    "build:my-new-app": "npm run build --prefix apps/my-new-app",
    "build": "npm run build:launcher && npm run build:calculator && npm run build:color-calculator && npm run build:temp-converter && npm run build:my-new-app",
    "install:all": "... && npm install --prefix apps/my-new-app"
  }
}
```

### Step 8 — Add the install step to GitHub Actions

Open `.github/workflows/deploy.yml` and add an `npm ci` step for your new app:

```yaml
      - name: Install my-new-app dependencies
        run: npm ci
        working-directory: apps/my-new-app
```

> Without this step, the CI build will fail because `node_modules` is never committed to the repo.

### Step 9 — Install, build, and verify

```bash
npm install --prefix apps/my-new-app
npm run build
npm run preview --prefix launcher
```

Open `http://localhost:4173` — your new app should appear in the launcher.

---

## How Install Detection Works

Each sub-app's `main.js` runs a check on every launch:

```js
if (window.matchMedia('(display-mode: standalone)').matches) {
  localStorage.setItem('knitnox-app-my-new-app', '1');
}
```

`display-mode: standalone` is only `true` when the app is running as a **genuinely installed PWA** (launched from the home screen, not a browser tab). The launcher reads these keys to show a green **✓ Installed** badge on the app detail page.

---

## Deployment

The site deploys **automatically** to GitHub Pages on every push to `main`.

### How it works

```
git push origin main
        ↓
GitHub Actions (.github/workflows/deploy.yml)
        ├── npm ci  (launcher)
        ├── npm ci  (apps/calculator)
        ├── npm ci  (apps/color-calculator)
        ├── npm ci  (apps/temp-converter)
        ├── npm run build  (all apps → dist/)
        └── deploy dist/ → GitHub Pages → knitnox.com
```

### First-time GitHub Pages setup

1. Go to your repo on GitHub
2. **Settings → Pages → Source** → set to **"GitHub Actions"**
3. Push to `main` — the workflow handles the rest

### Manual build and deploy

```bash
npm run build
# Upload the dist/ folder to any static host (Netlify, Vercel, Cloudflare Pages, etc.)
```

---

## Design System

All apps share `neumorphic.css` — a soft UI component library using CSS variables. The "neumorphic" effect creates a raised/pressed look using two shadows: one dark (depth) and one light (highlight).

### Theme tokens

```css
:root {
  --neumorph-bg:           #e6e9ef;  /* page background */
  --neumorph-light:        #ffffff;  /* highlight shadow */
  --neumorph-dark:         #c2c8d0;  /* depth shadow */
  --neumorph-text:         #2a2f3a;
  --neumorph-accent-blue:  #5a8dee;
  --neumorph-accent-red:   #ff6b6b;
  --neumorph-accent-green: #51cf66;
  --neumorph-accent-orange:#ffa500;
}
```

### Shadow system

| Effect | When to use | CSS |
|---|---|---|
| **Raised** | Default cards, buttons | `6px 6px 12px dark, -6px -6px 12px light` |
| **Pressed** | Active / clicked state | `inset 4px 4px 8px dark, inset -4px -4px 8px light` |
| **Large** | Hero cards | `10px 10px 20px dark, -10px -10px 20px light` |

### Key component classes

```html
<div class="neumorph-card">…</div>
<button class="neumorph-btn">Default</button>
<button class="neumorph-btn neumorph-btn-accent">Accent</button>
<input class="neumorph-input" />
<div class="neumorph-container">…</div>
<div class="neumorph-grid-2">…</div>          <!-- 2 columns -->
<div class="neumorph-grid-responsive">…</div> <!-- 2 col → 4 col at 1024px -->
```

---

## Tech Stack

| Layer | Technology | Notes |
|---|---|---|
| **UI Framework** | [Svelte 5](https://svelte.dev) | Uses runes: `$state`, `$derived`, `$effect`, `$props` |
| **Build Tool** | [Vite 6](https://vitejs.dev) | One config per app |
| **PWA** | [vite-plugin-pwa](https://vite-pwa-org.netlify.app) + Workbox | Service worker + offline caching |
| **Markdown** | [marked](https://marked.js.org) v12 | Renders `fullDescription` in app detail views |
| **Install Detection** | `display-mode: standalone` + localStorage | Sub-apps write a flag; launcher reads it |
| **Hardware APIs** | Web Bluetooth, Web Serial, WebUSB, WebSocket | Browser-native, no drivers required |
| **Styling** | Custom CSS (neumorphic design system) | No CSS framework |
| **Language** | JavaScript (no TypeScript) | |
| **Deployment** | GitHub Pages via GitHub Actions | Push to main = auto deploy |
| **Fonts** | [Orbitron](https://fonts.google.com/specimen/Orbitron) + Segoe UI | Headings + body |


---

## Table of Contents

- [Overview](#overview)
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

## Overview

Knitnox is a suite of small, focused **Progressive Web Apps (PWAs)** wrapped in a shared app-store launcher. Every app:

- Works fully **offline** via service workers
- Is **installable** to the home screen on any device
- Shares a consistent **neumorphic (soft UI)** visual language
- Is a completely **standalone Svelte 5 project** — easy to extend or extract independently

---

## Architecture

```
┌───────────────────────────────────────────────────────────┐
│                     knitnox.com  (/)                      │
│                                                           │
│   ┌─────────────────────────────────────────────────┐    │
│   │             Launcher (Svelte 5 SPA)             │    │
│   │       App Store UI — search, browse, detail     │    │
│   │         Reads: /apps.json  (flat data file)     │    │
│   └────────────┬───────────────────────┬────────────┘    │
│                │   opens in same tab    │                  │
│       ┌────────▼────────┐   ┌──────────▼──────────┐      │
│       │   Calculator    │   │   Color Calculator   │      │
│       │  /apps/calc…/   │   │  /apps/color-calc…/  │      │
│       │  Svelte5 + PWA  │   │   Svelte5 + PWA      │      │
│       └─────────────────┘   └──────────────────────┘      │
└───────────────────────────────────────────────────────────┘

Build output: everything merges into a single dist/ folder
Hosting: GitHub Pages (static, no server)
CI/CD: GitHub Actions — push to main → auto build → deploy
```

---

## Project Structure

```
knitnox/
├── .github/
│   └── workflows/
│       └── deploy.yml          ← GitHub Actions: build + deploy on push to main
│
├── launcher/                   ← App Store SPA (NOT a PWA)
│   ├── src/
│   │   ├── main.js             ← Svelte 5 mount
│   │   ├── App.svelte          ← View router ($state)
│   │   ├── lib/
│   │   │   ├── neumorphic.css  ← Design system tokens + components
│   │   │   └── styles.css      ← Launcher-specific styles
│   │   └── components/
│   │       ├── HomeView.svelte
│   │       ├── AppsView.svelte
│   │       ├── AppDetailView.svelte
│   │       └── AppCard.svelte
│   ├── public/
│   │   ├── apps.json           ← App registry (edit this to add/remove apps)
│   │   ├── icons/              ← App icons served by launcher
│   │   │   ├── calculator.svg
│   │   │   └── color-calculator.svg
│   │   ├── screenshots/        ← App screenshots served by launcher
│   │   │   └── calculator/
│   │   │       ├── screenshot.PNG
│   │   │       └── screenshot2.PNG
│   │   ├── CNAME               ← Custom domain: knitnox.com
│   │   ├── .nojekyll           ← Prevents GitHub Pages from ignoring _assets/
│   │   └── 404.html            ← GitHub Pages SPA fallback (redirects to index.html)
│   ├── index.html
│   ├── vite.config.js          ← base: '/', outDir: '../dist'
│   └── package.json
│
├── apps/
│   ├── calculator/             ← Installable PWA
│   │   ├── src/
│   │   │   ├── main.js
│   │   │   ├── App.svelte      ← Calculator state + keyboard handler ($state, $effect)
│   │   │   ├── lib/
│   │   │   │   └── neumorphic.css
│   │   │   └── components/
│   │   │       ├── Display.svelte
│   │   │       ├── ButtonGrid.svelte
│   │   │       └── CalcButton.svelte
│   │   ├── public/
│   │   │   └── icon.svg
│   │   ├── index.html
│   │   ├── vite.config.js      ← base: '/apps/calculator/', PWA manifest
│   │   └── package.json
│   │
│   └── color-calculator/       ← Installable PWA
│       ├── src/
│       │   ├── main.js
│       │   ├── App.svelte
│       │   ├── lib/
│       │   │   └── neumorphic.css
│       │   └── components/
│       │       ├── ColorPreview.svelte
│       │       ├── HexInput.svelte
│       │       ├── RgbInputs.svelte
│       │       └── HslInputs.svelte
│       ├── public/
│       │   └── icon.svg
│       ├── index.html
│       ├── vite.config.js      ← base: '/apps/color-calculator/', PWA manifest
│       └── package.json
│
└── apps/temp-converter/        ← Installable PWA
    ├── src/
    │   ├── main.js
    │   ├── App.svelte          ← °C ↔ °F ↔ K real-time conversion ($state)
    │   └── lib/
    │       └── neumorphic.css
    ├── public/
    │   └── icon.svg
    ├── index.html
    ├── vite.config.js      ← base: '/apps/temp-converter/', PWA manifest
    └── package.json
│
├── dist/                       ← Build output (gitignored — never edit directly)
├── .gitignore
└── package.json                ← Root: runs all 3 builds sequentially
```

---

## Apps

| App | URL | Description |
|---|---|---|
| **Launcher** | `/` | App store — browse, search, open, install |
| **Calculator** | `/apps/calculator/` | Neumorphic calculator with keyboard support |
| **Color Calculator** | `/apps/color-calculator/` | Real-time HEX ↔ RGB ↔ HSL converter |
| **Temp Converter** | `/apps/temp-converter/` | Real-time °C ↔ °F ↔ K temperature converter |

---

## Getting Started

### Prerequisites

- [Node.js](https://nodejs.org/) v18 or higher
- npm v9 or higher

Check your versions:

```bash
node --version
npm --version
```

### 1. Clone the repo

```bash
git clone https://github.com/your-username/knitnox.git
cd knitnox
```

### 2. Install dependencies for all projects

```bash
npm run install:all
```

This installs `node_modules` for the launcher and every app in a single command.

### 3. Build everything

```bash
npm run build
```

This runs all 3 builds in sequence and merges the output into `dist/`.

### 4. Preview the full site locally

```bash
npm run preview --prefix launcher
```

Open `http://localhost:4173` — you'll see the complete app store with all apps working.

---

## Development Workflow

Run any project in **hot-reload dev mode**:

```bash
# Launcher (app store UI)
npm run dev --prefix launcher
# → http://localhost:5173

# Calculator
npm run dev --prefix apps/calculator
# → http://localhost:5173/apps/calculator/

# Color Calculator
npm run dev --prefix apps/color-calculator
# → http://localhost:5173/apps/color-calculator/
```

> **Tip:** When working on the launcher, use `npm run build` + `npm run preview` to test the full linked flow. In `dev` mode, app links open deployed URLs.

---

## Adding a New App

Follow these 9 steps to add a brand new app to Knitnox:

### Step 1 — Copy an existing app as a template

```bash
# Windows (PowerShell)
Copy-Item apps/calculator apps/my-new-app -Recurse

# macOS / Linux
cp -r apps/calculator apps/my-new-app
```

### Step 2 — Update the Vite config

Edit `apps/my-new-app/vite.config.js`:

```js
export default defineConfig({
  base: '/apps/my-new-app/',          // ← change this
  build: {
    outDir: '../../dist/apps/my-new-app',  // ← and this
    emptyOutDir: true,
  },
  plugins: [
    svelte(),
    VitePWA({
      manifest: {
        name: 'Knitnox My New App',   // ← and these
        short_name: 'My App',
        start_url: '/apps/my-new-app/',
        scope: '/apps/my-new-app/',
        // ...
      },
    }),
  ],
});
```

### Step 3 — Update the splash screen in index.html

Open `apps/my-new-app/index.html` and change the app name shown during loading:

```html
<div id="splash" aria-hidden="true">
  <div class="splash-card">
    <img src="icon.svg" alt="" class="splash-icon" />
    <p class="splash-name">My New App</p>  <!-- ← change this -->
    <p class="splash-brand">Knitnox</p>
  </div>
</div>
```

### Step 4 — Build the app UI

Edit `apps/my-new-app/src/App.svelte` and components using Svelte 5 runes:

```svelte
<script>
  // Reactive state
  let count = $state(0);

  // Computed values
  let doubled = $derived(count * 2);

  // Side effects (DOM events, SW registration, etc.)
  $effect(() => {
    document.title = `Count: ${count}`;
  });
</script>

<button onclick={() => count++}>
  Clicked {count} times (doubled: {doubled})
</button>
```

### Step 5 — Add the app icon and screenshots

```
launcher/public/icons/my-new-app.svg          ← app icon (shown in launcher)
launcher/public/screenshots/my-new-app/       ← screenshots (shown in detail view)
  screenshot1.png
  screenshot2.png
```

### Step 6 — Register in apps.json

Open `launcher/public/apps.json` and add an entry:

```json
{
  "id": "my-new-app",
  "name": "My New App",
  "category": "Utilities",
  "icon": "/icons/my-new-app.svg",
  "shortDescription": "One line description shown on the card",
  "fullDescription": "Full description shown on the detail page.",
  "folder": "apps/my-new-app",
  "screenshots": [
    "/screenshots/my-new-app/screenshot1.png"
  ],
  "manifestUrl": "/apps/my-new-app/manifest.webmanifest",
  "url": "/apps/my-new-app/"
}
```

### Step 7 — Add build script to root package.json

```json
{
  "scripts": {
    "build:my-new-app": "npm run build --prefix apps/my-new-app",
    "build": "npm run build:launcher && npm run build:calculator && npm run build:color-calculator && npm run build:my-new-app"
  }
}
```

### Step 8 — Add the install step to the GitHub Actions workflow

Open `.github/workflows/deploy.yml` and add an `npm ci` step for your new app alongside the existing ones:

```yaml
      - name: Install my-new-app dependencies
        run: npm ci
        working-directory: apps/my-new-app
```

> Without this step, the GitHub Actions CI build will fail because `node_modules` is not committed to the repo.

### Step 9 — Install deps and build

```bash
npm install --prefix apps/my-new-app
npm run build
```

---

## Deployment

The site deploys **automatically** to GitHub Pages on every push to `main`.

### How it works

```
git push origin main
        │
        ▼
GitHub Actions (.github/workflows/deploy.yml)
        │
        ├── npm ci  (launcher)
        ├── npm ci  (apps/calculator)
        ├── npm ci  (apps/color-calculator)
        │
        ├── npm run build  (all 3 → dist/)
        │
        └── deploy dist/ → GitHub Pages → knitnox.com
```

### First-time GitHub Pages setup

1. Go to your repo on GitHub
2. **Settings → Pages → Source** → set to **"GitHub Actions"**
3. Push to `main` — the workflow handles the rest

### Manual build and deploy

```bash
npm run build
# Upload dist/ to any static host (Netlify, Vercel, Cloudflare Pages, etc.)
```

---

## Design System

All apps share `neumorphic.css` — a soft UI component library using CSS variables.

### Theme tokens

```css
:root {
  --neumorph-bg:           #e6e9ef;  /* main surface */
  --neumorph-light:        #ffffff;  /* highlight shadow */
  --neumorph-dark:         #c2c8d0;  /* depth shadow */
  --neumorph-text:         #2a2f3a;
  --neumorph-accent-blue:  #5a8dee;
  --neumorph-accent-red:   #ff6b6b;
  --neumorph-accent-green: #51cf66;
  --neumorph-accent-orange:#ffa500;
}
```

### Shadow system

| Effect | CSS |
|---|---|
| **Raised** (default) | `6px 6px 12px dark, -6px -6px 12px light` |
| **Pressed** (active) | `inset 4px 4px 8px dark, inset -4px -4px 8px light` |
| **Large** (cards) | `10px 10px 20px dark, -10px -10px 20px light` |

### Key component classes

```html
<!-- Cards -->
<div class="neumorph-card">…</div>
<div class="neumorph-card-hover">…</div>

<!-- Buttons -->
<button class="neumorph-btn">Default</button>
<button class="neumorph-btn neumorph-btn-accent">Accent</button>

<!-- Inputs -->
<input class="neumorph-input" />

<!-- Layout -->
<div class="neumorph-container">…</div>
<div class="neumorph-grid-2">…</div>   <!-- 2 columns -->
<div class="neumorph-grid-responsive">…</div>  <!-- 2 col → 4 col at 1024px -->

<!-- Typography -->
<h1 class="neumorph-heading neumorph-heading-xl">…</h1>
```

---

## Tech Stack

| Layer | Technology |
|---|---|
| **UI Framework** | [Svelte 5](https://svelte.dev) — runes-based reactivity (`$state`, `$derived`, `$effect`, `$props`) |
| **Build Tool** | [Vite 6](https://vitejs.dev) |
| **PWA** | [vite-plugin-pwa](https://vite-pwa-org.netlify.app) + Workbox |
| **Styling** | Custom CSS (neumorphic design system), no CSS framework |
| **Language** | JavaScript (no TypeScript) |
| **Deployment** | GitHub Pages via GitHub Actions |
| **Font** | [Orbitron](https://fonts.google.com/specimen/Orbitron) (headings) + Segoe UI (body) |
