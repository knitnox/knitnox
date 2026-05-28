/**
 * Shared store for passing MCP resource content from MCPInspectModal to Chat.
 * When a resource is "added to chat", its content is placed here and Chat.svelte picks it up.
 */
export interface ResourceContent {
	name: string;
	uri: string;
	content: string;
	sizeStr?: string;
	mimeType?: string;
}

let _pending = $state<ResourceContent[]>([]);

export const resourceContext = {
	get pending() {
		return _pending;
	},
	add(value: ResourceContent) {
		if (!_pending.find(r => r.uri === value.uri)) {
			_pending.push(value);
		}
	},
	remove(uri: string) {
		_pending = _pending.filter(r => r.uri !== uri);
	},
	consume(): ResourceContent[] {
		const val = [..._pending];
		_pending = [];
		return val;
	}
};