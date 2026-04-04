<script>
  let { onNavigate } = $props();
</script>

<div class="neumorph-container">

  <!-- Hero -->
  <div class="logo-card">
    <h1>{@html 'Knitnox'.split('').map(ch => `<span>${ch}</span>`).join('')}</h1>
    <p class="tagline">Your browser is the controller.</p>
    <p class="sub-tagline">Browser-native hardware control for ESP32 &amp; Arduino — plus offline-first tools built for privacy. Zero cloud. Zero latency. Total control.</p>
  </div>

  <!-- Stats Strip -->
  <div class="stats-strip">
    <div class="stat-item">
      <span class="stat-number">Zero</span>
      <span class="stat-label">Servers</span>
    </div>
    <div class="stat-item">
      <span class="stat-number">100%</span>
      <span class="stat-label">Offline</span>
    </div>
    <div class="stat-item">
      <span class="stat-number">All</span>
      <span class="stat-label">Browser APIs</span>
    </div>
    <div class="stat-item">
      <span class="stat-number">∞</span>
      <span class="stat-label">Privacy</span>
    </div>
  </div>

  <!-- Hardware Control -->
  <div class="section-label">🔧 Hardware Control</div>
  <p class="section-sub">Connect. Control. Monitor.</p>
  <p class="section-intro">Pair with embedded hardware directly from the browser using Web Bluetooth, Web Serial, WebUSB, and LAN — no drivers, no middleware, no cloud relay.</p>
  <div class="cards-grid-2">
    <div class="card">
      <h3><svg class="hw-icon" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" aria-hidden="true"><path fill="#0082FC" d="M17.71 7.71L12 2h-1v7.59L6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 11 14.41V22h1l5.71-5.71-4.3-4.29 4.3-4.29zM13 5.83l1.88 1.88L13 9.59V5.83zm1.88 10.46L13 18.17v-3.76l1.88 1.88z"/></svg> Web Bluetooth</h3>
      <p>Pair with ESP32/Arduino BLE peripherals in-browser. No adapters. Raw BLE via the Web Bluetooth API.</p>
    </div>
    <div class="card">
      <h3>🔌 Web Serial</h3>
      <p>Open UART/COM ports directly. Send commands, stream sensor data, and monitor logs in real-time.</p>
    </div>
    <div class="card">
      <h3><svg class="hw-icon" viewBox="0 0 24 24" fill="currentColor" xmlns="http://www.w3.org/2000/svg" aria-hidden="true"><rect x="10" y="19" width="4" height="3" rx="1"/><rect x="11" y="10" width="2" height="9"/><rect x="3" y="9.5" width="18" height="1.5"/><rect x="4.25" y="7.5" width="1.5" height="3"/><rect x="2.5" y="3" width="6" height="5" rx=".5"/><rect x="11" y="5" width="2" height="5.5"/><polygon points="12,1.5 9.5,5.5 14.5,5.5"/><rect x="18.25" y="7" width="1.5" height="4"/><circle cx="19" cy="5" r="2.5"/></svg> WebUSB</h3>
      <p>CDC, HID, and vendor-class USB devices — all supported from the browser. Zero drivers.</p>
    </div>
    <div class="card">
      <h3>📶 Local Network</h3>
      <p>Control Wi-Fi devices via WebSocket &amp; HTTP. Real-time, relay-free, LAN-local.</p>
    </div>
  </div>

  <!-- Privacy-First Apps -->
  <div class="section-label">🔒 Privacy-First Apps</div>
  <p class="section-sub">Built for your eyes only.</p>
  <p class="section-intro">Every utility and productivity tool runs entirely on your device. No accounts, no sync servers, no data collection. Ever.</p>
  <div class="cards-grid-2">
    <div class="card">
      <h3>📴 Fully Offline</h3>
      <p>All apps are cached as PWAs. Use them anywhere, with or without internet. Your device, your data.</p>
    </div>
    <div class="card">
      <h3>🛡️ Zero Telemetry</h3>
      <p>No analytics, no tracking, no accounts. Nothing leaves your browser. Knitnox is a platform you own, not a service you subscribe to.</p>
    </div>
    <div class="card">
      <h3>⚙️ How It Works</h3>
      <p>It uses browser APIs and client-side code. No cloud, no servers, no middleware. Just like your native apps, everything runs locally on your device.</p>
    </div>
    <div class="card">
      <h3>💡 Everything Open</h3>
      <p>All source code is open and auditable. You can inspect, modify, and contribute to the platform. Transparency and control are at the core of Knitnox. <a href="https://github.com/knitnox/knitnox" target="_blank" rel="noopener">GitHub</a></p>
    </div>
  </div>

  <!-- CTA -->
  <button
    class="btn"
    id="get-started-btn"
    onclick={() => onNavigate('apps')}
  >
    Explore Apps →
  </button>

  <p class="footer">Knitnox v2.0 · Local-first · Built by maker · Built for makers.</p>

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
