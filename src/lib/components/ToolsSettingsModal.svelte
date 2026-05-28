<script lang="ts">
	import { settings } from '$lib/settings.svelte';
	import { localTools } from '$lib/tools.svelte';
	import { getAllTools, type MCPTool } from '$lib/mcp.svelte';
	import { X, Wrench, Cpu, Loader2, Blocks } from '@lucide/svelte';
	import { fade, fly } from 'svelte/transition';

	let { isOpen = $bindable(false) } = $props();

	let mcpTools = $state<{ tool: MCPTool; serverUrl: string }[]>([]);
	let isLoading = $state(false);

	async function fetchMcpTools() {
		isLoading = true;
		try {
			mcpTools = await getAllTools(settings.mcpServers);
		} catch (e) {
			console.error('Failed to fetch MCP tools:', e);
		} finally {
			isLoading = false;
		}
	}

	$effect(() => {
		if (isOpen) {
			fetchMcpTools();
		}
	});

	function toggleTool(name: string) {
		if (settings.disabledTools.includes(name)) {
			settings.disabledTools = settings.disabledTools.filter(n => n !== name);
		} else {
			settings.disabledTools = [...settings.disabledTools, name];
		}
	}

	function close() {
		isOpen = false;
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
			class="relative w-full max-w-2xl overflow-hidden rounded-t-3xl sm:rounded-3xl bg-white shadow-2xl dark:bg-zinc-900 border border-zinc-200 dark:border-zinc-800"
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
						<Wrench size={20} />
					</div>
					<div>
						<h2 class="text-lg font-bold uppercase tracking-wider">Tool Inventory</h2>
						<p class="text-xs text-zinc-500">Configure active extensions and built-ins</p>
					</div>
				</div>
				<button onclick={close} class="rounded-full p-2 hover:bg-zinc-100 dark:hover:bg-zinc-800 transition-colors">
					<X size={20} />
				</button>
			</div>

			<div class="max-h-[60vh] overflow-y-auto p-6 space-y-8 custom-scrollbar">
				<!-- Local Tools -->
				<section class="space-y-4">
					<h3 class="flex items-center gap-2 text-xs font-bold uppercase tracking-widest text-zinc-400">
						<Blocks size={14} /> Core Capabilities
					</h3>
					<div class="grid gap-2">
						{#each localTools as tool}
							<div class="group flex items-center justify-between gap-4 rounded-2xl border border-zinc-100 bg-zinc-50/50 p-4 dark:border-zinc-800 dark:bg-zinc-800/30 hover:bg-white dark:hover:bg-zinc-800 transition-all">
								<div class="flex flex-1 flex-col overflow-hidden">
									<span class="truncate text-sm font-bold">{tool.name}</span>
									<span class="text-xs text-zinc-500 leading-relaxed">{tool.description}</span>
								</div>
								<button 
									onclick={() => toggleTool(tool.name)}
									class="relative h-6 w-11 flex-shrink-0 rounded-full transition-colors {!settings.disabledTools.includes(tool.name) ? 'bg-blue-600' : 'bg-zinc-300 dark:bg-zinc-700'}"
									aria-label="Toggle tool"
								>
									<div class="absolute top-1 left-1 h-4 w-4 rounded-full bg-white transition-transform {!settings.disabledTools.includes(tool.name) ? 'translate-x-5' : ''}"></div>
								</button>
							</div>
						{/each}
					</div>
				</section>

				<!-- MCP Tools -->
				<section class="space-y-4">
					<div class="flex items-center justify-between">
						<h3 class="flex items-center gap-2 text-xs font-bold uppercase tracking-widest text-zinc-400">
							<Cpu size={14} /> MCP Extensions
						</h3>
						{#if isLoading}
							<Loader2 size={14} class="animate-spin text-blue-500" />
						{/if}
					</div>
					
					{#if mcpTools.length === 0 && !isLoading}
						<div class="rounded-2xl border-2 border-dashed border-zinc-200 p-10 text-center dark:border-zinc-800">
							<Cpu size={32} class="mx-auto mb-3 text-zinc-300" />
							<p class="text-sm font-medium text-zinc-500">No external tools connected.</p>
							<p class="text-[10px] text-zinc-400 mt-1 uppercase tracking-tight">Connect servers in main settings</p>
						</div>
					{:else}
						<div class="grid gap-2">
							{#each mcpTools as { tool }}
								<div class="group flex items-center justify-between gap-4 rounded-2xl border border-zinc-100 bg-zinc-50/50 p-4 dark:border-zinc-800 dark:bg-zinc-800/30 hover:bg-white dark:hover:bg-zinc-800 transition-all">
									<div class="flex flex-col flex-1 min-w-0">
										<div class="flex items-center gap-2">
											<span class="text-sm font-bold truncate">{tool.name}</span>
											<span class="rounded-md bg-blue-50 px-1.5 py-0.5 text-[10px] font-bold text-blue-600 dark:bg-blue-900/30">MCP</span>
										</div>
										<span class="text-xs text-zinc-500 truncate">{tool.description}</span>
									</div>
									<button 
										onclick={() => toggleTool(tool.name)}
										class="relative h-6 w-11 flex-shrink-0 rounded-full transition-colors {!settings.disabledTools.includes(tool.name) ? 'bg-blue-600' : 'bg-zinc-300 dark:bg-zinc-700'}"
										aria-label="Toggle tool"
									>
										<div class="absolute top-1 left-1 h-4 w-4 rounded-full bg-white transition-transform {!settings.disabledTools.includes(tool.name) ? 'translate-x-5' : ''}"></div>
									</button>
								</div>
							{/each}
						</div>
					{/if}
				</section>
			</div>

			<div class="bg-zinc-50/50 p-6 dark:bg-zinc-800/50 border-t border-zinc-100 dark:border-zinc-800">
				<p class="text-[10px] font-bold text-center text-zinc-400 uppercase tracking-widest">
					Configurations are applied in real-time
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
