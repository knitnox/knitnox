<script lang="ts">
	import { settings } from '$lib/settings.svelte';
	import { Upload, Image as ImageIcon, QrCode, Camera, X } from '@lucide/svelte';
	import { fade, fly } from 'svelte/transition';
	import { maskApiKey, createCameraScanner, handleQRImageImport } from '$lib/import-utils.svelte';
	import ImportSuccessModal from './ImportSuccessModal.svelte';

	let importInput = $state<HTMLInputElement | null>(null);
	let qrImportInput = $state<HTMLInputElement | null>(null);

	let videoElement = $state<HTMLVideoElement | null>(null);

	const cameraScanner = createCameraScanner(
		() => videoElement,
		() => { stopCamera(); handleImportSuccess(); }
	);
	const startCamera = () => cameraScanner.startCamera();
	const stopCamera = () => cameraScanner.stopCamera();
	const showCameraScanner = $derived(cameraScanner.getShowCameraScanner());

	let showSuccessModal = $state(false);
	let importedModelName = $state('');
	let importedBaseUrl = $state('');
	let importedApiTokenMasked = $state('');

	function handleImportSuccess() {
		importedModelName = settings.model;
		importedBaseUrl = settings.baseUrl;
		importedApiTokenMasked = maskApiKey(settings.apiKey);
		showSuccessModal = true;
	}

	function handleImport(e: Event) {
		const target = e.target as HTMLInputElement;
		const file = target.files?.[0];
		if (!file) return;

		const reader = new FileReader();
		reader.onload = (e) => {
			const content = e.target?.result as string;
			if (settings.importSettings(content)) {
				handleImportSuccess();
			} else {
				alert('Failed to import settings. Please check the file format.');
			}
		};
		reader.readAsText(file);
		target.value = '';
	}

	async function handleQRImport(e: Event) {
		const target = e.target as HTMLInputElement;
		const file = target.files?.[0];
		if (!file) return;

		const success = await handleQRImageImport(file);
		if (success) handleImportSuccess();
		target.value = '';
	}
</script>

