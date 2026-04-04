import adapter from '@sveltejs/adapter-static';

export default {
  kit: {
    adapter: adapter({
      pages: '../dist',
      assets: '../dist',
      fallback: '404.html',
      precompress: false,
      strict: false,
    }),
    files: {
      assets: 'public',
    },
  },
};
