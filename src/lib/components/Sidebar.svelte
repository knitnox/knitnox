<script lang="ts">
	import { db } from '$lib/db';
	import { liveQuery } from 'dexie';
	import { Plus, MessageSquare, Settings as SettingsIcon, Trash2, Edit2, Check, X } from '@lucide/svelte';
	import { tick } from 'svelte';
	import ConfirmModal from './ConfirmModal.svelte';

	let { onSelectChat, onNewChat, onOpenSettings, onToggleSidebar, currentChatId } = $props<{
		onSelectChat: (id: number) => void;
		onNewChat: () => void;
		onOpenSettings: () => void;
		onToggleSidebar?: () => void;
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

	let confirmModal = $state({
		isOpen: false,
		title: '',
		message: '',
		confirmText: 'Confirm',
		isDanger: false,
		onConfirm: () => {}
	});

	function confirmDeleteChat(id: number, e: MouseEvent) {
		e.stopPropagation();
		confirmModal = {
			isOpen: true,
			title: 'Delete Chat',
			message: 'Are you sure you want to delete this chat session? This action cannot be undone.',
			confirmText: 'Delete',
			isDanger: true,
			onConfirm: async () => {
				await db.chats.delete(id);
				await db.messages.where('chatId').equals(id).delete();
				if (currentChatId === id) {
					onNewChat();
				}
			}
		};
	}
</script>

<aside class="flex h-full w-72 flex-col border-r border-zinc-200 bg-zinc-50 dark:border-zinc-800 dark:bg-zinc-950 shadow-xl">
	<!-- Sidebar Logo -->
	<div class="flex flex-col items-center pt-5 pb-3 px-4 w-full">
		<h1 class="sidebar-logo-title">
			{#each 'Knitnox'.split('') as char}
				<span>{char}</span>
			{/each}
		</h1>
		<div class="sidebar-logo-divider">
			<span class="line"></span>
			<span class="star">⛤</span>
			<span class="line"></span>
		</div>
	</div>

	<div class="flex items-center justify-between px-3 pb-3">
		<button
			onclick={onNewChat}
			class="flex flex-1 items-center justify-center gap-2 rounded-lg border border-zinc-200 bg-white px-3 py-1.5 text-sm font-medium shadow-sm hover:bg-zinc-50 dark:border-zinc-800 dark:bg-zinc-900 dark:hover:bg-zinc-800 transition-all cursor-pointer"
		>
			<Plus size={16} />
			New Chat
		</button>
		{#if onToggleSidebar}
			<button
				onclick={onToggleSidebar}
				class="ml-2 rounded-lg p-1.5 hover:bg-zinc-200 dark:hover:bg-zinc-800 text-zinc-500 transition-colors"
				aria-label="Close sidebar"
			>
				<X size={18} />
			</button>
		{/if}
	</div>

	<div class="flex-1 overflow-y-auto px-1.5">
		{#if $chats}
			{#each $chats as chat}
				<div
					class="group relative mb-0.5 flex w-full items-center rounded-lg transition-colors {currentChatId ===
					chat.id
						? 'bg-zinc-200 dark:bg-zinc-800'
						: 'hover:bg-zinc-100 dark:hover:bg-zinc-900'}"
				>
					{#if editingChatId === chat.id}
						<div class="flex flex-1 items-center gap-1.5 px-2 py-1.5">
							<input
								bind:this={editInput}
								bind:value={editingTitle}
								class="w-full rounded border border-blue-500 bg-white px-1 py-0.5 text-xs outline-none dark:bg-zinc-800"
								onkeydown={(e) => {
									if (e.key === 'Enter') saveTitle(chat.id!);
									if (e.key === 'Escape') cancelEditing();
								}}
								onblur={() => saveTitle(chat.id!)}
							/>
							<div class="flex shrink-0 gap-0.5">
								<button onclick={() => saveTitle(chat.id!)} class="text-green-600 hover:text-green-700">
									<Check size={12} />
								</button>
								<button onclick={cancelEditing} class="text-red-500 hover:text-red-600">
									<X size={12} />
								</button>
							</div>
						</div>
					{:else}
						<button
							onclick={() => onSelectChat(chat.id!)}
							class="flex flex-1 items-center gap-2.5 px-3 py-2 text-left text-sm cursor-pointer min-w-0"
						>
							<MessageSquare size={16} class="shrink-0 opacity-60" />
							<span class="truncate flex-1 pr-12">{chat.title}</span>
						</button>
						<div class="absolute right-1 flex items-center gap-0.5">
							<button
								onclick={(e) => startEditing(chat.id!, chat.title, e)}
								class="text-zinc-400 hover:text-blue-600 p-1.5 cursor-pointer rounded-md hover:bg-zinc-300 dark:hover:bg-zinc-700 transition-colors"
								aria-label="Edit title"
							>
								<Edit2 size={14} />
							</button>
							<button
								onclick={(e) => confirmDeleteChat(chat.id!, e)}
								class="text-zinc-400 hover:text-red-600 p-1.5 cursor-pointer rounded-md hover:bg-zinc-300 dark:hover:bg-zinc-700 transition-colors"
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

	<div class="mt-auto border-t border-zinc-200 p-2 dark:border-zinc-800">
		<button
			onclick={onOpenSettings}
			class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm font-medium hover:bg-zinc-100 dark:hover:bg-zinc-900 transition-colors cursor-pointer"
		>
			<SettingsIcon size={18} />
			Settings
		</button>
	</div>
</aside>

<ConfirmModal
	bind:isOpen={confirmModal.isOpen}
	title={confirmModal.title}
	message={confirmModal.message}
	confirmText={confirmModal.confirmText}
	isDanger={confirmModal.isDanger}
	onConfirm={confirmModal.onConfirm}
/>

<style>
	.sidebar-logo-title {
		margin: 0;
		font-size: 1.5rem;
		font-family: 'Orbitron', sans-serif;
		font-weight: 900;
		letter-spacing: 2px;
		color: #09090b; /* zinc-950 */
		display: flex;
		justify-content: center;
		transition: color 0.3s;
		text-shadow: 0 0 10px rgba(255, 255, 255, 0.5);
	}

	:global(.dark) .sidebar-logo-title {
		color: #ffffff;
		text-shadow: 0 0 10px rgba(255, 255, 255, 0.1);
	}

	.sidebar-logo-title span {
		display: inline-block;
	}

	.sidebar-logo-title span:nth-child(6) {
		animation: bulbGlowSidebar 2s ease-in-out infinite;
		color: #ffa500; /* orange */
	}

	@keyframes bulbGlowSidebar {
		0%, 100% { 
			text-shadow: 0 0 5px rgba(255, 165, 0, 0.4),
						 0 0 10px rgba(255, 165, 0, 0.3);
		}
		50% { 
			text-shadow: 0 0 8px rgba(255, 165, 0, 0.5),
						 0 0 15px rgba(255, 165, 0, 0.6),
						 0 0 20px rgba(255, 165, 0, 0.4);
		}
	}

	.sidebar-logo-divider {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		margin-top: -0.25rem;
		width: 100%;
		max-width: 150px;
		opacity: 0.6;
	}

	.sidebar-logo-divider .line {
		flex: 1;
		height: 1px;
		background: linear-gradient(to right, transparent, #09090b, transparent);
	}

	:global(.dark) .sidebar-logo-divider .line {
		background: linear-gradient(to right, transparent, #ffffff, transparent);
	}

	.sidebar-logo-divider .star {
		font-size: 1rem;
		color: #ffa500;
		animation: bulbGlowSidebar 2s ease-in-out infinite;
	}
</style>
