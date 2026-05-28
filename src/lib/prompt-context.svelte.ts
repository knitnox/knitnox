import { toast } from './toast.svelte';

export interface PromptAction {
	type: 'input' | 'messages';
	content: string | any[];
}

class PromptContext {
	private _pendingAction = $state<PromptAction | null>(null);

	get pendingAction() {
		return this._pendingAction;
	}

	applyPrompt(messages: any[]) {
		if (!messages || messages.length === 0) return;

		// Case 0: Check if it's a single message that contains a JSON-encoded set of messages
		// This is a workaround for MCP SDKs that only allow 'user' and 'assistant' roles
		if (messages.length === 1 && typeof messages[0].content?.text === 'string') {
			const text = messages[0].content.text.trim();
			if (text.startsWith('[') && text.endsWith(']')) {
				try {
					const parsed = JSON.parse(text);
					if (Array.isArray(parsed) && parsed.length > 0) {
						this._pendingAction = {
							type: 'messages',
							content: parsed
						};
						toast.add('Structured prompt applied to chat', 'success');
						return;
					}
				} catch (e) {
					// Not valid JSON or not an array, continue to other cases
				}
			}
		}

		// Case 1: Just a single text message or a string
		if (messages.length === 1 && messages[0].role === 'user' && typeof messages[0].content?.text === 'string') {
			this._pendingAction = {
				type: 'input',
				content: messages[0].content.text
			};
			toast.add('Prompt loaded into message box', 'success');
		} else {
			// Case 2: Multiple messages or more complex structure
			this._pendingAction = {
				type: 'messages',
				content: messages
			};
			toast.add('Prompt messages applied to chat', 'success');
		}
	}

	consume() {
		const action = this._pendingAction;
		this._pendingAction = null;
		return action;
	}
}

export const promptContext = new PromptContext();
