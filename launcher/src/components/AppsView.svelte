<script>
  import AppCard from './AppCard.svelte';

  let { onNavigate } = $props();

  const INITIAL_COUNT = 20;
  const MORE_COUNT = 10;

  let allApps = $state([]);
  let searchTerm = $state('');
  let displayCount = $state(INITIAL_COUNT);
  let loadError = $state(false);

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

  // Reset display count when search changes
  $effect(() => {
    searchTerm; // track
    displayCount = INITIAL_COUNT;
  });

  // Fetch apps.json once
  $effect(() => {
    fetch('/apps.json')
      .then(r => r.json())
      .then(data => { allApps = data; })
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
