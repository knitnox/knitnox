<script lang="ts">
	import './layout.css';
	import favicon from '$lib/assets/favicon.svg';
	import 'highlight.js/styles/github-dark.css';
	import { settings } from '$lib/settings.svelte';

	let { children } = $props();

	let dynamicStyles = $derived.by(() => {
		const family = settings.fontFamily === 'mono' ? 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace' : 
		               settings.fontFamily === 'serif' ? 'ui-serif, Georgia, Cambria, "Times New Roman", Times, serif' : 
					   'ui-sans-serif, system-ui, sans-serif';
		return `
			:root {
				--chat-font-size: ${settings.fontSize}px;
				--chat-font-family: ${family};
			}
		`;
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	{@html `<style>${dynamicStyles}</style>`}
</svelte:head>
{@render children()}
