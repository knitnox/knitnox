<script>
  let { onNavigate } = $props();
</script>

<div class="neumorph-container">
  <div class="logo-card">
    <h1>{@html 'Knitnox'.split('').map(ch => `<span>${ch}</span>`).join('')}</h1>
    <p>Your offline-first app ecosystem</p>
  </div>

  <div class="card">
    <h3>⚡ Works Offline</h3>
    <p>Every app is cached and available without an internet connection.</p>
  </div>

  <div class="card">
    <h3>📲 Installable Apps</h3>
    <p>Add any app to your home screen for a native-like experience.</p>
  </div>

  <div class="card">
    <h3>🎛️ Real-time Control</h3>
    <p>Instant, responsive interfaces built with the neumorphic design system.</p>
  </div>

  <button
    class="btn"
    id="get-started-btn"
    onclick={() => onNavigate('apps')}
  >
    Get Started →
  </button>

  <p class="footer">Knitnox v2.0 · Built with Svelte 5</p>
</div>

<style>
  h1 {
    margin: 0;
    font-size: 2.6rem;
    font-family: 'Orbitron', sans-serif;
    font-weight: 900;
    letter-spacing: 4px;
    color: var(--text);
  }

  h1 :global(span:nth-child(6)) {
    display: inline-block;
    animation: bulbGlow 2s ease-in-out infinite;
    color: #ffa500;
  }

  @keyframes bulbGlow {
    0%, 100% {
      text-shadow: 0 0 10px rgba(255, 200, 0, 0.4),
                   0 0 20px rgba(255, 165, 0, 0.5),
                   0 0 30px rgba(255, 140, 0, 0.3);
    }
    50% {
      text-shadow: 0 0 20px rgba(255, 200, 0, 0.8),
                   0 0 35px rgba(255, 165, 0, 0.9),
                   0 0 50px rgba(255, 140, 0, 0.6),
                   0 0 70px rgba(255, 120, 0, 0.4);
    }
  }
</style>
