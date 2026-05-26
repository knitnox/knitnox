<script lang="ts">
	import { settings } from '$lib/settings.svelte';
	import { localTools } from '$lib/tools.svelte';
	import { getAllTools, type MCPTool } from '$lib/mcp.svelte';
	import { X, Wrench, Cpu, Loader2 } from '@lucide/svelte';
	import { onMount } from 'svelte';
	import { fade, slide } from 'svelte/transition';

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
		class="fixed inset-0 z-50 flex items-end justify-center bg-black/20 backdrop-blur-[2px] p-4 sm:p-6"
		onclick={close}
		transition:fade={{ duration: 200 }}
	>
		<div 
			class="w-full max-w-2xl overflow-hidden rounded-2xl bg-white shadow-2xl dark:bg-zinc-900 border border-zinc-200 dark:border-zinc-800"
			onclick={e => e.stopPropagation()}
			transition:slide={{ axis: 'y', duration: 300 }}
		>
			<div class="flex items-center justify-between border-b border-zinc-100 p-4 dark:border-zinc-800">
				<div class="flex items-center gap-2">
					<div class="rounded-lg bg-blue-50 p-1.5 text-blue-600 dark:bg-blue-900/30">
						<Wrench size={18} />
					</div>
					<h2 class="text-sm font-bold uppercase tracking-wider text-zinc-500">Tool Settings</h2>
				</div>
				<button onclick={close} class="rounded-lg p-1 hover:bg-zinc-100 dark:hover:bg-zinc-800">
					<X size={20} />
				</button>
			</div>

			<div class="max-h-[60vh] overflow-y-auto p-4 custom-scrollbar">
				<div class="space-y-6">
					<!-- Local Tools -->
					<div>
						<h3 class="mb-3 flex items-center gap-2 text-xs font-bold uppercase tracking-widest text-zinc-400">
							Built-in Tools
						</h3>
						<div class="grid gap-2">
							{#each localTools as tool}
								<div class="flex items-center justify-between gap-4 rounded-xl border border-zinc-100 bg-zinc-50/50 p-3 dark:border-zinc-800 dark:bg-zinc-800/30">
									<div class="flex flex-1 flex-col overflow-hidden">
										<span class="truncate text-sm font-semibold">{tool.name}</span>
										<span class="text-xs text-zinc-500">{tool.description}</span>
									</div>
									<button 
										onclick={() => toggleTool(tool.name)}
										class="relative h-6 w-11 flex-shrink-0 rounded-full transition-colors {!settings.disabledTools.includes(tool.name) ? 'bg-blue-600' : 'bg-zinc-300 dark:bg-zinc-700'}"
									>
										<div class="absolute top-1 left-1 h-4 w-4 rounded-full bg-white transition-transform {!settings.disabledTools.includes(tool.name) ? 'translate-x-5' : ''}"></div>
									</button>
								</div>
							{/each}
						</div>
					</div>

					<!-- MCP Tools -->
					<div>
						<div class="mb-3 flex items-center justify-between">
							<h3 class="flex items-center gap-2 text-xs font-bold uppercase tracking-widest text-zinc-400">
								MCP Tools
							</h3>
							{#if isLoading}
								<Loader2 size={14} class="animate-spin text-zinc-400" />
							{/if}
						</div>
						
						{#if mcpTools.length === 0 && !isLoading}
							<div class="rounded-xl border border-dashed border-zinc-200 p-8 text-center dark:border-zinc-800">
								<Cpu size={24} class="mx-auto mb-2 text-zinc-300" />
								<p class="text-xs text-zinc-500">No MCP tools found. Add servers in main settings.</p>
							</div>
						{:else}
							<div class="grid gap-2">
								{#each mcpTools as { tool, serverUrl }}
									<div class="flex items-center justify-between rounded-xl border border-zinc-100 bg-zinc-50/50 p-3 dark:border-zinc-800 dark:bg-zinc-800/30">
										<div class="flex flex-col">
											<div class="flex items-center gap-2">
												<span class="text-sm font-semibold">{tool.name}</span>
												<span class="rounded-md bg-zinc-100 px-1.5 py-0.5 text-[10px] text-zinc-500 dark:bg-zinc-800">MCP</span>
											</div>
											<span class="text-xs text-zinc-500">{tool.description}</span>
										</div>
										<button 
											onclick={() => toggleTool(tool.name)}
											class="relative h-6 w-11 rounded-full transition-colors {!settings.disabledTools.includes(tool.name) ? 'bg-blue-600' : 'bg-zinc-300 dark:bg-zinc-700'}"
										>
											<div class="absolute top-1 left-1 h-4 w-4 rounded-full bg-white transition-transform {!settings.disabledTools.includes(tool.name) ? 'translate-x-5' : ''}"></div>
										</button>
									</div>
								{/each}
							</div>
						{/if}
					</div>
				</div>
			</div>

			<div class="bg-zinc-50 p-4 dark:bg-zinc-800/50">
				<p class="text-[10px] text-center text-zinc-400">
					Changes are saved automatically and will take effect in the next message.
				</p>
			</div>
		</div>
	</div>
{/if}
