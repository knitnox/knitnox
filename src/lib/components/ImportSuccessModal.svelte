<script lang="ts">
	import { X, CheckCircle2 } from '@lucide/svelte';

	let {
		isOpen = $bindable(false),
		modelName = '',
		baseUrl = '',
		apiTokenMasked = '',
		onClose = () => {}
	} = $props<{
		isOpen: boolean;
		modelName?: string;
		baseUrl?: string;
		apiTokenMasked?: string;
		onClose?: () => void;
	}>();

	function handleClose() {
		onClose();
		isOpen = false;
	}

	function portal(node: HTMLElement) {
		document.body.appendChild(node);
		return {
			destroy() {
				if (node.parentNode) {
					node.parentNode.removeChild(node);
				}
			}
		};
	}
</script>

{#if isOpen}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		use:portal
		class="fixed inset-0 z-[100] flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm transition-opacity"
		onclick={(e) => {
			if (e.target === e.currentTarget) handleClose();
		}}
	>
		<div class="w-full max-w-sm rounded-2xl border border-zinc-200 bg-white p-6 shadow-xl dark:border-zinc-800 dark:bg-zinc-900">
			<div class="mb-4 flex items-center justify-between">
				<div class="flex items-center gap-2 text-green-600 dark:text-green-500">
					<CheckCircle2 size={24} />
					<h2 class="text-lg font-bold text-zinc-900 dark:text-zinc-100">Settings Imported</h2>
				</div>
				<button
					onclick={handleClose}
					class="rounded-lg p-1 text-zinc-500 hover:bg-zinc-100 hover:text-zinc-900 dark:hover:bg-zinc-800 dark:hover:text-zinc-100 transition-colors"
				>
					<X size={20} />
				</button>
			</div>
			
			<div class="mb-6 space-y-3">
				<div class="rounded-xl bg-zinc-50 dark:bg-zinc-800/50 p-3 text-sm">
					<div class="mb-1 text-xs font-semibold text-zinc-500 dark:text-zinc-400">Model</div>
					<div class="font-medium text-zinc-900 dark:text-zinc-100 break-all">{modelName || 'Not specified'}</div>
				</div>
				<div class="rounded-xl bg-zinc-50 dark:bg-zinc-800/50 p-3 text-sm">
					<div class="mb-1 text-xs font-semibold text-zinc-500 dark:text-zinc-400">Base URL</div>
					<div class="font-medium text-zinc-900 dark:text-zinc-100 break-all">{baseUrl || 'Not specified'}</div>
				</div>
				<div class="rounded-xl bg-zinc-50 dark:bg-zinc-800/50 p-3 text-sm">
					<div class="mb-1 text-xs font-semibold text-zinc-500 dark:text-zinc-400">API Token</div>
					<div class="font-medium text-zinc-900 dark:text-zinc-100 break-all">{apiTokenMasked || 'Not provided'}</div>
				</div>
			</div>
			
			<div class="flex justify-end">
				<button
					onclick={handleClose}
					class="rounded-xl bg-blue-600 px-6 py-2 text-sm font-medium text-white hover:bg-blue-700 transition-colors shadow-sm w-full"
				>
					Continue
				</button>
			</div>
		</div>
	</div>
{/if}