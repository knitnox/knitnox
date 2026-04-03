<script>
  let { h, s, l, onchange } = $props();
</script>

<fieldset class="input-group">
  <legend>HSL <span class="small-label">(H: 0–360, S/L: 0–100)</span></legend>
  <div class="hsl-grid">
    <input type="number" class="input-field" placeholder="H" min="0" max="360"
      value={h} oninput={(e) => onchange({ h: +e.target.value, s, l })} />
    <input type="number" class="input-field" placeholder="S" min="0" max="100"
      value={s} oninput={(e) => onchange({ h, s: +e.target.value, l })} />
    <input type="number" class="input-field" placeholder="L" min="0" max="100"
      value={l} oninput={(e) => onchange({ h, s, l: +e.target.value })} />
  </div>
</fieldset>

<style>
  .input-group {
    margin-bottom: 18px;
    border: none;
    padding: 0;
  }

  .input-group legend {
    display: block;
    font-size: 0.85rem;
    font-weight: 600;
    margin-bottom: 8px;
    color: #666;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .small-label {
    font-size: 0.7rem;
    color: #999;
    text-transform: none;
    letter-spacing: 0;
  }

  .hsl-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 10px;
  }

  .input-field {
    width: 100%;
    padding: 14px 16px;
    border-radius: 12px;
    background: var(--bg);
    box-shadow: inset 4px 4px 8px var(--dark),
                inset -4px -4px 8px var(--light);
    border: none;
    font-size: 1rem;
    color: var(--text);
    font-family: 'Segoe UI', Tahoma, sans-serif;
    outline: none;
    text-align: center;
    transition: 0.2s ease;
    box-sizing: border-box;
  }

  .input-field:focus {
    box-shadow: inset 6px 6px 10px var(--dark),
                inset -6px -6px 10px var(--light);
  }
</style>
