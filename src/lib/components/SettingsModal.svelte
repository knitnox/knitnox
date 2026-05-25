<script lang="ts">
	import { settings } from '$lib/settings.svelte';
	import { X } from '@lucide/svelte';


	let { isOpen = $bindable(false) } = $props();

	function close() {
		isOpen = false;
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
							placeholder="gpt-4o"
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
						<div class="grid grid-cols-2 gap-4 rounded-lg border border-zinc-200 p-3 dark:border-zinc-800">
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
							</div>						</div>
					</div>

					<div>
						<label class="mb-1 block text-sm font-medium">System Prompt</label>
						<textarea
							bind:value={settings.systemPrompt}
							placeholder="You are a helpful assistant."
							class="w-full h-24 rounded-lg border border-zinc-300 bg-transparent px-3 py-2 outline-none focus:ring-2 focus:ring-blue-500 dark:border-zinc-700 resize-none text-sm"
						></textarea>
					</div>

					<div class="rounded-xl border border-zinc-200 bg-zinc-50 p-4 dark:border-zinc-800 dark:bg-zinc-800/50">
						<h3 class="mb-3 text-sm font-bold uppercase tracking-wider text-zinc-500">Token Usage Statistics</h3>
						<div class="grid grid-cols-2 gap-4">
							<div class="space-y-1">
								<p class="text-xs text-zinc-500 font-medium">TOTAL INPUT</p>
								<p class="text-lg font-mono font-bold text-blue-600">{(settings.totalInputTokens).toLocaleString()}</p>
							</div>
							<div class="space-y-1">
								<p class="text-xs text-zinc-500 font-medium">TOTAL OUTPUT</p>
								<p class="text-lg font-mono font-bold text-green-600">{(settings.totalOutputTokens).toLocaleString()}</p>
							</div>
							<div class="space-y-1">
								<p class="text-xs text-zinc-500 font-medium">LAST INPUT</p>
								<p class="text-sm font-mono font-semibold">{settings.lastInputTokens}</p>
							</div>
							<div class="space-y-1">
								<p class="text-xs text-zinc-500 font-medium">LAST OUTPUT</p>
								<p class="text-sm font-mono font-semibold">{settings.lastOutputTokens}</p>
							</div>
						</div>
						<button 
							onclick={() => {
								if (confirm('Are you sure you want to reset total token counts?')) {
									settings.totalInputTokens = 0;
									settings.totalOutputTokens = 0;
								}
							}}
							class="mt-4 text-[10px] font-bold text-red-500 hover:text-red-600 uppercase tracking-tighter"
						>
							Reset All Time Totals
						</button>
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
				</div>
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
