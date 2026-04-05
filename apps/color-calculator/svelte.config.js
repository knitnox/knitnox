import adapter from '@sveltejs/adapter-static';

export default {
  kit: {
    adapter: adapter({
      pages: '../../dist/apps/color-calculator',
      assets: '../../dist/apps/color-calculator',
      fallback: 'index.html',
      precompress: false,
      strict: false,
    }),
    paths: {
      base: '/apps/color-calculator',
    },
    files: {
      assets: 'public',
    },
  },
};
