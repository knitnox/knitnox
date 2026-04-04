<script>
  import './lib/neumorphic.css';

  // ── PWA install prompt ────────────────────────────────────────────────────
  let deferredPrompt = $state(null);
  let isStandalone = $state(
    window.matchMedia('(display-mode: standalone)').matches ||
    window.navigator.standalone === true
  );

  $effect(() => {
    function onBeforeInstall(e) {
      e.preventDefault();
      deferredPrompt = e;
    }
    window.addEventListener('beforeinstallprompt', onBeforeInstall);
    window.addEventListener('appinstalled', () => { deferredPrompt = null; isStandalone = true; });
    return () => window.removeEventListener('beforeinstallprompt', onBeforeInstall);
  });

  async function installApp() {
    if (!deferredPrompt) return;
    deferredPrompt.prompt();
    const { outcome } = await deferredPrompt.userChoice;
    if (outcome === 'accepted') { deferredPrompt = null; isStandalone = true; }
  }

  // ── conversion helpers ────────────────────────────────────────────────────
  function cToF(c) { return (c * 9) / 5 + 32; }
  function cToK(c) { return c + 273.15; }
  function fToC(f) { return ((f - 32) * 5) / 9; }
  function kToC(k) { return k - 273.15; }

  function fmt(n) {
    if (n === '' || n === null) return '';
    const v = parseFloat(n);
    if (isNaN(v)) return '';
    // Up to 4 decimal places, strip trailing zeros
    return parseFloat(v.toFixed(4)).toString();
  }

  // ── state ─────────────────────────────────────────────────────────────────
  const _savedC = localStorage.getItem('knitnox-temp-converter') ?? '0';
  const _initC = parseFloat(_savedC);
  let celsius    = $state(_savedC);
  let fahrenheit = $state(!isNaN(_initC) ? fmt(cToF(_initC)) : '32');
  let kelvin     = $state(!isNaN(_initC) ? fmt(cToK(_initC)) : '273.15');

  $effect(() => {
    localStorage.setItem('knitnox-temp-converter', celsius);
  });

  let updating = false;

  function fromCelsius(val) {
    if (updating) return;
    updating = true;
    celsius = val;
    const c = parseFloat(val);
    if (!isNaN(c)) {
      fahrenheit = fmt(cToF(c));
      kelvin     = fmt(cToK(c));
    } else {
      fahrenheit = '';
      kelvin     = '';
    }
    updating = false;
  }

  function fromFahrenheit(val) {
    if (updating) return;
    updating = true;
    fahrenheit = val;
    const f = parseFloat(val);
    if (!isNaN(f)) {
      const c = fToC(f);
      celsius = fmt(c);
      kelvin  = fmt(cToK(c));
    } else {
      celsius = '';
      kelvin  = '';
    }
    updating = false;
  }

  function fromKelvin(val) {
    if (updating) return;
    updating = true;
    kelvin = val;
    const k = parseFloat(val);
    if (!isNaN(k)) {
      const c = kToC(k);
      celsius    = fmt(c);
      fahrenheit = fmt(cToF(c));
    } else {
      celsius    = '';
      fahrenheit = '';
    }
    updating = false;
  }

  // ── copy helpers ───────────────────────────────────────────────────────────
  let copyLabel = $state('Copy all');

  async function copyAll() {
    const text = `${celsius} °C  =  ${fahrenheit} °F  =  ${kelvin} K`;
    await navigator.clipboard.writeText(text);
    copyLabel = 'Copied!';
    setTimeout(() => { copyLabel = 'Copy all'; }, 1500);
  }
</script>

