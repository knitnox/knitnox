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
    const path = target === 'home' ? '/' : target === 'apps' ? '/apps' : '/app';
    history.pushState(null, '', path);
    window.scrollTo(0, 0);
  }

  $effect(() => {
    function onPopState() {
      const path = window.location.pathname;
      if (path === '/apps') view = 'apps';
      else if (path === '/app') view = 'app';
      else view = 'home';
    }
    window.addEventListener('popstate', onPopState);
    // Handle initial path on load
    onPopState();
    return () => window.removeEventListener('popstate', onPopState);
  });
</script>

<div class="neumorph-body" style="min-height:100vh;">
  {#if view === 'home'}
    <HomeView onNavigate={navigate} />
  {:else if view === 'apps'}
    <AppsView onNavigate={navigate} />
  {:else if view === 'app' && selectedApp}
    <AppDetailView app={selectedApp} onNavigate={navigate} />
  {/if}
</div>
