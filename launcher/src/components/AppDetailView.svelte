<script>
  let { app, onNavigate } = $props();

  function openApp() {
    window.open(app.url, '_blank', 'noopener');
  }

  function installApp() {
    window.open(app.url, '_blank', 'noopener');
    alert("App opened! To install, use your browser's 'Add to Home Screen' or install option.");
  }
</script>

<div class="neumorph-container">
  <div class="logo-card">
    <h1>{@html 'Knitnox'.split('').map(ch => `<span>${ch}</span>`).join('')}</h1>
    <p>Your offline-first app ecosystem</p>
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

  <div class="detail-description">
    <p>{app.fullDescription}</p>
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
    <button class="btn action-btn" onclick={installApp}>Install</button>
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
</style>
