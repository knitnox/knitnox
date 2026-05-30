<script lang="ts">
	import './layout.css';
	import 'highlight.js/styles/github-dark.css';
	import { settings } from '$lib/settings.svelte';
	import { toast } from '$lib/toast.svelte';
	import Toaster from '$lib/components/Toaster.svelte';
	import { fade } from 'svelte/transition';
	import { pwaInfo } from 'virtual:pwa-info';
	import { onMount } from 'svelte';

	let { children } = $props();

	onMount(async () => {
		if (typeof window !== 'undefined' && 'serviceWorker' in navigator && !import.meta.env.DEV) {
			const { registerSW } = await import('virtual:pwa-register');
			registerSW({
				immediate: true,
				onRegistered(r) {
					console.log('SW Registered:', r);
				},
				onRegisterError(error) {
					console.log('SW Registration error:', error);
				}
			});
		}
	});

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
	<link rel="icon" href="/favicon.svg" />
	<link rel="preconnect" href="https://fonts.googleapis.com">
	<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous">
	<link href="https://fonts.googleapis.com/css2?family=Orbitron:wght@400..900&display=swap" rel="stylesheet">
	{@html pwaInfo?.webManifest.linkTag}
</svelte:head>

<svelte:body />

{#if settings.isLoading}
	<div out:fade={{ duration: 400 }} class="fixed inset-0 z-[100] flex items-center justify-center bg-white dark:bg-zinc-950">
		<div class="flex flex-col items-center gap-6">
			<div class="flex flex-col items-center">
				<h1 class="logo-title">
					{#each 'Knitnox'.split('') as char, i}
						<span class={i === 5 ? 'fast-glow' : ''}>{char}</span>
					{/each}
				</h1>
				<div class="logo-divider">
					<span class="line"></span>
					<span class="star fast-glow">⛤</span>
					<span class="line"></span>
				</div>
			</div>
			<p class="text-sm font-bold text-zinc-500 animate-pulse">Initializing Knitnox...</p>
		</div>
	</div>
{:else}
	{@render children()}
{/if}
<Toaster />

<style>
	.logo-title {
		margin: 0;
		font-size: 3rem;
		font-family: 'Orbitron', sans-serif;
		font-weight: 900;
		letter-spacing: 4px;
		color: #09090b;
		display: flex;
		justify-content: center;
		transition: color 0.3s;
		text-shadow: 0 0 10px rgba(255, 255, 255, 0.5);
	}

	:global(.dark) .logo-title {
		color: #ffffff;
		text-shadow: 0 0 10px rgba(255, 255, 255, 0.1);
	}

	.logo-title span {
		display: inline-block;
	}

	.fast-glow {
		animation: fastBulbGlow 0.8s ease-in-out infinite;
		color: #f59e0b; /* tailwind orange-500 */
	}

	@keyframes fastBulbGlow {
		0%, 100% { 
			text-shadow: 0 0 5px rgba(245, 158, 11, 0.4),
						 0 0 10px rgba(245, 158, 11, 0.3);
		}
		50% { 
			text-shadow: 0 0 10px rgba(245, 158, 11, 0.6),
						 0 0 20px rgba(245, 158, 11, 0.8),
						 0 0 30px rgba(245, 158, 11, 0.5);
		}
	}

	.logo-divider {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		margin-top: -0.25rem;
		width: 100%;
		max-width: 200px;
		opacity: 0.6;
	}

	.logo-divider .line {
		flex: 1;
		height: 1px;
		background: linear-gradient(to right, transparent, #09090b, transparent);
	}

	:global(.dark) .logo-divider .line {
		background: linear-gradient(to right, transparent, #ffffff, transparent);
	}

	.logo-divider .star {
		font-size: 1.25rem;
		color: #f59e0b;
	}
</style>
