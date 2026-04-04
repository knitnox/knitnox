<script>
  import { goto } from '$app/navigation';
  let { app } = $props();
</script>

<div
  class="app-card"
  onclick={() => goto('/app/' + app.id)}
  role="button"
  tabindex="0"
  onkeydown={(e) => e.key === 'Enter' && goto('/app/' + app.id)}
>
  <img src={app.icon} alt={app.name} class="app-icon" />
  <div class="app-info">
    <h3>{app.name}</h3>
    <p>{app.shortDescription}</p>
  </div>
</div>
