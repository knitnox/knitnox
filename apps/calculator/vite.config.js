import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import { VitePWA } from 'vite-plugin-pwa';

export default defineConfig({
  plugins: [
    sveltekit(),
    VitePWA({
      registerType: 'autoUpdate',
      manifest: {
        name: 'Knitnox Calculator',
        short_name: 'Calculator',
        description: 'A beautiful neumorphic calculator that works offline.',
        start_url: '/apps/calculator/',
        scope: '/apps/calculator/',
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
