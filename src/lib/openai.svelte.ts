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
			'X-Title': 'knitnox'
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
					content: 'Generate a very short, 3-5 word title for a chat session based on the first message provided by the user. Use meaningful markdown or a relevant emoji at the beginning of the title to make it look nice and easy to spot. Respond ONLY with the title text.'
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

export async function summarizeChat(messages: any[], oldSummary?: string) {
	try {
		const openai = createOpenAIClient();
		const systemPrompt = `You are a memory module for a long-running chat. 
Your goal is to extract important insights from the current chat history and update the long-term memory.
Create a very nice and LLM-friendly summary that includes:
- Summary of the conversation so far
- Key points
- Things already done
- Things currently under discussion
- Important insights and facts learned

If an old summary is provided, merge it with the new insights to maintain continuity.
Respond ONLY with the formatted memory/summary.`;

		const userContent = `Old Summary/Memory: ${oldSummary || 'None'}\n\nCurrent Chat History (last few messages):\n${messages.map(m => `${m.role.toUpperCase()}: ${m.content}`).join('\n')}`;

		const response = await openai.chat.completions.create({
			model: settings.model,
			messages: [
				{ role: 'system', content: systemPrompt },
				{ role: 'user', content: userContent }
			]
		});
		return response.choices[0]?.message?.content || '';
	} catch (error: any) {
		console.error('Summarization error:', error);
		if (error.status === 401) {
			throw new Error('Invalid API Key or Provider configuration. Please check your settings.');
		}
		return oldSummary || '';
	}
}

export async function* streamChat(messages: any[], tools?: any[], signal?: AbortSignal) {
	try {
		const openai = createOpenAIClient();
		if (!settings.apiKey) {
			throw new Error('API Key is missing. Please add it in Settings.');
		}
		const stream = await openai.chat.completions.create({
			model: settings.model,
			messages: messages as any,
			tools: tools?.length ? tools : undefined,
			stream: true,
			stream_options: { include_usage: true },
			...(settings.enableThinking ? { include_reasoning: true } : {})
		} as any, { signal });

		for await (const chunk of (stream as any)) {
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
		if (error.status === 401 || error.message?.includes('401')) {
			throw new Error('Invalid API Key or Provider configuration. Please check your Settings (API Key and Base URL).');
		}
		throw error;
	}
}
