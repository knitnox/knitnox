import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import { VitePWA } from 'vite-plugin-pwa';

export default defineConfig({
  plugins: [
    sveltekit(),
    VitePWA({
      registerType: 'autoUpdate',
      manifest: {
        name: 'Knitnox Color Calculator',
        short_name: 'Color Calc',
        description: 'Convert between HEX, RGB, and HSL color codes. Works offline.',
        start_url: '/apps/color-calculator/',
        scope: '/apps/color-calculator/',
        display: 'standalone',
        background_color: '#e6e9ef',
        orientation: 'portrait',
        theme_color: '#e6e9ef',
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
