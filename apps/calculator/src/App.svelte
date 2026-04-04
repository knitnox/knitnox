<script>
  import './lib/neumorphic.css';
  import Display from './components/Display.svelte';
  import ButtonGrid from './components/ButtonGrid.svelte';

  // ── PWA install prompt ────────────────────────────────────────────────────
  let deferredPrompt = $state(null);
  let isStandalone = $state(
    window.matchMedia('(display-mode: standalone)').matches ||
    window.navigator.standalone === true
  );

  $effect(() => {
    function onBeforeInstall(e) {
      e.preventDefault();
      deferredPrompt = e;
    }
    window.addEventListener('beforeinstallprompt', onBeforeInstall);
    window.addEventListener('appinstalled', () => { deferredPrompt = null; isStandalone = true; });
    return () => window.removeEventListener('beforeinstallprompt', onBeforeInstall);
  });

  async function installApp() {
    if (!deferredPrompt) return;
    deferredPrompt.prompt();
    const { outcome } = await deferredPrompt.userChoice;
    if (outcome === 'accepted') { deferredPrompt = null; isStandalone = true; }
  }

  const _saved = JSON.parse(localStorage.getItem('knitnox-calculator') || 'null');
  let currentValue = $state(_saved?.currentValue ?? '0');
  let previousValue = $state(_saved?.previousValue ?? '');
  let operation = $state(_saved?.operation ?? null);
  let shouldReset = $state(_saved?.shouldReset ?? false);

  $effect(() => {
    localStorage.setItem('knitnox-calculator', JSON.stringify({ currentValue, previousValue, operation, shouldReset }));
  });

  let previousDisplay = $derived(
    previousValue && operation ? `${previousValue} ${operation}` : ''
  );

  function appendNumber(num) {
    if (shouldReset) {
      currentValue = '0';
      shouldReset = false;
    }
    if (num === '.' && currentValue.includes('.')) return;
    if (currentValue === '0' && num !== '.') {
      currentValue = num;
    } else {
      currentValue += num;
    }
  }

  function setOperation(op) {
    if (operation !== null && previousValue !== '') calculate();
    operation = op;
    previousValue = currentValue;
    shouldReset = true;
  }

  function calculate() {
    if (operation === null || previousValue === '') return;
    const prev = parseFloat(previousValue);
    const cur = parseFloat(currentValue);
    let result;
    switch (operation) {
      case '+': result = prev + cur; break;
      case '-': result = prev - cur; break;
      case '×': result = prev * cur; break;
      case '÷': result = cur !== 0 ? prev / cur : 'Error'; break;
      case '%': result = prev % cur; break;
      default: return;
    }
    currentValue = result.toString();
    if (currentValue !== 'Error' && currentValue.length > 12) {
      currentValue = parseFloat(currentValue).toExponential(6);
    }
    operation = null;
    previousValue = '';
    shouldReset = true;
  }

  function clearAll() {
    currentValue = '0';
    previousValue = '';
    operation = null;
    shouldReset = false;
  }

  function deleteLast() {
    if (currentValue.length > 1) {
      currentValue = currentValue.slice(0, -1);
    } else {
      currentValue = '0';
    }
  }

  $effect(() => {
    function handleKeydown(e) {
      if (e.key >= '0' && e.key <= '9') {
        appendNumber(e.key);
      } else if (e.key === '.') {
        appendNumber('.');
      } else if (['+', '-', '*', '/'].includes(e.key)) {
        setOperation(e.key === '*' ? '×' : e.key === '/' ? '÷' : e.key);
      } else if (e.key === 'Enter' || e.key === '=') {
        e.preventDefault();
        calculate();
      } else if (e.key === 'Escape') {
        clearAll();
      } else if (e.key === 'Backspace') {
        deleteLast();
      }
    }
    document.addEventListener('keydown', handleKeydown);
    return () => document.removeEventListener('keydown', handleKeydown);
  });
</script>

<div class="page">
  <div class="calculator">
    <div class="header">
      <h1>Calculator</h1>
    </div>

    <div class="calc-body">
      <Display previous={previousDisplay} current={currentValue} />
      <ButtonGrid
        onNumber={appendNumber}
        onOperation={setOperation}
        onCalculate={calculate}
        onClear={clearAll}
        onDelete={deleteLast}
      />
    </div>

    <div class="footer">
      <a href="/">← Back to Apps</a>
    </div>
  </div>
</div>

<!-- Floating Install Button (only when not installed and prompt is available) -->
{#if !isStandalone && deferredPrompt}
  <button class="pwa-install-fab" onclick={installApp} aria-label="Install app">
    ⊕ Install App
  </button>
{/if}

<style>
  :global(*) {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
  }

  :global(:root) {
    --bg: #e6e9ef;
    --light: #ffffff;
    --dark: #c2c8d0;
    --text: #2a2f3a;
  }

  :global(body) {
    font-family: 'Segoe UI', Tahoma, sans-serif;
    background: var(--bg);
    color: var(--text);
  }

  .page {
    display: flex;
    justify-content: center;
    align-items: center;
    min-height: 100vh;
    min-height: 100dvh;
    padding: 20px;
  }

  .calculator {
    max-width: 380px;
    width: 100%;
  }

  .header {
    text-align: center;
    margin-bottom: 20px;
  }

  .header h1 {
    font-size: 1.8rem;
    font-family: 'Orbitron', sans-serif;
    font-weight: 900;
    letter-spacing: 2px;
    color: var(--text);
    margin-bottom: 6px;
  }

  .calc-body {
    padding: 24px;
    border-radius: 20px;
    background: var(--bg);
    box-shadow: 10px 10px 20px var(--dark),
                -10px -10px 20px var(--light);
  }

  .footer {
    text-align: center;
    margin-top: 20px;
    font-size: 0.8rem;
    color: #888;
  }

  .footer a {
    color: #5a8dee;
    text-decoration: none;
  }

  .footer a:hover {
    text-decoration: underline;
  }
</style>
