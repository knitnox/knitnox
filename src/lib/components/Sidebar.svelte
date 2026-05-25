<script lang="ts">
	import { db } from '$lib/db';
	import { liveQuery } from 'dexie';
	import { Plus, MessageSquare, Settings as SettingsIcon, Trash2, Edit2, Check, X } from '@lucide/svelte';
	import { tick } from 'svelte';

	let { onSelectChat, onNewChat, onOpenSettings, currentChatId } = $props<{
		onSelectChat: (id: number) => void;
		onNewChat: () => void;
		onOpenSettings: () => void;
		currentChatId: number | null;
	}>();

	const chats = liveQuery(() => db.chats.orderBy('createdAt').reverse().toArray());

	let editingChatId = $state<number | null>(null);
	let editingTitle = $state('');
	let editInput: HTMLInputElement | null = $state(null);

	async function startEditing(id: number, title: string, e: MouseEvent) {
		e.stopPropagation();
		editingChatId = id;
		editingTitle = title;
		await tick();
		editInput?.focus();
	}

	async function saveTitle(id: number) {
		if (editingTitle.trim()) {
			await db.chats.update(id, { title: editingTitle.trim() });
		}
		editingChatId = null;
	}

	function cancelEditing() {
		editingChatId = null;
	}

	async function deleteChat(id: number, e: MouseEvent) {
		e.stopPropagation();
		if (confirm('Are you sure you want to delete this chat?')) {
			await db.chats.delete(id);
			await db.messages.where('chatId').equals(id).delete();
			if (currentChatId === id) {
				onNewChat();
			}
		}
	}
</script>

<aside class="flex h-full w-72 flex-col border-r border-zinc-200 bg-zinc-50 dark:border-zinc-800 dark:bg-zinc-950">
	<div class="p-4">
		<button
			onclick={onNewChat}
			class="flex w-full items-center justify-center gap-2 rounded-lg border border-zinc-200 bg-white px-4 py-2 font-medium shadow-sm hover:bg-zinc-50 dark:border-zinc-800 dark:bg-zinc-900 dark:hover:bg-zinc-800 transition-all cursor-pointer"
		>
			<Plus size={18} />
			New Chat
		</button>
	</div>

	<div class="flex-1 overflow-y-auto px-2">
		{#if $chats}
			{#each $chats as chat}
				<div
					class="group relative mb-1 flex w-full items-center rounded-lg transition-colors {currentChatId ===
					chat.id
						? 'bg-zinc-200 dark:bg-zinc-800'
						: 'hover:bg-zinc-100 dark:hover:bg-zinc-900'}"
				>
					{#if editingChatId === chat.id}
						<div class="flex flex-1 items-center gap-2 px-3 py-2">
							<input
								bind:this={editInput}
								bind:value={editingTitle}
								class="w-full rounded border border-blue-500 bg-white px-1 py-0.5 text-sm outline-none dark:bg-zinc-800"
								onkeydown={(e) => {
									if (e.key === 'Enter') saveTitle(chat.id!);
									if (e.key === 'Escape') cancelEditing();
								}}
								onblur={() => saveTitle(chat.id!)}
							/>
							<div class="flex shrink-0 gap-1">
								<button onclick={() => saveTitle(chat.id!)} class="text-green-600 hover:text-green-700">
									<Check size={14} />
								</button>
								<button onclick={cancelEditing} class="text-red-500 hover:text-red-600">
									<X size={14} />
								</button>
							</div>
						</div>
					{:else}
						<button
							onclick={() => onSelectChat(chat.id!)}
							class="flex flex-1 items-center gap-3 px-3 py-2 text-left text-sm cursor-pointer min-w-0"
						>
							<MessageSquare size={16} class="shrink-0 opacity-60" />
							<span class="truncate flex-1 pr-16">{chat.title}</span>
						</button>
						<div class="absolute right-1 flex items-center gap-1">
							<button
								onclick={(e) => startEditing(chat.id!, chat.title, e)}
								class="text-zinc-400 hover:text-blue-600 p-1.5 cursor-pointer rounded-md hover:bg-zinc-200 dark:hover:bg-zinc-800 transition-colors"
								aria-label="Edit title"
							>
								<Edit2 size={14} />
							</button>
							<button
								onclick={(e) => deleteChat(chat.id!, e)}
								class="text-zinc-400 hover:text-red-600 p-1.5 cursor-pointer rounded-md hover:bg-zinc-200 dark:hover:bg-zinc-800 transition-colors"
								aria-label="Delete chat"
							>
								<Trash2 size={14} />
							</button>
						</div>
					{/if}
				</div>
			{/each}
		{/if}
	</div>

	<div class="mt-auto border-t border-zinc-200 p-4 dark:border-zinc-800">
		<button
			onclick={onOpenSettings}
			class="flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium hover:bg-zinc-100 dark:hover:bg-zinc-900 transition-colors cursor-pointer"
		>
			<SettingsIcon size={18} />
			Settings
		</button>
	</div>
</aside>
