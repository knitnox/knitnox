export interface MCPTool {
	name: string;
	description?: string;
	inputSchema: any;
}

export class MCPClient {
	private sessionId: string | null = null;
	private endpoint: string | null = null;
	private abortController: AbortController | null = null;
	private pendingRequests: Map<number | string, { resolve: (val: any) => void; reject: (err: any) => void }> = new Map();
	private messageIdCounter = 1;

	constructor(public url: string) {
		this.endpoint = url;
	}

	async connect(): Promise<void> {
		if (this.sessionId) return;

		try {
			// 1. Initialize Handshake via POST
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

			const initResponse = await fetch(this.endpoint!, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					'Accept': 'application/json, text/event-stream'
				},
				body: JSON.stringify(initPayload)
			});

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
			await this.sendRaw({
				jsonrpc: '2.0',
				method: 'notifications/initialized',
				params: {}
			}, true);
			
		} catch (e) {
			console.error("Failed to connect to MCP server:", e);
			throw e;
		}
	}

	private async startMessageStream() {
		this.abortController = new AbortController();
		try {
			const response = await fetch(this.endpoint!, {
				method: 'GET',
				headers: {
					'Accept': 'text/event-stream',
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
				buffer = lines.pop() || ''; // Keep the incomplete line in the buffer

				for (const line of lines) {
					if (line.startsWith('data: ')) {
						try {
							const data = JSON.parse(line.substring(6));
							this.handleIncomingMessage(data);
						} catch (e) {
							console.warn("Failed to parse SSE message data:", line, e);
						}
					}
				}
			}
		} catch (e: any) {
			if (e.name !== 'AbortError') {
				console.error("Streamable HTTP GET stream closed with error:", e);
			}
		}
	}

	private handleIncomingMessage(data: any) {
		if (data.id !== undefined && this.pendingRequests.has(data.id)) {
			const { resolve, reject } = this.pendingRequests.get(data.id)!;
			this.pendingRequests.delete(data.id);
			if (data.error) {
				reject(data.error);
			} else {
				resolve(data);
			}
		} else if (data.method) {
			// Handle server notifications (e.g. notifications/progress)
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
		// Also handle standard JSON if the server doesn't wrap in SSE
		if (text.trim().startsWith('{')) {
			try {
				const data = JSON.parse(text);
				this.handleIncomingMessage(data);
			} catch (e) {
				// Ignore
			}
		}
	}

	private async sendRaw(payload: any, isNotification = false): Promise<any> {
		const headers: Record<string, string> = {
			'Content-Type': 'application/json',
			'Accept': 'application/json, text/event-stream'
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
			this.pendingRequests.set(payload.id, { resolve: resolvePromise!, reject: rejectPromise! });
		}

		try {
			const response = await fetch(this.endpoint!, {
				method: 'POST',
				headers,
				body: JSON.stringify(payload)
			});

			if (!response.ok) {
				const errText = await response.text();
				throw new Error(`HTTP error! status: ${response.status} - ${errText}`);
			}

			if (isNotification) return null;

			// FastMCP optimization: sometimes returns the response directly in the POST body.
			// If we get content here, process it.
			const text = await response.text();
			if (text.trim().length > 0) {
				this.processResponseText(text);
			}

			return await promise;
		} catch (e) {
			if (!isNotification && this.pendingRequests.has(payload.id)) {
				this.pendingRequests.delete(payload.id);
			}
			throw e;
		}
	}

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

	close() {
		if (this.abortController) {
			this.abortController.abort();
			this.abortController = null;
		}
		this.sessionId = null;
		this.pendingRequests.forEach(({ reject }) => reject(new Error('Client closed')));
		this.pendingRequests.clear();
	}
}

export async function getAllTools(urls: string[]): Promise<{ tool: MCPTool; serverUrl: string }[]> {
	const allTools: { tool: MCPTool; serverUrl: string }[] = [];
	for (const url of urls) {
		if (!url) continue;
		try {
			const client = new MCPClient(url);
			const tools = await client.getTools();
			allTools.push(...tools.map((tool) => ({ tool, serverUrl: url })));
		} catch (e) {
			console.error(`Failed to get tools from ${url}`, e);
		}
	}
	return allTools;
}