<script lang="ts">
	import { settings } from '$lib/settings.svelte';
	import { MCPClient, mcpPool } from '$lib/mcp.svelte';
	import { resourceContext } from '$lib/resource-context.svelte';
	import { promptContext } from '$lib/prompt-context.svelte';
	import { type ResourceContent } from '$lib/db';
	import {
		X, FileText, Terminal, Search, Loader2, Library,
		Folder, FolderOpen, File, FileCode, FileImage, FileJson,
		FileArchive, Eye, PlusCircle, ChevronRight, ChevronDown,
		ArrowLeft
	} from '@lucide/svelte';
	import { fade, fly } from 'svelte/transition';

	let { isOpen = $bindable(false) } = $props<{
		isOpen: boolean;
	}>();

	type Tab = 'resources' | 'prompts';
	let activeTab = $state<Tab>('resources');

	let resources = $state<any[]>([]);
	let prompts = $state<any[]>([]);
	let isLoading = $state(false);
	let searchQuery = $state('');

	// Resource viewer state
	let viewingResource = $state<any | null>(null);
	let viewingContent = $state<string | null>(null);
	let viewingLoading = $state(false);
	let viewingError = $state<string | null>(null);
	let expandedFolders = $state<Set<string>>(new Set());

	// Generic viewer state
	let viewingType = $state<'raw' | 'tree' | 'list'>('raw');
	let treeData = $state<any>(null);
	let listData = $state<any[]>([]);
	let viewingTreeNodeContent = $state<string | null>(null);
	let viewingTreeNodeLabel = $state<string>('');
	let viewingTreeNodeLoading = $state(false);

	// --- Helpers ---

	function getFileIcon(name: string, mimeType?: string): typeof File {
		if (mimeType === 'inode/directory') return Folder;
		const ext = name.split('.').pop()?.toLowerCase() || '';
		const codeExts = ['svelte', 'ts', 'js', 'jsx', 'tsx', 'py', 'html', 'css', 'scss',
			'sass', 'less', 'vue', 'rs', 'go', 'java', 'c', 'cpp', 'h', 'hpp', 'rb', 'php',
			'sql', 'graphql', 'yaml', 'yml', 'toml', 'ini', 'cfg', 'sh', 'bash', 'zsh',
			'ps1', 'bat', 'xml', 'dart', 'swift', 'kt', 'kts'];
		const imageExts = ['svg', 'png', 'jpg', 'jpeg', 'gif', 'webp', 'ico', 'bmp', 'tiff'];
		const archiveExts = ['zip', 'tar', 'gz', 'bz2', 'xz', '7z', 'rar', 'deb', 'rpm'];

		if (codeExts.includes(ext)) return FileCode;
		if (imageExts.includes(ext)) return FileImage;
		if (ext === 'json' || mimeType === 'application/json') return FileJson;
		if (ext === 'md' || ext === 'txt' || ext === 'log' || mimeType?.startsWith('text/')) return FileText;
		if (archiveExts.includes(ext)) return FileArchive;

		return File;
	}

	function getFileIconColor(name: string, mimeType?: string): string {
		if (mimeType === 'inode/directory') return 'text-amber-500';
		const ext = name.split('.').pop()?.toLowerCase() || '';
		const codeExts = ['svelte', 'ts', 'js', 'jsx', 'tsx', 'py', 'html', 'css', 'scss',
			'sass', 'less', 'vue', 'rs', 'go', 'java', 'c', 'cpp', 'h', 'hpp', 'rb', 'php',
			'sql', 'graphql', 'yaml', 'yml', 'toml', 'ini', 'cfg', 'sh', 'bash', 'zsh',
			'ps1', 'bat', 'xml', 'dart', 'swift', 'kt', 'kts'];
		if (codeExts.includes(ext)) return 'text-blue-600 dark:text-blue-400';
		if (['svg', 'png', 'jpg', 'jpeg', 'gif', 'webp'].includes(ext)) return 'text-purple-600 dark:text-purple-400';
		if (ext === 'json') return 'text-yellow-600 dark:text-yellow-400';
		if (['md', 'txt'].includes(ext)) return 'text-zinc-600 dark:text-zinc-400';
		if (ext === 'svelte') return 'text-orange-600 dark:text-orange-400';
		if (ext === 'ts' || ext === 'tsx') return 'text-cyan-600 dark:text-cyan-400';
		if (ext === 'js' || ext === 'jsx') return 'text-amber-600 dark:text-amber-400';
		if (ext === 'py') return 'text-green-600 dark:text-green-400';
		if (ext === 'css' || ext === 'scss' || ext === 'less') return 'text-pink-600 dark:text-pink-400';
		if (ext === 'html') return 'text-red-600 dark:text-red-400';
		return 'text-zinc-500 dark:text-zinc-400';
	}

	function formatSize(sizeBytes: number | string | undefined): string {
		if (sizeBytes === undefined || sizeBytes === null) return '';
		if (typeof sizeBytes === 'string') return sizeBytes;
		if (sizeBytes < 1024) return `${sizeBytes} B`;
		if (sizeBytes < 1024 * 1024) return `${(sizeBytes / 1024).toFixed(1)} KB`;
		if (sizeBytes < 1024 * 1024 * 1024) return `${(sizeBytes / (1024 * 1024)).toFixed(1)} MB`;
		return `${(sizeBytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
	}

	function toggleFolder(path: string) {
		const newSet = new Set(expandedFolders);
		if (newSet.has(path)) {
			newSet.delete(path);
		} else {
			newSet.add(path);
		}
		expandedFolders = newSet;
	}

	// --- Fetch ---

	async function fetchAll() {
		isLoading = true;
		resources = [];
		prompts = [];
		try {
			const allResources: any[] = [];
			const allPrompts: any[] = [];
			for (const url of settings.mcpServers) {
				if (!url) continue;
				try {
					const client = mcpPool.get(url);
					const r = await client.listResources();
					const p = await client.listPrompts();
					allResources.push(...r.map((item: any) => ({ ...item, serverUrl: url })));
					allPrompts.push(...p.map((item: any) => ({ ...item, serverUrl: url })));
				} catch (e) {
					// Silently skip offline servers
				}
			}
			resources = allResources;
			prompts = allPrompts;
		} catch (e) {
			console.error('Failed to fetch MCP items:', e);
		} finally {
			isLoading = false;
		}
	}

	async function viewResource(item: any) {
		viewingResource = item;
		viewingContent = null;
		viewingError = null;
		viewingLoading = true;
		viewingType = 'raw';
		expandedFolders = new Set();
		treeData = null;
		listData = [];

		try {
			const client = mcpPool.get(item.serverUrl);
			const contents = await client.readResource(item.uri);
			let raw: string;
			if (Array.isArray(contents)) {
				raw = contents.map((c: any) => c.text || c.blob || JSON.stringify(c)).join('\n');
			} else if (typeof contents === 'string') {
				raw = contents;
			} else {
				raw = JSON.stringify(contents, null, 2);
			}

			viewingContent = raw;

			// Content-based structure detection
			try {
				const parsed = JSON.parse(raw);
				if (Array.isArray(parsed)) {
					if (parsed.length > 0 && typeof parsed[0] === 'object') {
						viewingType = 'list';
						listData = parsed;
					}
				} else if (typeof parsed === 'object' && parsed !== null) {
					if (parsed.children && Array.isArray(parsed.children)) {
						viewingType = 'tree';
						treeData = parsed;
					} else {
						// Fallback to pretty-printed JSON if it's just a generic object
						viewingContent = JSON.stringify(parsed, null, 2);
					}
				}
			} catch {
				// Not JSON, stay as 'raw'
			}
		} catch (e: any) {
			viewingError = e.message || 'Failed to read resource';
		} finally {
			viewingLoading = false;
		}
	}

	function closeViewer() {
		viewingResource = null;
		viewingContent = null;
		viewingError = null;
		treeData = null;
		listData = [];
	}

	async function addToChat(item: any) {
		try {
			const client = mcpPool.get(item.serverUrl);
			const contents = await client.readResource(item.uri);
			let content: string;
			if (Array.isArray(contents)) {
				content = contents.map((c: any) => c.text || c.blob || JSON.stringify(c)).join('\n');
			} else if (typeof contents === 'string') {
				content = contents;
			} else {
				content = JSON.stringify(contents, null, 2);
			}

			resourceContext.add({
				name: item.name || item.uri,
				uri: item.uri,
				content,
				mimeType: item.mimeType
			});
		} catch (e: any) {
			console.error('Failed to add resource to chat:', e);
		}
	}

	async function handleTreeNodeFileClick(targetUri: string) {
		viewingTreeNodeLoading = true;
		viewingTreeNodeContent = null;
		viewingTreeNodeLabel = targetUri.split('/').pop() || targetUri;
		try {
			const client = mcpPool.get(viewingResource.serverUrl);
			const contents = await client.readResource(targetUri);
			let raw: string;
			if (Array.isArray(contents)) {
				raw = contents.map((c: any) => c.text || c.blob || JSON.stringify(c)).join('\n');
			} else if (typeof contents === 'string') {
				raw = contents;
			} else {
				raw = JSON.stringify(contents, null, 2);
			}
			viewingTreeNodeContent = raw;
		} catch (e: any) {
			viewingTreeNodeContent = null;
			viewingError = e.message || 'Failed to read item. Try a different resource.';
		} finally {
			viewingTreeNodeLoading = false;
		}
	}

	function closeTreeNodeViewer() {
		viewingTreeNodeContent = null;
		viewingTreeNodeLabel = '';
	}

	async function addTreeNodeToChat(nodeName: string, content: string) {
		const baseUri = viewingResource?.uri || 'mcp://resource';
		resourceContext.add({
			name: nodeName,
			uri: `${baseUri}/${encodeURIComponent(nodeName)}`,
			content,
			mimeType: undefined
		});
	}

	function getHostname(serverUrl: string): string {
		try {
			return new URL(serverUrl).hostname;
		} catch {
			return serverUrl;
		}
	}

	$effect(() => {
		if (isOpen) {
			activeTab = 'resources';
			searchQuery = '';
			viewingResource = null;
			viewingContent = null;
			viewingError = null;
			treeData = null;
			fetchAll();
		}
	});

	const filteredItems = $derived(
		(activeTab === 'resources' ? resources : prompts).filter(item =>
			(item.name || item.uri || '').toLowerCase().includes(searchQuery.toLowerCase()) ||
			(item.description || '').toLowerCase().includes(searchQuery.toLowerCase())
		)
	);

	function close() {
		isOpen = false;
	}
</script>

{#if isOpen}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="fixed inset-0 z-[60] flex items-end justify-center bg-black/40 backdrop-blur-[2px] p-0 sm:items-center sm:p-4"
		onclick={close}
		transition:fade={{ duration: 200 }}
	>
		<div
			class="relative flex flex-col w-full max-w-2xl h-[85vh] sm:h-[600px] rounded-t-3xl sm:rounded-3xl bg-white shadow-2xl dark:bg-zinc-900 overflow-hidden"
			onclick={e => e.stopPropagation()}
			transition:fly={{ y: 100, duration: 300, opacity: 1 }}
		>
			<!-- Header -->
			<div class="flex items-center justify-between border-b border-zinc-100 p-4 sm:p-6 dark:border-zinc-800 shrink-0">
				<div class="flex items-center gap-3">
					<div class="rounded-xl p-2 bg-zinc-100 text-zinc-600 dark:bg-zinc-800">
						<Library size={20} />
					</div>
					<div>
						<h2 class="text-base sm:text-lg font-bold uppercase tracking-wider">MCP Library</h2>
						<p class="text-[10px] sm:text-xs text-zinc-500">Resources & Prompts from connected servers</p>
					</div>
				</div>
				<button onclick={close} class="rounded-full p-2 hover:bg-zinc-100 dark:hover:bg-zinc-800 transition-colors">
					<X size={20} />
				</button>
			</div>

			<!-- Tab Bar -->
			<div class="flex border-b border-zinc-100 dark:border-zinc-800 shrink-0">
				<button
					onclick={() => { activeTab = 'resources'; searchQuery = ''; }}
					class="flex-1 flex items-center justify-center gap-2 py-3 text-sm font-bold transition-all border-b-2 {activeTab === 'resources' ? 'border-zinc-900 dark:border-white text-zinc-900 dark:text-white' : 'border-transparent text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-300'}"
				>
					<FileText size={16} />
					Resources
					{#if resources.length > 0}
						<span class="text-[10px] bg-zinc-100 dark:bg-zinc-800 px-1.5 py-0.5 rounded-full">{resources.length}</span>
					{/if}
				</button>
				<button
					onclick={() => { activeTab = 'prompts'; searchQuery = ''; }}
					class="flex-1 flex items-center justify-center gap-2 py-3 text-sm font-bold transition-all border-b-2 {activeTab === 'prompts' ? 'border-zinc-900 dark:border-white text-zinc-900 dark:text-white' : 'border-transparent text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-300'}"
				>
					<Terminal size={16} />
					Prompts
					{#if prompts.length > 0}
						<span class="text-[10px] bg-zinc-100 dark:bg-zinc-800 px-1.5 py-0.5 rounded-full">{prompts.length}</span>
					{/if}
				</button>
			</div>

			<!-- Search -->
			<div class="p-3 sm:p-4 border-b border-zinc-100 dark:border-zinc-800 shrink-0">
				<div class="relative">
					<Search size={16} class="absolute left-3 top-1/2 -translate-y-1/2 text-zinc-400" />
					<input
						type="text"
						bind:value={searchQuery}
						placeholder="Search {activeTab}..."
						class="w-full bg-zinc-50 dark:bg-zinc-950 border border-zinc-200 dark:border-zinc-800 rounded-xl pl-10 pr-4 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
					/>
				</div>
			</div>

			<!-- List Content -->
			<div class="flex-1 overflow-y-auto p-3 sm:p-4 custom-scrollbar">
				{#if isLoading}
					<div class="flex flex-col items-center justify-center h-full gap-3 text-zinc-400">
						<Loader2 size={32} class="animate-spin text-blue-500" />
						<p class="text-sm font-medium">Scanning MCP servers...</p>
					</div>
				{:else if filteredItems.length === 0}
					<div class="flex flex-col items-center justify-center h-full text-center p-6 sm:p-8">
						<div class="rounded-full bg-zinc-100 dark:bg-zinc-800 p-4 mb-4">
							{#if activeTab === 'resources'}
								<FileText size={28} class="text-zinc-300" />
							{:else}
								<Terminal size={28} class="text-zinc-300" />
							{/if}
						</div>
						<p class="text-sm font-medium text-zinc-500">No {activeTab} found.</p>
						<p class="text-xs text-zinc-400 mt-1">Make sure your MCP servers are connected and support {activeTab}.</p>
					</div>
				{:else}
					<div class="grid gap-2 sm:gap-3">
						{#each filteredItems as item}
							<div class="group flex flex-col gap-1.5 sm:gap-2 rounded-xl sm:rounded-2xl border border-zinc-100 bg-zinc-50/50 p-3 sm:p-4 dark:border-zinc-800 dark:bg-zinc-800/30 hover:bg-white dark:hover:bg-zinc-800 transition-all border-l-4 border-l-transparent hover:border-l-zinc-500">
								<div class="flex items-start justify-between gap-2 sm:gap-4">
									<div class="flex items-center gap-3 flex-1 min-w-0">
										<!-- File-type icon -->
										<div class="shrink-0">
											{#if activeTab === 'resources'}
												{@const IconComponent = getFileIcon(item.name || item.uri, item.mimeType)}
												{@const iconColor = getFileIconColor(item.name || item.uri, item.mimeType)}
												<IconComponent size={20} class={iconColor} />
											{:else}
												<Terminal size={20} class="text-purple-500" />
											{/if}
										</div>
										<div class="flex-1 min-w-0">
											<div class="flex items-center gap-2 mb-0.5 sm:mb-1">
												<span class="font-bold text-xs sm:text-sm truncate">{item.name || (activeTab === 'resources' ? item.uri : '')}</span>
												{#if activeTab === 'resources' && item.mimeType}
													<span class="text-[9px] sm:text-[10px] bg-zinc-200 dark:bg-zinc-700 px-1.5 py-0.5 rounded-md text-zinc-600 dark:text-zinc-300 font-mono truncate max-w-[80px] sm:max-w-none">{item.mimeType}</span>
												{/if}
											</div>
											<div class="flex items-center gap-2">
												{#if activeTab === 'resources' && (item.sizeStr || item.size)}
													<span class="text-[10px] text-zinc-400 font-mono">{item.sizeStr || formatSize(item.size)}</span>
												{/if}
												<p class="text-[11px] sm:text-xs text-zinc-500 line-clamp-1 leading-relaxed">
													{item.description || ''}
												</p>
											</div>
										</div>
									</div>
									<div class="flex flex-col items-end gap-2 shrink-0">
										<span class="text-[8px] sm:text-[9px] font-bold text-zinc-400 uppercase tracking-tighter bg-zinc-100 dark:bg-zinc-900 px-1.5 py-0.5 rounded">
											{getHostname(item.serverUrl)}
										</span>
									</div>
								</div>

								<!-- Actions -->
								<div class="flex items-center gap-1.5 sm:gap-2 mt-1.5 sm:mt-2 pt-1.5 sm:pt-2 border-t border-zinc-100 dark:border-zinc-700/50">
									<code class="text-[9px] sm:text-[10px] text-zinc-400 truncate flex-1">{activeTab === 'resources' ? item.uri : ''}</code>
									{#if activeTab === 'resources'}
										<button
											onclick={() => viewResource(item)}
											class="flex items-center gap-1 sm:gap-1.5 text-[9px] sm:text-[10px] font-bold text-zinc-600 hover:text-zinc-900 bg-zinc-100 dark:bg-zinc-800 px-1.5 sm:px-2 py-1 rounded-lg transition-colors shrink-0"
										>
											<Eye size={11} class="sm:w-3 sm:h-3" />
											View
										</button>
										<button
											onclick={() => addToChat(item)}
											class="flex items-center gap-1 sm:gap-1.5 text-[9px] sm:text-[10px] font-bold text-blue-600 hover:text-blue-700 bg-blue-50 dark:bg-blue-900/30 dark:text-blue-400 px-1.5 sm:px-2 py-1 rounded-lg transition-colors shrink-0"
										>
											<PlusCircle size={11} class="sm:w-3 sm:h-3" />
											Add to Chat
										</button>
									{:else}
										<button
											onclick={() => {
												const client = mcpPool.get(item.serverUrl);
												client.getPrompt(item.name).then(res => {
													if (res && res.messages) {
														promptContext.applyPrompt(res.messages);
														close();
													}
												}).catch(err => {
													console.error('Failed to get prompt:', err);
												});
											}}
											class="flex items-center gap-1 sm:gap-1.5 text-[9px] sm:text-[10px] font-bold text-purple-600 hover:text-purple-700 bg-purple-50 dark:bg-purple-900/30 dark:text-purple-400 px-1.5 sm:px-2 py-1 rounded-lg transition-colors shrink-0"
										>
											<PlusCircle size={11} class="sm:w-3 sm:h-3" />
											Use Prompt
										</button>
									{/if}
								</div>
							</div>
						{/each}
					</div>
				{/if}
			</div>

			<!-- Footer -->
			<div class="bg-zinc-50/50 p-3 sm:p-4 dark:bg-zinc-800/50 border-t border-zinc-100 dark:border-zinc-800 shrink-0">
				<p class="text-[9px] sm:text-[10px] font-bold text-center text-zinc-400 uppercase tracking-widest">
					{resources.length} resources • {prompts.length} prompts
				</p>
			</div>
		</div>
	</div>

	<!-- Resource Viewer Overlay -->
	{#if viewingResource}
		{@const IconComponent = getFileIcon(viewingResource.name || viewingResource.uri, viewingResource.mimeType)}
		{@const iconColor = getFileIconColor(viewingResource.name || viewingResource.uri, viewingResource.mimeType)}
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div
			class="fixed inset-0 z-[70] flex items-center justify-center bg-black/50 backdrop-blur-sm p-4"
			onclick={closeViewer}
			transition:fade={{ duration: 150 }}
		>
			<div
				class="w-full max-w-3xl h-[85vh] overflow-hidden rounded-2xl bg-white shadow-2xl dark:bg-zinc-900 border border-zinc-200 dark:border-zinc-800 flex flex-col"
				onclick={e => e.stopPropagation()}
				transition:fly={{ y: 50, duration: 250 }}
			>
				<!-- Viewer Header -->
				<div class="flex items-center justify-between border-b border-zinc-100 dark:border-zinc-800 p-4 shrink-0">
					<div class="flex items-center gap-3 min-w-0">
						<button
							onclick={closeViewer}
							class="shrink-0 rounded-lg p-1.5 hover:bg-zinc-100 dark:hover:bg-zinc-800 transition-colors"
						>
							<ArrowLeft size={18} />
						</button>
						<div class="min-w-0">
							<div class="flex items-center gap-2">
								<IconComponent size={18} class={iconColor + ' shrink-0'} />
								<span class="font-bold text-sm truncate">{viewingResource.name || viewingResource.uri}</span>
							</div>
							<div class="flex items-center gap-3 text-[10px] text-zinc-500 mt-0.5">
								<span class="font-mono truncate">{viewingResource.uri}</span>
								<span class="font-mono">{getHostname(viewingResource.serverUrl)}</span>
							</div>
						</div>
					</div>
					<button
						onclick={() => addToChat(viewingResource)}
						class="flex items-center gap-1.5 text-xs font-bold text-blue-600 hover:text-blue-700 bg-blue-50 dark:bg-blue-900/30 dark:text-blue-400 px-3 py-1.5 rounded-lg transition-colors shrink-0"
					>
						<PlusCircle size={14} />
						Add to Chat
					</button>
				</div>

				<!-- Viewer Content -->
				<div class="flex-1 overflow-y-auto p-4 custom-scrollbar">
					{#if viewingLoading}
						<div class="flex flex-col items-center justify-center h-full gap-3 text-zinc-400">
							<Loader2 size={28} class="animate-spin text-blue-500" />
							<p class="text-sm">Loading resource...</p>
						</div>
					{:else if viewingError}
						<div class="flex flex-col items-center justify-center h-full gap-3 text-red-500">
							<p class="text-sm font-medium">Error loading resource</p>
							<p class="text-xs">{viewingError}</p>
						</div>
					{:else if viewingType === 'tree' && treeData}
						<!-- Generic Tree View -->
						<div class="space-y-0.5 font-mono text-xs">
							{#snippet renderTreeNode(node: any, path: string, depth: number)}
								{@const nodeName = node.name || node.label || 'unnamed'}
								{@const nodePath = path ? `${path}/${nodeName}` : nodeName}
								{@const isExpanded = expandedFolders.has(nodePath)}
								{@const hasChildren = node.children && Array.isArray(node.children) && node.children.length > 0}
								
								{#if hasChildren || node.type === 'directory'}
									<!-- svelte-ignore a11y_click_events_have_key_events -->
									<!-- svelte-ignore a11y_no_static_element_interactions -->
									<div
										class="flex items-center gap-1.5 py-1 px-1 rounded-md hover:bg-zinc-50 dark:hover:bg-zinc-800 cursor-pointer transition-colors group"
										style="padding-left: {depth * 16 + 4}px"
										onclick={() => toggleFolder(nodePath)}
									>
										<span class="shrink-0 text-zinc-400 group-hover:text-zinc-600 dark:group-hover:text-zinc-300">
											{#if isExpanded}
												<ChevronDown size={14} />
											{:else}
												<ChevronRight size={14} />
											{/if}
										</span>
										<span class="shrink-0 text-amber-500">
											{#if isExpanded}
												<FolderOpen size={14} />
											{:else}
												<Folder size={14} />
											{/if}
										</span>
										<span class="font-semibold text-zinc-700 dark:text-zinc-200 truncate">{nodeName}</span>
										{#if node.sizeStr}
											<span class="ml-auto text-[10px] text-zinc-400 shrink-0">{node.sizeStr}</span>
										{/if}
									</div>
									{#if isExpanded && node.children}
										{#each node.children as child}
											{@render renderTreeNode(child, nodePath, depth + 1)}
										{/each}
									{/if}
								{:else}
									{@const FileIcon = getFileIcon(nodeName, node.mimeType)}
									{@const iconColor = getFileIconColor(nodeName, node.mimeType)}
									<div
										class="flex items-center gap-1.5 py-1 px-1 rounded-md hover:bg-zinc-50 dark:hover:bg-zinc-800 transition-colors w-full text-left"
										style="padding-left: {depth * 16 + 4}px"
									>
										<span class="w-[14px] shrink-0"></span>
										<FileIcon size={14} class={iconColor + ' shrink-0'} />
										<span class="text-zinc-600 dark:text-zinc-300 truncate flex-1">{nodeName}</span>
										{#if node.sizeStr}
											<span class="text-[10px] text-zinc-400 shrink-0 mx-2">{node.sizeStr}</span>
										{/if}
										<div class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
											<button
												onclick={() => {
													if (node.content !== undefined) {
														viewingTreeNodeContent = node.content;
														viewingTreeNodeLabel = nodeName;
													} else {
														let finalUri = node.uri;
														if (!finalUri && viewingResource.uri.includes('project://tree')) {
															// Heuristic for project tree
															const cleanPath = nodePath.replace(/^\.\//, '');
															finalUri = `project://file/${encodeURIComponent(cleanPath)}`;
														}
														handleTreeNodeFileClick(finalUri || nodePath);
													}
												}}
												class="p-1 hover:bg-zinc-200 dark:hover:bg-zinc-700 rounded text-zinc-500"
												title="View details"
											>
												<Eye size={12} />
											</button>
											<button
												onclick={() => {
													if (node.content !== undefined) {
														resourceContext.add({
															name: nodeName,
															uri: node.uri || `${viewingResource.uri}/${encodeURIComponent(nodeName)}`,
															content: node.content,
															mimeType: node.mimeType
														});
													} else {
														addTreeNodeToChat(nodeName, JSON.stringify(node, null, 2));
													}
												}}
												class="p-1 hover:bg-zinc-200 dark:hover:bg-zinc-700 rounded text-blue-500"
												title="Add to chat"
											>
												<PlusCircle size={12} />
											</button>
										</div>
									</div>
								{/if}
							{/snippet}
							{@render renderTreeNode(treeData, '', 0)}
						</div>
					{:else if viewingType === 'list' && listData.length > 0}
						<!-- Generic List View -->
						<div class="grid gap-3">
							{#each listData as item, i}
								<div class="group p-3 rounded-xl border border-zinc-100 dark:border-zinc-800 bg-zinc-50/50 dark:bg-zinc-800/30">
									<div class="flex items-start justify-between gap-4">
										<div class="flex-1 overflow-x-auto">
											<div class="flex flex-wrap gap-2 mb-2">
												{#each Object.entries(item) as [key, value]}
													{#if typeof value !== 'object' && key !== 'uri' && key !== 'content'}
														<div class="flex flex-col">
															<span class="text-[9px] font-bold uppercase text-zinc-400">{key}</span>
															<span class="text-xs font-mono text-zinc-700 dark:text-zinc-300 whitespace-nowrap">{value}</span>
														</div>
													{/if}
												{/each}
											</div>
										</div>
										<div class="flex items-center gap-1 shrink-0">
											<button
												onclick={() => {
													if (item.content !== undefined) {
														viewingTreeNodeContent = item.content;
														viewingTreeNodeLabel = item.name || item.path || `Item ${i}`;
													} else if (item.uri) {
														handleTreeNodeFileClick(item.uri);
													} else {
														viewingTreeNodeContent = JSON.stringify(item, null, 2);
														viewingTreeNodeLabel = item.name || item.path || `Item ${i}`;
													}
												}}
												class="p-1.5 hover:bg-zinc-200 dark:hover:bg-zinc-700 rounded-lg text-zinc-600 transition-colors"
												title="View details"
											>
												<Eye size={14} />
											</button>
											<button
												onclick={() => {
													const finalContent = item.content !== undefined ? item.content : JSON.stringify(item, null, 2);
													if (item.uri) {
														resourceContext.add({
															name: item.name || item.path || `Item ${i}`,
															uri: item.uri,
															content: finalContent,
															mimeType: item.mimeType
														});
													} else {
														resourceContext.add({
															name: item.name || item.path || `Item ${i}`,
															uri: `${viewingResource.uri}#${i}`,
															content: finalContent,
															mimeType: item.mimeType
														});
													}
												}}
												class="p-1.5 hover:bg-zinc-200 dark:hover:bg-zinc-700 rounded-lg text-blue-600 transition-colors"
												title="Add to chat"
											>
												<PlusCircle size={14} />
											</button>
										</div>
									</div>
								</div>
							{/each}
						</div>
					{:else if viewingContent !== null}
						<!-- Raw content view -->
						<pre class="text-xs font-mono text-zinc-700 dark:text-zinc-300 whitespace-pre-wrap break-words leading-relaxed">{viewingContent}</pre>
					{/if}

					<!-- Inner Item Previewer (shared by Tree and List) -->
					{#snippet innerItemPreview()}
						{@const FileIcon = getFileIcon(viewingTreeNodeLabel, undefined)}
						{@const iconColor = getFileIconColor(viewingTreeNodeLabel, undefined)}
						<div class="mt-4 p-4 rounded-xl border border-zinc-200 dark:border-zinc-700 bg-zinc-50 dark:bg-zinc-800/50">
							<div class="flex items-center justify-between mb-3">
								<div class="flex items-center gap-2">
									<FileIcon size={16} class={iconColor} />
									<span class="font-bold text-sm">{viewingTreeNodeLabel}</span>
								</div>
								<div class="flex items-center gap-2">
									<button
										onclick={closeTreeNodeViewer}
										class="text-[10px] font-bold text-zinc-500 hover:text-zinc-700 dark:hover:text-zinc-300"
									>
										Close
									</button>
									{#if viewingTreeNodeContent !== null}
										<button
											onclick={() => addTreeNodeToChat(viewingTreeNodeLabel, viewingTreeNodeContent!)}
											class="flex items-center gap-1.5 text-[10px] font-bold text-blue-600 hover:text-blue-700 bg-blue-50 dark:bg-blue-900/30 dark:text-blue-400 px-2 py-1 rounded-lg transition-colors"
										>
											<PlusCircle size={12} />
											Add to Chat
										</button>
									{/if}
								</div>
							</div>
							{#if viewingTreeNodeLoading}
								<div class="flex items-center justify-center py-6">
									<Loader2 size={20} class="animate-spin text-blue-500" />
								</div>
							{:else if viewingTreeNodeContent !== null}
								<pre class="text-xs font-mono text-zinc-700 dark:text-zinc-300 whitespace-pre-wrap break-words leading-relaxed max-h-64 overflow-y-auto custom-scrollbar">{viewingTreeNodeContent}</pre>
							{/if}
						</div>
					{/snippet}
					{#if viewingTreeNodeContent !== null || viewingTreeNodeLoading}
						{@render innerItemPreview()}
					{/if}
				</div>
			</div>
		</div>
	{/if}
{/if}

<style>
	.custom-scrollbar::-webkit-scrollbar {
		width: 4px;
	}
	.custom-scrollbar::-webkit-scrollbar-track { background: transparent; }
	.custom-scrollbar::-webkit-scrollbar-thumb {
		background: #e4e4e7;
		border-radius: 10px;
	}
	:global(.dark) .custom-scrollbar::-webkit-scrollbar-thumb { background: #3f3f46; }
</style>