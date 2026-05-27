<script lang="ts">
	import { settings } from '$lib/settings.svelte';
	import { MCPClient } from '$lib/mcp.svelte';
	import { X, FileText, Terminal, ExternalLink, Search, Loader2, Database, Zap, Library } from '@lucide/svelte';
	import { fade, fly } from 'svelte/transition';

	let { isOpen = $bindable(false), mode = $bindable('resources') } = $props<{
		isOpen: boolean;
		mode: 'resources' | 'prompts';
	}>();

	let items = $state<any[]>([]);
	let isLoading = $state(false);
	let searchQuery = $state('');

	async function fetchItems(targetMode: 'resources' | 'prompts') {
		isLoading = true;
		items = [];
		try {
			const allItems: any[] = [];
			for (const url of settings.mcpServers) {
				if (!url) continue;
				const client = new MCPClient(url);
				const serverItems = targetMode === 'resources' 
					? await client.listResources() 
					: await client.listPrompts();
				
				allItems.push(...serverItems.map((item: any) => ({ ...item, serverUrl: url })));
			}
			// Only update if the mode hasn't changed during the fetch
			if (mode === targetMode) {
				items = allItems;
			}
		} catch (e) {
			console.error(`Failed to fetch ${targetMode}:`, e);
		} finally {
			if (mode === targetMode) {
				isLoading = false;
			}
		}
	}

	$effect(() => {
		if (isOpen) {
			fetchItems(mode);
		}
	});

	const filteredItems = $derived(
		items.filter(item => 
			(item.name || item.uri || '').toLowerCase().includes(searchQuery.toLowerCase()) ||
			(item.description || '').toLowerCase().includes(searchQuery.toLowerCase())
		)
	);

	function close() {
		isOpen = false;
	}
</script>

