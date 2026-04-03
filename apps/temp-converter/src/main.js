import App from './App.svelte';
import { mount } from 'svelte';

const app = mount(App, { target: document.getElementById('app') });

// Fade out splash screen once app has mounted
const splash = document.getElementById('splash');
if (splash) {
  setTimeout(() => {
    splash.classList.add('hidden');
    splash.addEventListener('transitionend', () => splash.remove(), { once: true });
  }, 400);
}

export default app;
