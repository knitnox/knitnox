<script lang="ts">
	import Sidebar from '$lib/components/Sidebar.svelte';
	import Chat from '$lib/components/Chat.svelte';
	import SettingsModal from '$lib/components/SettingsModal.svelte';

	let currentChatId: number | null = $state(null);
	let isSettingsOpen = $state(false);
	let isSidebarOpen = $state(true);

	$effect(() => {
		if (window.innerWidth < 1024) {
			isSidebarOpen = false;
		}
	});

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
	<!-- Sidebar Overlay (Mobile only) -->
	{#if isSidebarOpen}
		<div 
			class="fixed inset-0 z-40 bg-black/50 lg:hidden" 
			onclick={() => isSidebarOpen = false}
			onkeydown={(e) => e.key === 'Escape' && (isSidebarOpen = false)}
			role="button"
			tabindex="0"
		></div>
	{/if}

	<!-- Sidebar Container -->
	<div class="fixed inset-y-0 left-0 z-50 w-72 transform transition-transform duration-300 {isSidebarOpen ? 'translate-x-0' : '-translate-x-full'}">
		<Sidebar
			onSelectChat={handleSelectChat}
			onNewChat={handleNewChat}
			onOpenSettings={handleOpenSettings}
			{currentChatId}
		/>
	</div>

	<!-- Main Content -->
	<main class="relative flex flex-1 flex-col overflow-hidden transition-all duration-300 {isSidebarOpen ? 'lg:ml-72' : 'ml-0'}">
		<Chat 
			bind:chatId={currentChatId} 
			onToggleSidebar={() => isSidebarOpen = !isSidebarOpen}
		/>
	</main>

	<SettingsModal bind:isOpen={isSettingsOpen} />
</div>
