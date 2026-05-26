<script lang="ts">
	import './layout.css';
	import favicon from '$lib/assets/favicon.svg';
	import 'highlight.js/styles/github-dark.css';
	import { settings } from '$lib/settings.svelte';
	import Toaster from '$lib/components/Toaster.svelte';

	let { children } = $props();

	let family = $derived(
		settings.fontFamily === 'mono' ? 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace' : 
		settings.fontFamily === 'serif' ? 'ui-serif, Georgia, Cambria, "Times New Roman", Times, serif' : 
		'ui-sans-serif, system-ui, sans-serif'
	);

	$effect(() => {
		document.documentElement.style.setProperty('--chat-font-size', `${settings.fontSize}px`);
		document.documentElement.style.setProperty('--chat-font-family', family);
		document.documentElement.style.fontSize = `${settings.fontSize}px`;
		document.documentElement.style.fontFamily = family;
		
		// Also apply to body specifically to ensure standard CSS inheritance works as expected
		document.body.style.fontSize = `${settings.fontSize}px`;
		document.body.style.fontFamily = family;

		// Theme handling
		const applyTheme = () => {
			const isDark = 
				settings.theme === 'dark' || 
				(settings.theme === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches);
			
			if (isDark) {
				document.documentElement.classList.add('dark');
			} else {
				document.documentElement.classList.remove('dark');
			}
		};

		applyTheme();

		// Listen for system theme changes if in 'system' mode
		const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
		const handler = () => {
			if (settings.theme === 'system') applyTheme();
		};
		mediaQuery.addEventListener('change', handler);
		return () => mediaQuery.removeEventListener('change', handler);
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<link rel="preconnect" href="https://fonts.googleapis.com">
	<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous">
	<link href="https://fonts.googleapis.com/css2?family=Orbitron:wght@400..900&display=swap" rel="stylesheet">
</svelte:head>

<svelte:body />

{@render children()}
<Toaster />
