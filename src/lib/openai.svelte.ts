import OpenAI from 'openai';
import { settings } from './settings.svelte';

export function createOpenAIClient() {
	const apiKey = settings.apiKey || 'sk-dummy';
	const baseURL = settings.baseUrl?.trim() || 'https://api.openai.com/v1';
	
	const isOpenRouter = baseURL.includes('openrouter.ai');

	return new OpenAI({
		apiKey,
		baseURL,
		dangerouslyAllowBrowser: true,
		defaultHeaders: isOpenRouter ? {
			'HTTP-Referer': window.location.origin,
			'X-Title': 'OpenWebUI-Lite'
		} : undefined
	});
}

export async function generateChatTitle(firstMessage: string) {
	try {
		const openai = createOpenAIClient();
		const response = await openai.chat.completions.create({
			model: settings.model,
			messages: [
				{
					role: 'system',
					content: 'Generate a very short, 3-5 word title for a chat session based on the first message provided by the user. Respond ONLY with the title text.'
				},
				{ role: 'user', content: firstMessage }
			]
		});
		return response.choices[0]?.message?.content?.replace(/^["']|["']$/g, '') || firstMessage.slice(0, 30);
	} catch (error) {
		console.error('Title generation error:', error);
		return firstMessage.slice(0, 30);
	}
}

export async function* streamChat(messages: { role: string; content: string }[], tools?: any[]) {
	try {
		const openai = createOpenAIClient();
		const stream = await openai.chat.completions.create({
			model: settings.model,
			messages: messages as any,
			tools: tools?.length ? tools : undefined,
			stream: true,
			stream_options: { include_usage: true },
			...(settings.enableThinking ? { include_reasoning: true } : {})
		} as any);

		for await (const chunk of stream) {
			if (chunk.usage) {
				yield { type: 'usage', data: chunk.usage };
			}

			const choice = chunk.choices?.[0];
			if (!choice) continue;

			const delta = choice.delta as any;
			if (!delta) continue;
			
			// Handle OpenRouter reasoning/thinking standard
			const reasoning = delta.reasoning || delta.reasoning_content;
			if (reasoning) {
				yield { type: 'thinking', data: reasoning };
			}

			if (delta.tool_calls) {
				for (const toolCall of delta.tool_calls) {
					yield { type: 'tool_call', data: toolCall };
				}
			}

			const content = delta.content || '';
			if (content) {
				yield { type: 'content', data: content };
			}
		}
	} catch (error: any) {
		console.error('OpenAI Stream Error:', error);
		throw error;
	}
}
