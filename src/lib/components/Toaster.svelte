<script lang="ts">
	import { toast } from '$lib/toast.svelte';
	import { Info, CheckCircle, AlertCircle, X } from '@lucide/svelte';
	import { fly } from 'svelte/transition';
</script>

<div class="fixed bottom-4 right-4 z-[9999] flex flex-col gap-2 pointer-events-none">
	{#each toast.toasts as t (t.id)}
		<div 
			in:fly={{ y: 20, duration: 300 }}
			out:fly={{ x: 20, duration: 300 }}
			class="pointer-events-auto flex items-center gap-3 rounded-lg border px-4 py-3 shadow-lg backdrop-blur-md 
			{t.type === 'success' ? 'bg-green-50/90 border-green-200 text-green-800 dark:bg-green-900/90 dark:border-green-800 dark:text-green-100' : 
			 t.type === 'error' ? 'bg-red-50/90 border-red-200 text-red-800 dark:bg-red-900/90 dark:border-red-800 dark:text-red-100' : 
			 'bg-white/90 border-zinc-200 text-zinc-800 dark:bg-zinc-900/90 dark:border-zinc-800 dark:text-zinc-100'}"
		>
			{#if t.type === 'success'}
				<CheckCircle size={18} class="text-green-500" />
			{:else if t.type === 'error'}
				<AlertCircle size={18} class="text-red-500" />
			{:else}
				<Info size={18} class="text-blue-500" />
			{/if}
			
			<span class="text-sm font-medium">{t.message}</span>
			
			<button 
				onclick={() => toast.remove(t.id)}
				class="ml-2 rounded-full p-1 hover:bg-black/5 dark:hover:bg-white/5"
			>
				<X size={14} />
			</button>
		</div>
	{/each}
</div>
