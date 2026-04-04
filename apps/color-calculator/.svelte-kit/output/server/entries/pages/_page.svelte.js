import { a0 as attr_style, e as escape_html, a1 as stringify, a2 as attr, $ as derived } from "../../chunks/renderer.js";
function ColorPreview($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let { hex, textColor, textShadow } = $$props;
    $$renderer2.push(`<div class="color-preview svelte-hrg4db"><div class="color-display svelte-hrg4db"${attr_style(`background-color: ${stringify(hex)};`)}></div> <div class="color-text svelte-hrg4db"${attr_style(`color: ${stringify(textColor)}; text-shadow: ${stringify(textShadow)};`)}>${escape_html(hex.toUpperCase())}</div></div>`);
  });
}
function HexInput($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let { value } = $$props;
    $$renderer2.push(`<div class="input-group svelte-19l7hfx"><label for="hex-input" class="svelte-19l7hfx">HEX Color Code</label> <input id="hex-input" type="text" class="input-field svelte-19l7hfx" placeholder="#000000" maxlength="7"${attr("value", value)}/></div>`);
  });
}
function RgbInputs($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let { r, g, b } = $$props;
    $$renderer2.push(`<fieldset class="input-group svelte-n4d0lq"><legend class="svelte-n4d0lq">RGB <span class="small-label svelte-n4d0lq">(0–255)</span></legend> <div class="rgb-grid svelte-n4d0lq"><input type="number" class="input-field svelte-n4d0lq" placeholder="R" min="0" max="255"${attr("value", r)}/> <input type="number" class="input-field svelte-n4d0lq" placeholder="G" min="0" max="255"${attr("value", g)}/> <input type="number" class="input-field svelte-n4d0lq" placeholder="B" min="0" max="255"${attr("value", b)}/></div></fieldset>`);
  });
}
function HslInputs($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let { h, s, l } = $$props;
    $$renderer2.push(`<fieldset class="input-group svelte-1iovmca"><legend class="svelte-1iovmca">HSL <span class="small-label svelte-1iovmca">(H: 0–360, S/L: 0–100)</span></legend> <div class="hsl-grid svelte-1iovmca"><input type="number" class="input-field svelte-1iovmca" placeholder="H" min="0" max="360"${attr("value", h)}/> <input type="number" class="input-field svelte-1iovmca" placeholder="S" min="0" max="100"${attr("value", s)}/> <input type="number" class="input-field svelte-1iovmca" placeholder="L" min="0" max="100"${attr("value", l)}/></div></fieldset>`);
  });
}
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    window.matchMedia("(display-mode: standalone)").matches || window.navigator.standalone === true;
    function hexToRgb(hex2) {
      const m = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(hex2);
      return m ? {
        r: parseInt(m[1], 16),
        g: parseInt(m[2], 16),
        b: parseInt(m[3], 16)
      } : null;
    }
    function rgbToHsl(r2, g2, b2) {
      r2 /= 255;
      g2 /= 255;
      b2 /= 255;
      const max = Math.max(r2, g2, b2), min = Math.min(r2, g2, b2);
      let h2, s2, l2 = (max + min) / 2;
      if (max === min) {
        h2 = s2 = 0;
      } else {
        const d = max - min;
        s2 = l2 > 0.5 ? d / (2 - max - min) : d / (max + min);
        switch (max) {
          case r2:
            h2 = ((g2 - b2) / d + (g2 < b2 ? 6 : 0)) / 6;
            break;
          case g2:
            h2 = ((b2 - r2) / d + 2) / 6;
            break;
          case b2:
            h2 = ((r2 - g2) / d + 4) / 6;
            break;
        }
      }
      return {
        h: Math.round(h2 * 360),
        s: Math.round(s2 * 100),
        l: Math.round(l2 * 100)
      };
    }
    const _savedHex = localStorage.getItem("knitnox-color-calc") || "#5a8dee";
    const _initRgb = hexToRgb(_savedHex) || { r: 90, g: 141, b: 238 };
    const _initHsl = rgbToHsl(_initRgb.r, _initRgb.g, _initRgb.b);
    let hex = _savedHex;
    let r = _initRgb.r;
    let g = _initRgb.g;
    let b = _initRgb.b;
    let h = _initHsl.h;
    let s = _initHsl.s;
    let l = _initHsl.l;
    let copyLabel = "Copy HEX";
    let brightness = derived(() => (r * 299 + g * 587 + b * 114) / 1e3);
    let textColor = derived(() => brightness() > 128 ? "#2a2f3a" : "#ffffff");
    let textShadow = derived(() => brightness() > 128 ? "0 0 5px rgba(255,255,255,0.8), 0 0 10px rgba(255,255,255,0.5)" : "0 0 5px rgba(0,0,0,0.8), 0 0 10px rgba(0,0,0,0.5)");
    $$renderer2.push(`<div class="page svelte-1uha8ag"><div class="calculator svelte-1uha8ag"><div class="header svelte-1uha8ag"><h1 class="svelte-1uha8ag">Color Calculator</h1> <p class="svelte-1uha8ag">HEX • RGB • HSL Converter</p></div> <div class="calc-body svelte-1uha8ag">`);
    ColorPreview($$renderer2, { hex, textColor: textColor(), textShadow: textShadow() });
    $$renderer2.push(`<!----> `);
    HexInput($$renderer2, { value: hex });
    $$renderer2.push(`<!----> `);
    RgbInputs($$renderer2, { r, g, b });
    $$renderer2.push(`<!----> `);
    HslInputs($$renderer2, { h, s, l });
    $$renderer2.push(`<!----> <button class="copy-btn svelte-1uha8ag">${escape_html(copyLabel)}</button></div> <div class="footer svelte-1uha8ag"><a href="/" class="svelte-1uha8ag">← Back to Apps</a></div></div></div> `);
    {
      $$renderer2.push("<!--[-1-->");
    }
    $$renderer2.push(`<!--]-->`);
  });
}
export {
  _page as default
};
