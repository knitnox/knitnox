<script>
  let { app, onSelect } = $props();
</script>

<div class="app-card" onclick={() => onSelect(app)} role="button" tabindex="0"
  onkeydown={(e) => e.key === 'Enter' && onSelect(app)}>
  <img src={app.icon} alt={app.name} class="app-icon" />
  <div class="app-info">
    <h3>{app.name}</h3>
    <p>{app.shortDescription}</p>
  </div>
</div>
