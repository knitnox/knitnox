<script>
  import { marked } from 'marked';

  let { data } = $props();

  let app = $derived(data.app);
  let descriptionHtml = $derived(marked.parse(app.fullDescription ?? ''));
  let showInstallGuide = $state(false);

  let browser = $derived.by(() => {
    const ua = navigator.userAgent;
    if (/safari/i.test(ua) && !/chrome/i.test(ua) && !/android/i.test(ua)) return 'safari-ios';
    if (/firefox/i.test(ua)) return 'firefox';
    if (/samsung/i.test(ua)) return 'samsung';
    return 'chromium';
  });

  function openApp() {
    window.open(app.url, '_blank', 'noopener');
  }
</script>

<div class="neumorph-container">
  <div class="logo-card">
    <h1>{@html 'Knitnox'.split('').map(ch => `<span>${ch}</span>`).join('')}</h1>
  </div>
  <a href="/apps" class="back-btn">← Apps</a>

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
    <button class="btn action-btn" onclick={() => showInstallGuide = true}>+ Install</button>
  </div>

  <p class="footer">Knitnox v2.0</p>
</div>

<!-- Install Guide Modal -->
{#if showInstallGuide}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <div class="modal-backdrop" onclick={() => showInstallGuide = false} role="button" tabindex="0">
    <div class="modal-card" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-labelledby="install-guide-title" tabindex="-1">
      <h3 id="install-guide-title">📲 Install {app.name}</h3>
      <p class="modal-sub">Open the app first, then install it using your browser:</p>

      {#if browser === 'safari-ios'}
        <ol class="install-steps">
          <li>Tap the <strong>Share</strong> button <span class="icon-hint">⎋</span> at the bottom of Safari</li>
          <li>Scroll down and tap <strong>"Add to Home Screen"</strong></li>
          <li>Tap <strong>Add</strong> in the top-right corner</li>
        </ol>
      {:else if browser === 'firefox'}
        <ol class="install-steps">
          <li>Tap the <strong>⋮ menu</strong> in the top-right</li>
          <li>Tap <strong>"Install"</strong> or <strong>"Add to Home Screen"</strong></li>
          <li>Confirm the prompt</li>
        </ol>
      {:else if browser === 'samsung'}
        <ol class="install-steps">
          <li>Tap the <strong>⋮ menu</strong></li>
          <li>Tap <strong>"Add page to"</strong> → <strong>"Home screen"</strong></li>
          <li>Tap <strong>Add</strong></li>
        </ol>
      {:else}
        <ol class="install-steps">
          <li>Open <strong>{app.name}</strong> using the button below</li>
          <li>Look for the <strong>install icon</strong> <span class="icon-hint">⊕</span> in the browser address bar</li>
          <li>Click it and confirm — the app installs instantly</li>
        </ol>
        <p class="modal-tip">💡 On mobile Chrome: tap the <strong>⋮ menu</strong> → <strong>"Add to Home Screen"</strong></p>
      {/if}

      <div class="modal-actions">
        <button class="btn action-btn" onclick={() => { showInstallGuide = false; openApp(); }}>Open App →</button>
        <button class="btn action-btn modal-close-btn" onclick={() => showInstallGuide = false}>Close</button>
      </div>
    </div>
  </div>
{/if}

<style>
  h1 {
    margin: 0;
    font-size: 2.6rem;
    font-family: 'Orbitron', sans-serif;
    font-weight: 900;
    letter-spacing: 4px;
    color: var(--text);
  }

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

  /* Modal */
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.35);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
    padding: 20px;
  }

  .modal-card {
    background: var(--bg, #e6e9ef);
    border-radius: 20px;
    padding: 28px 24px;
    max-width: 380px;
    width: 100%;
    box-shadow: 10px 10px 20px var(--dark, #c2c8d0),
                -10px -10px 20px var(--light, #ffffff);
  }

  .modal-card h3 {
    margin: 0 0 6px;
    font-size: 1.15rem;
    font-weight: 700;
    color: var(--text, #2a2f3a);
  }

  .modal-sub {
    font-size: 0.88rem;
    color: #666;
    margin: 0 0 16px;
    text-align: left;
  }

  .install-steps {
    padding-left: 1.3em;
    margin: 0 0 14px;
  }

  .install-steps li {
    font-size: 0.9rem;
    color: #444;
    margin: 8px 0;
    line-height: 1.5;
  }

  .icon-hint {
    display: inline-block;
    font-size: 1rem;
    color: #ffa500;
  }

  .modal-tip {
    font-size: 0.83rem;
    color: #888;
    margin: 0 0 16px;
    text-align: left;
  }

  .modal-actions {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
    margin-top: 4px;
  }

  .modal-close-btn {
    font-weight: 600;
    color: #888;
  }
</style>
