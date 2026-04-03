<script>
  let { hex, textColor, textShadow } = $props();
</script>

<div class="color-preview">
  <div class="color-display" style="background-color: {hex};"></div>
  <div class="color-text" style="color: {textColor}; text-shadow: {textShadow};">
    {hex.toUpperCase()}
  </div>
</div>

<style>
  .color-preview {
    margin-bottom: 24px;
    padding: 60px 20px;
    border-radius: 16px;
    background: var(--bg);
    box-shadow: inset 6px 6px 12px var(--dark),
                inset -6px -6px 12px var(--light);
    text-align: center;
    position: relative;
    overflow: hidden;
  }

  .color-display {
    position: absolute;
    inset: 0;
    border-radius: 16px;
    transition: background-color 0.3s ease;
  }

  .color-text {
    position: relative;
    z-index: 1;
    font-size: 1.2rem;
    font-weight: 600;
  }
</style>
