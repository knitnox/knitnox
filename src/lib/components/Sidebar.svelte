<script lang="ts">
	import { db } from '$lib/db';
	import { liveQuery } from 'dexie';
	import { Plus, MessageSquare, Settings as SettingsIcon, Trash2 } from '@lucide/svelte';

	let { onSelectChat, onNewChat, onOpenSettings, currentChatId } = $props<{
		onSelectChat: (id: number) => void;
		onNewChat: () => void;
		onOpenSettings: () => void;
		currentChatId: number | null;
	}>();

	const chats = liveQuery(() => db.chats.orderBy('createdAt').reverse().toArray());

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

<aside class="flex h-full w-64 flex-col border-r border-zinc-200 bg-zinc-50 dark:border-zinc-800 dark:bg-zinc-950">
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
					<button
						onclick={() => onSelectChat(chat.id!)}
						class="flex flex-1 items-center gap-3 px-3 py-2 text-left text-sm cursor-pointer"
					>
						<MessageSquare size={16} class="shrink-0 opacity-60" />
						<span class="truncate pr-6">{chat.title}</span>
					</button>
					<button
						onclick={(e) => deleteChat(chat.id!, e)}
						class="absolute right-2 opacity-0 group-hover:opacity-100 hover:text-red-500 transition-opacity p-1 cursor-pointer"
						aria-label="Delete chat"
					>
						<Trash2 size={14} />
					</button>
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
