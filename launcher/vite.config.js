import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import { readFileSync, existsSync, statSync } from 'fs';
import { join, extname, resolve as resolvePath, dirname } from 'path';
import { fileURLToPath } from 'url';
import { VitePWA } from 'vite-plugin-pwa';

const __dirname = dirname(fileURLToPath(import.meta.url));
const distDir = resolvePath(__dirname, '../dist');

const MIME = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'application/javascript',
  '.mjs': 'application/javascript',
  '.css': 'text/css',
  '.json': 'application/json',
  '.webmanifest': 'application/manifest+json',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.svg': 'image/svg+xml',
  '.ico': 'image/x-icon',
  '.woff2': 'font/woff2',
  '.woff': 'font/woff',
  '.ttf': 'font/ttf',
};

// In dev: serves pre-built sub-apps from ../dist/apps/* directly.
// In preview: rewrites /apps/<name>/ → /apps/<name>/index.html for sirv.
function routeAppsPlugin() {
  function devMiddleware(req, res, next) {
    const url = req.url || '';
    const urlPath = url.split('?')[0];
    if (!urlPath.startsWith('/apps/')) return next();

    // Resolve to a concrete file path (add index.html for directory-style URLs)
    const filePath = /\.[^/]+$/.test(urlPath)
      ? urlPath
      : urlPath.replace(/\/?$/, '/index.html');

    const fullPath = join(distDir, filePath);
    if (existsSync(fullPath) && statSync(fullPath).isFile()) {
      const mime = MIME[extname(fullPath)] || 'application/octet-stream';
      res.setHeader('Content-Type', mime);
      res.end(readFileSync(fullPath));
      return;
    }
    next();
  }

  function previewMiddleware(req, res, next) {
    const url = req.url || '';
    const urlPath = url.split('?')[0];
    if (!urlPath.startsWith('/apps/')) return next();

    const filePath = /\.[^/]+$/.test(urlPath)
      ? urlPath
      : urlPath.replace(/\/?$/, '/index.html');

    const fullPath = join(distDir, filePath);
    if (existsSync(fullPath) && statSync(fullPath).isFile()) {
      const mime = MIME[extname(fullPath)] || 'application/octet-stream';
      res.setHeader('Content-Type', mime);
      res.end(readFileSync(fullPath));
      return;
    }
    next();
  }

  return {
    name: 'route-apps',
    configureServer(server) { server.middlewares.use(devMiddleware); },
    configurePreviewServer(server) { server.middlewares.use(previewMiddleware); },
  };
}

export default defineConfig({
  plugins: [
    routeAppsPlugin(),
    sveltekit(),
    VitePWA({
      registerType: 'autoUpdate',
      manifest: {
        name: 'Knitnox',
        short_name: 'Knitnox',
        description: 'Hardware control & privacy-first tools — zero cloud, zero latency.',
        start_url: '/',
        scope: '/',
        display: 'standalone',
        background_color: '#e6e9ef',
        theme_color: '#e6e9ef',
        orientation: 'portrait',
        icons: [
          {
            src: '/favicon.svg',
            sizes: 'any',
            type: 'image/svg+xml',
            purpose: 'any maskable',
          },
        ],
      },
      workbox: {
        globPatterns: ['**/*.{js,css,html,svg,ico,woff,woff2,png}'],
        navigateFallback: '/404.html',
        navigateFallbackDenylist: [/^\/apps\//],
      },
    }),
  ],
});

