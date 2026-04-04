<script>
  import { marked } from 'marked';

  let { app, onNavigate } = $props();

  let descriptionHtml = $derived(marked.parse(app.fullDescription ?? ''));

  let installed = $state(false);

  $effect(() => {
    // Re-evaluate whenever app changes
    app.id;
    installed = localStorage.getItem(`knitnox-app-${app.id}`) === '1';
  });

  function openApp() {
    window.open(app.url, '_blank', 'noopener');
  }

  function installApp() {
    window.open(app.url, '_blank', 'noopener');
    // Optimistically mark installed; confirmed when app runs in standalone and writes its own key
    try { localStorage.setItem(`knitnox-app-${app.id}`, '1'); } catch {}
    installed = true;
  }
</script>

<div class="neumorph-container">
  <div class="logo-card">
    <h1>{@html 'Knitnox'.split('').map(ch => `<span>${ch}</span>`).join('')}</h1>
  </div>
  <a
    href="/apps"
    class="back-btn"
    onclick={(e) => { e.preventDefault(); onNavigate('apps'); }}
  >← Apps</a>

  <div class="detail-header">
    <img src={app.icon} alt={app.name} class="detail-icon" />
    <div class="detail-title">
      <h2>{app.name}</h2>
      <p>{app.shortDescription}</p>
    </div>
  </div>

  <div class="detail-description md-prose">
    {@html descriptionHtml}
  </div>

  {#if app.screenshots && app.screenshots.length > 0}
    <div class="screenshots">
      {#each app.screenshots as src}
        <img {src} alt="{app.name} screenshot" />
      {/each}
    </div>
  {/if}

  <div class="action-buttons">
    <button class="btn action-btn" onclick={openApp}>Open App</button>
    {#if installed}
      <button class="btn action-btn installed-btn" disabled>✓ Installed</button>
    {:else}
      <button class="btn action-btn" onclick={installApp}>+ Install</button>
    {/if}
  </div>

  <p class="footer">Knitnox v2.0</p>
</div>

<style>
  .action-buttons {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
    margin-top: 20px;
  }

  .action-btn {
    width: 100%;
    padding: 16px;
    text-align: center;
    box-sizing: border-box;
  }

  .installed-btn {
    color: #4caf50;
    font-weight: 700;
    cursor: default;
    opacity: 0.85;
  }
</style>
