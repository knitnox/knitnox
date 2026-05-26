<script lang="ts">
	import { settings } from '$lib/settings.svelte';
	import { X, Download, Upload, QrCode, Camera, Image as ImageIcon } from '@lucide/svelte';
	import QRCode from 'qrcode';
	import jsQR from 'jsqr';

	let { isOpen = $bindable(false) } = $props();
	let importInput = $state<HTMLInputElement | null>(null);
	let qrImportInput = $state<HTMLInputElement | null>(null);

	let showExportQR = $state(false);
	let exportTagName = $state('');
	let generatedQRUrl = $state('');
	let qrCanvas = $state<HTMLCanvasElement | null>(null);

	function close() {
		isOpen = false;
		showExportQR = false;
	}

	function handleImport(e: Event) {
		const target = e.target as HTMLInputElement;
		const file = target.files?.[0];
		if (!file) return;

		const reader = new FileReader();
		reader.onload = (e) => {
			const content = e.target?.result as string;
			if (settings.importSettings(content)) {
				alert('Settings imported successfully!');
			} else {
				alert('Failed to import settings. Please check the file format.');
			}
		};
		reader.readAsText(file);
		target.value = ''; // Reset
	}

	async function generateQR() {
		if (!exportTagName.trim()) {
			alert('Please provide a name for your settings tag.');
			return;
		}

		const data = settings.getQRCodeData();
		
		// We'll create a composite image: Tag Name at top, QR code in middle
		const canvas = document.createElement('canvas');
		const ctx = canvas.getContext('2d');
		if (!ctx) return;

		const qrSize = 400;
		const padding = 40;
		const headerHeight = 80;
		
		canvas.width = qrSize + (padding * 2);
		canvas.height = qrSize + headerHeight + (padding * 2);

		// Background
		ctx.fillStyle = '#ffffff';
		ctx.fillRect(0, 0, canvas.width, canvas.height);

		// Header Text (Tag Name)
		ctx.fillStyle = '#000000';
		ctx.font = 'bold 24px sans-serif';
		ctx.textAlign = 'center';
		ctx.fillText(exportTagName.toUpperCase(), canvas.width / 2, padding + 30);
		
		ctx.font = '12px sans-serif';
		ctx.fillStyle = '#666666';
		ctx.fillText('KNITNOX SETTINGS TAG', canvas.width / 2, padding + 55);

		// QR Code
		const qrCanvasInternal = document.createElement('canvas');
		await QRCode.toCanvas(qrCanvasInternal, data, {
			width: qrSize,
			margin: 1,
			errorCorrectionLevel: 'L'
		});
		
		ctx.drawImage(qrCanvasInternal, padding, padding + headerHeight);

		// Branding footer
		ctx.fillStyle = '#3b82f6'; // blue-600
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

	async function handleQRImport(e: Event) {
		const target = e.target as HTMLInputElement;
		const file = target.files?.[0];
		if (!file) return;

		const img = new Image();
		img.onload = () => {
			const canvas = document.createElement('canvas');
			const ctx = canvas.getContext('2d');
			if (!ctx) return;

			canvas.width = img.width;
			canvas.height = img.height;
			ctx.drawImage(img, 0, 0);

			const imageData = ctx.getImageData(0, 0, canvas.width, canvas.height);
			const code = jsQR(imageData.data, imageData.width, imageData.height);

			if (code) {
				if (settings.importFromQRCodeData(code.data)) {
					alert('Settings imported successfully from Tag!');
				} else {
					alert('Failed to decode settings from this image.');
				}
			} else {
				alert('No QR code found in the image.');
			}
		};
		img.src = URL.createObjectURL(file);
		target.value = '';
	}
</script>

{#if isOpen}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm p-4">
		<div class="flex flex-col w-full max-w-md max-h-[90vh] rounded-xl bg-white shadow-2xl dark:bg-zinc-900 overflow-hidden">
			<div class="flex items-center justify-between border-b border-zinc-200 p-6 dark:border-zinc-800">
				<h2 class="text-xl font-bold">Settings</h2>
				<button onclick={close} class="rounded-lg p-1 hover:bg-zinc-100 dark:hover:bg-zinc-800">
					<X size={20} />
				</button>
			</div>

			<div class="flex-1 overflow-y-auto p-6">
				{#if showExportQR}
					<div class="space-y-6 text-center">
						<h3 class="text-lg font-bold">Your Settings Tag</h3>
						<div class="mx-auto overflow-hidden rounded-xl border border-zinc-200 bg-white p-2 shadow-inner">
							<img src={generatedQRUrl} alt="Settings QR Code" class="w-full" />
						</div>
						<p class="text-xs text-zinc-500">
							This image contains your API key and configuration. Only share it with trusted sources.
						</p>
						<div class="flex gap-2">
							<button 
								onclick={() => showExportQR = false}
								class="flex-1 rounded-lg border border-zinc-300 px-4 py-2 text-sm font-medium hover:bg-zinc-50 dark:border-zinc-700 dark:hover:bg-zinc-800"
							>
								Back
							</button>
							<button 
								onclick={downloadQR}
								class="flex-1 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
							>
								Download Image
							</button>
						</div>
					</div>
				{:else}
					<div class="space-y-6">
						<div>
							<label class="mb-1 block text-sm font-medium">Base URL</label>
							<input
								type="text"
								bind:value={settings.baseUrl}
								placeholder="https://api.openai.com/v1"
								class="w-full rounded-lg border border-zinc-300 bg-transparent px-3 py-2 outline-none focus:ring-2 focus:ring-blue-500 dark:border-zinc-700"
							/>
						</div>

						<div>
							<label class="mb-1 block text-sm font-medium">API Key</label>
							<input
								type="password"
								bind:value={settings.apiKey}
								placeholder="sk-..."
								class="w-full rounded-lg border border-zinc-300 bg-transparent px-3 py-2 outline-none focus:ring-2 focus:ring-blue-500 dark:border-zinc-700"
							/>
						</div>

						<div>
							<label class="mb-1 block text-sm font-medium">Model</label>
							<input
								type="text"
								bind:value={settings.model}
								placeholder="deepseek/deepseek-v4-flash"
								class="w-full rounded-lg border border-zinc-300 bg-transparent px-3 py-2 outline-none focus:ring-2 focus:ring-blue-500 dark:border-zinc-700"
							/>
						</div>

						<div class="flex items-center justify-between">
							<span class="text-sm font-medium">Enable Thinking</span>
							<button 
								onclick={() => settings.enableThinking = !settings.enableThinking}
								class="relative h-6 w-11 rounded-full transition-colors {settings.enableThinking ? 'bg-blue-600' : 'bg-zinc-300 dark:bg-zinc-700'}"
							>
								<div class="absolute top-1 left-1 h-4 w-4 rounded-full bg-white transition-transform {settings.enableThinking ? 'translate-x-5' : ''}"></div>
							</button>
						</div>

						<div class="flex items-center justify-between">
							<div class="space-y-0.5">
								<span class="text-sm font-medium">Message Compression</span>
								<p class="text-[10px] text-zinc-500">Summarizes history when context is full.</p>
							</div>
							<button 
								onclick={() => settings.enableCompression = !settings.enableCompression}
								class="relative h-6 w-11 rounded-full transition-colors {settings.enableCompression ? 'bg-blue-600' : 'bg-zinc-300 dark:bg-zinc-700'}"
							>
								<div class="absolute top-1 left-1 h-4 w-4 rounded-full bg-white transition-transform {settings.enableCompression ? 'translate-x-5' : ''}"></div>
							</button>
						</div>

						<div>
							<div class="mb-1 flex items-center justify-between">
								<label class="text-sm font-medium">Context Window (Messages)</label>
								<span class="text-xs text-zinc-500">{settings.contextWindow === 0 ? 'All' : settings.contextWindow}</span>
							</div>
							<input
								type="number"
								bind:value={settings.contextWindow}
								min="0"
								step="1"
								placeholder="0 for all messages"
								class="w-full rounded-lg border border-zinc-300 bg-transparent px-3 py-2 outline-none focus:ring-2 focus:ring-blue-500 dark:border-zinc-700"
							/>
							<p class="mt-1 text-xs text-zinc-500">Number of previous messages to include. Set to 0 for unlimited.</p>
						</div>

						<div>
							<div class="mb-1 flex items-center justify-between">
								<label class="text-sm font-medium">Max Agent Turns (Loop Limit)</label>
								<span class="text-xs text-zinc-500">{settings.maxAgentTurns}</span>
							</div>
							<input
								type="number"
								bind:value={settings.maxAgentTurns}
								min="1"
								step="1"
								placeholder="10"
								class="w-full rounded-lg border border-zinc-300 bg-transparent px-3 py-2 outline-none focus:ring-2 focus:ring-blue-500 dark:border-zinc-700"
							/>
							<p class="mt-1 text-xs text-zinc-500">Maximum number of consecutive tool calls in a single agent loop.</p>
						</div>

						<div>
							<label class="mb-2 block text-sm font-medium">Model Capabilities</label>
							<div class="grid grid-cols-2 gap-2">
								<label class="flex items-center gap-2 rounded-lg border border-zinc-200 p-2 dark:border-zinc-800">
									<input type="checkbox" checked disabled class="h-4 w-4 rounded text-blue-600" />
									<span class="text-sm">Text</span>
								</label>
								<label class="flex cursor-pointer items-center gap-2 rounded-lg border border-zinc-200 p-2 hover:bg-zinc-50 dark:border-zinc-800 dark:hover:bg-zinc-900">
									<input type="checkbox" bind:checked={settings.supportsImages} class="h-4 w-4 rounded text-blue-600" />
									<span class="text-sm">Images</span>
								</label>
								<label class="flex cursor-pointer items-center gap-2 rounded-lg border border-zinc-200 p-2 hover:bg-zinc-50 dark:border-zinc-800 dark:hover:bg-zinc-900">
									<input type="checkbox" bind:checked={settings.supportsAudio} class="h-4 w-4 rounded text-blue-600" />
									<span class="text-sm">Audio</span>
								</label>
								<label class="flex cursor-pointer items-center gap-2 rounded-lg border border-zinc-200 p-2 hover:bg-zinc-50 dark:border-zinc-800 dark:hover:bg-zinc-900">
									<input type="checkbox" bind:checked={settings.supportsVideo} class="h-4 w-4 rounded text-blue-600" />
									<span class="text-sm">Video</span>
								</label>
							</div>
						</div>

						<div>
							<label class="mb-2 block text-sm font-medium">Display & Typography</label>
							<div class="grid grid-cols-1 gap-4 rounded-lg border border-zinc-200 p-3 dark:border-zinc-800">
								<div class="grid grid-cols-2 gap-4">
									<div>
										<label class="mb-1 block text-xs font-medium text-zinc-500 uppercase">Font Size (px)</label>
										<input
											type="number"
											bind:value={settings.fontSize}
											min="12"
											max="24"
											class="w-full rounded-md border border-zinc-300 bg-transparent px-2 py-1 text-sm outline-none focus:ring-2 focus:ring-blue-500 dark:border-zinc-700"
										/>
									</div>
									<div>
										<label class="mb-1 block text-xs font-medium text-zinc-500 uppercase">Font Family</label>
										<select
											bind:value={settings.fontFamily}
											class="w-full rounded-md border border-zinc-300 bg-white px-2 py-1 text-sm outline-none focus:ring-2 focus:ring-blue-500 dark:border-zinc-700 dark:bg-zinc-800 dark:text-zinc-100"
										>
											<option value="sans" class="bg-white dark:bg-zinc-800">Sans-serif</option>
											<option value="serif" class="bg-white dark:bg-zinc-800">Serif</option>
											<option value="mono" class="bg-white dark:bg-zinc-800">Monospace</option>
										</select>
									</div>
								</div>
								<div>
									<label class="mb-1 block text-xs font-medium text-zinc-500 uppercase">Theme</label>
									<div class="flex gap-1 rounded-lg bg-zinc-100 p-1 dark:bg-zinc-800">
										{#each ['system', 'light', 'dark'] as t}
											<button
												onclick={() => settings.theme = t as any}
												class="flex-1 rounded-md py-1 text-xs font-medium transition-all {settings.theme === t ? 'bg-white shadow-sm dark:bg-zinc-700 dark:text-white' : 'text-zinc-500 hover:text-zinc-700 dark:hover:text-zinc-300'}"
											>
												{t.charAt(0).toUpperCase() + t.slice(1)}
											</button>
										{/each}
									</div>
								</div>
							</div>
						</div>

						<div>
							<label class="mb-1 block text-sm font-medium">System Prompt</label>
							<textarea
								bind:value={settings.systemPrompt}
								placeholder="You are a helpful assistant."
								class="w-full h-24 rounded-lg border border-zinc-300 bg-transparent px-3 py-2 outline-none focus:ring-2 focus:ring-blue-500 dark:border-zinc-700 resize-none text-sm"
							></textarea>
						</div>

						<div class="pt-4 border-t border-zinc-200 dark:border-zinc-800">
							<label class="mb-2 block text-sm font-bold">MCP Servers</label>
							<div class="space-y-2">
								{#each settings.mcpServers as server, i}
									<div class="flex gap-2">
										<input
											type="text"
											bind:value={settings.mcpServers[i]}
											placeholder="http://localhost:8000/sse"
											class="flex-1 rounded-lg border border-zinc-300 bg-transparent px-3 py-1 text-sm outline-none focus:ring-2 focus:ring-blue-500 dark:border-zinc-700"
										/>
										<button 
											onclick={() => settings.mcpServers = settings.mcpServers.filter((_, j) => i !== j)}
											class="text-red-500 hover:text-red-600 p-1"
										>
											<X size={16} />
										</button>
									</div>
								{/each}
								<button
									onclick={() => settings.mcpServers = [...settings.mcpServers, '']}
									class="text-sm text-blue-600 hover:text-blue-700 font-medium"
								>
									+ Add MCP Server
								</button>
							</div>
						</div>

						<div class="pt-4 border-t border-zinc-200 dark:border-zinc-800">
							<label class="mb-2 block text-sm font-bold">Data Management</label>
							
							<div class="mb-4 space-y-2 rounded-xl bg-zinc-50 p-4 dark:bg-zinc-800/50">
								<p class="text-[10px] font-bold uppercase tracking-wider text-zinc-400">Settings Tag (QR Export)</p>
								<div class="flex gap-2">
									<input 
										type="text" 
										bind:value={exportTagName}
										placeholder="Tag Name (e.g. Coding Pro)"
										class="flex-1 rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 dark:border-zinc-700 dark:bg-zinc-900"
									/>
									<button 
										onclick={generateQR}
										class="flex items-center justify-center gap-2 rounded-lg bg-blue-600 px-3 py-2 text-sm font-medium text-white hover:bg-blue-700"
									>
										<QrCode size={16} />
										Export Tag
									</button>
								</div>
							</div>

							<div class="grid grid-cols-2 gap-2">
								<button 
									onclick={() => settings.exportSettings()}
									class="flex items-center justify-center gap-2 rounded-lg border border-zinc-300 px-3 py-2 text-sm font-medium hover:bg-zinc-50 dark:border-zinc-700 dark:hover:bg-zinc-800"
								>
									<Download size={16} />
									Export JSON
								</button>
								
								<button 
									onclick={() => importInput?.click()}
									class="flex items-center justify-center gap-2 rounded-lg border border-zinc-300 px-3 py-2 text-sm font-medium hover:bg-zinc-50 dark:border-zinc-700 dark:hover:bg-zinc-800"
								>
									<Upload size={16} />
									Import JSON
								</button>

								<button 
									onclick={() => qrImportInput?.click()}
									class="col-span-2 flex items-center justify-center gap-2 rounded-lg border border-zinc-300 px-3 py-2 text-sm font-medium hover:bg-zinc-50 dark:border-zinc-700 dark:hover:bg-zinc-800"
								>
									<ImageIcon size={16} />
									Import from Tag (Image)
								</button>
							</div>

							<input 
								type="file" 
								accept=".json" 
								class="hidden" 
								bind:this={importInput} 
								onchange={handleImport} 
							/>
							<input 
								type="file" 
								accept="image/*" 
								class="hidden" 
								bind:this={qrImportInput} 
								onchange={handleQRImport} 
							/>
						</div>
					</div>
				{/if}
			</div>

			<div class="flex justify-end border-t border-zinc-200 p-6 dark:border-zinc-800">
				<button
					onclick={close}
					class="w-full rounded-lg bg-blue-600 px-4 py-2 font-medium text-white hover:bg-blue-700 transition-colors"
				>
					Save & Close
				</button>
			</div>
		</div>
	</div>
{/if}