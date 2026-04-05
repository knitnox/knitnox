<div class="neumorph-container">

  <!-- Hero -->
  <div class="logo-card">
    <h1>{@html 'Knitnox'.split('').map(ch => `<span>${ch}</span>`).join('')}</h1>
    <!-- <p class="tagline">Your browser is the controller.</p> -->
    <p class="sub-tagline">zero cloud dependacy, zero latency.</p>
    <p class="sub-tagline"><small> 🛠️This App is Under Construction</small></p>
  </div>

  <!-- Stats Strip -->
  <div class="stats-strip">
    <div class="stat-item">
      <span class="stat-number">0</span>
      <span class="stat-label">Servers</span>
    </div>
    <div class="stat-item">
      <span class="stat-number">100%</span>
      <span class="stat-label">Local-First</span>
    </div>
    <div class="stat-item">
      <span class="stat-number">n+</span>
      <span class="stat-label">Browser APIs</span>
    </div>
    <div class="stat-item">
      <span class="stat-number">∞</span>
      <span class="stat-label">Privacy</span>
    </div>
  </div>

  <!-- Hardware Control -->
  <div class="card feature-card">
    <div class="feature-header">
      <span class="feature-badge">🔧</span>
      <div>
        <h2 class="feature-title">Hardware Control</h2>
        <p class="feature-desc">Talk to your Raspberry Pi, ESP32, and any other hardware directly from your browser over WebUSB, WebSerial, WebBluetooth or local network; no drivers, no middleware, no cloud relay.</p>
      </div>
    </div>
    <div class="hw-svg-wrap">
      <!-- Dual-node diagram: phone → two signal paths → ESP32/Arduino (top) + Raspberry Pi (bottom) -->
      <svg viewBox="0 0 260 130" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" class="hw-diagram">
        <defs>
          <marker id="hw-arr" markerWidth="6" markerHeight="6" refX="5" refY="3" orient="auto">
            <path d="M0,0 L6,3 L0,6 Z" fill="currentColor" opacity="0.45"/>
          </marker>
          <marker id="hw-arr-blue" markerWidth="6" markerHeight="6" refX="5" refY="3" orient="auto">
            <path d="M0,0 L6,3 L0,6 Z" fill="#5a8dee"/>
          </marker>
          <marker id="hw-arr-red" markerWidth="6" markerHeight="6" refX="5" refY="3" orient="auto">
            <path d="M0,0 L6,3 L0,6 Z" fill="#c0392b"/>
          </marker>
        </defs>

        <!-- ── Phone (left, vertically centred) ── -->
        <rect x="10" y="22" width="42" height="72" rx="7" fill="none" stroke="currentColor" stroke-width="2" opacity="0.7"/>
        <rect x="10" y="22" width="42" height="11" rx="7" fill="currentColor" opacity="0.07"/>
        <rect x="10" y="29" width="42" height="4" fill="currentColor" opacity="0.07"/>
        <rect x="22" y="26" width="14" height="3" rx="1.5" fill="currentColor" opacity="0.3"/>
        <circle cx="45" cy="27.5" r="1.5" fill="currentColor" opacity="0.25"/>
        <rect x="14" y="37" width="34" height="3" rx="1.5" fill="currentColor" opacity="0.2"/>
        <rect x="14" y="43" width="26" height="2.5" rx="1.25" fill="currentColor" opacity="0.14"/>
        <rect x="14" y="49" width="30" height="2.5" rx="1.25" fill="currentColor" opacity="0.14"/>
        <rect x="14" y="55" width="20" height="2.5" rx="1.25" fill="currentColor" opacity="0.11"/>
        <rect x="14" y="61" width="34" height="20" rx="3" fill="currentColor" opacity="0.05" stroke="currentColor" stroke-width="0.8"/>
        <rect x="18" y="65" width="26" height="2" rx="1" fill="currentColor" opacity="0.12"/>
        <rect x="18" y="70" width="18" height="2" rx="1" fill="currentColor" opacity="0.09"/>
        <rect x="22" y="87" width="18" height="2.5" rx="1.25" fill="currentColor" opacity="0.22"/>
        <rect x="8" y="38" width="2" height="7" rx="1" fill="currentColor" opacity="0.3"/>
        <rect x="8" y="48" width="2" height="7" rx="1" fill="currentColor" opacity="0.3"/>
        <rect x="52" y="42" width="2" height="9" rx="1" fill="currentColor" opacity="0.3"/>

        <!-- ── Branch split line from phone ── -->
        <line x1="54" y1="58" x2="90" y2="58" stroke="currentColor" stroke-width="1.5" opacity="0.35"/>
        <line x1="90" y1="58" x2="90" y2="28" stroke="currentColor" stroke-width="1.5" opacity="0.35"/>
        <line x1="90" y1="58" x2="90" y2="98" stroke="currentColor" stroke-width="1.5" opacity="0.35"/>

        <!-- ── Waves + arrow → ESP32/Arduino (top node) ── -->
        <line x1="90" y1="28" x2="116" y2="28" stroke="currentColor" stroke-width="1.5" opacity="0.35" marker-end="url(#hw-arr)"/>
        <circle cx="122" cy="28" r="2.3" fill="currentColor"/>
        <path d="M 122 22 A 6 6 0 0 1 122 34" fill="none" stroke="currentColor" stroke-width="0.38" stroke-linecap="round"/>
        <path d="M 122 17 A 11 11 0 0 1 122 39" fill="none" stroke="currentColor" stroke-width="0.38" stroke-linecap="round" opacity="0.65"/>
        <line x1="134" y1="28" x2="156" y2="28" stroke="currentColor" stroke-width="0.38" opacity="0.38" marker-end="url(#hw-arr-blue)"/>
        <!-- ESP32/Arduino chip -->
        <rect x="158" y="8" width="44" height="38" rx="4" fill="none" stroke="currentColor" stroke-width="0.38" opacity="0.38"/>
        <rect x="164" y="14" width="32" height="26" rx="2" fill="currentColor" opacity="0.05"/>
        <rect x="164" y="14" width="32" height="26" rx="2" fill="none" stroke="currentColor" stroke-width="0.38" opacity="0.18"/>
        <rect x="150" y="16" width="8" height="2.5" rx="1" fill="currentColor" opacity="0.38"/>
        <rect x="150" y="22" width="8" height="2.5" rx="1" fill="currentColor" opacity="0.38"/>
        <rect x="150" y="28" width="8" height="2.5" rx="1" fill="currentColor" opacity="0.38"/>
        <rect x="150" y="34" width="8" height="2.5" rx="1" fill="currentColor" opacity="0.38"/>
        <rect x="202" y="16" width="8" height="2.5" rx="1" fill="currentColor" opacity="0.38"/>
        <rect x="202" y="22" width="8" height="2.5" rx="1" fill="currentColor" opacity="0.38"/>
        <rect x="202" y="28" width="8" height="2.5" rx="1" fill="currentColor" opacity="0.38"/>
        <rect x="202" y="34" width="8" height="2.5" rx="1" fill="currentColor" opacity="0.38"/>
        <text x="180" y="26" font-size="6.5" font-family="monospace" fill="currentColor" opacity="0.38" text-anchor="middle">ESP32</text>
        <text x="180" y="35" font-size="5" font-family="monospace" fill="currentColor" opacity="0.38" text-anchor="middle">Arduino</text>

        <!-- ── Waves + arrow → Raspberry Pi (bottom node) ── -->
        <line x1="90" y1="98" x2="116" y2="98" stroke="currentColor" stroke-width="1.5" opacity="0.38" marker-end="url(#hw-arr)"/>
        <circle cx="122" cy="98" r="2.3" fill="currentColor"/>
        <path d="M 122 92 A 6 6 0 0 1 122 104" fill="none" stroke="currentColor" stroke-width="0.38" stroke-linecap="round"/>
        <path d="M 122 87 A 11 11 0 0 1 122 109" fill="none" stroke="currentColor" stroke-width="0.38" stroke-linecap="round" opacity="0.65"/>
        <line x1="134" y1="98" x2="156" y2="98" stroke="currentColor" stroke-width="0.38" opacity="0.38" marker-end="url(#hw-arr-red)"/>
        <!-- Raspberry Pi board -->
        <rect x="158" y="80" width="62" height="38" rx="4" fill="none" stroke="currentColor" stroke-width="0.38" opacity="0.38"/>
        <rect x="172" y="88" width="18" height="18" rx="2" fill="currentColor" opacity="0.08"/>
        <rect x="172" y="88" width="18" height="18" rx="2" fill="none" stroke="currentColor" stroke-width="0.38" opacity="0.2"/>
        <text x="181" y="99" font-size="4.5" font-family="monospace" fill="currentColor" opacity="0.5" text-anchor="middle">CPU</text>
        <rect x="196" y="84" width="5" height="2.5" rx="0.8" fill="currentColor" opacity="0.35"/>
        <rect x="196" y="89" width="5" height="2.5" rx="0.8" fill="currentColor" opacity="0.35"/>
        <rect x="196" y="94" width="5" height="2.5" rx="0.8" fill="currentColor" opacity="0.35"/>
        <rect x="196" y="99" width="5" height="2.5" rx="0.8" fill="currentColor" opacity="0.35"/>
        <rect x="196" y="104" width="5" height="2.5" rx="0.8" fill="currentColor" opacity="0.35"/>
        <rect x="203" y="84" width="5" height="2.5" rx="0.8" fill="currentColor" opacity="0.35"/>
        <rect x="203" y="89" width="5" height="2.5" rx="0.8" fill="currentColor" opacity="0.35"/>
        <rect x="203" y="94" width="5" height="2.5" rx="0.8" fill="currentColor" opacity="0.35"/>
        <rect x="203" y="99" width="5" height="2.5" rx="0.8" fill="currentColor" opacity="0.35"/>
        <rect x="203" y="104" width="5" height="2.5" rx="0.8" fill="currentColor" opacity="0.35"/>
        <rect x="158" y="86" width="6" height="5" rx="1" fill="none" stroke="currentColor" stroke-width="0.38" opacity="0.3"/>
        <rect x="158" y="94" width="6" height="5" rx="1" fill="none" stroke="currentColor" stroke-width="0.38" opacity="0.3"/>
        <text x="165" y="114" font-size="5.5" font-family="monospace" fill="currentColor" opacity="0.45" text-anchor="start">Raspberry Pi</text>
      </svg>
    </div>
    <div class="pill-row">
      <span class="pill">
        <svg viewBox="0 0 24 24" width="11" height="11" aria-hidden="true"><path fill="currentColor" d="M17.71 7.71L12 2h-1v7.59L6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 11 14.41V22h1l5.71-5.71-4.3-4.29 4.3-4.29z"/></svg>
        <span>Bluetooth</span>
      </span>
      <span class="pill">🔌 Serial</span>
      <span class="pill">
        <svg viewBox="0 0 24 24" width="11" height="11" aria-hidden="true" style="fill:currentColor;flex-shrink:0"><rect x="10" y="19" width="4" height="3" rx="1"/><rect x="11" y="10" width="2" height="9"/><rect x="3" y="9.5" width="18" height="1.5"/><rect x="4.25" y="7.5" width="1.5" height="3"/><rect x="2.5" y="3" width="6" height="5" rx=".5"/><rect x="11" y="5" width="2" height="5.5"/><polygon points="12,1.5 9.5,5.5 14.5,5.5"/><rect x="18.25" y="7" width="1.5" height="4"/><circle cx="19" cy="5" r="2.5"/></svg>
        WebUSB
      </span>
      <span class="pill">⌔ LAN/WiFi</span>
    </div>
  </div>

  <!-- PWA App Collection -->
  <div class="card feature-card">
    <div class="feature-header">
      <span class="feature-badge">📲</span>
      <div>
        <h2 class="feature-title">Local-First App Collection</h2>
        <p class="feature-desc">Lightweight PWAs that run entirely on your device or local network; no cloud installation, no store approval, no internet required after first load. Utility tools work fully offline; hardware tools talk directly to devices on your LAN.</p>
      </div>
    </div>
    <div class="hw-svg-wrap">
      <svg viewBox="0 0 200 110" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" class="hw-diagram">
        <!-- Phone shell -->
        <rect x="70" y="4" width="60" height="102" rx="9" fill="none" stroke="currentColor" stroke-width="2" opacity="0.65"/>
        <rect x="70" y="4" width="60" height="14" rx="9" fill="currentColor" opacity="0.07"/>
        <rect x="70" y="14" width="60" height="4" fill="currentColor" opacity="0.06"/>
        <rect x="84" y="8" width="18" height="3" rx="1.5" fill="currentColor" opacity="0.28"/>
        <circle cx="119" cy="9.5" r="1.5" fill="currentColor" opacity="0.22"/>
        <rect x="88" y="97" width="24" height="3" rx="1.5" fill="currentColor" opacity="0.22"/>
        <rect x="68" y="32" width="2" height="8" rx="1" fill="currentColor" opacity="0.28"/>
        <rect x="68" y="43" width="2" height="8" rx="1" fill="currentColor" opacity="0.28"/>
        <rect x="130" y="36" width="2" height="10" rx="1" fill="currentColor" opacity="0.28"/>
        <!-- App grid: 3×2 icons inside screen -->
        <rect x="76" y="22" width="14" height="14" rx="3" fill="#5a8dee" opacity="0.55"/>
        <rect x="93" y="22" width="14" height="14" rx="3" fill="#51cf66" opacity="0.55"/>
        <rect x="110" y="22" width="14" height="14" rx="3" fill="#ffa500" opacity="0.55"/>
        <rect x="76" y="40" width="14" height="14" rx="3" fill="#ffa500" opacity="0.45"/>
        <rect x="93" y="40" width="14" height="14" rx="3" fill="#5a8dee" opacity="0.45"/>
        <rect x="110" y="40" width="14" height="14" rx="3" fill="#51cf66" opacity="0.45"/>
        <!-- Row 3 (partial / incoming) -->
        <rect x="76" y="58" width="14" height="14" rx="3" fill="currentColor" opacity="0.1" stroke="currentColor" stroke-width="0.8"/>
        <rect x="93" y="58" width="14" height="14" rx="3" fill="currentColor" opacity="0.1" stroke="currentColor" stroke-width="0.8"/>
        <rect x="110" y="58" width="14" height="14" rx="3" fill="currentColor" opacity="0.1" stroke="currentColor" stroke-width="0.8"/>
        <!-- Down-arrow install badge -->
        <line x1="100" y1="61" x2="100" y2="68" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
        <polyline points="97,66 100,70 103,66" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
        <!-- Cloud source (left) -->
        <ellipse cx="22" cy="42" rx="14" ry="10" fill="none" stroke="currentColor" stroke-width="1.5" opacity="0.4"/>
        <ellipse cx="15" cy="47" rx="9" ry="6" fill="none" stroke="currentColor" stroke-width="1.5" opacity="0.35"/>
        <ellipse cx="30" cy="47" rx="9" ry="6" fill="none" stroke="currentColor" stroke-width="1.5" opacity="0.35"/>
        <text x="22" y="46" font-size="5" font-family="monospace" fill="currentColor" opacity="0.45" text-anchor="middle">PWA</text>
        <!-- Arrow cloud → phone -->
        <line x1="38" y1="50" x2="66" y2="58" stroke="currentColor" stroke-width="1.5" opacity="0.35" stroke-dasharray="4 3"/>
        <polygon points="66,55 70,59 64,61" fill="currentColor" opacity="0.3"/>
        <!-- Cache badge -->
        <rect x="140" y="46" width="46" height="18" rx="5" fill="none" stroke="currentColor" stroke-width="1.5" opacity="0.6"/>
        <text x="163" y="58" font-size="6" font-family="monospace" fill="currentColor" opacity="0.75" text-anchor="middle">CACHED</text>
        <line x1="132" y1="55" x2="140" y2="55" stroke="currentColor" stroke-width="1.5" opacity="0.5" stroke-dasharray="3 2"/>
      </svg>
    </div>
    <ul class="privacy-list">
      <li>
        <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path fill="#51cf66" d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41L9 16.17z"/></svg>
        <span><strong>Utilities</strong> — calculator, converters, timers, unit tools</span>
      </li>
      <li>
        <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path fill="#51cf66" d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41L9 16.17z"/></svg>
        <span><strong>Productivity</strong> — notes, checklists, timers, Markdown editor</span>
      </li>
      <li>
        <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path fill="#51cf66" d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41L9 16.17z"/></svg>
        <span><strong>Maker tools</strong> — PWM calculator, resistor lookup, pinout viewer</span>
      </li>
      <li>
        <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path fill="#51cf66" d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41L9 16.17z"/></svg>
        <span><strong>Everyday life</strong> — color picker, budget tracker, QR generator</span>
      </li>
    </ul>
  </div>

  <!-- Privacy-First Apps -->
  <div class="card feature-card">
    <div class="feature-header">
      <span class="feature-badge">🔒</span>
      <div>
        <h2 class="feature-title">Privacy-First Apps</h2>
        <p class="feature-desc">Every tool runs on your device. No accounts, no sync servers, no data ever leaves your browser.</p>
      </div>
    </div>
    <div class="hw-svg-wrap">
      <!-- Phone shielded from cloud & trackers -->
      <svg viewBox="0 0 240 110" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" class="hw-diagram">

        <!-- ── Shield ── -->
        <path d="M 82 5 L 130 5 L 130 62 Q 130 90 106 100 Q 82 90 82 62 Z"
              fill="#51cf66" fill-opacity="0.05" stroke="#51cf66" stroke-width="1.8" stroke-opacity="0.5"/>

        <!-- ── Phone ── -->
        <rect x="89" y="14" width="34" height="58" rx="5" fill="none" stroke="currentColor" stroke-width="1.8" opacity="0.65"/>
        <rect x="89" y="14" width="34" height="10" rx="5" fill="currentColor" opacity="0.06"/>
        <rect x="89" y="20" width="34" height="4" fill="currentColor" opacity="0.05"/>
        <rect x="97" y="17" width="12" height="2" rx="1" fill="currentColor" opacity="0.22"/>
        <circle cx="120" cy="18" r="1.5" fill="currentColor" opacity="0.2"/>
        <circle cx="106" cy="65" r="3" fill="none" stroke="currentColor" stroke-width="1" opacity="0.28"/>
        <!-- lock icon on screen -->
        <rect x="100" y="31" width="12" height="9" rx="1.5" fill="#51cf66" opacity="0.28"/>
        <rect x="100" y="31" width="12" height="9" rx="1.5" fill="none" stroke="#51cf66" stroke-width="1" opacity="0.6"/>
        <path d="M102 31 Q102 26 106 26 Q110 26 110 31" fill="none" stroke="#51cf66" stroke-width="1.8" stroke-linecap="round" opacity="0.7"/>
        <circle cx="106" cy="36" r="1.5" fill="#51cf66" opacity="0.9"/>
        <rect x="94" y="46" width="24" height="2" rx="1" fill="currentColor" opacity="0.1"/>
        <rect x="96" y="51" width="20" height="2" rx="1" fill="currentColor" opacity="0.08"/>
        <rect x="98" y="56" width="16" height="2" rx="1" fill="currentColor" opacity="0.08"/>

        <!-- ── Cloud (left, blocked) ── -->
        <ellipse cx="26" cy="40" rx="14" ry="9" fill="none" stroke="currentColor" stroke-width="1.4" opacity="0.38"/>
        <ellipse cx="18" cy="45" rx="8" ry="5" fill="none" stroke="currentColor" stroke-width="1.4" opacity="0.32"/>
        <ellipse cx="34" cy="45" rx="8" ry="5" fill="none" stroke="currentColor" stroke-width="1.4" opacity="0.32"/>
        <text x="26" y="59" font-size="5" font-family="monospace" fill="currentColor" opacity="0.38" text-anchor="middle">CLOUD</text>
        <!-- red X through cloud -->
        <line x1="12" y1="31" x2="40" y2="49" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" opacity="0.65"/>
        <line x1="40" y1="31" x2="12" y2="49" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" opacity="0.65"/>
        <!-- dashed arrow to shield, stopped -->
        <line x1="42" y1="44" x2="78" y2="54" stroke="currentColor" stroke-width="1.4" stroke-dasharray="4 3" opacity="0.28"/>
        <polygon points="76,51 82,55 75,58" fill="currentColor" opacity="0.2"/>

        <!-- ── Eye / Tracker (right, blocked) ── -->
        <ellipse cx="214" cy="37" rx="16" ry="9" fill="none" stroke="currentColor" stroke-width="1.4" opacity="0.38"/>
        <circle cx="214" cy="37" r="4.5" fill="none" stroke="currentColor" stroke-width="1.2" opacity="0.35"/>
        <circle cx="214" cy="37" r="1.8" fill="currentColor" opacity="0.22"/>
        <text x="214" y="57" font-size="5" font-family="monospace" fill="currentColor" opacity="0.38" text-anchor="middle">TRACKER</text>
        <!-- red X through eye -->
        <line x1="198" y1="28" x2="230" y2="46" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" opacity="0.65"/>
        <line x1="230" y1="28" x2="198" y2="46" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" opacity="0.65"/>
        <!-- dashed arrow to shield, stopped -->
        <line x1="198" y1="42" x2="136" y2="55" stroke="currentColor" stroke-width="1.4" stroke-dasharray="4 3" opacity="0.28"/>
        <polygon points="138,52 132,56 138,60" fill="currentColor" opacity="0.2"/>

        <!-- ── On-device label ── -->
        <text x="106" y="107" font-size="5.5" font-family="monospace" fill="#51cf66" stroke-opacity="0.7" text-anchor="middle" opacity="0.7">ON-DEVICE ONLY</text>
      </svg>
    </div>
    <ul class="privacy-list">
      <li>
        <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path fill="#51cf66" d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41L9 16.17z"/></svg>
        <span><strong>Offline-capable</strong> — utility apps cached as PWAs, usable with no internet at all</span>
      </li>
      <li>
        <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path fill="#51cf66" d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41L9 16.17z"/></svg>
        <span><strong>LAN / local network</strong> — hardware tools connect to your devices directly over WiFi, Bluetooth, USB or even NFC, never through a cloud relay</span>
      </li>
      <li>
        <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path fill="#51cf66" d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41L9 16.17z"/></svg>
        <span><strong>Zero telemetry</strong> — no analytics, no tracking, no accounts</span>
      </li>
      <li>
        <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path fill="#51cf66" d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41L9 16.17z"/></svg>
        <span><strong>Client-side only</strong> — browser APIs, no cloud, no middleware</span>
      </li>
      <li>
        <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path fill="#51cf66" d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41L9 16.17z"/></svg>
        <span><strong>Open source</strong> — inspect, fork, contribute · <a href="https://github.com/knitnox/knitnox" target="_blank" rel="noopener">GitHub</a></span>
      </li>
    </ul>
  </div>

  <!-- CTA -->
  <a href="/apps" class="btn" id="get-started-btn">Explore Apps →</a>

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

  /* Feature cards */
  .feature-header {
    display: flex;
    align-items: flex-start;
    gap: 14px;
  }

  .feature-badge {
    font-size: 1.5rem;
    flex-shrink: 0;
    margin-top: 2px;
  }

  .feature-title {
    margin: 0 0 4px 0;
    font-size: 1.1rem;
    font-weight: 700;
    color: var(--text);
  }

  .feature-desc {
    margin: 0;
    font-size: 0.88rem;
    color: #777;
    text-align: left;
    line-height: 1.5;
  }

  /* Hardware SVG */
  .hw-svg-wrap {
    margin: 18px 0 14px;
    display: flex;
    justify-content: center;
  }

  .hw-diagram {
    width: 100%;
    max-width: 300px;
    height: auto;
    color: var(--text);
  }

  /* Connection type pills */
  .pill-row {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .pill {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 5px 12px;
    border-radius: 20px;
    font-size: 0.8rem;
    font-weight: 600;
    background: var(--bg);
    color: var(--text);
    box-shadow: 3px 3px 6px var(--dark), -3px -3px 6px var(--light);
  }

  /* Privacy checklist */
  .privacy-list {
    list-style: none;
    margin: 14px 0 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .privacy-list li {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    font-size: 0.88rem;
    color: #555;
    line-height: 1.5;
    text-align: left;
  }

  .privacy-list li svg {
    flex-shrink: 0;
    margin-top: 2px;
  }

  .privacy-list a {
    color: #5a8dee;
    text-decoration: none;
  }

  .privacy-list a:hover {
    text-decoration: underline;
  }

  /* CTA button as link */
  a#get-started-btn {
    display: block;
    text-align: center;
    text-decoration: none;
  }
</style>
