<script lang="ts">
	import { X, Database, Plus, Trash2, Edit3, Save, RotateCcw } from '@lucide/svelte';
	import { fade, fly } from 'svelte/transition';
	import { db, type Knowledge } from '$lib/db';

	let { isOpen = $bindable(false) } = $props();

	let memoryEntries = $state<Knowledge[]>([]);
	let isLoading = $state(false);

	let newContent = $state('');
	let newCategory = $state('');

	let editingId = $state<number | null>(null);
	let editContent = $state('');
	let editCategory = $state('');

	async function fetchMemory() {
		isLoading = true;
		try {
			// Sort descending by updated time
			memoryEntries = await db.knowledge.orderBy('updatedAt').reverse().toArray();
		} catch (e) {
			console.error('Failed to fetch memory entries:', e);
		} finally {
			isLoading = false;
		}
	}

	$effect(() => {
		if (isOpen) {
			fetchMemory();
			resetAddForm();
			resetEditForm();
		}
	});

	function close() {
		isOpen = false;
	}

	function resetAddForm() {
		newContent = '';
		newCategory = '';
	}

	function resetEditForm() {
		editingId = null;
		editContent = '';
		editCategory = '';
	}

	async function handleAdd() {
		if (!newContent.trim()) return;
		try {
			await db.knowledge.add({
				content: newContent.trim(),
				category: newCategory.trim() || undefined,
				createdAt: Date.now(),
				updatedAt: Date.now()
			});
			resetAddForm();
			await fetchMemory();
		} catch (e) {
			console.error('Failed to add memory:', e);
		}
	}

	async function handleDelete(id: number) {
		if (!confirm('Are you sure you want to delete this memory entry?')) return;
		try {
			await db.knowledge.delete(id);
			await fetchMemory();
		} catch (e) {
			console.error('Failed to delete memory:', e);
		}
	}

	function startEdit(entry: Knowledge) {
		if (!entry.id) return;
		editingId = entry.id;
		editContent = entry.content;
		editCategory = entry.category || '';
	}

	async function handleSaveEdit() {
		if (!editingId || !editContent.trim()) return;
		try {
			await db.knowledge.update(editingId, {
				content: editContent.trim(),
				category: editCategory.trim() || undefined,
				updatedAt: Date.now()
			});
			resetEditForm();
			await fetchMemory();
		} catch (e) {
			console.error('Failed to update memory:', e);
		}
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
			class="relative flex flex-col w-full max-w-3xl h-[90vh] sm:h-auto sm:max-h-[85vh] rounded-t-3xl sm:rounded-3xl bg-white shadow-2xl dark:bg-zinc-900 overflow-hidden border border-zinc-200 dark:border-zinc-800"
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
						<Database size={20} />
					</div>
					<div>
						<h2 class="text-lg font-bold uppercase tracking-wider">Global Memory</h2>
						<p class="text-xs text-zinc-500">Manage facts and preferences remembered by the AI</p>
					</div>
				</div>
				<button onclick={close} class="rounded-full p-2 hover:bg-zinc-100 dark:hover:bg-zinc-800 transition-colors">
					<X size={20} />
				</button>
			</div>

			<div class="flex-1 overflow-y-auto p-6 space-y-6 custom-scrollbar">
				
				<!-- Add New Entry Section -->
				<div class="rounded-2xl border border-zinc-100 bg-zinc-50/50 p-4 dark:border-zinc-800 dark:bg-zinc-800/30">
					<h3 class="text-xs font-bold uppercase tracking-widest text-zinc-400 mb-3">Add New Memory</h3>
					<div class="flex flex-col sm:flex-row gap-3">
						<div class="flex-1">
							<input
								type="text"
								bind:value={newContent}
								placeholder="What should the AI remember?"
								class="w-full rounded-xl border border-zinc-200 bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 dark:border-zinc-700 dark:bg-zinc-900"
								onkeydown={(e) => { if(e.key === 'Enter') handleAdd(); }}
							/>
						</div>
						<div class="sm:w-1/3">
							<input
								type="text"
								bind:value={newCategory}
								placeholder="Category (Optional)"
								class="w-full rounded-xl border border-zinc-200 bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 dark:border-zinc-700 dark:bg-zinc-900"
								onkeydown={(e) => { if(e.key === 'Enter') handleAdd(); }}
							/>
						</div>
						<button
							onclick={handleAdd}
							disabled={!newContent.trim()}
							class="flex items-center justify-center gap-2 rounded-xl bg-blue-600 px-4 py-2 text-sm font-bold text-white hover:bg-blue-700 transition-all active:scale-95 disabled:opacity-50 sm:w-auto"
						>
							<Plus size={16} /> Add
						</button>
					</div>
				</div>

				<!-- Memory List Section -->
				<div class="space-y-3">
					<div class="flex items-center justify-between">
						<h3 class="text-xs font-bold uppercase tracking-widest text-zinc-400">Stored Memories ({memoryEntries.length})</h3>
						<button 
							onclick={fetchMemory}
							class="text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-300 transition-colors"
							title="Refresh list"
						>
							<RotateCcw size={14} class={isLoading ? 'animate-spin' : ''} />
						</button>
					</div>

					{#if isLoading}
						<div class="py-8 text-center text-sm text-zinc-500">Loading memories...</div>
					{:else if memoryEntries.length === 0}
						<div class="rounded-2xl border-2 border-dashed border-zinc-200 p-8 text-center dark:border-zinc-800">
							<Database size={24} class="mx-auto mb-2 text-zinc-300" />
							<p class="text-sm font-medium text-zinc-500">No memories stored yet.</p>
							<p class="text-xs text-zinc-400">The AI can automatically save things here, or you can add them above.</p>
						</div>
					{:else}
						<div class="space-y-2">
							{#each memoryEntries as entry (entry.id)}
								<div class="group flex flex-col sm:flex-row sm:items-start justify-between gap-3 rounded-xl border border-zinc-100 bg-white p-4 shadow-sm dark:border-zinc-800 dark:bg-zinc-800/50 hover:border-blue-200 dark:hover:border-blue-900/50 transition-colors">
									
									{#if editingId === entry.id}
										<!-- Edit Mode -->
										<div class="flex-1 space-y-2 w-full">
											<input
												type="text"
												bind:value={editContent}
												class="w-full rounded-lg border border-blue-200 bg-blue-50/30 px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 dark:border-blue-900/50 dark:bg-blue-900/10 dark:text-white"
												placeholder="Memory content"
												onkeydown={(e) => { if(e.key === 'Enter') handleSaveEdit(); else if (e.key === 'Escape') resetEditForm(); }}
											/>
											<div class="flex flex-col sm:flex-row gap-2">
												<input
													type="text"
													bind:value={editCategory}
													class="w-full sm:w-1/2 rounded-lg border border-blue-200 bg-blue-50/30 px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-blue-500 dark:border-blue-900/50 dark:bg-blue-900/10 dark:text-white"
													placeholder="Category (Optional)"
													onkeydown={(e) => { if(e.key === 'Enter') handleSaveEdit(); else if (e.key === 'Escape') resetEditForm(); }}
												/>
												<div class="flex gap-2 sm:ml-auto w-full sm:w-auto">
													<button 
														onclick={resetEditForm}
														class="flex-1 sm:flex-none rounded-lg border border-zinc-200 px-3 py-1.5 text-xs font-bold hover:bg-zinc-50 dark:border-zinc-700 dark:hover:bg-zinc-800 transition-colors"
													>
														Cancel
													</button>
													<button 
														onclick={handleSaveEdit}
														disabled={!editContent.trim()}
														class="flex-1 sm:flex-none flex items-center justify-center gap-1 rounded-lg bg-blue-600 px-3 py-1.5 text-xs font-bold text-white hover:bg-blue-700 transition-colors disabled:opacity-50"
													>
														<Save size={14} /> Save
													</button>
												</div>
											</div>
										</div>
									{:else}
										<!-- View Mode -->
										<div class="flex-1 min-w-0">
											<p class="text-sm font-medium break-words whitespace-pre-wrap text-zinc-900 dark:text-zinc-100">{entry.content}</p>
											<div class="mt-2 flex items-center gap-2">
												{#if entry.category}
													<span class="rounded bg-zinc-100 px-1.5 py-0.5 text-[10px] font-bold text-zinc-500 dark:bg-zinc-800">{entry.category}</span>
												{/if}
												<span class="text-[10px] text-zinc-400">
													{new Date(entry.updatedAt).toLocaleDateString()} {new Date(entry.updatedAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
												</span>
											</div>
										</div>
										
										<!-- Action Buttons -->
										<div class="flex items-center gap-1 opacity-100 [@media(hover:hover)]:sm:opacity-0 [@media(hover:hover)]:sm:group-hover:opacity-100 transition-opacity">
											<button 
												onclick={() => startEdit(entry)}
												class="rounded-lg p-2 text-zinc-400 hover:bg-zinc-100 hover:text-blue-600 dark:hover:bg-zinc-800 dark:hover:text-blue-400 transition-colors"
												title="Edit"
											>
												<Edit3 size={16} />
											</button>
											<button 
												onclick={() => entry.id && handleDelete(entry.id)}
												class="rounded-lg p-2 text-zinc-400 hover:bg-red-50 hover:text-red-500 dark:hover:bg-red-900/20 transition-colors"
												title="Delete"
											>
												<Trash2 size={16} />
											</button>
										</div>
									{/if}

								</div>
							{/each}
						</div>
					{/if}
				</div>

			</div>
		</div>
	</div>
{/if}

<style>
	.custom-scrollbar::-webkit-scrollbar {
		width: 4px;
	}
	.custom-scrollbar::-webkit-scrollbar-track {
		background: transparent;
	}
	.custom-scrollbar::-webkit-scrollbar-thumb {
		background: #e4e4e7;
		border-radius: 10px;
	}
	:global(.dark) .custom-scrollbar::-webkit-scrollbar-thumb {
		background: #3f3f46;
	}
</style>
