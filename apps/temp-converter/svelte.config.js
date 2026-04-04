import adapter from '@sveltejs/adapter-static';

export default {
  kit: {
    adapter: adapter({
      pages: '../../dist/apps/temp-converter',
      assets: '../../dist/apps/temp-converter',
      fallback: 'index.html',
      precompress: false,
      strict: false,
    }),
    paths: {
      base: '/apps/temp-converter',
    },
  },
};
