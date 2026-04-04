<script>
  import '$lib/neumorphic.css';
  let { children } = $props();

  $effect(() => {
    const splash = document.getElementById('splash');
    let timer;
    if (splash) {
      timer = setTimeout(() => {
        splash.classList.add('hidden');
        splash.addEventListener('transitionend', () => splash.remove(), { once: true });
      }, 400);
    }

    if (window.matchMedia('(display-mode: standalone)').matches || window.navigator.standalone === true) {
      try {
        const pathParts = window.location.pathname.replace(/\/$/, '').split('/');
        const appId = pathParts[pathParts.length - 1];
        if (appId) localStorage.setItem(`knitnox-app-${appId}`, '1');
      } catch {}
    }

    return () => clearTimeout(timer);
  });
</script>

{@render children()}
