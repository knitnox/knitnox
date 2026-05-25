<script lang="ts">
	import Sidebar from '$lib/components/Sidebar.svelte';
	import Chat from '$lib/components/Chat.svelte';
	import SettingsModal from '$lib/components/SettingsModal.svelte';

	import { Menu } from '@lucide/svelte';

	let currentChatId: number | null = $state(null);
	let isSettingsOpen = $state(false);
	let isSidebarOpen = $state(false);

	function handleSelectChat(id: number) {
		currentChatId = id;
		isSidebarOpen = false;
	}

	function handleNewChat() {
		currentChatId = null;
		isSidebarOpen = false;
	}

	function handleOpenSettings() {
		isSettingsOpen = true;
	}
</script>

<div class="flex h-screen w-full overflow-hidden text-zinc-900 dark:text-zinc-100 dark:bg-zinc-950">
	<!-- Mobile Sidebar Overlay -->
	{#if isSidebarOpen}
		<div 
			class="fixed inset-0 z-40 bg-black/50 lg:hidden" 
			onclick={() => isSidebarOpen = false}
			onkeydown={(e) => e.key === 'Escape' && (isSidebarOpen = false)}
			role="button"
			tabindex="0"
		></div>
	{/if}

	<div class="fixed inset-y-0 left-0 z-50 w-64 transform transition-transform duration-300 lg:relative lg:translate-x-0 {isSidebarOpen ? 'translate-x-0' : '-translate-x-full'}">
		<Sidebar
			onSelectChat={handleSelectChat}
			onNewChat={handleNewChat}
			onOpenSettings={handleOpenSettings}
			{currentChatId}
		/>
	</div>

	<main class="relative flex flex-1 flex-col overflow-hidden">
		<!-- Mobile Header -->
		<header class="flex h-14 items-center gap-4 border-b border-zinc-200 px-4 dark:border-zinc-800 lg:hidden">
			<button 
				onclick={() => isSidebarOpen = true}
				class="rounded-lg p-1.5 hover:bg-zinc-100 dark:hover:bg-zinc-800"
			>
				<Menu size={20} />
			</button>
			<span class="text-sm font-bold">OpenWebUI-Lite</span>
		</header>

		<Chat bind:chatId={currentChatId} />
	</main>

	<SettingsModal bind:isOpen={isSettingsOpen} />
</div>
