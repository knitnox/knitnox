<script>
  import AppCard from './AppCard.svelte';

  let { onNavigate } = $props();

  const INITIAL_COUNT = 20;
  const MORE_COUNT = 10;

  let allApps = $state([]);
  let searchTerm = $state('');
  let displayCount = $state(INITIAL_COUNT);
  let loadError = $state(false);
  let installedIds = $state(new Set());

  let filtered = $derived(
    searchTerm.trim() === ''
      ? allApps
      : allApps.filter(app => {
          const t = searchTerm.toLowerCase();
          return (
            app.name.toLowerCase().includes(t) ||
            app.shortDescription.toLowerCase().includes(t) ||
            app.fullDescription.toLowerCase().includes(t) ||
            (app.category && app.category.toLowerCase().includes(t))
          );
        })
  );

  let displayed = $derived(filtered.slice(0, displayCount));
  let hasMore = $derived(displayCount < filtered.length);

  // Group displayed apps by category (sorted alphabetically)
  let categories = $derived.by(() => {
    const map = {};
    for (const app of displayed) {
      const cat = app.category || 'Other';
      if (!map[cat]) map[cat] = [];
      map[cat].push(app);
    }
    return Object.keys(map).sort().map(name => ({ name, apps: map[name] }));
  });

  // Read per-app localStorage keys written by sub-apps when they run in standalone mode.
  // Also falls back to the old knitnox-installed JSON array for backwards compatibility.
  function refreshInstalledIds(apps) {
    const ids = new Set();
    for (const app of apps) {
      if (localStorage.getItem(`knitnox-app-${app.id}`) === '1') ids.add(app.id);
    }
    // backwards compat: honour anything manually added via old key
    try {
      const raw = localStorage.getItem('knitnox-installed');
      if (raw) for (const id of JSON.parse(raw)) ids.add(id);
    } catch {}
    installedIds = ids;
  }

  // Re-check every time the tab becomes visible (user may have just installed an app then come back)
  $effect(() => {
    function onVisibilityChange() {
      if (!document.hidden && allApps.length) refreshInstalledIds(allApps);
    }
    document.addEventListener('visibilitychange', onVisibilityChange);
    return () => document.removeEventListener('visibilitychange', onVisibilityChange);
  });

  // Also react to storage events so the launcher updates if a sub-app writes the key in another tab
  $effect(() => {
    function onStorage(e) {
      if (e.key?.startsWith('knitnox-') && allApps.length) refreshInstalledIds(allApps);
    }
    window.addEventListener('storage', onStorage);
    return () => window.removeEventListener('storage', onStorage);
  });

  function handleInstall(app) {
    window.open(app.url, '_blank', 'noopener');
    // Optimistically mark as installed so UI updates immediately;
    // the real confirmation comes when the app runs in standalone and writes its own key.
    try { localStorage.setItem(`knitnox-app-${app.id}`, '1'); } catch {}
    const ids = new Set(installedIds);
    ids.add(app.id);
    installedIds = ids;
  }

  // Reset display count when search changes
  $effect(() => {
    searchTerm; // track
    displayCount = INITIAL_COUNT;
  });

  // Fetch apps.json once
  $effect(() => {
    fetch('/apps.json')
      .then(r => r.json())
      .then(data => {
        allApps = data;
        refreshInstalledIds(data);
      })
      .catch(() => { loadError = true; });
  });

  // Infinite scroll
  $effect(() => {
    let timeout;
    function onScroll() {
      clearTimeout(timeout);
      timeout = setTimeout(() => {
        if (!hasMore) return;
        const pos = window.innerHeight + window.scrollY;
        const threshold = document.documentElement.scrollHeight - 500;
        if (pos >= threshold) displayCount += MORE_COUNT;
      }, 100);
    }
    window.addEventListener('scroll', onScroll, { passive: true });
    return () => {
      window.removeEventListener('scroll', onScroll);
      clearTimeout(timeout);
    };
  });
</script>

<div class="neumorph-container">
  <div class="logo-card">
    <h1>{@html 'Knitnox'.split('').map(ch => `<span>${ch}</span>`).join('')}</h1>
  </div>
  <a href="/" class="back-btn" onclick={(e) => { e.preventDefault(); onNavigate('home'); }}>← Home</a>
  <h2 style="margin: 16px 0 4px; font-size:1.4rem;">All Apps</h2>

  <input
    class="search-box"
    type="search"
    placeholder="Search apps…"
    bind:value={searchTerm}
  />

  {#if loadError}
    <div class="no-apps">Error loading apps. Please try again.</div>
  {:else if allApps.length === 0}
    <div class="no-apps" style="padding:40px 0; text-align:center; color:#888;">Loading…</div>
  {:else if displayed.length === 0}
    <div class="no-apps">No apps match your search.</div>
  {:else}
    {#each categories as category}
      <div class="category-section">
        <h2 class="category-header">{category.name}</h2>
        <div class="apps-grid">
          {#each category.apps as app (app.id)}
            <AppCard {app} onSelect={(a) => onNavigate('app', a)} />
          {/each}
        </div>
      </div>
    {/each}

    {#if hasMore}
      <div style="text-align:center; padding:20px; color:#888; font-size:0.9rem;">
        Loading more apps…
      </div>
    {/if}
  {/if}

  <p class="footer">Knitnox v2.0</p>
</div>
