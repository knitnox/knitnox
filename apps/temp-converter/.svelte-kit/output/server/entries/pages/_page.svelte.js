import { l as attr, e as escape_html } from "../../chunks/attributes.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    window.matchMedia("(display-mode: standalone)").matches || window.navigator.standalone === true;
    function cToF(c) {
      return c * 9 / 5 + 32;
    }
    function cToK(c) {
      return c + 273.15;
    }
    function fmt(n) {
      if (n === "" || n === null) return "";
      const v = parseFloat(n);
      if (isNaN(v)) return "";
      return parseFloat(v.toFixed(4)).toString();
    }
    const _savedC = localStorage.getItem("knitnox-temp-converter") ?? "0";
    const _initC = parseFloat(_savedC);
    let celsius = _savedC;
    let fahrenheit = !isNaN(_initC) ? fmt(cToF(_initC)) : "32";
    let kelvin = !isNaN(_initC) ? fmt(cToK(_initC)) : "273.15";
    let copyLabel = "Copy all";
    $$renderer2.push(`<div class="page svelte-1uha8ag"><div class="converter svelte-1uha8ag"><div class="header svelte-1uha8ag"><h1 class="svelte-1uha8ag">Temp Converter</h1> <p class="svelte-1uha8ag">°C • °F • K</p></div> <div class="card svelte-1uha8ag"><div class="field svelte-1uha8ag"><label for="celsius" class="svelte-1uha8ag">Celsius</label> <div class="input-row svelte-1uha8ag"><input id="celsius" type="number" inputmode="decimal"${attr("value", celsius)} class="svelte-1uha8ag"/> <span class="unit svelte-1uha8ag">°C</span></div></div> <div class="divider svelte-1uha8ag"></div> <div class="field svelte-1uha8ag"><label for="fahrenheit" class="svelte-1uha8ag">Fahrenheit</label> <div class="input-row svelte-1uha8ag"><input id="fahrenheit" type="number" inputmode="decimal"${attr("value", fahrenheit)} class="svelte-1uha8ag"/> <span class="unit svelte-1uha8ag">°F</span></div></div> <div class="divider svelte-1uha8ag"></div> <div class="field svelte-1uha8ag"><label for="kelvin" class="svelte-1uha8ag">Kelvin</label> <div class="input-row svelte-1uha8ag"><input id="kelvin" type="number" inputmode="decimal"${attr("value", kelvin)} class="svelte-1uha8ag"/> <span class="unit svelte-1uha8ag">K</span></div></div> <button class="copy-btn svelte-1uha8ag">${escape_html(copyLabel)}</button></div> <div class="footer svelte-1uha8ag"><a href="/" class="svelte-1uha8ag">← Back to Apps</a></div></div></div> `);
    {
      $$renderer2.push("<!--[-1-->");
    }
    $$renderer2.push(`<!--]-->`);
  });
}
export {
  _page as default
};
