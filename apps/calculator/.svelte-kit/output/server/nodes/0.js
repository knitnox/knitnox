

export const index = 0;
let component_cache;
export const component = async () => component_cache ??= (await import('../entries/pages/_layout.svelte.js')).default;
export const universal = {
  "prerender": false,
  "ssr": false
};
export const universal_id = "src/routes/+layout.js";
export const imports = ["_app/immutable/nodes/0.CDqRQBud.js","_app/immutable/chunks/nK2oQkYf.js","_app/immutable/chunks/BzxVPEC4.js","_app/immutable/chunks/BWdWBhQN.js"];
export const stylesheets = ["_app/immutable/assets/0.CRsnxl8_.css"];
export const fonts = [];
