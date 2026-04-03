<script>
  import './lib/neumorphic.css';
  import './lib/styles.css';
  import HomeView from './components/HomeView.svelte';
  import AppsView from './components/AppsView.svelte';
  import AppDetailView from './components/AppDetailView.svelte';

  let view = $state('home');
  let selectedApp = $state(null);

  function navigate(target, app = null) {
    if (target === 'app' && app) selectedApp = app;
    view = target;
    const path =
      target === 'home' ? '/' :
      target === 'apps' ? '/apps' :
      `/app?id=${app ? app.id : (selectedApp ? selectedApp.id : '')}`;
    history.pushState(null, '', path);
    window.scrollTo(0, 0);
  }

  $effect(() => {
    async function resolveRoute() {
      const path = window.location.pathname;
      const params = new URLSearchParams(window.location.search);
      if (path === '/apps') {
        view = 'apps';
      } else if (path === '/app') {
        const id = params.get('id');
        if (id && !selectedApp) {
          const res = await fetch('/apps.json');
          const apps = await res.json();
          selectedApp = apps.find(a => a.id === id) || null;
        }
        view = selectedApp ? 'app' : 'apps';
        if (!selectedApp) history.replaceState(null, '', '/apps');
      } else {
        view = 'home';
      }
    }

    function onPopState() { resolveRoute(); }
    window.addEventListener('popstate', onPopState);
    resolveRoute();
    return () => window.removeEventListener('popstate', onPopState);
  });
</script>

<div class="neumorph-body">
  {#if view === 'home'}
    <HomeView onNavigate={navigate} />
  {:else if view === 'apps'}
    <AppsView onNavigate={navigate} />
  {:else if view === 'app' && selectedApp}
    <AppDetailView app={selectedApp} onNavigate={navigate} />
  {/if}
</div>
