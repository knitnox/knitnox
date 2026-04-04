<script>
  import AppCard from '$lib/components/AppCard.svelte';

  let { data } = $props();

  const INITIAL_COUNT = 20;
  const MORE_COUNT = 10;

  let allApps = $derived(data.apps);
  let searchTerm = $state('');
  let displayCount = $state(INITIAL_COUNT);

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

  let categories = $derived.by(() => {
    const map = {};
    for (const app of displayed) {
      const cat = app.category || 'Other';
      if (!map[cat]) map[cat] = [];
      map[cat].push(app);
    }
    return Object.keys(map).sort().map(name => ({ name, apps: map[name] }));
  });

  $effect(() => {
    searchTerm;
    displayCount = INITIAL_COUNT;
  });

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
  <a href="/" class="back-btn">← Home</a>
  <h2 style="margin: 16px 0 4px; font-size:1.4rem;">All Apps</h2>

  <input class="search-box" type="search" placeholder="Search apps…" bind:value={searchTerm} />

  {#if allApps.length === 0}
    <div class="no-apps" style="padding:40px 0; text-align:center; color:#888;">No apps found.</div>
  {:else if displayed.length === 0}
    <div class="no-apps">No apps match your search.</div>
  {:else}
    {#each categories as category}
      <div class="category-section">
        <h2 class="category-header">{category.name}</h2>
        <div class="apps-grid">
          {#each category.apps as app (app.id)}
            <AppCard {app} />
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

<style>
  h1 {
    margin: 0;
    font-size: 2.6rem;
    font-family: 'Orbitron', sans-serif;
    font-weight: 900;
    letter-spacing: 4px;
    color: var(--text);
  }
</style>
