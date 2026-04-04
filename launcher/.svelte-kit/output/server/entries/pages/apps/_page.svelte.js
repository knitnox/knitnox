import { a as attr, e as escape_html, b as ensure_array_like, d as derived } from "../../../chunks/renderer.js";
import "@sveltejs/kit/internal";
import "../../../chunks/exports.js";
import "../../../chunks/utils.js";
import "@sveltejs/kit/internal/server";
import "../../../chunks/root.js";
import "../../../chunks/state.svelte.js";
import { h as html } from "../../../chunks/html.js";
function AppCard($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let { app } = $$props;
    $$renderer2.push(`<div class="app-card" role="button" tabindex="0"><img${attr("src", app.icon)}${attr("alt", app.name)} class="app-icon"/> <div class="app-info"><h3>${escape_html(app.name)}</h3> <p>${escape_html(app.shortDescription)}</p></div></div>`);
  });
}
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let { data } = $$props;
    const INITIAL_COUNT = 20;
    let allApps = derived(() => data.apps);
    let searchTerm = "";
    let displayCount = INITIAL_COUNT;
    let filtered = derived(() => searchTerm.trim() === "" ? allApps() : allApps().filter((app) => {
      const t = searchTerm.toLowerCase();
      return app.name.toLowerCase().includes(t) || app.shortDescription.toLowerCase().includes(t) || app.fullDescription.toLowerCase().includes(t) || app.category && app.category.toLowerCase().includes(t);
    }));
    let displayed = derived(() => filtered().slice(0, displayCount));
    let hasMore = derived(() => displayCount < filtered().length);
    let categories = derived(() => {
      const map = {};
      for (const app of displayed()) {
        const cat = app.category || "Other";
        if (!map[cat]) map[cat] = [];
        map[cat].push(app);
      }
      return Object.keys(map).sort().map((name) => ({ name, apps: map[name] }));
    });
    $$renderer2.push(`<div class="neumorph-container"><div class="logo-card"><h1 class="svelte-12ewbr5">${html("Knitnox".split("").map((ch) => `<span>${ch}</span>`).join(""))}</h1></div> <a href="/" class="back-btn">← Home</a> <h2 style="margin: 16px 0 4px; font-size:1.4rem;">All Apps</h2> <input class="search-box" type="search" placeholder="Search apps…"${attr("value", searchTerm)}/> `);
    if (allApps().length === 0) {
      $$renderer2.push("<!--[0-->");
      $$renderer2.push(`<div class="no-apps" style="padding:40px 0; text-align:center; color:#888;">No apps found.</div>`);
    } else if (displayed().length === 0) {
      $$renderer2.push("<!--[1-->");
      $$renderer2.push(`<div class="no-apps">No apps match your search.</div>`);
    } else {
      $$renderer2.push("<!--[-1-->");
      $$renderer2.push(`<!--[-->`);
      const each_array = ensure_array_like(categories());
      for (let $$index_1 = 0, $$length = each_array.length; $$index_1 < $$length; $$index_1++) {
        let category = each_array[$$index_1];
        $$renderer2.push(`<div class="category-section"><h2 class="category-header">${escape_html(category.name)}</h2> <div class="apps-grid"><!--[-->`);
        const each_array_1 = ensure_array_like(category.apps);
        for (let $$index = 0, $$length2 = each_array_1.length; $$index < $$length2; $$index++) {
          let app = each_array_1[$$index];
          AppCard($$renderer2, { app });
        }
        $$renderer2.push(`<!--]--></div></div>`);
      }
      $$renderer2.push(`<!--]--> `);
      if (hasMore()) {
        $$renderer2.push("<!--[0-->");
        $$renderer2.push(`<div style="text-align:center; padding:20px; color:#888; font-size:0.9rem;">Loading more apps…</div>`);
      } else {
        $$renderer2.push("<!--[-1-->");
      }
      $$renderer2.push(`<!--]-->`);
    }
    $$renderer2.push(`<!--]--> <p class="footer">Knitnox v2.0</p></div>`);
  });
}
export {
  _page as default
};