<div class="flex flex-col items-center justify-center min-h-[70vh] px-4 py-12 transition-colors duration-300">
	<!-- Logo Section -->
	<div class="mb-4 flex flex-col items-center">
		<h1 class="logo-title">
			{#each 'Knitnox'.split('') as char}
				<span>{char}</span>
			{/each}
		</h1>
		<div class="logo-divider">
			<span class="line"></span>
			<span class="star">⛤</span>
			<span class="line"></span>
		</div>
	</div>

	<!-- Hero Text -->
	<div class="max-w-4xl text-center space-y-8">
		<div class="space-y-4">
			<h2 class="text-2xl sm:text-4xl font-bold tracking-tight text-zinc-900 dark:text-white leading-tight px-2">
				Chat with any AI using any <br class="hidden sm:block" />
				<span class="text-blue-600 dark:text-blue-500">OpenAI-compatible API.</span>
			</h2>
			<p class="text-lg sm:text-xl text-zinc-600 dark:text-zinc-400 leading-relaxed max-w-2xl mx-auto">
				You're in control, private, and never locked in. Feel free to <a href="https://github.com/knitnox/knitnox" target="_blank" rel="noopener noreferrer" class="text-blue-600 dark:text-blue-500 hover:underline">download the source</a> to customize, run locally, or host it yourself.
			</p>
		</div>

		<!-- Quick Settings Section -->
		<div class="flex flex-col items-center gap-6 pt-8 animate-in fade-in slide-in-from-bottom-4 duration-1000 w-full">
			<div class="flex flex-col sm:flex-row gap-2 w-full max-w-xl">
				<button 
					onclick={() => qrImportInput?.click()}
					class="flex-1 flex items-center justify-center gap-1.5 rounded-lg border-2 border-blue-100 bg-blue-50/50 dark:border-blue-900/30 dark:bg-blue-900/10 p-2 text-[11px] font-bold text-blue-600 dark:text-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900/20 transition-all hover:scale-[1.02] active:scale-95 shadow-md shadow-blue-500/5"
				>
					<ImageIcon size={14} class="shrink-0" />
					<span class="truncate text-center">Import Tag Image</span>
				</button>

				<button 
					onclick={startCamera}
					class="flex-1 flex items-center justify-center gap-1.5 rounded-lg border-2 border-blue-100 bg-blue-50/50 dark:border-blue-900/30 dark:bg-blue-900/10 p-2 text-[11px] font-bold text-blue-600 dark:text-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900/20 transition-all hover:scale-[1.02] active:scale-95 shadow-md shadow-blue-500/5"
				>
					<Camera size={14} class="shrink-0" />
					<span class="truncate text-center">Scan with Camera</span>
				</button>
				
				<button 
					onclick={() => importInput?.click()}
					class="flex-1 flex items-center justify-center gap-1.5 rounded-lg border-2 border-blue-100 bg-blue-50/50 dark:border-blue-900/30 dark:bg-blue-900/10 p-2 text-[11px] font-bold text-blue-600 dark:text-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900/20 transition-all hover:scale-[1.02] active:scale-95 shadow-md shadow-blue-500/5"
				>
					<Upload size={14} class="shrink-0" />
					<span class="truncate text-center">Import from JSON</span>
				</button>
			</div>
		</div>

		<input type="file" accept=".json" class="hidden" bind:this={importInput} onchange={handleImport} />
		<input type="file" accept="image/*" class="hidden" bind:this={qrImportInput} onchange={handleQRImport} />
	</div>
</div>

{#if showCameraScanner}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div 
		class="fixed inset-0 z-[100] flex items-center justify-center bg-black/80 backdrop-blur-sm p-4"
		onclick={stopCamera}
		transition:fade={{ duration: 200 }}
	>
		<div 
			class="relative flex flex-col w-full max-w-sm rounded-3xl bg-white shadow-2xl dark:bg-zinc-900 overflow-hidden p-6"
			onclick={e => e.stopPropagation()}
			transition:fly={{ y: 50, duration: 300 }}
		>
			<div class="flex items-center justify-between mb-6">
				<div class="flex items-center gap-2 text-blue-600">
					<Camera size={20} />
					<h3 class="text-lg font-bold">Scan Settings Tag</h3>
				</div>
				<button onclick={stopCamera} class="rounded-full p-2 hover:bg-zinc-100 dark:hover:bg-zinc-800 transition-colors">
					<X size={20} />
				</button>
			</div>
			
			<div class="mx-auto w-full aspect-square overflow-hidden rounded-2xl border-4 border-white bg-black shadow-xl dark:border-zinc-800 relative">
				<!-- svelte-ignore a11y_media_has_caption -->
				<video 
					bind:this={videoElement} 
					class="w-full h-full object-cover" 
					playsinline 
					muted 
					autoplay
				></video>
				<div class="absolute inset-0 border-[3px] border-dashed border-white/50 m-8 rounded-2xl pointer-events-none animate-pulse"></div>
			</div>
			<p class="text-xs text-zinc-500 text-center mt-6">Point your camera at a Settings Tag QR code</p>
		</div>
	</div>
{/if}

<ImportSuccessModal
	bind:isOpen={showSuccessModal}
	modelName={importedModelName}
	baseUrl={importedBaseUrl}
	apiTokenMasked={importedApiTokenMasked}
/>

<style>
	:root {
		--logo-text: #09090b; /* zinc-950 */
		--logo-glow: rgba(255, 255, 255, 0.8);
		--logo-accent: #ffa500; /* orange */
		--logo-accent-glow: rgba(255, 165, 0, 0.4);
	}

	:global(.dark) {
		--logo-text: #ffffff; /* pure white */
		--logo-glow: rgba(255, 255, 255, 0.1);
		--logo-accent: #ffa500; /* orange */
		--logo-accent-glow: rgba(255, 165, 0, 0.5);
	}

	.logo-title {
		margin: 0;
		font-size: 4rem;
		font-family: 'Orbitron', sans-serif;
		font-weight: 900;
		letter-spacing: 8px;
		color: var(--logo-text);
		display: flex;
		justify-content: center;
		transition: color 0.3s;
		text-shadow: 0 0 20px var(--logo-glow), 0 0 40px var(--logo-glow);
	}

	.logo-title span {
		display: inline-block;
	}

	.logo-title span:nth-child(6) {
		animation: bulbGlow 2s ease-in-out infinite;
		color: var(--logo-accent);
	}

	@keyframes bulbGlow {
		0%, 100% { 
			text-shadow: 0 0 10px var(--logo-accent-glow),
						 0 0 20px rgba(255, 165, 0, 0.3),
						 0 0 30px rgba(255, 165, 0, 0.2);
		}
		50% { 
			text-shadow: 0 0 15px var(--logo-accent-glow),
						 0 0 25px rgba(255, 165, 0, 0.6),
						 0 0 40px rgba(255, 165, 0, 0.4),
						 0 0 60px rgba(255, 165, 0, 0.2);
		}
	}

	@media (max-width: 640px) {
		.logo-title {
			font-size: 2.5rem;
			letter-spacing: 4px;
		}
	}

	.logo-divider {
		display: flex;
		align-items: center;
		gap: 1.5rem;
		margin-top: -0.5rem;
		width: 100%;
		max-width: 400px;
		opacity: 0.6;
	}

	.logo-divider .line {
		flex: 1;
		height: 1px;
		background: linear-gradient(to right, transparent, var(--logo-text), transparent);
	}

	.logo-divider .star {
		font-size: 1.5rem;
		color: var(--logo-accent);
		animation: bulbGlow 2s ease-in-out infinite;
	}
</style>