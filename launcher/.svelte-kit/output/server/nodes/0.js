

export const index = 0;
let component_cache;
export const component = async () => component_cache ??= (await import('../entries/pages/_layout.svelte.js')).default;
export const universal = {
  "prerender": false,
  "ssr": false
};
export const universal_id = "src/routes/+layout.js";
export const imports = ["_app/immutable/nodes/0.3UFeRRFV.js","_app/immutable/chunks/DsnmJJEf.js","_app/immutable/chunks/CG0alVau.js","_app/immutable/chunks/DIeogL5L.js","_app/immutable/chunks/DcHa5gG-.js","_app/immutable/chunks/Bwj5OFLd.js"];
export const stylesheets = ["_app/immutable/assets/0.CM0-zsez.css"];
export const fonts = [];