<div class="page">
  <div class="converter">
    <div class="header">
      <h1>Temp Converter</h1>
      <p>°C &bull; °F &bull; K</p>
    </div>

    <div class="card">
      <div class="field">
        <label for="celsius">Celsius</label>
        <div class="input-row">
          <input
            id="celsius"
            type="number"
            inputmode="decimal"
            value={celsius}
            oninput={(e) => fromCelsius(e.target.value)}
          />
          <span class="unit">°C</span>
        </div>
      </div>

      <div class="divider"></div>

      <div class="field">
        <label for="fahrenheit">Fahrenheit</label>
        <div class="input-row">
          <input
            id="fahrenheit"
            type="number"
            inputmode="decimal"
            value={fahrenheit}
            oninput={(e) => fromFahrenheit(e.target.value)}
          />
          <span class="unit">°F</span>
        </div>
      </div>

      <div class="divider"></div>

      <div class="field">
        <label for="kelvin">Kelvin</label>
        <div class="input-row">
          <input
            id="kelvin"
            type="number"
            inputmode="decimal"
            value={kelvin}
            oninput={(e) => fromKelvin(e.target.value)}
          />
          <span class="unit">K</span>
        </div>
      </div>

      <button class="copy-btn" onclick={copyAll}>{copyLabel}</button>
    </div>

    <div class="footer">
      <a href="/">← Back to Apps</a>
    </div>
  </div>
</div>

<!-- Floating Install Button (only when not installed and prompt is available) -->
{#if !isStandalone && deferredPrompt}
  <button class="pwa-install-fab" onclick={installApp} aria-label="Install app">
    ⊕ Install App
  </button>
{/if}

<style>
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

  .converter {
    max-width: 420px;
    width: 100%;
  }

  /* ── header ─────────────────────────────────────────────────────────────── */
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

  /* ── main card ───────────────────────────────────────────────────────────── */
  .card {
    padding: 28px 24px;
    border-radius: 20px;
    background: var(--bg);
    box-shadow: 10px 10px 20px var(--dark), -10px -10px 20px var(--light);
    display: flex;
    flex-direction: column;
    gap: 0;
  }

  /* ── fields ─────────────────────────────────────────────────────────────── */
  .field {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 16px 0;
  }

  label {
    font-size: 0.78rem;
    font-weight: 600;
    color: #888;
    letter-spacing: 0.12em;
    text-transform: uppercase;
  }

  .input-row {
    display: flex;
    align-items: center;
    gap: 10px;
    border-radius: 12px;
    background: var(--bg);
    box-shadow: inset 4px 4px 8px var(--dark), inset -4px -4px 8px var(--light);
    padding: 10px 14px;
  }

  input[type='number'] {
    flex: 1;
    background: transparent;
    border: none;
    outline: none;
    font-size: 1.4rem;
    font-weight: 600;
    color: var(--text);
    font-family: 'Segoe UI', Tahoma, sans-serif;
    -moz-appearance: textfield;
    appearance: textfield;
    min-width: 0;
  }

  input[type='number']::-webkit-outer-spin-button,
  input[type='number']::-webkit-inner-spin-button {
    -webkit-appearance: none;
  }

  .unit {
    font-size: 1rem;
    font-weight: 700;
    color: var(--accent);
    flex-shrink: 0;
    min-width: 28px;
    text-align: right;
  }

  .divider {
    height: 1px;
    background: linear-gradient(to right, transparent, var(--dark), transparent);
    opacity: 0.5;
  }

  /* ── copy button ─────────────────────────────────────────────────────────── */
  .copy-btn {
    margin-top: 20px;
    padding: 14px;
    border-radius: 12px;
    background: var(--bg);
    box-shadow: 6px 6px 12px var(--dark), -6px -6px 12px var(--light);
    border: none;
    font-size: 0.9rem;
    font-weight: 600;
    color: var(--text);
    cursor: pointer;
    transition: box-shadow 0.15s ease, transform 0.1s ease;
    font-family: 'Segoe UI', Tahoma, sans-serif;
    width: 100%;
    touch-action: manipulation;
  }

  .copy-btn:active {
    box-shadow: inset 4px 4px 8px var(--dark), inset -4px -4px 8px var(--light);
    transform: scale(0.98);
  }

  /* ── footer ─────────────────────────────────────────────────────────────── */
  .footer {
    text-align: center;
    margin-top: 20px;
  }

  .footer a {
    color: #8a9ab0;
    text-decoration: none;
    font-size: 0.85rem;
    font-weight: 600;
    transition: color 0.15s ease;
  }

  .footer a:hover {
    color: var(--accent);
  }
</style>
