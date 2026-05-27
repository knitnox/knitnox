<script lang="ts">
	import { db, type Message, type Attachment } from '$lib/db';
	import { streamChat, generateChatTitle, summarizeChat } from '$lib/openai.svelte';
	import { settings } from '$lib/settings.svelte';
	import { toast } from '$lib/toast.svelte';
	import { getAllTools, MCPClient } from '$lib/mcp.svelte';
	import { localTools } from '$lib/tools.svelte';
	import ToolsSettingsModal from './ToolsSettingsModal.svelte';
	import ToolInspectModal from './ToolInspectModal.svelte';
	import MCPInspectModal from './MCPInspectModal.svelte';
	import LandingPage from './LandingPage.svelte';
	import ConfirmModal from './ConfirmModal.svelte';
	import { liveQuery } from 'dexie';
	import { SendHorizontal, User, Bot, Loader2, Wrench, Toolbox, ChevronRight, Trash2, Paperclip, File, X as CloseIcon, Menu, Square, Copy, Check, Settings as SettingsIcon, Brain, MessageSquare, PlusCircle, Pencil, Database, Zap, Library, Terminal } from '@lucide/svelte';
	import { tick } from 'svelte';
	import { Marked } from 'marked';
	import { markedHighlight } from 'marked-highlight';
	import hljs from 'highlight.js';
	import DOMPurify from 'dompurify';

	const marked = new Marked(
		markedHighlight({
			langPrefix: 'hljs language-',
			highlight(code, lang) {
				const language = hljs.getLanguage(lang) ? lang : 'plaintext';
				return hljs.highlight(code, { language }).value;
			}
		})
	);

	function renderMarkdown(content: string) {
		const rawHtml = marked.parse(content) as string;
		return DOMPurify.sanitize(rawHtml);
	}

	function autoscroll(node: HTMLElement) {
		const update = () => {
			// Only autoscroll if user is already near the bottom (within 100px)
			const threshold = 100;
			const isNearBottom = node.scrollHeight - node.scrollTop - node.clientHeight < threshold;
			
			if (isNearBottom) {
				node.scrollTo({
					top: node.scrollHeight,
					behavior: 'smooth'
				});
			}
		};
		
		const observer = new MutationObserver(update);
		observer.observe(node, { childList: true, characterData: true, subtree: true });
		
		// Handle viewport changes (like keyboard opening)
		if (typeof window !== 'undefined' && window.visualViewport) {
			const onResize = () => {
				const threshold = 100;
				if (node.scrollHeight - node.scrollTop - node.clientHeight < threshold) {
					update();
				}
			};
			window.visualViewport.addEventListener('resize', onResize);
			window.visualViewport.addEventListener('scroll', onResize);
			return {
				destroy() {
					observer.disconnect();
					window.visualViewport?.removeEventListener('resize', onResize);
					window.visualViewport?.removeEventListener('scroll', onResize);
				}
			};
		}

		return {
			destroy() {
				observer.disconnect();
			}
		};
	}

	let { chatId = $bindable(), onToggleSidebar, onOpenSettings, onNewChat } = $props<{ 
		chatId: number | null;
		onToggleSidebar?: () => void;
		onOpenSettings?: () => void;
		onNewChat?: () => void;
	}>();

	let input = $state('');
	let attachments = $state<Attachment[]>([]);
	let fileInput = $state<HTMLInputElement | null>(null);
	let textareaElement = $state<HTMLTextAreaElement | null>(null);

	$effect(() => {
		if (textareaElement && input !== undefined) {
			// Pre-calculated style to avoid jumping
			const style = window.getComputedStyle(textareaElement);
			const borderTop = parseFloat(style.borderTopWidth);
			const borderBottom = parseFloat(style.borderBottomWidth);
			const paddingTop = parseFloat(style.paddingTop);
			const paddingBottom = parseFloat(style.paddingBottom);
			
			textareaElement.style.height = 'auto';
			const scrollHeight = textareaElement.scrollHeight;
			
			// If we're using border-box (default in Tailwind), we don't need to add borders manually
			// but scrollHeight doesn't include borders if box-sizing is border-box in some browsers.
			// However, setting height to scrollHeight is usually enough.
			textareaElement.style.height = (scrollHeight < 38 ? 38 : scrollHeight) + 'px';
		}
	});

	const isMobile = typeof window !== 'undefined' && /Mobi|Android|iPad|iPhone/i.test(navigator.userAgent);

	let isStreaming = $state(false);
	let isToolsModalOpen = $state(false);
	let isMcpInspectOpen = $state(false);
	let mcpInspectMode = $state<'resources' | 'prompts'>('resources');
	let hasMcpResources = $state(false);
	let hasMcpPrompts = $state(false);

	async function checkMcpCapabilities() {
		try {
			let resourcesCount = 0;
			let promptsCount = 0;
			for (const url of settings.mcpServers) {
				if (!url) continue;
				const client = new MCPClient(url);
				const r = await client.listResources();
				const p = await client.listPrompts();
				resourcesCount += r.length;
				promptsCount += p.length;
			}
			hasMcpResources = resourcesCount > 0;
			hasMcpPrompts = promptsCount > 0;
		} catch (e) {
			console.warn('Failed to check MCP capabilities:', e);
		}
	}

	$effect(() => {
		checkMcpCapabilities();
	});

	let abortController = $state<AbortController | null>(null);
	let inspectToolData = $state<{ isOpen: boolean; toolName: string; args: any; result: any }>({
		isOpen: false,
		toolName: '',
		args: null,
		result: null
	});
	let messagesContainer: HTMLDivElement | null = $state(null);
	
	let streamingMessageId = $state<number | null>(null);
	let streamingContent = $state('');
	let streamingThinking = $state('');
	let streamingToolCalls = $state<any[]>([]);
	let streamingError = $state<string | null>(null);
	let streamingStartTime = $state<number>(0);

	let streamingTokens = $derived(marked.lexer(streamingContent));

	function toTitleCase(str: string) {
		return str.replace(/\b\w/g, (l) => l.toUpperCase());
	}

	let messagesList = $state<Message[]>([]);
	let currentChat = $state<import('$lib/db').Chat | null>(null);

	let isCheckingSettings = $state(true);
	
	let confirmModal = $state({
		isOpen: false,
		title: '',
		message: '',
		confirmText: 'Confirm',
		isDanger: false,
		onConfirm: () => {}
	});

	function confirmDelete(message: Message) {
		confirmModal = {
			isOpen: true,
			title: 'Delete Message',
			message: 'Are you sure you want to delete this message? All subsequent messages in this chat will also be deleted.',
			confirmText: 'Delete',
			isDanger: true,
			onConfirm: () => deleteMessageAndAfter(message)
		};
	}
	
	let editingMessageId = $state<number | null>(null);
	let editingContent = $state('');

	$effect(() => {
		const timer = setTimeout(() => {
			isCheckingSettings = false;
		}, 1000);
		return () => clearTimeout(timer);
	});

	let copiedStates = $state<Record<string, boolean>>({});

	async function copyToClipboard(text: string, id: string) {
		try {
			if (navigator.clipboard && window.isSecureContext) {
				await navigator.clipboard.writeText(text);
			} else {
				// Fallback for non-secure contexts (like mobile local testing)
				const textArea = document.createElement("textarea");
				textArea.value = text;
				textArea.style.position = "fixed";
				textArea.style.left = "-999999px";
				textArea.style.top = "-999999px";
				document.body.appendChild(textArea);
				textArea.focus();
				textArea.select();
				try {
					document.execCommand('copy');
				} catch (err) {
					console.error('Fallback copy failed', err);
				}
				document.body.removeChild(textArea);
			}
			
			copiedStates[id] = true;
			setTimeout(() => {
				copiedStates[id] = false;
			}, 2000);
		} catch (err) {
			console.error('Failed to copy text: ', err);
			toast.add('Failed to copy to clipboard', 'error');
		}
	}

	function stopGeneration() {
		if (abortController) {
			abortController.abort();
			abortController = null;
		}
	}

	$effect(() => {
		const id = chatId;
		if (!id) {
			messagesList = [];
			currentChat = null;
			return;
		}
		
		const msgObservable = liveQuery(() => db.messages.where('chatId').equals(id).toArray());
		const msgSubscription = msgObservable.subscribe({
			next: (val) => {
				messagesList = val;
			},
			error: (err) => console.error('Dexie messages subscription error:', err)
		});

		const chatObservable = liveQuery(() => db.chats.get(id));
		const chatSubscription = chatObservable.subscribe({
			next: (val) => {
				currentChat = val || null;
			},
			error: (err) => console.error('Dexie chat subscription error:', err)
		});

		return () => {
			msgSubscription.unsubscribe();
			chatSubscription.unsubscribe();
		};
	});

	async function scrollToBottom() {
		await tick();
		if (messagesContainer) {
			messagesContainer.scrollTo({
				top: messagesContainer.scrollHeight,
				behavior: 'smooth'
			});
		}
	}

	async function handleFileChange(e: Event) {
		const target = e.target as HTMLInputElement;
		if (!target.files) return;

		for (const file of Array.from(target.files)) {
			const reader = new FileReader();
			reader.onload = async (e) => {
				const data = e.target?.result as string;
				let type: Attachment['type'] = 'file';
				if (file.type.startsWith('image/')) type = 'image';
				else if (file.type.startsWith('audio/')) type = 'audio';
				else if (file.type.startsWith('video/')) type = 'video';

				attachments = [...attachments, {
					type,
					mimeType: file.type,
					data,
					name: file.name
				}];
			};
			reader.readAsDataURL(file);
		}
		target.value = ''; // Reset input
	}

	function removeAttachment(index: number) {
		attachments = attachments.filter((_, i) => i !== index);
	}

	async function handleEdit(message: Message) {
		editingMessageId = message.id!;
		editingContent = message.content;
		await tick();
		const el = document.querySelector('textarea[data-editing="true"]') as HTMLTextAreaElement;
		if (el) el.focus();
	}

	function cancelEdit() {
		editingMessageId = null;
		editingContent = '';
	}

	async function handleEditSubmit() {
		if (!editingMessageId || !editingContent.trim() || isStreaming) return;

		const messageId = editingMessageId;
		const newContent = editingContent.trim();
		
		editingMessageId = null;
		editingContent = '';
		streamingError = null;

		// Delete all subsequent messages
		const allMessages = await db.messages.where('chatId').equals(chatId!).toArray();
		const messageIndex = allMessages.findIndex(m => m.id === messageId);
		if (messageIndex === -1) return;

		const idsToDelete = allMessages.slice(messageIndex + 1).map(m => m.id!);
		if (idsToDelete.length > 0) {
			await db.messages.bulkDelete(idsToDelete);
		}

		// Update the edited message
		await db.messages.update(messageId, {
			content: newContent,
		});

		isStreaming = true;
		scrollToBottom();

		try {
			await processChat();
		} catch (error: any) {
			console.error(error);
			streamingError = error.message || 'An error occurred';
		} finally {
			isStreaming = false;
		}
	}

	async function deleteMessageAndAfter(message: Message) {
		if (isStreaming) return;
		
		const allMessages = await db.messages.where('chatId').equals(chatId!).toArray();
		const messageIndex = allMessages.findIndex(m => m.id === message.id);
		if (messageIndex === -1) return;

		const idsToDelete = allMessages.slice(messageIndex).map(m => m.id!);
		if (idsToDelete.length > 0) {
			await db.messages.bulkDelete(idsToDelete);
		}
	}

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		if ((!input.trim() && attachments.length === 0) || isStreaming) return;

		const userContent = input.trim();
		const currentAttachments = $state.snapshot(attachments);
		input = '';
		attachments = [];
		streamingError = null;

		let isNewChat = false;
		if (!chatId) {
			isNewChat = true;
			chatId = await db.chats.add({
				title: 'New Chat',
				createdAt: Date.now()
			});
		}

		await db.messages.add({
			chatId: chatId!,
			role: 'user',
			content: userContent,
			attachments: currentAttachments.length > 0 ? currentAttachments : undefined,
			createdAt: Date.now()
		});

		isStreaming = true;
		scrollToBottom();

		if (isNewChat) {
			generateChatTitle(userContent || 'New Chat').then((title) => {
				if (chatId) db.chats.update(chatId, { title });
			});
		}

		try {
			await processChat();
		} catch (error: any) {
			console.error(error);
			streamingError = error.message || 'An error occurred';
		} finally {
			isStreaming = false;
		}
	}

	async function processChat() {
		const mcpTools = await getAllTools(settings.mcpServers);
		const filteredMcpTools = mcpTools.filter(t => !settings.disabledTools.includes(t.tool.name));
		const filteredLocalTools = localTools.filter(t => !settings.disabledTools.includes(t.name));

		const openaiTools = [
			...filteredMcpTools.map(({ tool }) => ({
				type: 'function' as const,
				function: {
					name: tool.name,
					description: tool.description,
					parameters: tool.inputSchema
				}
			})),
			...filteredLocalTools.map((tool) => ({
				type: 'function' as const,
				function: {
					name: tool.name,
					description: tool.description,
					parameters: tool.parameters
				}
			}))
		];

		let keepGoing = true;
		let agentTurn = 0;
		while (keepGoing && agentTurn < settings.maxAgentTurns) {
			agentTurn++;
			const allMessages = await db.messages.where('chatId').equals(chatId!).toArray();
			
			// Context window logic:
			// 1. Get the last N messages based on settings.contextWindow
			// 2. For these messages, filter out 'thinking' (though they aren't separate messages in DB usually) 
			//    and 'tool' roles/tool_calls UNLESS they are in the very last 2 messages.
			
			const windowSize = settings.contextWindow || 0;
			const windowMessages = windowSize === 0 ? allMessages : allMessages.slice(-windowSize);
			
			// Periodic Compression Logic:
			// Trigger whenever we've added (windowSize - 2) new messages since the last compression.
			if (settings.enableCompression && windowSize > 3) {
				const threshold = windowSize - 2;
				const chat = await db.chats.get(chatId!);
				const lastSummaryCount = chat?.lastSummaryCount || 0;
				const currentCount = allMessages.length;

				if (currentCount >= threshold && (currentCount - lastSummaryCount) >= threshold) {
					const toastId = toast.add('Updating long-term memory...', 'info', 0);
					try {
						const newSummary = await summarizeChat(windowMessages, chat?.summary);
						if (chatId) {
							await db.chats.update(chatId, { 
								summary: newSummary,
								lastSummaryCount: currentCount
							});
						}
						toast.remove(toastId);
						toast.add('Memory updated successfully!', 'success');
					} catch (e) {
						console.error('Compression error:', e);
						toast.remove(toastId);
						toast.add('Failed to update memory.', 'error');
					}
				}
			}

			const memory = currentChat?.summary ? `\n\n[Long-term Memory/Summary of earlier conversation]:\n${currentChat.summary}` : '';
			const turnInfo = `\n\n[System Info: Turn ${agentTurn}/${settings.maxAgentTurns} in agent loop.]`;
			
			const apiMessages = [
				{ role: 'system', content: settings.systemPrompt + memory + turnInfo },
				...windowMessages.map((m) => {
					if (m.attachments?.length) {
						const content: any[] = [];
						if (m.content) {
							content.push({ type: 'text', text: m.content });
						}
						for (const att of m.attachments) {
							if (att.type === 'image') {
								content.push({
									type: 'image_url',
									image_url: { url: att.data }
								});
							} else {
								// For audio/video/other, use a generic format that some providers support
								// or that can be extended later.
								content.push({
									type: att.type === 'audio' ? 'input_audio' : att.type === 'video' ? 'input_video' : 'file',
									[att.type === 'audio' ? 'input_audio' : att.type === 'video' ? 'input_video' : 'file']: {
										data: att.data.split(',')[1],
										format: att.mimeType.split('/')[1]
									}
								} as any);
							}
						}
						return {
							role: m.role,
							content,
							tool_calls: m.toolCalls,
							tool_call_id: (m as any).toolCallId
						};
					}
					return {
						role: m.role,
						content: m.content,
						tool_calls: m.toolCalls,
						tool_call_id: (m as any).toolCallId
					};
				})
			];

			let assistantContent = '';
			let toolCalls: any[] = [];
			
			const messageId = await db.messages.add({
				chatId: chatId!,
				role: 'assistant',
				content: '',
				createdAt: Date.now()
			});

			streamingMessageId = messageId as number;
			streamingContent = '';
			streamingThinking = '';
			streamingToolCalls = [];
			streamingStartTime = Date.now();

			abortController = new AbortController();

			let lastUpdate = Date.now();
			try {
				for await (const chunk of streamChat(apiMessages, openaiTools, abortController.signal)) {
					if (chunk.type === 'usage') {
						const usage = chunk.data;
						// Global stats
						settings.lastInputTokens = usage.prompt_tokens;
						settings.lastOutputTokens = usage.completion_tokens;
						settings.totalInputTokens += usage.prompt_tokens;
						settings.totalOutputTokens += usage.completion_tokens;

						// Session stats
						if (chatId) {
							db.chats.get(chatId).then((chat) => {
								if (chat) {
									db.chats.update(chatId!, {
										lastInputTokens: usage.prompt_tokens,
										lastOutputTokens: usage.completion_tokens,
										totalInputTokens: (chat.totalInputTokens || 0) + usage.prompt_tokens,
										totalOutputTokens: (chat.totalOutputTokens || 0) + usage.completion_tokens
									});
								}
							});
						}
					} else if (chunk.type === 'thinking') {
						streamingThinking += chunk.data;
						
						if (Date.now() - lastUpdate > 50) {
							db.messages.update(messageId, { thinkingContent: streamingThinking });
							lastUpdate = Date.now();
						}
					} else if (chunk.type === 'content') {
						if (!assistantContent && streamingThinking) {
							// First content chunk after thinking
							const duration = (Date.now() - streamingStartTime) / 1000;
							db.messages.update(messageId, { thinkingDuration: duration });
						}
						assistantContent += chunk.data;
						streamingContent = assistantContent;
						
						// Update DB at most every 50ms for performance, or if it's the first chunk
						if (Date.now() - lastUpdate > 50 || assistantContent.length < 5) {
							db.messages.update(messageId, { content: assistantContent });
							lastUpdate = Date.now();
						}
					} else if (chunk.type === 'tool_call') {
						const tc = chunk.data as any;
						if (!streamingToolCalls[tc.index]) {
							streamingToolCalls[tc.index] = {
								id: tc.id,
								type: 'function',
								function: { name: '', arguments: '' }
							};
						}
						if (tc.function?.name) streamingToolCalls[tc.index].function.name += tc.function.name;
						if (tc.function?.arguments) streamingToolCalls[tc.index].function.arguments += tc.function.arguments;
						
						db.messages.update(messageId, { toolCalls: $state.snapshot(streamingToolCalls) });
					}
				}
			} catch (err: any) {
				if (err.name === 'AbortError') {
					assistantContent += '\n\n[Stopped by user]';
					streamingContent = assistantContent;
					keepGoing = false;
				} else {
					throw err;
				}
			} finally {
				abortController = null;
			}
			// Final update to ensure everything is saved
			await db.messages.update(messageId, { 
				content: streamingContent, 
				toolCalls: $state.snapshot(streamingToolCalls.filter(Boolean).length ? streamingToolCalls.filter(Boolean) : undefined)
			});
			
			const finalToolCalls = [...streamingToolCalls.filter(Boolean)];
			
			// Wait a bit for Dexie to propagate the update to the subscription
			await new Promise(resolve => setTimeout(resolve, 100));

			streamingMessageId = null;
			streamingContent = '';
			streamingToolCalls = [];

			if (finalToolCalls.length > 0) {
				for (const tc of finalToolCalls) {
					const toolName = tc.function.name;
					const toolArgs = JSON.parse(tc.function.arguments);
					
					const mcpTool = filteredMcpTools.find(t => t.tool.name === toolName);
					if (mcpTool) {
						const client = new MCPClient(mcpTool.serverUrl);
						const result = await client.callTool(toolName, toolArgs);
						
						await db.messages.add({
							chatId: chatId!,
							role: 'tool',
							content: JSON.stringify(result),
							createdAt: Date.now(),
							toolCallId: tc.id
						} as any);
						continue;
					}

					const localTool = filteredLocalTools.find(t => t.name === toolName);
					if (localTool) {
						const result = await localTool.handler(toolArgs);
						await db.messages.add({
							chatId: chatId!,
							role: 'tool',
							content: JSON.stringify(result),
							createdAt: Date.now(),
							toolCallId: tc.id
						} as any);
					}
				}
				// Continue the loop to let the LLM process tool results
			} else {
				keepGoing = false;
			}
		}
	}
	let acceptedMimeTypes = $derived.by(() => {
		const types = [];
		if (settings.supportsImages) types.push('image/*');
		if (settings.supportsAudio) types.push('audio/*');
		if (settings.supportsVideo) types.push('video/*');
		return types.join(',');
	});
