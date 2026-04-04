

export const index = 4;
let component_cache;
export const component = async () => component_cache ??= (await import('../entries/pages/app/_id_/_page.svelte.js')).default;
export const universal = {
  "prerender": false,
  "ssr": false,
  "load": null
};
export const universal_id = "src/routes/app/[id]/+page.js";
export const imports = ["_app/immutable/nodes/4.BDOPK5qt.js","_app/immutable/chunks/hp4PFHFv.js","_app/immutable/chunks/BUApaBEI.js","_app/immutable/chunks/DsnmJJEf.js","_app/immutable/chunks/CG0alVau.js","_app/immutable/chunks/DIeogL5L.js","_app/immutable/chunks/o-iWUkey.js","_app/immutable/chunks/DcHa5gG-.js","_app/immutable/chunks/IjSwQgmF.js","_app/immutable/chunks/Bwj5OFLd.js","_app/immutable/chunks/mfW-oSg8.js","_app/immutable/chunks/B8nHbVQm.js"];
export const stylesheets = ["_app/immutable/assets/4.CPEMqkZI.css"];
export const fonts = [];
