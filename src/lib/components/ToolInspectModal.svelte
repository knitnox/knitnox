<script lang="ts">
	import { X, Wrench, Code } from '@lucide/svelte';
	import { fade, scale } from 'svelte/transition';

	let { isOpen = $bindable(false), toolName, args, result } = $props<{
		isOpen: boolean;
		toolName: string;
		args: any;
		result: any;
	}>();

	function close() {
		isOpen = false;
	}
</script>

{#if isOpen}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div 
		class="fixed inset-0 z-[60] flex items-center justify-center bg-black/40 backdrop-blur-sm p-4"
		onclick={close}
		transition:fade={{ duration: 150 }}
	>
		<div 
			class="w-full max-w-lg overflow-hidden rounded-2xl bg-white shadow-2xl dark:bg-zinc-900 border border-zinc-200 dark:border-zinc-800"
			onclick={e => e.stopPropagation()}
			transition:scale={{ duration: 200, start: 0.95 }}
		>
			<div class="flex items-center justify-between border-b border-zinc-100 p-4 dark:border-zinc-800">
				<div class="flex items-center gap-2">
					<div class="rounded-lg bg-blue-50 p-1.5 text-blue-600 dark:bg-blue-900/30">
						<Wrench size={16} />
					</div>
					<h2 class="text-sm font-bold uppercase tracking-wider text-zinc-500">Tool Execution Details</h2>
				</div>
				<button onclick={close} class="rounded-lg p-1 hover:bg-zinc-100 dark:hover:bg-zinc-800">
					<X size={18} />
				</button>
			</div>

			<div class="max-h-[70vh] overflow-y-auto p-4 space-y-4 custom-scrollbar">
				<div>
					<div class="mb-1 text-[10px] font-bold uppercase tracking-widest text-zinc-400">Tool Name</div>
					<div class="font-mono text-sm font-semibold text-blue-600 dark:text-blue-400">{toolName}</div>
				</div>

				<div>
					<div class="mb-1 text-[10px] font-bold uppercase tracking-widest text-zinc-400">Arguments</div>
					<pre class="overflow-x-auto rounded-lg bg-zinc-50 p-3 text-xs dark:bg-zinc-950 border border-zinc-100 dark:border-zinc-800 font-mono">{JSON.stringify(args, null, 2)}</pre>
				</div>

				<div>
					<div class="mb-1 text-[10px] font-bold uppercase tracking-widest text-zinc-400">Result</div>
					<pre class="overflow-x-auto rounded-lg bg-zinc-50 p-3 text-xs dark:bg-zinc-950 border border-zinc-100 dark:border-zinc-800 font-mono text-green-600 dark:text-green-400">{JSON.stringify(result, null, 2)}</pre>
				</div>
			</div>

			<div class="bg-zinc-50 p-3 dark:bg-zinc-800/50 flex justify-end">
				<button 
					onclick={close}
					class="text-xs font-bold uppercase tracking-widest text-zinc-500 hover:text-zinc-700 dark:hover:text-zinc-300"
				>
					Close
				</button>
			</div>
		</div>
	</div>
{/if}
