import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

// Custom plugin that rewrites /apps/<name>[/<route>] → /apps/<name>/index.html
// BEFORE sirv runs. This is necessary because Vite's preview server uses sirv
// with single:true (SPA fallback to root index.html), which intercepts app paths
// before historyApiFallback gets a chance to run.
function routeAppsPlugin() {
  function middleware(req, _res, next) {
    const url = req.url || '';
    const path = url.split('?')[0];
    const m = path.match(/^\/apps\/([^/]+)(\/|$)/);
    // Only rewrite HTML navigation requests (no file extension = not a static asset)
    if (m && !/\.[a-zA-Z0-9]{1,8}$/.test(path)) {
      const query = url.slice(path.length);
      req.url = `/apps/${m[1]}/index.html${query}`;
    }
    next();
  }
  return {
    name: 'route-apps',
    // configureServer/configurePreviewServer hooks run BEFORE sirv and historyApiFallback
    configureServer(server) { server.middlewares.use(middleware); },
    configurePreviewServer(server) { server.middlewares.use(middleware); },
  };
}

export default defineConfig({
  base: '/',
  build: {
    outDir: '../dist',
    emptyOutDir: true,
  },
  server: {
    historyApiFallback: true,
  },
  preview: {
    historyApiFallback: true,
  },
  plugins: [routeAppsPlugin(), svelte()],
});
