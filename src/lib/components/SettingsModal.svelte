<script lang="ts">
	import { settings } from '$lib/settings.svelte';
	import { MCPClient } from '$lib/mcp.svelte';
	import { X, Download, Upload, QrCode, Camera, Image as ImageIcon, Settings as SettingsIcon, Type, Shield, Share2, Scan, CheckCircle, AlertCircle, Loader2, Layout, Database, Cpu, Plus, Copy, Trash2, Edit3 } from '@lucide/svelte';
	import QRCode from 'qrcode';
	import { fade, fly } from 'svelte/transition';
	import { maskApiKey, createCameraScanner, handleQRImageImport } from '$lib/import-utils.svelte';
	import ImportSuccessModal from './ImportSuccessModal.svelte';

	let { isOpen = $bindable(false) } = $props();
	let importInput = $state<HTMLInputElement | null>(null);
	let qrImportInput = $state<HTMLInputElement | null>(null);

	let connectionStatuses = $state<Record<string, 'checking' | 'success' | 'error' | 'idle'>>({});
	let mcpClients = new Map<string, MCPClient>();

	async function checkMcpConnection(url: string) {
		if (!url) return;
		connectionStatuses[url] = 'checking';
		try {
			const client = new MCPClient(url);
			mcpClients.set(url, client);
			await client.getTools();
			connectionStatuses[url] = 'success';
		} catch (e) {
			connectionStatuses[url] = 'error';
		}
	}

	$effect(() => {
		const urls = settings.mcpServers;
		const timeout = setTimeout(() => {
			for (const url of urls) {
				if (url && !connectionStatuses[url]) {
					checkMcpConnection(url);
				}
			}
		}, 500);
		return () => clearTimeout(timeout);
	});

	let showExportQR = $state(false);
	let videoElement = $state<HTMLVideoElement | null>(null);

	const cameraScanner = createCameraScanner(
		() => videoElement,
		() => { stopCamera(); handleImportSuccess(); }
	);
	const startCamera = () => cameraScanner.startCamera();
	const stopCamera = () => cameraScanner.stopCamera();
	const showCameraScanner = $derived(cameraScanner.getShowCameraScanner());

	let exportTagName = $state('');
	let jsonExportName = $state('');
	let generatedQRUrl = $state('');

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

	function close() {
		stopCamera();
		isOpen = false;
		setTimeout(() => {
			showExportQR = false;
		}, 300);
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

	async function generateQR() {
		if (!exportTagName.trim()) {
			alert('Please provide a name for your settings tag.');
			return;
		}

		const data = settings.getQRCodeData();
		
		const canvas = document.createElement('canvas');
		const ctx = canvas.getContext('2d');
		if (!ctx) return;

		const qrSize = 400;
		const padding = 40;
		const headerHeight = 80;
		
		canvas.width = qrSize + (padding * 2);
		canvas.height = qrSize + headerHeight + (padding * 2);

		ctx.fillStyle = '#ffffff';
		ctx.fillRect(0, 0, canvas.width, canvas.height);

		ctx.fillStyle = '#000000';
		ctx.font = 'bold 24px sans-serif';
		ctx.textAlign = 'center';
		ctx.fillText(exportTagName.toUpperCase(), canvas.width / 2, padding + 30);
		
		ctx.font = '12px sans-serif';
		ctx.fillStyle = '#666666';
		ctx.fillText('KNITNOX SETTINGS TAG', canvas.width / 2, padding + 55);

		const qrCanvasInternal = document.createElement('canvas');
		await QRCode.toCanvas(qrCanvasInternal, data, {
			width: qrSize,
			margin: 1,
			errorCorrectionLevel: 'L'
		});
		
		ctx.drawImage(qrCanvasInternal, padding, padding + headerHeight);

		ctx.fillStyle = '#3b82f6';
		ctx.font = 'bold 14px sans-serif';
		ctx.fillText('knitnox', canvas.width / 2, canvas.height - padding + 10);

		generatedQRUrl = canvas.toDataURL('image/png');
		showExportQR = true;
	}

	function downloadQR() {
		const a = document.createElement('a');
		a.href = generatedQRUrl;
		a.download = `knitnox-tag-${exportTagName.toLowerCase().replace(/\s+/g, '-')}.png`;
		a.click();
	}

</script>

{#if isOpen}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div 
		class="fixed inset-0 z-50 flex items-end justify-center bg-black/40 backdrop-blur-[2px] p-0 sm:items-center sm:p-4"
		onclick={close}
		transition:fade={{ duration: 200 }}
	>
		<div 
			class="relative flex flex-col w-full max-w-2xl h-[90vh] sm:h-auto sm:max-h-[85vh] rounded-t-3xl sm:rounded-3xl bg-white shadow-2xl dark:bg-zinc-900 overflow-hidden"
			onclick={e => e.stopPropagation()}
			transition:fly={{ y: 100, duration: 300, opacity: 1 }}
		>
			<!-- Handle for mobile -->
			<div class="flex justify-center pt-3 pb-1 sm:hidden">
				<div class="h-1.5 w-12 rounded-full bg-zinc-300 dark:bg-zinc-700"></div>
			</div>

			<div class="flex items-center justify-between border-b border-zinc-100 p-6 dark:border-zinc-800">
				<div class="flex items-center gap-3">
					<div class="rounded-xl bg-blue-50 p-2 text-blue-600 dark:bg-blue-900/30">
						<SettingsIcon size={20} />
					</div>
					<div>
						<h2 class="text-lg font-bold">Settings & Preferences</h2>
						<p class="text-xs text-zinc-500">Customize your AI experience</p>
					</div>
				</div>
				<button onclick={close} class="rounded-full p-2 hover:bg-zinc-100 dark:hover:bg-zinc-800 transition-colors">
					<X size={20} />
				</button>
			</div>

			<div class="flex-1 overflow-y-auto p-6 space-y-8 custom-scrollbar">
				{#if showCameraScanner}
					<div class="space-y-6 text-center animate-in fade-in slide-in-from-bottom-4 duration-300 h-full flex flex-col justify-center">
						<div class="flex items-center gap-2 justify-center text-blue-600">
							<Camera size={20} />
							<h3 class="text-lg font-bold">Scan Settings Tag</h3>
						</div>
						<div class="mx-auto w-full max-w-sm overflow-hidden rounded-2xl border-4 border-white bg-black shadow-xl dark:border-zinc-800 relative aspect-square">
							<!-- svelte-ignore a11y_media_has_caption -->
							<video 
								bind:this={videoElement} 
								class="w-full h-full object-cover" 
								playsinline 
								muted 
								autoplay
							></video>
							<div class="absolute inset-0 border-[3px] border-dashed border-white/50 m-12 rounded-2xl pointer-events-none animate-pulse"></div>
						</div>
						<p class="text-xs text-zinc-500">Point your camera at a Settings Tag QR code</p>
						<div class="flex gap-3 mt-auto">
							<button 
								onclick={stopCamera}
								class="flex-1 rounded-xl border border-zinc-200 py-3 text-sm font-bold hover:bg-zinc-50 dark:border-zinc-700 dark:hover:bg-zinc-800 transition-colors"
							>
								Cancel
							</button>
						</div>
					</div>
				{:else if showExportQR}
					<div class="space-y-6 text-center animate-in fade-in slide-in-from-bottom-4 duration-300">
						<div class="flex items-center gap-2 justify-center text-blue-600">
							<Share2 size={20} />
							<h3 class="text-lg font-bold">Settings Tag Generated</h3>
						</div>
						<div class="mx-auto max-w-sm overflow-hidden rounded-2xl border-4 border-white bg-white shadow-xl dark:border-zinc-800">
							<img src={generatedQRUrl} alt="Settings QR Code" class="w-full" />
						</div>
						<div class="rounded-xl bg-amber-50 p-4 text-left dark:bg-amber-900/20">
							<div class="flex gap-3">
								<Shield size={18} class="shrink-0 text-amber-600" />
								<p class="text-xs leading-relaxed text-amber-800 dark:text-amber-200">
									This "Tag" contains your <strong>API Key</strong>. Treat it like a password. Only share it with people you trust or use it to move your own settings between devices.
								</p>
							</div>
						</div>
						<div class="flex gap-3">
							<button 
								onclick={() => showExportQR = false}
								class="flex-1 rounded-xl border border-zinc-200 py-3 text-sm font-bold hover:bg-zinc-50 dark:border-zinc-700 dark:hover:bg-zinc-800 transition-colors"
							>
								Cancel
							</button>
							<button 
								onclick={downloadQR}
								class="flex-1 rounded-xl bg-blue-600 py-3 text-sm font-bold text-white hover:bg-blue-700 shadow-lg shadow-blue-500/20 transition-all active:scale-95"
							>
								Save to Gallery
							</button>
						</div>
					</div>
				{:else}
					<!-- Model Profiles Section -->
					<section class="space-y-4">
						<h3 class="flex items-center gap-2 text-xs font-bold uppercase tracking-widest text-zinc-400">
							<Layout size={14} /> Model Profiles
						</h3>
						<div class="grid gap-4 rounded-2xl border border-blue-100 bg-blue-50/30 p-5 dark:border-blue-900/20 dark:bg-blue-900/10">
							<div class="flex flex-col sm:flex-row gap-4 items-end">
								<div class="flex-1 space-y-1.5 w-full">
									<label for="profileSelect" class="text-xs font-semibold text-blue-600/70">ACTIVE PROFILE</label>
									<select
										id="profileSelect"
										value={settings.activeProfileId}
										onchange={(e) => settings.switchProfile(parseInt(e.currentTarget.value))}
										class="w-full rounded-xl border border-blue-200 bg-white px-3 py-2 text-sm font-bold outline-none focus:ring-2 focus:ring-blue-500 dark:border-blue-800 dark:bg-zinc-900"
									>
										{#each settings.profiles as profile}
											<option value={profile.id}>{profile.name}</option>
										{/each}
									</select>
								</div>
								<div class="flex gap-2 w-full sm:w-auto">
									<button 
										onclick={() => {
											const name = prompt('Enter profile name:');
											if (name) settings.createProfile(name);
										}}
										title="Add New Profile"
										class="flex-1 sm:flex-none flex items-center justify-center gap-2 rounded-xl bg-blue-600 p-2.5 text-white hover:bg-blue-700 transition-all active:scale-95"
									>
										<Plus size={18} />
									</button>
									<button 
										onclick={() => settings.activeProfileId && settings.duplicateProfile(settings.activeProfileId)}
										title="Duplicate Current Profile"
										class="flex-1 sm:flex-none flex items-center justify-center gap-2 rounded-xl border border-blue-200 bg-white p-2.5 text-blue-600 hover:bg-blue-50 transition-all active:scale-95 dark:border-blue-800 dark:bg-zinc-900"
									>
										<Copy size={18} />
									</button>
									<button 
										onclick={() => {
											if (settings.activeProfileId && confirm('Are you sure you want to delete this profile?')) {
												settings.deleteProfile(settings.activeProfileId);
											}
										}}
										title="Delete Current Profile"
										disabled={settings.profiles.length <= 1}
										class="flex-1 sm:flex-none flex items-center justify-center gap-2 rounded-xl border border-red-200 bg-white p-2.5 text-red-600 hover:bg-red-50 transition-all active:scale-95 disabled:opacity-50 disabled:grayscale dark:border-red-900/30 dark:bg-zinc-900"
									>
										<Trash2 size={18} />
									</button>
								</div>
							</div>
							
							<div class="space-y-1.5 pt-2 border-t border-blue-100 dark:border-blue-900/30">
								<label for="profileName" class="text-xs font-semibold text-blue-600/70">RENAME PROFILE</label>
								<div class="relative">
									<input
										id="profileName"
										type="text"
										value={settings.profiles.find(p => p.id === settings.activeProfileId)?.name || ''}
										onchange={(e) => settings.activeProfileId && settings.updateProfileName(settings.activeProfileId, e.currentTarget.value)}
										placeholder="e.g. Coding Assistant"
										class="w-full rounded-xl border border-blue-200 bg-white pl-3 pr-10 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 dark:border-blue-800 dark:bg-zinc-900"
									/>
									<Edit3 size={14} class="absolute right-3 top-1/2 -translate-y-1/2 text-blue-400 pointer-events-none" />
								</div>
							</div>
						</div>
					</section>

					<div class="relative mt-4">
						<div class="absolute -top-3 left-1/2 -translate-x-1/2 z-10 whitespace-nowrap">
							<span class="bg-white px-3 text-[10px] font-black uppercase tracking-widest text-zinc-400 dark:bg-zinc-900 border-2 border-dotted border-zinc-200 dark:border-zinc-800/50 rounded-full py-0.5">
								Configuring: {settings.profiles.find(p => p.id === settings.activeProfileId)?.name || 'Default'}
							</span>
						</div>
						<div class="rounded-[2rem] border-2 border-dotted border-zinc-200 p-6 pt-10 space-y-8 dark:border-zinc-800/50">
							<!-- Connectivity Section -->
							<section class="space-y-4">
								<h3 class="flex items-center gap-2 text-xs font-bold uppercase tracking-widest text-zinc-400">
									<Cpu size={14} /> AI Connectivity
								</h3>
							<div class="grid gap-4 rounded-2xl border border-zinc-100 bg-zinc-50/50 p-5 dark:border-zinc-800 dark:bg-zinc-800/30">
								<div class="grid sm:grid-cols-2 gap-4">
									<div class="space-y-1.5">
										<label for="baseUrl" class="text-xs font-semibold text-zinc-500">BASE URL</label>
										<input
											id="baseUrl"
											type="text"
											bind:value={settings.baseUrl}
											placeholder="https://api.openai.com/v1"
											class="w-full rounded-xl border border-zinc-200 bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 dark:border-zinc-700 dark:bg-zinc-900"
										/>
									</div>
									<div class="space-y-1.5">
										<label for="apiKey" class="text-xs font-semibold text-zinc-500">API KEY</label>
										<input
											id="apiKey"
											type="password"
											bind:value={settings.apiKey}
											placeholder="sk-..."
											class="w-full rounded-xl border border-zinc-200 bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 dark:border-zinc-700 dark:bg-zinc-900"
										/>
									</div>
								</div>
								<div class="space-y-1.5">
									<label for="model" class="text-xs font-semibold text-zinc-500">MODEL NAME</label>
									<input
										id="model"
										type="text"
										bind:value={settings.model}
										placeholder="deepseek/deepseek-v4-flash"
										class="w-full rounded-xl border border-zinc-200 bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 dark:border-zinc-700 dark:bg-zinc-900"
									/>
								</div>
							</div>
						</section>

						<!-- Intelligence Section -->
						<section class="space-y-4">
							<h3 class="flex items-center gap-2 text-xs font-bold uppercase tracking-widest text-zinc-400">
								<Layout size={14} /> Behavior & Logic
							</h3>
							<div class="grid gap-6 rounded-2xl border border-zinc-100 bg-zinc-50/50 p-5 dark:border-zinc-800 dark:bg-zinc-800/30">
								<div class="flex items-center justify-between">
									<div class="space-y-0.5">
										<span class="text-sm font-bold">Enable Thinking</span>
										<p class="text-[10px] text-zinc-500">Show reasoning process for supporting models.</p>
									</div>
									<button 
										onclick={() => settings.enableThinking = !settings.enableThinking}
										class="relative h-6 w-11 rounded-full transition-colors {settings.enableThinking ? 'bg-blue-600' : 'bg-zinc-300 dark:bg-zinc-700'}"
										aria-label="Toggle thinking"
									>
										<div class="absolute top-1 left-1 h-4 w-4 rounded-full bg-white transition-transform {settings.enableThinking ? 'translate-x-5' : ''}"></div>
									</button>
								</div>

								<div class="flex items-center justify-between">
									<div class="space-y-0.5">
										<span class="text-sm font-bold">Context Compression</span>
										<p class="text-[10px] text-zinc-500">Auto-summarize long conversations.</p>
									</div>
									<button 
										onclick={() => settings.enableCompression = !settings.enableCompression}
										class="relative h-6 w-11 rounded-full transition-colors {settings.enableCompression ? 'bg-blue-600' : 'bg-zinc-300 dark:bg-zinc-700'}"
										aria-label="Toggle compression"
									>
										<div class="absolute top-1 left-1 h-4 w-4 rounded-full bg-white transition-transform {settings.enableCompression ? 'translate-x-5' : ''}"></div>
									</button>
								</div>

								<div class="grid sm:grid-cols-2 gap-4 pt-2 border-t border-zinc-100 dark:border-zinc-800">
									<div class="space-y-1.5">
										<div class="flex justify-between items-center">
											<label for="contextWindow" class="text-xs font-semibold text-zinc-500 uppercase">Context window</label>
											<span class="text-[10px] font-mono bg-blue-50 text-blue-600 px-1.5 rounded dark:bg-blue-900/30">{settings.contextWindow || '∞'}</span>
										</div>
										<input
											id="contextWindow"
											type="number"
											bind:value={settings.contextWindow}
											min="0"
											class="w-full rounded-xl border border-zinc-200 bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 dark:border-zinc-700 dark:bg-zinc-900"
										/>
									</div>
									<div class="space-y-1.5">
										<div class="flex justify-between items-center">
											<label for="maxTurns" class="text-xs font-semibold text-zinc-500 uppercase">Agent Loop Limit</label>
											<span class="text-[10px] font-mono bg-green-50 text-green-600 px-1.5 rounded dark:bg-green-900/30">{settings.maxAgentTurns}</span>
										</div>
										<input
											id="maxTurns"
											type="number"
											bind:value={settings.maxAgentTurns}
											min="1"
											class="w-full rounded-xl border border-zinc-200 bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 dark:border-zinc-700 dark:bg-zinc-900"
										/>
									</div>
								</div>

								<div class="space-y-3 pt-2 border-t border-zinc-100 dark:border-zinc-800">
									<span class="text-xs font-semibold text-zinc-500 uppercase">Capabilities</span>
									<div class="grid grid-cols-2 sm:grid-cols-4 gap-2">
										{#each [{key: 'text', label: 'Text', disabled: true, checked: true}, {key: 'supportsImages', label: 'Images'}, {key: 'supportsAudio', label: 'Audio'}, {key: 'supportsVideo', label: 'Video'}] as cap}
											<label class="flex items-center gap-2 rounded-xl border border-zinc-100 p-2 text-xs font-medium cursor-pointer hover:bg-white dark:border-zinc-800 dark:hover:bg-zinc-800 transition-colors">
												<input 
													type="checkbox" 
													checked={cap.checked ?? (settings as any)[cap.key]} 
													disabled={cap.disabled}
													onchange={(e) => !(cap.disabled) && ((settings as any)[cap.key] = (e.target as HTMLInputElement).checked)}
													class="h-4 w-4 rounded border-zinc-300 text-blue-600 focus:ring-blue-500" 
												/>
												{cap.label}
											</label>
										{/each}
									</div>
								</div>

								<div class="space-y-1.5">
									<label for="systemPrompt" class="text-xs font-semibold text-zinc-500 uppercase">Global System Prompt</label>
									<textarea
										id="systemPrompt"
										bind:value={settings.systemPrompt}
										placeholder="You are a helpful assistant."
										class="w-full h-24 rounded-xl border border-zinc-200 bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 dark:border-zinc-700 dark:bg-zinc-900 resize-none leading-relaxed"
									></textarea>
								</div>
							</div>
						</section>

						<!-- MCP Section -->
						<section class="space-y-4">
							<h3 class="flex items-center gap-2 text-xs font-bold uppercase tracking-widest text-zinc-400">
								<Database size={14} /> Extension Servers (MCP)
							</h3>
							<div class="rounded-2xl border border-zinc-100 bg-zinc-50/50 p-5 dark:border-zinc-800 dark:bg-zinc-800/30 space-y-4">
								<div class="space-y-2">
									{#each settings.mcpServers as server, i}
										<div class="flex gap-2 animate-in slide-in-from-left-2 duration-200">
											<div class="relative flex-1">
												<input
													type="text"
													bind:value={settings.mcpServers[i]}
													placeholder="http://127.0.0.1:8000/mcp"
													class="w-full rounded-xl border border-zinc-200 bg-white pl-3 pr-10 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 dark:border-zinc-700 dark:bg-zinc-900"
												/>
												<div class="absolute right-3 top-1/2 -translate-y-1/2 pointer-events-none">
													{#if server && connectionStatuses[server] === 'checking'}
														<Loader2 size={16} class="animate-spin text-zinc-400" />
													{:else if server && connectionStatuses[server] === 'success'}
														<CheckCircle size={16} class="text-green-500" />
													{:else if server && connectionStatuses[server] === 'error'}
														<AlertCircle size={16} class="text-red-500" />
													{/if}
												</div>
											</div>
											<button 
												onclick={() => settings.mcpServers = settings.mcpServers.filter((_, j) => i !== j)}
												class="text-red-500 hover:bg-red-50 p-2 rounded-xl transition-colors dark:hover:bg-red-900/20 shrink-0"
											>
												<X size={18} />
											</button>
										</div>
									{/each}
								</div>
								<button
									onclick={() => settings.mcpServers = [...settings.mcpServers, '']}
									class="w-full flex items-center justify-center gap-2 rounded-xl border-2 border-dashed border-zinc-200 p-3 text-sm font-bold text-zinc-500 hover:border-blue-500 hover:text-blue-500 transition-all"
								>
									+ Add New MCP Server
								</button>
							</div>
						</section>

						<!-- Visuals Section -->
					<section class="space-y-4">
						<h3 class="flex items-center gap-2 text-xs font-bold uppercase tracking-widest text-zinc-400">
							<Type size={14} /> Display & Theme
						</h3>
						<div class="grid gap-5 rounded-2xl border border-zinc-100 bg-zinc-50/50 p-5 dark:border-zinc-800 dark:bg-zinc-800/30">
							<div class="grid grid-cols-2 gap-4">
								<div class="space-y-1.5">
									<label for="fontSize" class="text-xs font-semibold text-zinc-500">FONT SIZE (PX)</label>
									<input
										id="fontSize"
										type="number"
										bind:value={settings.fontSize}
										min="12"
										max="24"
										class="w-full rounded-xl border border-zinc-200 bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 dark:border-zinc-700 dark:bg-zinc-900"
									/>
								</div>
								<div class="space-y-1.5">
									<label for="fontFamily" class="text-xs font-semibold text-zinc-500">FONT FAMILY</label>
									<select
										id="fontFamily"
										bind:value={settings.fontFamily}
										class="w-full rounded-xl border border-zinc-200 bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 dark:border-zinc-700 dark:bg-zinc-900"
									>
										<option value="sans">Sans-serif</option>
										<option value="serif">Serif</option>
										<option value="mono">Monospace</option>
									</select>
								</div>
							</div>
							<div class="space-y-2">
								<span class="text-xs font-semibold text-zinc-500 uppercase">App Theme</span>
								<div class="flex gap-1 rounded-xl bg-zinc-200/50 p-1 dark:bg-zinc-800">
									{#each ['system', 'light', 'dark'] as t}
										<button
											onclick={() => settings.theme = t as any}
											class="flex-1 rounded-lg py-2 text-xs font-bold transition-all {settings.theme === t ? 'bg-white shadow-sm dark:bg-zinc-700 text-blue-600 dark:text-white' : 'text-zinc-500 hover:text-zinc-700 dark:hover:text-zinc-300'}"
										>
											{t.charAt(0).toUpperCase() + t.slice(1)}
										</button>
									{/each}
								</div>
							</div>
						</div>
					</section>

					<!-- Export/Import Section -->
					<section class="space-y-4">
						<h3 class="flex items-center gap-2 text-xs font-bold uppercase tracking-widest text-zinc-400">
							<Share2 size={14} /> Data Management
						</h3>
						<div class="rounded-2xl border border-zinc-100 bg-zinc-50/50 p-5 dark:border-zinc-800 dark:bg-zinc-800/30 space-y-6">
							<!-- QR Tag Group -->
							<div class="space-y-4">
								<div class="flex flex-col gap-2">
									<label for="tagName" class="text-xs font-bold text-zinc-400 uppercase tracking-tight">SETTINGS TAG (QR EXPORT)</label>
									<div class="flex flex-col sm:flex-row gap-2">
										<input 
											id="tagName"
											type="text" 
											bind:value={exportTagName}
											placeholder="Profile Name (e.g. Coding Pro)"
											class="flex-1 rounded-xl border border-zinc-200 bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 dark:border-zinc-700 dark:bg-zinc-900"
										/>
										<button 
											onclick={generateQR}
											class="group flex items-center justify-center gap-2 rounded-xl border border-zinc-200 bg-zinc-50 px-4 py-2 text-sm font-bold text-zinc-600 hover:bg-zinc-100 hover:text-zinc-900 dark:border-zinc-700 dark:bg-zinc-800 dark:text-zinc-300 dark:hover:bg-zinc-100 dark:hover:text-zinc-900 transition-all active:scale-95 w-full sm:w-auto"
										>
											<QrCode size={18} class="text-zinc-400 group-hover:text-zinc-900 transition-colors" />
											Export
										</button>
									</div>
								</div>
								
								<div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
									<button 
										onclick={() => qrImportInput?.click()}
										class="group w-full flex items-center justify-center gap-2 rounded-xl border border-zinc-200 bg-zinc-50 py-3 text-sm font-bold text-zinc-600 hover:bg-zinc-100 hover:text-zinc-900 dark:border-zinc-700 dark:bg-zinc-800 dark:text-zinc-300 dark:hover:bg-zinc-100 dark:hover:text-zinc-900 transition-all active:scale-95"
									>
										<ImageIcon size={18} class="text-zinc-400 group-hover:text-zinc-900 transition-colors" />
										Upload QR Image
									</button>
									<button 
										onclick={startCamera}
										class="group w-full flex items-center justify-center gap-2 rounded-xl border border-zinc-200 bg-zinc-50 py-3 text-sm font-bold text-zinc-600 hover:bg-zinc-100 hover:text-zinc-900 dark:border-zinc-700 dark:bg-zinc-800 dark:text-zinc-300 dark:hover:bg-zinc-100 dark:hover:text-zinc-900 transition-all active:scale-95"
									>
										<Camera size={18} class="text-zinc-400 group-hover:text-zinc-900 transition-colors" />
										Scan with Camera
									</button>
								</div>
							</div>

							<!-- Separator with OR -->
							<div class="relative flex items-center py-2">
								<div class="flex-grow border-t border-dotted border-zinc-200 dark:border-zinc-700"></div>
								<span class="flex-shrink mx-4 text-[10px] font-black text-zinc-400 uppercase tracking-widest">OR</span>
								<div class="flex-grow border-t border-dotted border-zinc-200 dark:border-zinc-700"></div>
							</div>

							<!-- JSON Group (Secondary) -->
							<div class="space-y-4 opacity-80 transition-opacity hover:opacity-100">
								<div class="flex flex-col gap-2">
									<label for="jsonName" class="text-xs font-bold text-zinc-400 uppercase tracking-tight">JSON Settings (Backup)</label>
									<div class="flex flex-col sm:flex-row gap-2">
										<input 
											id="jsonName"
											type="text" 
											bind:value={jsonExportName}
											placeholder="Profile Name (e.g. My Settings)"
											class="flex-1 rounded-xl border border-zinc-200 bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-zinc-400 dark:border-zinc-700 dark:bg-zinc-900"
										/>
										<button 
											onclick={() => settings.exportSettings(jsonExportName)}
											class="group flex items-center justify-center gap-2 rounded-xl border border-zinc-200 bg-zinc-50 px-4 py-2 text-sm font-bold text-zinc-600 hover:bg-zinc-100 hover:text-zinc-900 dark:border-zinc-700 dark:bg-zinc-800 dark:text-zinc-300 dark:hover:bg-zinc-100 dark:hover:text-zinc-900 transition-all active:scale-95 w-full sm:w-auto"
										>
											<Download size={18} class="text-zinc-400 group-hover:text-zinc-900 transition-colors" />
											Export
										</button>
									</div>
								</div>

								<button 
									onclick={() => importInput?.click()}
									class="group w-full flex items-center justify-center gap-2 rounded-xl border border-zinc-200 bg-zinc-50 py-3 text-sm font-bold text-zinc-600 hover:bg-zinc-100 hover:text-zinc-900 dark:border-zinc-700 dark:bg-zinc-800 dark:text-zinc-300 dark:hover:bg-zinc-100 dark:hover:text-zinc-900 transition-all active:scale-95"
								>
									<Upload size={18} class="text-zinc-400 group-hover:text-zinc-900 transition-colors" />
									Import from JSON File
								</button>
							</div>

							<input type="file" accept=".json" class="hidden" bind:this={importInput} onchange={handleImport} />
							<input type="file" accept="image/*" class="hidden" bind:this={qrImportInput} onchange={handleQRImport} />
						</div>
					</section>
					</div>
					</div>
				{/if}
			</div>

			<div class="flex justify-end border-t border-zinc-100 p-6 dark:border-zinc-800 bg-white dark:bg-zinc-900">
				<button
					onclick={close}
					class="w-full rounded-2xl bg-zinc-900 px-4 py-4 font-bold text-white hover:bg-black dark:bg-white dark:text-zinc-900 dark:hover:bg-zinc-200 transition-all shadow-xl active:scale-[0.98]"
				>
					Save All Changes
				</button>
			</div>
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
	.custom-scrollbar::-webkit-scrollbar {
		width: 4px;
	}
	.custom-scrollbar::-webkit-scrollbar-track {
		background: transparent;
	}
	.custom-scrollbar::-webkit-scrollbar-thumb {
		background: #e4e4e7;
		border-radius: 10px;
	}
	:global(.dark) .custom-scrollbar::-webkit-scrollbar-thumb {
		background: #3f3f46;
	}
</style>
