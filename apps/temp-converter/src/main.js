import App from './App.svelte';
import { mount } from 'svelte';

const app = mount(App, { target: document.getElementById('app') });

// Notify the Knitnox launcher that this app is installed (runs in standalone = PWA is installed)
if (window.matchMedia('(display-mode: standalone)').matches || window.navigator.standalone === true) {
  try {
    const pathParts = window.location.pathname.replace(/\/$/, '').split('/');
    const appId = pathParts[pathParts.length - 1];
    if (appId) localStorage.setItem(`knitnox-app-${appId}`, '1');
  } catch {}
}

// Fade out splash screen once app has mounted
const splash = document.getElementById('splash');
if (splash) {
  setTimeout(() => {
    splash.classList.add('hidden');
    splash.addEventListener('transitionend', () => splash.remove(), { once: true });
  }, 400);
}

export default app;
