<script lang="ts">
	import { settings } from '$lib/settings.svelte';
	import { MCPClient } from '$lib/mcp.svelte';
	import { X, FileText, Terminal, Search, Loader2, Library } from '@lucide/svelte';
	import { fade, fly } from 'svelte/transition';

	let { isOpen = $bindable(false) } = $props<{
		isOpen: boolean;
	}>();

	type Tab = 'resources' | 'prompts';
	let activeTab = $state<Tab>('resources');

	let resources = $state<any[]>([]);
	let prompts = $state<any[]>([]);
	let isLoading = $state(false);
	let searchQuery = $state('');

	async function fetchAll() {
		isLoading = true;
		resources = [];
		prompts = [];
		try {
			const allResources: any[] = [];
			const allPrompts: any[] = [];
			for (const url of settings.mcpServers) {
				if (!url) continue;
				const client = new MCPClient(url);
				try {
					const r = await client.listResources();
					const p = await client.listPrompts();
					allResources.push(...r.map((item: any) => ({ ...item, serverUrl: url })));
					allPrompts.push(...p.map((item: any) => ({ ...item, serverUrl: url })));
				} catch (e) {
					// Silently skip offline servers
				}
			}
			resources = allResources;
			prompts = allPrompts;
		} catch (e) {
			console.error('Failed to fetch MCP items:', e);
		} finally {
			isLoading = false;
		}
	}

	$effect(() => {
		if (isOpen) {
			activeTab = 'resources';
			searchQuery = '';
			fetchAll();
		}
	});

	const filteredItems = $derived(
		(activeTab === 'resources' ? resources : prompts).filter(item =>
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
			class="relative flex flex-col w-full max-w-2xl h-[85vh] sm:h-[600px] rounded-t-3xl sm:rounded-3xl bg-white shadow-2xl dark:bg-zinc-900 overflow-hidden"
			onclick={e => e.stopPropagation()}
			transition:fly={{ y: 100, duration: 300, opacity: 1 }}
		>
			<!-- Header -->
			<div class="flex items-center justify-between border-b border-zinc-100 p-4 sm:p-6 dark:border-zinc-800 shrink-0">
				<div class="flex items-center gap-3">
					<div class="rounded-xl p-2 bg-zinc-100 text-zinc-600 dark:bg-zinc-800">
						<Library size={20} />
					</div>
					<div>
						<h2 class="text-base sm:text-lg font-bold uppercase tracking-wider">MCP Library</h2>
						<p class="text-[10px] sm:text-xs text-zinc-500">Resources & Prompts from connected servers</p>
					</div>
				</div>
				<button onclick={close} class="rounded-full p-2 hover:bg-zinc-100 dark:hover:bg-zinc-800 transition-colors">
					<X size={20} />
				</button>
			</div>

			<!-- Tab Bar -->
			<div class="flex border-b border-zinc-100 dark:border-zinc-800 shrink-0">
				<button
					onclick={() => { activeTab = 'resources'; searchQuery = ''; }}
					class="flex-1 flex items-center justify-center gap-2 py-3 text-sm font-bold transition-all border-b-2 {activeTab === 'resources' ? 'border-zinc-900 dark:border-white text-zinc-900 dark:text-white' : 'border-transparent text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-300'}"
				>
					<FileText size={16} />
					Resources
					{#if resources.length > 0}
						<span class="text-[10px] bg-zinc-100 dark:bg-zinc-800 px-1.5 py-0.5 rounded-full">{resources.length}</span>
					{/if}
				</button>
				<button
					onclick={() => { activeTab = 'prompts'; searchQuery = ''; }}
					class="flex-1 flex items-center justify-center gap-2 py-3 text-sm font-bold transition-all border-b-2 {activeTab === 'prompts' ? 'border-zinc-900 dark:border-white text-zinc-900 dark:text-white' : 'border-transparent text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-300'}"
				>
					<Terminal size={16} />
					Prompts
					{#if prompts.length > 0}
						<span class="text-[10px] bg-zinc-100 dark:bg-zinc-800 px-1.5 py-0.5 rounded-full">{prompts.length}</span>
					{/if}
				</button>
			</div>

			<!-- Search -->
			<div class="p-3 sm:p-4 border-b border-zinc-100 dark:border-zinc-800 shrink-0">
				<div class="relative">
					<Search size={16} class="absolute left-3 top-1/2 -translate-y-1/2 text-zinc-400" />
					<input 
						type="text" 
						bind:value={searchQuery}
						placeholder="Search {activeTab}..."
						class="w-full bg-zinc-50 dark:bg-zinc-950 border border-zinc-200 dark:border-zinc-800 rounded-xl pl-10 pr-4 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
					/>
				</div>
			</div>

			<!-- List Content -->
			<div class="flex-1 overflow-y-auto p-3 sm:p-4 custom-scrollbar">
				{#if isLoading}
					<div class="flex flex-col items-center justify-center h-full gap-3 text-zinc-400">
						<Loader2 size={32} class="animate-spin text-blue-500" />
						<p class="text-sm font-medium">Scanning MCP servers...</p>
					</div>
				{:else if filteredItems.length === 0}
					<div class="flex flex-col items-center justify-center h-full text-center p-6 sm:p-8">
						<div class="rounded-full bg-zinc-100 dark:bg-zinc-800 p-4 mb-4">
							{#if activeTab === 'resources'}
								<FileText size={28} class="text-zinc-300" />
							{:else}
								<Terminal size={28} class="text-zinc-300" />
							{/if}
						</div>
						<p class="text-sm font-medium text-zinc-500">No {activeTab} found.</p>
						<p class="text-xs text-zinc-400 mt-1">Make sure your MCP servers are connected and support {activeTab}.</p>
					</div>
				{:else}
					<div class="grid gap-2 sm:gap-3">
						{#each filteredItems as item}
							<div class="group flex flex-col gap-1.5 sm:gap-2 rounded-xl sm:rounded-2xl border border-zinc-100 bg-zinc-50/50 p-3 sm:p-4 dark:border-zinc-800 dark:bg-zinc-800/30 hover:bg-white dark:hover:bg-zinc-800 transition-all border-l-4 border-l-transparent hover:border-l-zinc-500">
								<div class="flex items-start justify-between gap-2 sm:gap-4">
									<div class="flex-1 min-w-0">
										<div class="flex items-center gap-2 mb-0.5 sm:mb-1">
											<span class="font-bold text-xs sm:text-sm truncate">{item.name || item.uri}</span>
											{#if item.mimeType}
												<span class="text-[9px] sm:text-[10px] bg-zinc-200 dark:bg-zinc-700 px-1.5 py-0.5 rounded-md text-zinc-600 dark:text-zinc-300 font-mono truncate max-w-[80px] sm:max-w-none">{item.mimeType}</span>
											{/if}
										</div>
										<p class="text-[11px] sm:text-xs text-zinc-500 line-clamp-2 leading-relaxed">
											{item.description || 'No description provided.'}
										</p>
									</div>
									<div class="flex flex-col items-end gap-2 shrink-0">
										<span class="text-[8px] sm:text-[9px] font-bold text-zinc-400 uppercase tracking-tighter bg-zinc-100 dark:bg-zinc-900 px-1.5 py-0.5 rounded">
											{new URL(item.serverUrl).hostname}
										</span>
									</div>
								</div>
								
								{#if activeTab === 'resources'}
									<div class="flex items-center gap-1.5 sm:gap-2 mt-1.5 sm:mt-2 pt-1.5 sm:pt-2 border-t border-zinc-100 dark:border-zinc-700/50">
										<code class="text-[9px] sm:text-[10px] text-zinc-400 truncate flex-1">{item.uri}</code>
										<button class="flex items-center gap-1 sm:gap-1.5 text-[9px] sm:text-[10px] font-bold text-zinc-600 hover:text-zinc-900 bg-zinc-100 dark:bg-zinc-800 px-1.5 sm:px-2 py-1 rounded-lg transition-colors shrink-0">
											<FileText size={11} class="sm:w-3 sm:h-3" />
											Open
										</button>
									</div>
								{:else}
									<div class="flex items-center gap-1.5 sm:gap-2 mt-1.5 sm:mt-2 pt-1.5 sm:pt-2 border-t border-zinc-100 dark:border-zinc-700/50">
										<div class="flex-1 flex gap-1 flex-wrap">
											{#if item.arguments}
												{#each item.arguments as arg}
													<span class="text-[8px] sm:text-[9px] bg-zinc-200 dark:bg-zinc-700 px-1 rounded text-zinc-500">{arg.name}</span>
												{/each}
											{/if}
										</div>
										<button class="flex items-center gap-1 sm:gap-1.5 text-[9px] sm:text-[10px] font-bold text-zinc-600 hover:text-zinc-900 bg-zinc-100 dark:bg-zinc-800 px-1.5 sm:px-2 py-1 rounded-lg transition-colors shrink-0">
											<Terminal size={11} class="sm:w-3 sm:h-3" />
											Use
										</button>
									</div>
								{/if}
							</div>
						{/each}
					</div>
				{/if}
			</div>

			<!-- Footer -->
			<div class="bg-zinc-50/50 p-3 sm:p-4 dark:bg-zinc-800/50 border-t border-zinc-100 dark:border-zinc-800 shrink-0">
				<p class="text-[9px] sm:text-[10px] font-bold text-center text-zinc-400 uppercase tracking-widest">
					{resources.length} resources • {prompts.length} prompts
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
	:global(.dark) .custom-scrollbar::-webkit-scrollbar-thumb { background: #3f3f46; }
</style>