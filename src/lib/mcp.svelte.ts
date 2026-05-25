export interface MCPTool {
	name: string;
	description?: string;
	inputSchema: any;
}

export class MCPClient {
	private sessionId: string | null = null;
	private endpoint: string | null = null;
	private eventSource: EventSource | null = null;

	constructor(public url: string) {}

	async connect(): Promise<void> {
		return new Promise((resolve, reject) => {
			this.eventSource = new EventSource(this.url);
			
			this.eventSource.addEventListener('endpoint', (event: any) => {
				const url = new URL(event.data, this.url);
				this.endpoint = url.toString();
				this.sessionId = url.searchParams.get('sessionId');
				resolve();
			});

			this.eventSource.onerror = (err) => {
				reject(err);
			};
		});
	}

	async request(method: string, params: any): Promise<any> {
		if (!this.endpoint) await this.connect();

		const response = await fetch(this.endpoint!, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				jsonrpc: '2.0',
				id: Date.now(),
				method,
				params
			})
		});

		if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
		
		// For SSE, responses usually come back via the SSE stream, 
		// but some implementations might return them directly in the POST response.
		// Standard MCP SSE transport expects response in the SSE stream.
		// This is a bit complex for a simple client.
		
		// Simplified: let's assume the POST response contains the result if available, 
		// OR we wait for a message in the SSE stream with the same ID.
		
		return await response.json();
	}

	async getTools(): Promise<MCPTool[]> {
		const res = await this.request('tools/list', {});
		return res.result?.tools || [];
	}

	async callTool(name: string, args: any): Promise<any> {
		const res = await this.request('tools/call', { name, arguments: args });
		return res.result;
	}

	close() {
		this.eventSource?.close();
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
