<script>
  import './lib/neumorphic.css';
  import ColorPreview from './components/ColorPreview.svelte';
  import HexInput from './components/HexInput.svelte';
  import RgbInputs from './components/RgbInputs.svelte';
  import HslInputs from './components/HslInputs.svelte';

  // ── helpers ──────────────────────────────────────────────────────────────
  function hexToRgb(hex) {
    const m = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(hex);
    return m ? { r: parseInt(m[1], 16), g: parseInt(m[2], 16), b: parseInt(m[3], 16) } : null;
  }

  function rgbToHex(r, g, b) {
    return '#' + ((1 << 24) + (r << 16) + (g << 8) + b).toString(16).slice(1);
  }

  function rgbToHsl(r, g, b) {
    r /= 255; g /= 255; b /= 255;
    const max = Math.max(r, g, b), min = Math.min(r, g, b);
    let h, s, l = (max + min) / 2;
    if (max === min) {
      h = s = 0;
    } else {
      const d = max - min;
      s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
      switch (max) {
        case r: h = ((g - b) / d + (g < b ? 6 : 0)) / 6; break;
        case g: h = ((b - r) / d + 2) / 6; break;
        case b: h = ((r - g) / d + 4) / 6; break;
      }
    }
    return { h: Math.round(h * 360), s: Math.round(s * 100), l: Math.round(l * 100) };
  }

  function hslToRgb(h, s, l) {
    h /= 360; s /= 100; l /= 100;
    let r, g, b;
    if (s === 0) {
      r = g = b = l;
    } else {
      const hue2rgb = (p, q, t) => {
        if (t < 0) t += 1;
        if (t > 1) t -= 1;
        if (t < 1/6) return p + (q - p) * 6 * t;
        if (t < 1/2) return q;
        if (t < 2/3) return p + (q - p) * (2/3 - t) * 6;
        return p;
      };
      const q = l < 0.5 ? l * (1 + s) : l + s - l * s;
      const p = 2 * l - q;
      r = hue2rgb(p, q, h + 1/3);
      g = hue2rgb(p, q, h);
      b = hue2rgb(p, q, h - 1/3);
    }
    return { r: Math.round(r * 255), g: Math.round(g * 255), b: Math.round(b * 255) };
  }

  // ── state ─────────────────────────────────────────────────────────────────
  let hex = $state('#5a8dee');
  let r = $state(90);
  let g = $state(141);
  let b = $state(238);
  let h = $state(220);
  let s = $state(80);
  let l = $state(64);
  let copyLabel = $state('Copy HEX');

  let isUpdating = false;

  // ── derived preview ───────────────────────────────────────────────────────
  let brightness = $derived((r * 299 + g * 587 + b * 114) / 1000);
  let textColor = $derived(brightness > 128 ? '#2a2f3a' : '#ffffff');
  let textShadow = $derived(
    brightness > 128
      ? '0 0 5px rgba(255,255,255,0.8), 0 0 10px rgba(255,255,255,0.5)'
      : '0 0 5px rgba(0,0,0,0.8), 0 0 10px rgba(0,0,0,0.5)'
  );

  // ── update handlers ───────────────────────────────────────────────────────
  function fromHex(rawHex) {
    if (isUpdating) return;
    isUpdating = true;
    let value = rawHex.startsWith('#') ? rawHex : '#' + rawHex;
    hex = rawHex; // keep what user typed
    const rgb = hexToRgb(value);
    if (rgb) {
      r = rgb.r; g = rgb.g; b = rgb.b;
      const hsl = rgbToHsl(r, g, b);
      h = hsl.h; s = hsl.s; l = hsl.l;
    }
    isUpdating = false;
  }

  function fromRgb({ r: nr, g: ng, b: nb }) {
    if (isUpdating) return;
    isUpdating = true;
    r = nr; g = ng; b = nb;
    hex = rgbToHex(r, g, b);
    const hsl = rgbToHsl(r, g, b);
    h = hsl.h; s = hsl.s; l = hsl.l;
    isUpdating = false;
  }

  function fromHsl({ h: nh, s: ns, l: nl }) {
    if (isUpdating) return;
    isUpdating = true;
    h = nh; s = ns; l = nl;
    const rgb = hslToRgb(h, s, l);
    r = rgb.r; g = rgb.g; b = rgb.b;
    hex = rgbToHex(r, g, b);
    isUpdating = false;
  }

  async function copyHex() {
    const fullHex = hex.startsWith('#') ? hex : '#' + hex;
    await navigator.clipboard.writeText(fullHex);
    copyLabel = 'Copied!';
    setTimeout(() => { copyLabel = 'Copy HEX'; }, 1500);
  }
</script>

<div class="page">
  <div class="calculator">
    <div class="header">
      <h1>Color Calculator</h1>
      <p>HEX • RGB • HSL Converter</p>
    </div>

    <div class="calc-body">
      <ColorPreview {hex} {textColor} {textShadow} />
      <HexInput value={hex} onchange={fromHex} />
      <RgbInputs {r} {g} {b} onchange={fromRgb} />
      <HslInputs {h} {s} {l} onchange={fromHsl} />
      <button class="copy-btn" onclick={copyHex}>{copyLabel}</button>
    </div>

    <div class="footer">
      <a href="/">← Back to Apps</a>
    </div>
  </div>
</div>

<style>
  :global(*) {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
  }

  :global(:root) {
    --bg: #e6e9ef;
    --light: #ffffff;
    --dark: #c2c8d0;
    --text: #2a2f3a;
  }

  :global(body) {
    font-family: 'Segoe UI', Tahoma, sans-serif;
    background: var(--bg);
    color: var(--text);
  }

  .page {
    display: flex;
    justify-content: center;
    align-items: center;
    min-height: 100vh;
    min-height: 100dvh;
    padding: 20px;
  }

  .calculator {
    max-width: 420px;
    width: 100%;
  }

  .header {
    text-align: center;
    margin-bottom: 24px;
  }

  .header h1 {
    font-size: 1.8rem;
    font-family: 'Orbitron', sans-serif;
    font-weight: 900;
    letter-spacing: 2px;
    color: var(--text);
    margin-bottom: 6px;
  }

  .header p {
    font-size: 0.85rem;
    color: #888;
  }

  .calc-body {
    padding: 24px;
    border-radius: 20px;
    background: var(--bg);
    box-shadow: 10px 10px 20px var(--dark),
                -10px -10px 20px var(--light);
  }

  .copy-btn {
    margin-top: 4px;
    padding: 12px 24px;
    border-radius: 12px;
    background: var(--bg);
    box-shadow: 6px 6px 12px var(--dark),
                -6px -6px 12px var(--light);
    border: none;
    font-size: 0.9rem;
    font-weight: 600;
    color: var(--text);
    cursor: pointer;
    transition: 0.15s ease;
    font-family: 'Segoe UI', Tahoma, sans-serif;
    width: 100%;
    touch-action: manipulation;
  }

  .copy-btn:active {
    box-shadow: inset 4px 4px 8px var(--dark),
                inset -4px -4px 8px var(--light);
  }

  .footer {
    text-align: center;
    margin-top: 20px;
    font-size: 0.8rem;
    color: #888;
  }

  .footer a {
    color: #5a8dee;
    text-decoration: none;
  }

  .footer a:hover {
    text-decoration: underline;
  }
</style>
