<script lang="ts">
	import { db, type Message, type Attachment } from '$lib/db';
	import { streamChat, generateChatTitle } from '$lib/openai.svelte';
	import { settings } from '$lib/settings.svelte';
	import { getAllTools, MCPClient } from '$lib/mcp.svelte';
	import { liveQuery } from 'dexie';
	import { Send, User, Bot, Loader2, Wrench, ChevronRight, Trash2, Paperclip, File, X as CloseIcon, Menu } from '@lucide/svelte';
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
			node.scrollTo({
				top: node.scrollHeight,
				behavior: 'smooth'
			});
		};
		
		const observer = new MutationObserver(update);
		observer.observe(node, { childList: true, characterData: true, subtree: true });
		
		return {
			destroy() {
				observer.disconnect();
			}
		};
	}

	let { chatId = $bindable(), onToggleSidebar } = $props<{ 
		chatId: number | null;
		onToggleSidebar?: () => void;
	}>();

	let input = $state('');
	let attachments = $state<Attachment[]>([]);
	let fileInput = $state<HTMLInputElement | null>(null);
	let isStreaming = $state(false);
	let messagesContainer: HTMLDivElement | null = $state(null);
	
	let streamingMessageId = $state<number | null>(null);
	let streamingContent = $state('');
	let streamingThinking = $state('');
	let streamingToolCalls = $state<any[]>([]);
	let streamingError = $state<string | null>(null);
	let streamingStartTime = $state<number>(0);

	let messagesList = $state<Message[]>([]);
	let currentChat = $state<{ title: string } | null>(null);

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
			messagesContainer.scrollTop = messagesContainer.scrollHeight;
		}
	}

	$effect(() => {
		if (messagesList.length || streamingContent || streamingToolCalls.length) {
			scrollToBottom();
		}
	});

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
		const openaiTools = mcpTools.map(({ tool }) => ({
			type: 'function' as const,
			function: {
				name: tool.name,
				description: tool.description,
				parameters: tool.inputSchema
			}
		}));

		let keepGoing = true;
		while (keepGoing) {
			const allMessages = await db.messages.where('chatId').equals(chatId!).toArray();
			
			// Context window logic:
			// 1. Get the last N messages based on settings.contextWindow
			// 2. For these messages, filter out 'thinking' (though they aren't separate messages in DB usually) 
			//    and 'tool' roles/tool_calls UNLESS they are in the very last 2 messages.
			
			const windowSize = settings.contextWindow || 20;
			const windowMessages = allMessages.slice(-windowSize);
			
			const apiMessages = [
				{ role: 'system', content: settings.systemPrompt },
				...windowMessages.filter((m, idx) => {
					// Always include the last 2 messages regardless of type (to preserve tool chain if it's recent)
					if (idx >= windowMessages.length - 2) return true;
					
					// Otherwise only include user and assistant messages that AREN'T tool calls
					if (m.role === 'user') return true;
					if (m.role === 'assistant' && !m.toolCalls?.length) return true;
					
					return false;
				}).map((m) => {
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

			let lastUpdate = Date.now();
			for await (const chunk of streamChat(apiMessages, openaiTools)) {
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
					
					const toolInfo = mcpTools.find(t => t.tool.name === toolName);
					if (toolInfo) {
						const client = new MCPClient(toolInfo.serverUrl);
						const result = await client.callTool(toolName, toolArgs);
						
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

<div class="flex h-full flex-col bg-white dark:bg-zinc-900">
	<header class="shrink-0 border-b border-zinc-200 bg-white/80 backdrop-blur-md dark:border-zinc-800 dark:bg-zinc-900/80">
		<div class="flex h-12 items-center px-1 sm:px-4">
			<button 
				onclick={onToggleSidebar}
				class="rounded-lg p-1 hover:bg-zinc-100 dark:hover:bg-zinc-800 shrink-0"
				aria-label="Toggle Sidebar"
			>
				<Menu size={18} />
			</button>
			<div class="mx-auto flex w-full max-w-4xl items-center justify-between gap-2 pl-1 sm:pl-2">
				<h1 class="truncate text-xs sm:text-sm font-semibold">
					{chatId ? (currentChat?.title || 'New Chat') : 'knitnox'}
				</h1>
				{#if chatId}
					<div class="flex items-center gap-3 sm:gap-6 text-[9px] sm:text-[10px]">
						<div class="flex flex-col items-end">
							<span class="font-bold uppercase tracking-wider text-zinc-600 opacity-60">Last</span>
							<span class="font-mono text-zinc-600 dark:text-zinc-400"
								>{currentChat?.lastInputTokens || 0}<span class="opacity-40 ml-0.5">I</span> / {currentChat?.lastOutputTokens || 0}<span class="opacity-40 ml-0.5">O</span></span
							>
						</div>
						<div class="flex flex-col items-end border-l border-zinc-200 pl-3 sm:pl-6 dark:border-zinc-800">
							<span class="font-bold uppercase tracking-wider text-zinc-600 opacity-60">Total</span>
							<span class="font-mono text-zinc-600 dark:text-zinc-400"
								>{((currentChat?.totalInputTokens || 0) / 1000).toFixed(1)}k / {(
									(currentChat?.totalOutputTokens || 0) / 1000
								).toFixed(1)}k</span
							>
						</div>
					</div>
				{/if}
			</div>
		</div>
	</header>

	<div bind:this={messagesContainer} class="flex-1 overflow-y-auto p-1.5 sm:p-4">
		<div class="mx-auto max-w-4xl space-y-3 sm:space-y-6">
			{#if messagesList}
				{#each messagesList as message}
					{#if message.role !== 'tool' && message.id !== streamingMessageId}
						<div class="flex gap-3 sm:gap-4 {message.role === 'user' ? 'justify-end' : ''}">
							<div
								class="flex max-w-[95%] sm:max-w-[90%] gap-2 sm:gap-3 {message.role === 'user'
									? 'flex-row-reverse'
									: 'flex-row'}"
							>
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
								<div class="space-y-2 overflow-hidden">
									{#if message.thinkingContent}
										<details class="group rounded-xl border border-zinc-200 bg-zinc-50/50 p-2 dark:border-zinc-800 dark:bg-zinc-800/30">
											<summary class="flex cursor-pointer list-none items-center gap-2 text-xs font-medium text-zinc-500">
												<ChevronRight size={14} class="transition-transform group-open:rotate-90" />
												<span>{message.thinkingDuration ? `Thought (${message.thinkingDuration.toFixed(1)}s)` : 'Thought'}</span>
											</summary>
											<div 
												use:autoscroll
												class="mt-2 max-h-48 overflow-y-auto border-l-2 border-zinc-200 pl-3 text-xs italic text-zinc-600 dark:border-zinc-700 dark:text-zinc-400 custom-scrollbar whitespace-pre-wrap"
											>
												{message.thinkingContent}
											</div>
											</details>
											{/if}									<div
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
										{#if message.content}
											{#if message.role === 'user'}
												<p class="whitespace-pre-wrap leading-relaxed">{message.content}</p>
											{:else}
												<div class="markdown-content">
													{@html renderMarkdown(message.content)}
												</div>
											{/if}
										{/if}
									</div>
									{#if message.toolCalls}
										<div class="space-y-2">
											{#each message.toolCalls as tc}
												<div class="flex items-center gap-2 rounded-lg bg-zinc-50 px-3 py-1.5 text-xs font-medium dark:bg-zinc-800/50">
													<Wrench size={14} class="text-blue-500" />
													<span>Using tool: <span class="text-blue-500">{tc.function.name}</span></span>
												</div>
											{/each}
										</div>
									{/if}
								</div>
							</div>
						</div>
					{/if}
				{/each}

				{#if streamingMessageId}
					<div class="flex gap-4">
						<div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-zinc-200 dark:bg-zinc-800">
							<Bot size={18} />
						</div>
						<div class="space-y-2 overflow-hidden max-w-[90%]">
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
									{/if}							{#if streamingContent}
								<div class="rounded-2xl bg-zinc-100 px-3 py-1.5 dark:bg-zinc-800 prose prose-sm dark:prose-invert max-w-none">
									<div class="markdown-content">
										{@html renderMarkdown(streamingContent)}
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
				{:else if isStreaming && messagesList[messagesList.length - 1]?.role === 'user'}
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
			{/if}
		</div>
	</div>

	<div class="border-t border-zinc-200 p-2 sm:p-4 dark:border-zinc-800">
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
			<div class="relative flex items-end gap-2 rounded-2xl border border-zinc-200 bg-zinc-50 p-1.5 sm:p-2 focus-within:ring-2 focus-within:ring-blue-500 dark:border-zinc-800 dark:bg-zinc-950">
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
						class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl text-zinc-500 hover:bg-zinc-100 dark:hover:bg-zinc-800 transition-colors"
					>
						<Paperclip size={20} />
					</button>
				{/if}
				<textarea
					bind:value={input}
					placeholder="Message..."
					class="flex-1 bg-transparent px-2 py-1 outline-none resize-none min-h-[40px] max-h-48 text-sm"
					onkeydown={(e) => {
						if (e.key === 'Enter' && !e.shiftKey) {
							e.preventDefault();
							handleSubmit(e as any);
						}
					}}
				></textarea>
				<button
					type="submit"
					disabled={(!input.trim() && attachments.length === 0) || isStreaming}
					class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-blue-600 text-white shadow-sm hover:bg-blue-700 disabled:opacity-50 disabled:hover:bg-blue-600 transition-all cursor-pointer"
				>
					{#if isStreaming}
						<Loader2 size={18} class="animate-spin" />
					{:else}
						<Send size={18} />
					{/if}
				</button>
			</div>
		</form>
		<p class="mt-2 text-center text-xs text-zinc-500">
			knitnox can make mistakes. Check important info.
		</p>
	</div>
</div>
