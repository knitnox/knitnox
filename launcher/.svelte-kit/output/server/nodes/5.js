

export const index = 5;
let component_cache;
export const component = async () => component_cache ??= (await import('../entries/pages/apps/_page.svelte.js')).default;
export const universal = {
  "prerender": false,
  "ssr": false,
  "load": null
};
export const universal_id = "src/routes/apps/+page.js";
export const imports = ["_app/immutable/nodes/5.DM9ofAlr.js","_app/immutable/chunks/DsnmJJEf.js","_app/immutable/chunks/CG0alVau.js","_app/immutable/chunks/DIeogL5L.js","_app/immutable/chunks/o-iWUkey.js","_app/immutable/chunks/DcHa5gG-.js","_app/immutable/chunks/IjSwQgmF.js","_app/immutable/chunks/Bwj5OFLd.js","_app/immutable/chunks/mfW-oSg8.js","_app/immutable/chunks/B8nHbVQm.js","_app/immutable/chunks/qj8kuolN.js","_app/immutable/chunks/BUApaBEI.js","_app/immutable/chunks/DdDE79wW.js"];
export const stylesheets = ["_app/immutable/assets/5.DUqp0DY_.css"];
export const fonts = [];
