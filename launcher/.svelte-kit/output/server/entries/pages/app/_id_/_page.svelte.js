import { a as attr, e as escape_html, b as ensure_array_like, s as stringify, d as derived } from "../../../../chunks/renderer.js";
import { marked } from "marked";
import { h as html } from "../../../../chunks/html.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let { data } = $$props;
    let app = derived(() => data.app);
    let descriptionHtml = derived(() => marked.parse(app().fullDescription ?? ""));
    $$renderer2.push(`<div class="neumorph-container"><div class="logo-card"><h1 class="svelte-lhrd3w">${html("Knitnox".split("").map((ch) => `<span>${ch}</span>`).join(""))}</h1></div> <a href="/apps" class="back-btn">← Apps</a> <div class="detail-header"><img${attr("src", app().icon)}${attr("alt", app().name)} class="detail-icon"/> <div class="detail-title"><h2>${escape_html(app().name)}</h2> <p>${escape_html(app().shortDescription)}</p></div></div> <div class="detail-description md-prose">${html(descriptionHtml())}</div> `);
    if (app().screenshots && app().screenshots.length > 0) {
      $$renderer2.push("<!--[0-->");
      $$renderer2.push(`<div class="screenshots"><!--[-->`);
      const each_array = ensure_array_like(app().screenshots);
      for (let $$index = 0, $$length = each_array.length; $$index < $$length; $$index++) {
        let src = each_array[$$index];
        $$renderer2.push(`<img${attr("src", src)}${attr("alt", `${stringify(app().name)} screenshot`)}/>`);
      }
      $$renderer2.push(`<!--]--></div>`);
    } else {
      $$renderer2.push("<!--[-1-->");
    }
    $$renderer2.push(`<!--]--> <div class="action-buttons svelte-lhrd3w"><button class="btn action-btn svelte-lhrd3w">Open App</button> <button class="btn action-btn svelte-lhrd3w">+ Install</button></div> <p class="footer">Knitnox v2.0</p></div> `);
    {
      $$renderer2.push("<!--[-1-->");
    }
    $$renderer2.push(`<!--]-->`);
  });
}
export {
  _page as default
};