</script>

<svelte:head>
	<title>{chatId ? (currentChat?.title || 'New Chat') : 'knitnox Assistant'}</title>
</svelte:head>

<div class="relative flex h-full flex-col bg-white dark:bg-zinc-900 overflow-hidden">
	<header class="shrink-0 border-b border-zinc-200 bg-white/80 backdrop-blur-md dark:border-zinc-800 dark:bg-zinc-900/80 z-20">
		<div class="flex h-14 items-center px-1 sm:px-4">
			<div class="flex items-center gap-1">
				<button 
					onclick={onToggleSidebar}
					class="rounded-lg p-1 hover:bg-zinc-100 dark:hover:bg-zinc-800 shrink-0"
					aria-label="Toggle Sidebar"
				>
					<Menu size={20} />
				</button>
			</div>
			<div class="mx-auto flex w-full max-w-4xl items-center justify-between gap-2 pl-1 sm:pl-2">
				<div class="flex flex-col min-w-0 leading-tight">
					<div class="flex items-center gap-2.5 mb-0.5">
						{#if chatId}
							<h1 class="truncate text-lg sm:text-base font-bold tracking-tight">
								{currentChat?.title || 'New Chat'}
							</h1>
						{:else}
							<h1 class="text-lg sm:text-base font-bold tracking-tight">
								knitnox Assistant
							</h1>
						{/if}
					</div>
					<div class="flex items-center gap-1.5">
						{#if isCheckingSettings}
							<SettingsIcon size={12} class="animate-spin opacity-50" />
						{:else if settings.model?.trim()}
							<div class="flex items-center gap-1 text-zinc-400">
								<Brain size={14} />
								<span class="text-[11px] font-mono tracking-widest matrix-text">
									{toTitleCase(settings.model)}
								</span>
							</div>
						{:else}
							<button 
								onclick={onOpenSettings}
								class="text-[11px] font-bold text-blue-600 hover:underline cursor-pointer"
							>
								Open Settings
							</button>
						{/if}
					</div>
				</div>

				{#if chatId}
					<button 
						onclick={onNewChat}
						class="text-zinc-900 dark:text-white transition-all active:scale-95 shrink-0 mr-3 sm:mr-5 hover:text-zinc-600 dark:hover:text-zinc-300"
						aria-label="New Chat"
						title="New Chat"
					>
						<PlusCircle size={28} />
					</button>
				{/if}
			</div>
		</div>
	</header>

	<div bind:this={messagesContainer} use:autoscroll class="flex-1 overflow-y-auto p-1.5 sm:p-4 pb-36 sm:pb-40">
		<div class="mx-auto max-w-4xl space-y-3 sm:space-y-6">
			{#if (messagesList && messagesList.length > 0) || isStreaming || streamingError}
				{#if messagesList && messagesList.length > 0}
					{#each messagesList as message}
						{#if message.role !== 'tool' && message.id !== streamingMessageId}
							<div class="group/message flex gap-3 sm:gap-4 {message.role === 'user' ? 'justify-end' : ''}">
								<div
									class="flex w-full gap-2 sm:gap-3 {message.role === 'user'
										? 'flex-row-reverse'
										: 'flex-col'}"
								>
									<div class="flex items-start gap-2 sm:gap-3 {message.role === 'user' ? 'flex-row' : 'flex-row'}">
										<div
											class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg {message.role ===
											'user'
												? 'bg-gray-600 text-white'
												: 'bg-zinc-200 dark:bg-zinc-800'}"
										>
											{#if message.role === 'user'}
												<User size={18} />
											{:else}
												<Bot size={18} />
											{/if}
										</div>
										<div class="{message.role === 'assistant' ? 'flex-1' : ''} group relative min-h-[1.5rem]">
											{#if message.role === 'assistant'}
												<div class="absolute -top-2 -right-2 opacity-0 group-hover/message:opacity-100 flex items-center gap-1 bg-white dark:bg-zinc-800 rounded-md px-1 py-0.5 border border-zinc-200 dark:border-zinc-700 z-30 transition-all shadow-md ring-1 ring-black/5">
													<button
														onclick={() => confirmDelete(message)}
														class="p-1 text-zinc-500 hover:text-red-500 transition-colors"
														title="Delete from here"
													>
														<Trash2 size={12} />
													</button>
												</div>
											{/if}
											{#if message.thinkingContent}
												<details class="group rounded-xl border border-zinc-200 bg-zinc-50/50 p-2 dark:border-zinc-800 dark:bg-zinc-800/30">
													<summary class="flex cursor-pointer list-none items-center gap-2 text-xs font-medium text-zinc-500">
														<ChevronRight size={14} class="transition-transform group-open:rotate-90" />
														<span>{message.thinkingDuration ? `Thought (${message.thinkingDuration.toFixed(1)}s)` : 'Thought'}</span>

														{#if message.toolCalls}
															<div class="flex flex-wrap gap-1 ml-auto">
																{#each message.toolCalls as tc}
																	{@const toolResult = messagesList.find(m => m.role === 'tool' && m.toolCallId === tc.id)}
																	<button 
																		onclick={(e) => {
																			e.preventDefault();
																			e.stopPropagation();
																			inspectToolData = {
																				isOpen: true,
																				toolName: tc.function.name,
																				args: JSON.parse(tc.function.arguments),
																				result: toolResult ? JSON.parse(toolResult.content) : 'Pending...'
																			};
																		}}
																		class="flex items-center gap-1 rounded-md bg-blue-50 px-1.5 py-0.5 text-[10px] font-bold text-blue-600 hover:bg-blue-100 dark:bg-blue-900/30 dark:text-blue-400"
																	>
																		<Wrench size={10} />
																		{tc.function.name}
																	</button>
																{/each}
															</div>
														{/if}
													</summary>
													<div 
														use:autoscroll
														class="mt-2 max-h-48 overflow-y-auto border-l-2 border-zinc-200 pl-3 text-xs italic text-zinc-600 dark:border-zinc-700 dark:text-zinc-400 custom-scrollbar whitespace-pre-wrap"
													>
														{message.thinkingContent}
													</div>
												</details>
											{/if}

											{#if !message.thinkingContent && message.role === 'assistant' && message.toolCalls}
												<div class="flex flex-wrap gap-1 mb-2">
													{#each message.toolCalls as tc}
														{@const toolResult = messagesList.find(m => m.role === 'tool' && m.toolCallId === tc.id)}
														<button 
															onclick={() => {
																inspectToolData = {
																	isOpen: true,
																	toolName: tc.function.name,
																	args: JSON.parse(tc.function.arguments),
																	result: toolResult ? JSON.parse(toolResult.content) : 'Pending...'
																};
															}}
															class="flex items-center gap-1 rounded-md bg-blue-50 px-1.5 py-0.5 text-[10px] font-bold text-blue-600 hover:bg-blue-100 dark:bg-blue-900/30 dark:text-blue-400 border border-blue-100 dark:border-blue-800"
														>
															<Wrench size={10} />
															{tc.function.name}
														</button>
													{/each}
												</div>
											{/if}
										</div>
									</div>

									{#if message.content}
										<div class="space-y-2 {message.role === 'user' && editingMessageId === message.id ? 'flex-1 min-w-0' : ''}">
											<div
												class="rounded-2xl px-3 py-1.5 {message.role === 'user'
													? 'bg-gray-600 text-white'
													: 'bg-zinc-100 dark:bg-zinc-800 prose prose-sm dark:prose-invert max-w-none'}"
											>
												{#if message.attachments}
													<div class="mb-2 flex flex-wrap gap-2">
														{#each message.attachments as att}
															<div class="overflow-hidden rounded-lg border border-zinc-200 dark:border-zinc-700">
																{#if att.type === 'image'}
																	<img src={att.data} alt={att.name} class="max-h-48 object-contain" />
																{:else if att.type === 'audio'}
																	<audio src={att.data} controls class="h-10 w-48"></audio>
																{:else if att.type === 'video'}
																	<video src={att.data} controls class="max-h-48 w-48"></video>
																{:else}
																	<div class="flex items-center gap-2 bg-zinc-50 p-2 dark:bg-zinc-900">
																		<File size={16} />
																		<span class="text-xs">{att.name}</span>
																	</div>
																{/if}
															</div>
														{/each}
													</div>
												{/if}

												{#if message.role === 'user'}
													{#if editingMessageId === message.id}
														<div class="flex flex-col gap-2 w-full">
															<textarea
																bind:value={editingContent}
																data-editing="true"
																class="w-full bg-zinc-700 text-white rounded-lg p-2 outline-none border border-zinc-500 focus:border-blue-400 min-h-[100px]"
																onkeydown={(e) => {
																	if (e.key === 'Enter' && !e.shiftKey) {
																		e.preventDefault();
																		handleEditSubmit();
																	}
																	if (e.key === 'Escape') cancelEdit();
																}}
															></textarea>
															<div class="flex justify-end gap-2">
																<button
																	onclick={cancelEdit}
																	class="px-2 py-1 text-xs font-medium hover:bg-white/10 rounded transition-colors"
																>
																	Cancel
																</button>
																<button
																	onclick={handleEditSubmit}
																	class="px-2 py-1 text-xs font-medium bg-blue-600 hover:bg-blue-500 rounded transition-colors"
																>
																	Save
																</button>
															</div>
														</div>
													{:else}
														<div class="px-2 py-1">
															<p class="whitespace-pre-wrap leading-relaxed">{message.content}</p>
														</div>
													{/if}
												{:else}
													<div class="markdown-content">
														{#each marked.lexer(message.content) as token, i (i)}
															{#if token.type === 'code'}
																{@const codeId = `${message.id}-${i}`}
																	<div class="group relative my-2 overflow-hidden rounded-xl bg-[#1e1e1e]">
																		<div class="hidden items-center justify-between bg-white/5 px-3 py-1 backdrop-blur-sm sm:flex">
																			<span class="text-[10px] font-mono font-medium text-zinc-500 uppercase tracking-wider">{token.lang || 'code'}</span>
																			<button
																				onclick={() => copyToClipboard(token.text, codeId)}
																				class="flex items-center gap-1 rounded-md px-1.5 py-0.5 text-xs text-zinc-400 transition-all hover:bg-white/20 hover:text-white active:scale-95"
																			>
																				{#if copiedStates[codeId]}
																					<Check size={12} class="text-green-500" />
																					<span class="text-green-500 text-[10px] font-bold">Copied!</span>
																				{:else}
																					<Copy size={12} />
																					<span class="text-[10px]">Copy</span>
																				{/if}
																			</button>
																		</div>
																		<button
																			onclick={() => copyToClipboard(token.text, codeId)}
																			class="absolute right-2 top-2 z-10 flex h-8 w-8 items-center justify-center rounded-lg bg-zinc-900/90 text-zinc-400 backdrop-blur-md transition-all active:scale-90 sm:hidden border border-white/10"
																			aria-label="Copy code"
																		>
																			{#if copiedStates[codeId]}
																				<Check size={16} class="text-green-500" />
																			{:else}
																				<Copy size={16} class="hover:text-white" />
																			{/if}
																		</button>
																		<div class="overflow-x-auto p-3 sm:p-2">
																			{@html DOMPurify.sanitize(marked.parse(token.raw) as string)}
																		</div>
																	</div>
															{:else}
																{@html DOMPurify.sanitize(marked.parse(token.raw) as string)}
															{/if}
														{/each}
													</div>
												{/if}
											</div>
											{#if message.role === 'user' && editingMessageId !== message.id}
												<div class="flex justify-end items-center gap-3 pr-2 -mt-1 -mb-1">
													<button
														onclick={() => handleEdit(message)}
														class="text-white opacity-50 hover:opacity-100 transition-opacity drop-shadow-md"
														title="Edit"
													>
														<Pencil size={18} />
													</button>
													<button
														onclick={() => confirmDelete(message)}
														class="text-white opacity-50 hover:opacity-100 hover:text-red-400 transition-colors drop-shadow-md"
														title="Delete"
													>
														<Trash2 size={18} />
													</button>
												</div>
											{/if}
										</div>
									{/if}
								</div>
							</div>
						{/if}
					{/each}
				{/if}

				{#if streamingMessageId}
					<div class="flex gap-3 sm:gap-4">
						<div class="flex w-full gap-2 sm:gap-3 flex-col">
							<div class="flex items-start gap-2 sm:gap-3 flex-row">
								<div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-zinc-200 dark:bg-zinc-800">
									<Bot size={18} />
								</div>
								<div class="overflow-hidden flex-1">
									{#if streamingThinking}
										<details open class="group rounded-xl border border-zinc-200 bg-zinc-50/50 p-2 dark:border-zinc-800 dark:bg-zinc-800/30">
											<summary class="flex cursor-pointer list-none items-center gap-2 text-xs font-medium text-zinc-500">
												<ChevronRight size={14} class="transition-transform group-open:rotate-90" />
												<span class="flex items-center gap-2">
													Thinking...
													<Loader2 size={12} class="animate-spin" />
												</span>
											</summary>
											<div 
												use:autoscroll
												class="mt-2 max-h-48 overflow-y-auto border-l-2 border-zinc-200 pl-3 text-xs italic text-zinc-600 dark:border-zinc-700 dark:text-zinc-400 custom-scrollbar whitespace-pre-wrap"
											>
												{streamingThinking}
											</div>
										</details>
									{/if}
								</div>
							</div>

							<div class="space-y-2">
								{#if streamingContent}
									<div class="rounded-2xl bg-zinc-100 px-3 py-1.5 dark:bg-zinc-800 prose prose-sm dark:prose-invert max-w-none">
										<div class="markdown-content">
											{#each streamingTokens as token, i (i)}
												<div class="token-container">
													{#if token.type === 'code'}
														{@const codeId = `streaming-${i}`}
														<div class="group relative my-2 overflow-hidden rounded-xl bg-[#1e1e1e]">
															<div class="hidden items-center justify-between bg-white/5 px-3 py-1 backdrop-blur-sm sm:flex">
																<span class="text-[10px] font-mono font-medium text-zinc-500 uppercase tracking-wider">{token.lang || 'code'}</span>
																<button
																	onclick={() => copyToClipboard(token.text, codeId)}
																	class="flex items-center gap-1 rounded-md px-1.5 py-0.5 text-xs text-zinc-400 transition-all hover:bg-white/20 hover:text-white active:scale-95"
																>
																	{#if copiedStates[codeId]}
																		<Check size={12} class="text-green-500" />
																		<span class="text-green-500 text-[10px] font-bold">Copied!</span>
																	{:else}
																		<Copy size={12} />
																		<span class="text-[10px]">Copy</span>
																	{/if}
																</button>
															</div>
															<button
																onclick={() => copyToClipboard(token.text, codeId)}
																class="absolute right-2 top-2 z-10 flex h-8 w-8 items-center justify-center rounded-lg bg-zinc-900/90 text-zinc-400 backdrop-blur-md transition-all active:scale-90 sm:hidden border border-white/10"
																aria-label="Copy code"
															>
																{#if copiedStates[codeId]}
																	<Check size={16} class="text-green-500" />
																{:else}
																	<Copy size={16} class="hover:text-white" />
																{/if}
															</button>
															<div class="overflow-x-auto p-3 sm:p-2">
																{@html DOMPurify.sanitize(marked.parse(token.raw) as string)}
															</div>
														</div>
													{:else}
														{@html DOMPurify.sanitize(marked.parse(token.raw) as string)}
													{/if}
												</div>
											{/each}
											<span class="inline-block w-1.5 h-4 ml-1 bg-zinc-400 dark:bg-zinc-500 animate-pulse align-middle"></span>
										</div>
									</div>
								{/if}

								{#if streamingToolCalls.length > 0}
									<div class="space-y-2">
										{#each streamingToolCalls as tc}
											{#if tc}
												<div class="flex items-center gap-2 rounded-lg bg-zinc-50 px-3 py-1.5 text-xs font-medium dark:bg-zinc-800/50">
													<Wrench size={14} class="text-blue-500" />
													<span>Using tool: <span class="text-blue-500">{tc.function.name || '...'}</span></span>
												</div>
											{/if}
										{/each}
									</div>
								{/if}

								{#if !streamingContent && !streamingThinking && streamingToolCalls.filter(Boolean).length === 0}
									<div class="flex items-center gap-2 rounded-2xl bg-zinc-100 px-3 py-1.5 dark:bg-zinc-800">
										<Loader2 size={18} class="animate-spin opacity-50" />
									</div>
								{/if}
							</div>
						</div>
					</div>
				{:else if isStreaming}
					<div class="flex gap-4">
						<div class="flex h-8 w-8 items-center justify-center rounded-lg bg-zinc-200 dark:bg-zinc-800">
							<Bot size={18} />
						</div>
						<div class="flex items-center gap-2 rounded-2xl bg-zinc-100 px-3 py-1.5 dark:bg-zinc-800">
							<Loader2 size={18} class="animate-spin opacity-50" />
						</div>
					</div>
				{/if}

				{#if streamingError}
					<div class="flex gap-4">
						<div class="flex h-8 w-8 items-center justify-center rounded-lg bg-red-100 text-red-600 dark:bg-red-900/30">
							<Bot size={18} />
						</div>
						<div class="rounded-2xl bg-red-50 px-3 py-1.5 text-red-600 dark:bg-red-900/20">
							<p class="text-sm font-medium">Error: {streamingError}</p>
						</div>
					</div>
				{/if}
			{:else}
				<LandingPage />
			{/if}
		</div>
	</div>

	<div class="absolute bottom-0 left-0 right-0 w-full px-2 sm:px-4 pt-12 pb-1 pointer-events-none bg-gradient-to-t from-white via-white/95 to-transparent dark:from-zinc-900 dark:via-zinc-900/95 z-20">
		<div class="pointer-events-auto">
			<form onsubmit={handleSubmit} class="mx-auto max-w-4xl">
				{#if attachments.length > 0}
					<div class="mb-2 flex flex-wrap gap-2 px-2">
						{#each attachments as att, i}
							<div class="relative group">
								<div class="h-16 w-16 overflow-hidden rounded-lg border border-zinc-200 dark:border-zinc-800 bg-zinc-50 dark:bg-zinc-900">
									{#if att.type === 'image'}
										<img src={att.data} alt="preview" class="h-full w-full object-cover" />
									{:else if att.type === 'audio'}
										<div class="flex h-full w-full items-center justify-center">
											<File size={20} class="text-blue-500" />
										</div>
									{:else if att.type === 'video'}
										<video src={att.data} class="h-full w-full object-cover"></video>
									{:else}
										<div class="flex h-full w-full items-center justify-center">
											<File size={20} />
										</div>
									{/if}
								</div>
								<button 
									type="button"
									onclick={() => removeAttachment(i)}
									class="absolute -top-1.5 -right-1.5 rounded-full bg-zinc-800 p-0.5 text-white shadow-sm hover:bg-zinc-700 dark:bg-white dark:text-zinc-800"
								>
									<CloseIcon size={12} />
								</button>
							</div>
						{/each}
					</div>
				{/if}

				<div class="flex flex-col rounded-2xl border-2 border-dotted border-zinc-300 bg-zinc-50 p-1.5 focus-within:border-blue-500/50 focus-within:ring-2 focus-within:ring-blue-500/20 dark:border-zinc-700 dark:bg-zinc-900 transition-all">
					<div class="relative flex items-center">
						{#if settings.supportsImages || settings.supportsAudio || settings.supportsVideo}
							<input 
								type="file" 
								multiple 
								class="hidden" 
								accept={acceptedMimeTypes}
								bind:this={fileInput}
								onchange={handleFileChange}
							/>
							<button
								type="button"
								onclick={() => fileInput?.click()}
								class="ml-1 flex h-9 w-9 shrink-0 items-center justify-center rounded-xl text-zinc-500 hover:bg-zinc-200/50 dark:hover:bg-zinc-800 transition-all"
								title="Attach Files"
							>
								<Paperclip size={20} />
							</button>
						{/if}

						<textarea
							bind:this={textareaElement}
							bind:value={input}
							rows="1"
							placeholder="Message..."
							class="flex-1 bg-transparent px-2 py-2 outline-none resize-none min-h-[44px] max-h-64 text-base sm:text-sm leading-relaxed"
							onkeydown={(e) => {
								if (e.key === 'Enter') {
									if (isMobile) return;
									if (!e.shiftKey) {
										e.preventDefault();
										handleSubmit(e as any);
									}
								}
							}}
						></textarea>
					</div>

					<div class="flex items-center justify-between mt-0.5 px-0.5">
						<div class="flex items-center gap-0.5">
							{#if hasMcpResources}
								<button
									type="button"
									onclick={() => { mcpInspectMode = 'resources'; isMcpInspectOpen = true; }}
									class="flex h-9 w-9 items-center justify-center rounded-xl text-zinc-500 hover:bg-zinc-200/50 dark:hover:bg-zinc-800 transition-all"
									title="MCP Resources"
								>
									<Library size={20} />
								</button>
							{/if}
							{#if hasMcpPrompts}
								<button
									type="button"
									onclick={() => { mcpInspectMode = 'prompts'; isMcpInspectOpen = true; }}
									class="flex h-9 w-9 items-center justify-center rounded-xl text-zinc-500 hover:bg-zinc-200/50 dark:hover:bg-zinc-800 transition-all"
									title="MCP Prompts"
								>
									<Terminal size={20} />
								</button>
							{/if}
							<button
								type="button"
								onclick={() => isToolsModalOpen = true}
								class="flex h-9 w-9 items-center justify-center rounded-xl text-zinc-500 hover:bg-zinc-200/50 dark:hover:bg-zinc-800 transition-all"
								title="Tool Settings"
							>
								<Toolbox size={20} />
							</button>
							<button
								type="button"
								onclick={onOpenSettings}
								class="flex h-9 w-9 items-center justify-center rounded-xl text-zinc-500 hover:bg-zinc-200/50 dark:hover:bg-zinc-800 transition-all"
								title="General Settings"
							>
								<Wrench size={20} />
							</button>
						</div>

						<div class="flex items-center gap-2">
							{#if isStreaming}
								<button
									type="button"
									onclick={stopGeneration}
									class="flex h-9 w-9 items-center justify-center rounded-xl text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 transition-all cursor-pointer"
									title="Stop Generation"
								>
									<Square size={16} fill="currentColor" />
								</button>
							{:else}
								<button
									type="submit"
									disabled={(!input.trim() && attachments.length === 0)}
									class="flex h-9 w-9 items-center justify-center rounded-xl text-blue-600 hover:bg-blue-50 dark:hover:bg-blue-900/20 disabled:opacity-30 disabled:hover:bg-transparent transition-all cursor-pointer"
									title="Send Message"
								>
									<SendHorizontal size={26} strokeWidth={2.5} />
								</button>
							{/if}
						</div>
					</div>
				</div>
			</form>
			{#if chatId}
				<div class="mx-auto max-w-4xl px-2 mt-0.5 flex items-center justify-center gap-2 text-[10px] sm:text-xs text-zinc-500 dark:text-zinc-400 font-mono">
					<span>Last: {((currentChat?.lastInputTokens || 0) / 1000).toFixed(2)}k in / {((currentChat?.lastOutputTokens || 0) / 1000).toFixed(2)}k out</span>
					<span class="opacity-30">|</span>
					<span>Total: {((currentChat?.totalInputTokens || 0) / 1000).toFixed(2)}k in / {((currentChat?.totalOutputTokens || 0) / 1000).toFixed(2)}k out</span>
				</div>
			{/if}
		</div>
	</div></div>

<ToolsSettingsModal bind:isOpen={isToolsModalOpen} />
<ToolInspectModal 
	bind:isOpen={inspectToolData.isOpen} 
	toolName={inspectToolData.toolName}
	args={inspectToolData.args}
	result={inspectToolData.result}
/>
<MCPInspectModal bind:isOpen={isMcpInspectOpen} bind:mode={mcpInspectMode} />
<ConfirmModal
	bind:isOpen={confirmModal.isOpen}
	title={confirmModal.title}
	message={confirmModal.message}
	confirmText={confirmModal.confirmText}
	isDanger={confirmModal.isDanger}
	onConfirm={confirmModal.onConfirm}
/>

<style>
	@keyframes token-fade-in {
		from {
			opacity: 0;
			transform: translateY(4px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	.token-container {
		animation: token-fade-in 0.3s ease-out forwards;
	}

	/* Optional: make the cursor more visible */
	.animate-pulse {
		animation: pulse 1s cubic-bezier(0.4, 0, 0.6, 1) infinite;
	}

	@keyframes pulse {
		0%, 100% {
			opacity: 1;
		}
		50% {
			opacity: 0.3;
		}
	}

	.matrix-text {
		text-shadow: 0 0 5px rgba(113, 113, 122, 0.2);
		letter-spacing: 0.15em;
	}
</style>
