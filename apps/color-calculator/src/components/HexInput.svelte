<script>
  let { value, onchange } = $props();
</script>

<div class="input-group">
  <label for="hex-input">HEX Color Code</label>
  <input
    id="hex-input"
    type="text"
    class="input-field"
    placeholder="#000000"
    maxlength="7"
    value={value}
    oninput={(e) => onchange(e.target.value)}
  />
</div>

<style>
  .input-group {
    margin-bottom: 18px;
  }

  .input-group label {
    display: block;
    font-size: 0.85rem;
    font-weight: 600;
    margin-bottom: 8px;
    color: #666;
    text-transform: uppercase;
    letter-spacing: 0.5px;
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
    transition: 0.2s ease;
    box-sizing: border-box;
  }

  .input-field:focus {
    box-shadow: inset 6px 6px 10px var(--dark),
                inset -6px -6px 10px var(--light);
  }
</style>
