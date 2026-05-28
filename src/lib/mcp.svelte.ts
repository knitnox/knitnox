export interface MCPTool {
	name: string;
	description?: string;
	inputSchema: any;
}

const DEFAULT_TIMEOUT_MS = 30_000;
const POOL_IDLE_MAX_MS = 5 * 60_000; // Recycle connections after 5 min idle

// ---------------------------------------------------------------------------
// Connection Pool
// ---------------------------------------------------------------------------
class MCPConnectionPool {
	private clients = new Map<string, { client: MCPClient; lastUsed: number }>();
	private cleanupInterval: ReturnType<typeof setInterval> | null = null;

	get(url: string, timeoutMs = DEFAULT_TIMEOUT_MS): MCPClient {
		const entry = this.clients.get(url);
		if (entry) {
			entry.lastUsed = Date.now();
			// Reconnect if the underlying session was closed
			entry.client.ensureAlive();
			return entry.client;
		}
		const client = new MCPClient(url, timeoutMs);
		this.clients.set(url, { client, lastUsed: Date.now() });
		this.startCleanup();
		return client;
	}

	invalidate(url: string) {
		const entry = this.clients.get(url);
		if (entry) {
			entry.client.close();
			this.clients.delete(url);
		}
	}

	private startCleanup() {
		if (this.cleanupInterval) return;
		this.cleanupInterval = setInterval(() => {
			const now = Date.now();
			for (const [url, entry] of this.clients) {
				if (now - entry.lastUsed > POOL_IDLE_MAX_MS) {
					entry.client.close();
					this.clients.delete(url);
				}
			}
			if (this.clients.size === 0 && this.cleanupInterval) {
				clearInterval(this.cleanupInterval);
				this.cleanupInterval = null;
			}
		}, 60_000);
	}

	closeAll() {
		for (const [, entry] of this.clients) {
			entry.client.close();
		}
		this.clients.clear();
		if (this.cleanupInterval) {
			clearInterval(this.cleanupInterval);
			this.cleanupInterval = null;
		}
	}
}

export const mcpPool = new MCPConnectionPool();

// Close all pooled connections on page unload
if (typeof window !== 'undefined') {
	window.addEventListener('beforeunload', () => mcpPool.closeAll());
}

// ---------------------------------------------------------------------------
// MCPClient
// ---------------------------------------------------------------------------
export class MCPClient {
	private sessionId: string | null = null;
	private endpoint: string | null = null;
	private abortController: AbortController | null = null;
	private pendingRequests: Map<
		number | string,
		{
			resolve: (val: any) => void;
			reject: (err: any) => void;
			timer: ReturnType<typeof setTimeout>;
		}
	> = new Map();
	private messageIdCounter = 1;
	public createdAt = Date.now();

	constructor(public url: string, public timeoutMs: number = DEFAULT_TIMEOUT_MS) {
		this.endpoint = url;
	}

	// ---- Public helpers ----

	ensureAlive() {
		// If the session was closed externally, reset so a new connect() happens on next request
		if (!this.sessionId) {
			this.messageIdCounter = 1;
		}
	}

	get isConnected() {
		return this.sessionId !== null;
	}

	// ---- Connection lifecycle ----

	async connect(): Promise<void> {
		if (this.sessionId) return;

		try {
			const initPayload = {
				jsonrpc: '2.0',
				id: this.messageIdCounter++,
				method: 'initialize',
				params: {
					protocolVersion: '2024-11-05',
					capabilities: {},
					clientInfo: { name: 'KnitnoxClient', version: '1.0.0' }
				}
			};

			const controller = new AbortController();
			const timer = setTimeout(() => controller.abort(), this.timeoutMs);

			let initResponse: Response;
			try {
				initResponse = await fetch(this.endpoint!, {
					method: 'POST',
					headers: {
						'Content-Type': 'application/json',
						'Accept': 'application/json, text/event-stream'
					},
					body: JSON.stringify(initPayload),
					signal: controller.signal
				});
			} finally {
				clearTimeout(timer);
			}

			this.sessionId = initResponse.headers.get('mcp-session-id');
			if (!this.sessionId) {
				console.warn('No mcp-session-id returned during initialization. Might be a legacy server.');
			}

			// Read the init response
			const initText = await initResponse.text();
			this.processResponseText(initText);

			// 2. Open persistent GET stream for server-to-client messages (Streamable HTTP standard)
			if (this.sessionId) {
				this.startMessageStream();
			}

			// 3. Send initialized notification
			await this.sendRaw(
				{
					jsonrpc: '2.0',
					method: 'notifications/initialized',
					params: {}
				},
				true
			);
		} catch (e: any) {
			// On any failure, reset so next call retries cleanly
			this.sessionId = null;
			if (this.abortController) {
				this.abortController.abort();
				this.abortController = null;
			}
			console.error('Failed to connect to MCP server:', e);
			throw new Error(`MCP connection failed to ${this.url}: ${e.message || e}`);
		}
	}

	private async startMessageStream() {
		this.abortController = new AbortController();
		try {
			const response = await fetch(this.endpoint!, {
				method: 'GET',
				headers: {
					Accept: 'text/event-stream',
					'mcp-session-id': this.sessionId!
				},
				signal: this.abortController.signal
			});

			if (!response.body) return;

			const reader = response.body.getReader();
			const decoder = new TextDecoder();
			let buffer = '';

			while (true) {
				const { done, value } = await reader.read();
				if (done) break;

				buffer += decoder.decode(value, { stream: true });
				const lines = buffer.split('\n');
				buffer = lines.pop() || '';

				for (const line of lines) {
					if (line.startsWith('data: ')) {
						try {
							const data = JSON.parse(line.substring(6));
							this.handleIncomingMessage(data);
						} catch (e) {
							console.warn('Failed to parse SSE message data:', line, e);
						}
					}
				}
			}
		} catch (e: any) {
			if (e.name !== 'AbortError') {
				console.error('Streamable HTTP GET stream closed with error:', e);
			}
			// SSE stream died — close all pending requests so they don't hang
			this.failAllPending(new Error(`MCP stream closed for ${this.url}: ${e.message || e}`));
		}
	}

