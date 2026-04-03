<script>
  let { previous, current } = $props();
</script>

<div class="display">
  <div class="display-previous">{previous}</div>
  <div class="display-current">{current}</div>
</div>

<style>
  .display {
    margin-bottom: 24px;
    padding: 20px;
    border-radius: 14px;
    background: var(--bg);
    box-shadow: inset 6px 6px 12px var(--dark),
                inset -6px -6px 12px var(--light);
    text-align: right;
    min-height: 80px;
    display: flex;
    flex-direction: column;
    justify-content: center;
  }

  .display-previous {
    font-size: 0.9rem;
    color: #888;
    min-height: 20px;
    margin-bottom: 4px;
  }

  .display-current {
    font-size: 2rem;
    font-weight: 600;
    color: var(--text);
    word-break: break-all;
  }
</style>
