import adapter from '@sveltejs/adapter-static';

export default {
  kit: {
    adapter: adapter({
      pages: '../../dist/apps/calculator',
      assets: '../../dist/apps/calculator',
      fallback: 'index.html',
      precompress: false,
      strict: false,
    }),
    paths: {
      base: '/apps/calculator',
    },
    files: {
      assets: 'public',
    },
  },
};