{#if isOpen}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div 
		class="fixed inset-0 z-[60] flex items-end justify-center bg-black/40 backdrop-blur-[2px] p-0 sm:items-center sm:p-4"
		onclick={close}
		transition:fade={{ duration: 200 }}
	>
		<div 
			class="relative flex flex-col w-full max-w-2xl h-[80vh] sm:h-[600px] rounded-t-3xl sm:rounded-3xl bg-white shadow-2xl dark:bg-zinc-900 overflow-hidden"
			onclick={e => e.stopPropagation()}
			transition:fly={{ y: 100, duration: 300, opacity: 1 }}
		>
			<!-- Header -->
			<div class="flex items-center justify-between border-b border-zinc-100 p-6 dark:border-zinc-800">
				<div class="flex items-center gap-3">
					<div class="rounded-xl p-2 {mode === 'resources' ? 'bg-zinc-100 text-zinc-600 dark:bg-zinc-800' : 'bg-zinc-100 text-zinc-600 dark:bg-zinc-800'}">
						{#if mode === 'resources'}
							<Library size={20} />
						{:else}
							<Terminal size={20} />
						{/if}
					</div>
					<div>
						<h2 class="text-lg font-bold uppercase tracking-wider">{mode === 'resources' ? 'MCP Resources' : 'MCP Prompts'}</h2>
						<p class="text-xs text-zinc-500">Available from connected servers</p>
					</div>
				</div>
				<button onclick={close} class="rounded-full p-2 hover:bg-zinc-100 dark:hover:bg-zinc-800 transition-colors">
					<X size={20} />
				</button>
			</div>

			<!-- Tabs/Search -->
			<div class="p-4 border-b border-zinc-100 dark:border-zinc-800 space-y-4">
				<div class="flex gap-1 p-1 bg-zinc-100 dark:bg-zinc-800 rounded-xl">
					<button 
						onclick={() => { mode = 'resources'; }}
						class="flex-1 flex items-center justify-center gap-2 py-2 text-xs font-bold rounded-lg transition-all {mode === 'resources' ? 'bg-white dark:bg-zinc-700 shadow-sm text-zinc-900 dark:text-white' : 'text-zinc-500'}"
					>
						<Library size={14} />
						Resources
					</button>
					<button 
						onclick={() => { mode = 'prompts'; }}
						class="flex-1 flex items-center justify-center gap-2 py-2 text-xs font-bold rounded-lg transition-all {mode === 'prompts' ? 'bg-white dark:bg-zinc-700 shadow-sm text-zinc-900 dark:text-white' : 'text-zinc-500'}"
					>
						<Terminal size={14} />
						Prompts
					</button>
				</div>

				<div class="relative">
					<Search size={16} class="absolute left-3 top-1/2 -translate-y-1/2 text-zinc-400" />
					<input 
						type="text" 
						bind:value={searchQuery}
						placeholder="Search {mode}..."
						class="w-full bg-zinc-50 dark:bg-zinc-950 border border-zinc-200 dark:border-zinc-800 rounded-xl pl-10 pr-4 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
					/>
				</div>
			</div>

			<!-- List Content -->
			<div class="flex-1 overflow-y-auto p-4 custom-scrollbar">
				{#if isLoading}
					<div class="flex flex-col items-center justify-center h-full gap-3 text-zinc-400">
						<Loader2 size={32} class="animate-spin text-blue-500" />
						<p class="text-sm font-medium">Scanning MCP servers...</p>
					</div>
				{:else if filteredItems.length === 0}
					<div class="flex flex-col items-center justify-center h-full text-center p-8">
						<div class="rounded-full bg-zinc-100 dark:bg-zinc-800 p-4 mb-4">
							{#if mode === 'resources'}
								<Library size={32} class="text-zinc-300" />
							{:else}
								<Terminal size={32} class="text-zinc-300" />
							{/if}
						</div>
						<p class="text-sm font-medium text-zinc-500">No {mode} found.</p>
						<p class="text-xs text-zinc-400 mt-1">Make sure your MCP servers are connected and support {mode}.</p>
					</div>
				{:else}
					<div class="grid gap-3">
						{#each filteredItems as item}
							<div class="group flex flex-col gap-2 rounded-2xl border border-zinc-100 bg-zinc-50/50 p-4 dark:border-zinc-800 dark:bg-zinc-800/30 hover:bg-white dark:hover:bg-zinc-800 transition-all border-l-4 border-l-transparent hover:border-l-zinc-500">
								<div class="flex items-start justify-between gap-4">
									<div class="flex-1 min-w-0">
										<div class="flex items-center gap-2 mb-1">
											<span class="font-bold text-sm truncate">{item.name || item.uri}</span>
											{#if item.mimeType}
												<span class="text-[10px] bg-zinc-200 dark:bg-zinc-700 px-1.5 py-0.5 rounded-md text-zinc-600 dark:text-zinc-300 font-mono">{item.mimeType}</span>
											{/if}
										</div>
										<p class="text-xs text-zinc-500 line-clamp-2 leading-relaxed">
											{item.description || 'No description provided.'}
										</p>
									</div>
									<div class="flex flex-col items-end gap-2">
										<span class="text-[9px] font-bold text-zinc-400 uppercase tracking-tighter bg-zinc-100 dark:bg-zinc-900 px-1.5 py-0.5 rounded">
											{new URL(item.serverUrl).hostname}
										</span>
									</div>
								</div>
								
								{#if mode === 'resources'}
									<div class="flex items-center gap-2 mt-2 pt-2 border-t border-zinc-100 dark:border-zinc-700/50">
										<code class="text-[10px] text-zinc-400 truncate flex-1">{item.uri}</code>
										<button class="flex items-center gap-1.5 text-[10px] font-bold text-zinc-600 hover:text-zinc-900 bg-zinc-100 dark:bg-zinc-800 px-2 py-1 rounded-lg transition-colors">
											<FileText size={12} />
											Open Resource
										</button>
									</div>
								{:else}
									<div class="flex items-center gap-2 mt-2 pt-2 border-t border-zinc-100 dark:border-zinc-700/50">
										<div class="flex-1 flex gap-1">
											{#if item.arguments}
												{#each item.arguments as arg}
													<span class="text-[9px] bg-zinc-200 dark:bg-zinc-700 px-1 rounded text-zinc-500">{arg.name}</span>
												{/each}
											{/if}
										</div>
										<button class="flex items-center gap-1.5 text-[10px] font-bold text-zinc-600 hover:text-zinc-900 bg-zinc-100 dark:bg-zinc-800 px-2 py-1 rounded-lg transition-colors">
											<Terminal size={12} />
											Use Prompt
										</button>
									</div>
								{/if}
							</div>
						{/each}
					</div>
				{/if}
			</div>

			<div class="bg-zinc-50/50 p-4 dark:bg-zinc-800/50 border-t border-zinc-100 dark:border-zinc-800">
				<p class="text-[10px] font-bold text-center text-zinc-400 uppercase tracking-widest">
					Select an item to interact with its content
				</p>
			</div>
		</div>
	</div>
{/if}

<style>
	.custom-scrollbar::-webkit-scrollbar {
		width: 4px;
	}
	.custom-scrollbar::-webkit-scrollbar-track { background: transparent; }
	.custom-scrollbar::-webkit-scrollbar-thumb {
		background: #e4e4e7;
		border-radius: 10px;
	}
	.dark .custom-scrollbar::-webkit-scrollbar-thumb { background: #3f3f46; }
</style>