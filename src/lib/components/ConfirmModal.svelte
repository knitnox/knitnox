<script lang="ts">
	import { X } from '@lucide/svelte';

	let {
		isOpen = $bindable(false),
		title = 'Confirm',
		message = 'Are you sure you want to proceed?',
		confirmText = 'Confirm',
		cancelText = 'Cancel',
		isDanger = false,
		onConfirm = () => {},
		onCancel = () => {}
	} = $props<{
		isOpen: boolean;
		title?: string;
		message?: string;
		confirmText?: string;
		cancelText?: string;
		isDanger?: boolean;
		onConfirm?: () => void;
		onCancel?: () => void;
	}>();

	function handleConfirm() {
		onConfirm();
		isOpen = false;
	}

	function handleCancel() {
		onCancel();
		isOpen = false;
	}
</script>

{#if isOpen}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="fixed inset-0 z-[100] flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm transition-opacity"
		onclick={(e) => {
			if (e.target === e.currentTarget) handleCancel();
		}}
	>
		<div class="w-full max-w-sm rounded-2xl border border-zinc-200 bg-white p-6 shadow-xl dark:border-zinc-800 dark:bg-zinc-900">
			<div class="mb-4 flex items-center justify-between">
				<h2 class="text-lg font-bold text-zinc-900 dark:text-zinc-100">{title}</h2>
				<button
					onclick={handleCancel}
					class="rounded-lg p-1 text-zinc-500 hover:bg-zinc-100 hover:text-zinc-900 dark:hover:bg-zinc-800 dark:hover:text-zinc-100"
				>
					<X size={20} />
				</button>
			</div>
			<p class="mb-6 text-sm text-zinc-600 dark:text-zinc-400">
				{message}
			</p>
			<div class="flex justify-end gap-3">
				<button
					onclick={handleCancel}
					class="rounded-xl px-4 py-2 text-sm font-medium text-zinc-600 hover:bg-zinc-100 dark:text-zinc-400 dark:hover:bg-zinc-800 dark:hover:text-zinc-100 transition-colors"
				>
					{cancelText}
				</button>
				<button
					onclick={handleConfirm}
					class="rounded-xl px-4 py-2 text-sm font-medium text-white transition-colors {isDanger ? 'bg-red-600 hover:bg-red-700' : 'bg-blue-600 hover:bg-blue-700'}"
				>
					{confirmText}
				</button>
			</div>
		</div>
	</div>
{/if}
