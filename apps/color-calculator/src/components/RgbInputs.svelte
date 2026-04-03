<script>
  let { r, g, b, onchange } = $props();
</script>

<fieldset class="input-group">
  <legend>RGB <span class="small-label">(0–255)</span></legend>
  <div class="rgb-grid">
    <input type="number" class="input-field" placeholder="R" min="0" max="255"
      value={r} oninput={(e) => onchange({ r: +e.target.value, g, b })} />
    <input type="number" class="input-field" placeholder="G" min="0" max="255"
      value={g} oninput={(e) => onchange({ r, g: +e.target.value, b })} />
    <input type="number" class="input-field" placeholder="B" min="0" max="255"
      value={b} oninput={(e) => onchange({ r, g, b: +e.target.value })} />
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

  .rgb-grid {
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
