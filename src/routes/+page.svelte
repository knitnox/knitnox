<script lang="ts">
	import Sidebar from '$lib/components/Sidebar.svelte';
	import Chat from '$lib/components/Chat.svelte';
	import SettingsModal from '$lib/components/SettingsModal.svelte';

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

<div class="flex h-[100dvh] w-full overflow-hidden text-zinc-900 dark:text-zinc-100 dark:bg-zinc-950">
	<!-- Sidebar Overlay -->
	{#if isSidebarOpen}
		<button 
			onclick={() => isSidebarOpen = false}
			class="fixed inset-0 z-40 bg-zinc-950/20 backdrop-blur-[2px]"
			aria-label="Close sidebar"
		></button>
	{/if}

	<!-- Sidebar Container -->
	<div class="fixed inset-y-0 left-0 z-50 w-72 transform transition-transform duration-300 {isSidebarOpen ? 'translate-x-0' : '-translate-x-full'}">
		<Sidebar
			onSelectChat={handleSelectChat}
			onNewChat={handleNewChat}
			onOpenSettings={handleOpenSettings}
			onToggleSidebar={() => isSidebarOpen = !isSidebarOpen}
			{currentChatId}
		/>
	</div>

	<!-- Main Content -->
	<main class="relative flex flex-1 flex-col overflow-hidden">
		<Chat 
			bind:chatId={currentChatId} 
			onToggleSidebar={() => isSidebarOpen = !isSidebarOpen}
			onOpenSettings={handleOpenSettings}
			onNewChat={handleNewChat}
		/>
	</main>

	<SettingsModal bind:isOpen={isSettingsOpen} />
</div>
