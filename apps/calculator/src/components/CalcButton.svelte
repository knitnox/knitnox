<script>
  let { label, type = 'default', span = 1, onpress } = $props();
  let pressed = $state(false);
</script>

<button
  class="btn"
  class:operator={type === 'operator'}
  class:equals={type === 'equals'}
  class:clear={type === 'clear'}
  class:zero={span === 2}
  class:pressed
  style={span === 2 ? 'grid-column: span 2' : ''}
  onpointerdown={() => { pressed = true; }}
  onpointerup={() => { pressed = false; onpress(); }}
  onpointerleave={() => { pressed = false; }}
>
  {label}
</button>

<style>
  .btn {
    padding: 22px;
    border-radius: 14px;
    background: var(--bg);
    box-shadow: 6px 6px 12px var(--dark),
                -6px -6px 12px var(--light);
    border: none;
    font-size: 1.3rem;
    font-weight: 600;
    color: var(--text);
    cursor: pointer;
    transition: 0.15s ease;
    font-family: 'Segoe UI', Tahoma, sans-serif;
    touch-action: manipulation;
  }

  .btn.pressed,
  .btn:active {
    box-shadow: inset 4px 4px 8px var(--dark),
                inset -4px -4px 8px var(--light);
  }

  .btn.operator {
    color: #5a8dee;
    font-weight: 700;
  }

  .btn.clear {
    color: #ff6b6b;
    font-weight: 700;
  }

  .btn.equals {
    background: linear-gradient(145deg, #5a8dee, #4a7bde);
    color: white;
  }

  .btn.equals.pressed,
  .btn.equals:active {
    box-shadow: inset 4px 4px 8px rgba(0,0,0,0.2),
                inset -2px -2px 6px rgba(255,255,255,0.1);
  }
</style>