	// ---- Message handling ----

	private handleIncomingMessage(data: any) {
		if (data.id !== undefined && this.pendingRequests.has(data.id)) {
			const entry = this.pendingRequests.get(data.id)!;
			this.pendingRequests.delete(data.id);
			clearTimeout(entry.timer);
			if (data.error) {
				entry.reject(data.error);
			} else {
				entry.resolve(data);
			}
		} else if (data.method) {
			console.log('Received server notification:', data);
		}
	}

	private processResponseText(text: string) {
		const lines = text.split('\n');
		for (const line of lines) {
			if (line.startsWith('data: ')) {
				try {
					const data = JSON.parse(line.substring(6));
					this.handleIncomingMessage(data);
				} catch (e) {
					// Ignore
				}
			}
		}
		if (text.trim().startsWith('{')) {
			try {
				const data = JSON.parse(text);
				this.handleIncomingMessage(data);
			} catch (e) {
				// Ignore
			}
		}
	}

	// ---- Low-level transport ----

	private async sendRaw(payload: any, isNotification = false): Promise<any> {
		const headers: Record<string, string> = {
			'Content-Type': 'application/json',
			Accept: 'application/json, text/event-stream'
		};
		if (this.sessionId) {
			headers['mcp-session-id'] = this.sessionId;
		}

		let resolvePromise: (val: any) => void;
		let rejectPromise: (err: any) => void;
		const promise = new Promise((resolve, reject) => {
			resolvePromise = resolve;
			rejectPromise = reject;
		});

		if (!isNotification) {
			const timer = setTimeout(() => {
				if (this.pendingRequests.has(payload.id)) {
					this.pendingRequests.delete(payload.id);
					rejectPromise!(
						new Error(`MCP request timed out after ${this.timeoutMs / 1000}s: ${payload.method} (${this.url})`)
					);
				}
			}, this.timeoutMs);

			this.pendingRequests.set(payload.id, {
				resolve: resolvePromise!,
				reject: rejectPromise!,
				timer
			});
		}

		try {
			const controller = new AbortController();
			// Shorter timeout on the actual HTTP fetch so we don't wait twice
			const httpTimer = setTimeout(() => controller.abort(), this.timeoutMs + 2000);

			let response: Response;
			try {
				response = await fetch(this.endpoint!, {
					method: 'POST',
					headers,
					body: JSON.stringify(payload),
					signal: controller.signal
				});
			} finally {
				clearTimeout(httpTimer);
			}

			if (!response.ok) {
				const errText = await response.text();
				throw new Error(`HTTP error! status: ${response.status} - ${errText}`);
			}

			if (isNotification) return null;

			// FastMCP optimization: sometimes returns the response directly in the POST body
			const text = await response.text();
			if (text.trim().length > 0) {
				this.processResponseText(text);
				// If the response was resolved inline, clear the timer
				if (!this.pendingRequests.has(payload.id)) {
					return await promise; // Already resolved via processResponseText
				}
			}

			// Wait for the response to come via SSE
			return await promise;
		} catch (e) {
			if (!isNotification && this.pendingRequests.has(payload.id)) {
				const entry = this.pendingRequests.get(payload.id)!;
				clearTimeout(entry.timer);
				this.pendingRequests.delete(payload.id);
			}
			throw e;
		}
	}

	private failAllPending(reason: Error) {
		for (const [, entry] of this.pendingRequests) {
			clearTimeout(entry.timer);
			entry.reject(reason);
		}
		this.pendingRequests.clear();
	}

	// ---- High-level API ----

	async request(method: string, params: any): Promise<any> {
		if (!this.sessionId) {
			await this.connect();
		}
		return await this.sendRaw({
			jsonrpc: '2.0',
			id: this.messageIdCounter++,
			method,
			params
		});
	}

	async callTool(name: string, args: any): Promise<any> {
		const res = await this.request('tools/call', { name, arguments: args });
		return res.result;
	}

	async getTools(): Promise<MCPTool[]> {
		const res = await this.request('tools/list', {});
		return res.result?.tools || [];
	}

	async listResources() {
		const res = await this.request('resources/list', {});
		return res.result?.resources || [];
	}

	async readResource(uri: string) {
		const res = await this.request('resources/read', { uri });
		return res.result?.contents || [];
	}

	async listPrompts() {
		const res = await this.request('prompts/list', {});
		return res.result?.prompts || [];
	}

	async getPrompt(name: string, args?: any) {
		const res = await this.request('prompts/get', { name, arguments: args });
		return res.result;
	}

	// ---- Cleanup ----

	close() {
		if (this.abortController) {
			this.abortController.abort();
			this.abortController = null;
		}
		this.sessionId = null;
		this.failAllPending(new Error('MCP client closed'));
	}
}

// ---------------------------------------------------------------------------
// Helper — get all tools across servers
// ---------------------------------------------------------------------------
export async function getAllTools(urls: string[]): Promise<{ tool: MCPTool; serverUrl: string }[]> {
	const allTools: { tool: MCPTool; serverUrl: string }[] = [];
	for (const url of urls) {
		if (!url) continue;
		try {
			const client = new MCPClient(url);
			const tools = await client.getTools();
			client.close();
			allTools.push(...tools.map((tool) => ({ tool, serverUrl: url })));
		} catch (e) {
			console.error(`Failed to get tools from ${url}`, e);
		}
	}
	return allTools;
}