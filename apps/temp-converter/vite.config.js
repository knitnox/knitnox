import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import { VitePWA } from 'vite-plugin-pwa';

export default defineConfig({
  plugins: [
    sveltekit(),
    VitePWA({
      registerType: 'autoUpdate',
      manifest: {
        name: 'Knitnox Temp Converter',
        short_name: 'Temp Converter',
        description: 'Convert between Celsius, Fahrenheit, and Kelvin. Works offline.',
        start_url: '/apps/temp-converter/',
        scope: '/apps/temp-converter/',
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
