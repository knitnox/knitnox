

export const index = 0;
let component_cache;
export const component = async () => component_cache ??= (await import('../entries/pages/_layout.svelte.js')).default;
export const universal = {
  "prerender": false,
  "ssr": false
};
export const universal_id = "src/routes/+layout.js";
export const imports = ["_app/immutable/nodes/0.w04QDQSw.js","_app/immutable/chunks/DBM8aOVT.js","_app/immutable/chunks/B010l9nW.js","_app/immutable/chunks/CtBTFLmV.js"];
export const stylesheets = ["_app/immutable/assets/0.CRsnxl8_.css"];
export const fonts = [];
