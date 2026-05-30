import Dexie, { type Table } from 'dexie';

export interface Chat {
	id?: number;
	title: string;
	createdAt: number;
	totalInputTokens?: number;
	totalOutputTokens?: number;
	lastInputTokens?: number;
	lastOutputTokens?: number;
	summary?: string;
	lastSummaryCount?: number;
}

export interface Attachment {
	type: 'image' | 'audio' | 'video' | 'file';
	mimeType: string;
	data: string; // Base64
	name?: string;
}

export interface ResourceContent {
	name: string;
	uri: string;
	content: string;
	sizeStr?: string;
	mimeType?: string;
}

export interface Message {
	id?: number;
	chatId: number;
	role: 'user' | 'assistant' | 'system' | 'tool';
	content: string;
	createdAt: number;
	attachments?: Attachment[];
	resources?: ResourceContent[];
	toolCalls?: any[];
	toolResult?: any;
	toolCallId?: string;
	thinkingContent?: string;
	thinkingDuration?: number;
}

export interface Knowledge {
	id?: number;
	content: string;
	category?: string;
	createdAt: number;
	updatedAt: number;
}

export interface Profile {
	id?: number;
	name: string;
	baseUrl: string;
	apiKey: string;
	model: string;
	systemPrompt: string;
	enableThinking: boolean;
	contextWindow: number;
	maxAgentTurns: number;
	supportsImages: boolean;
	supportsAudio: boolean;
	supportsVideo: boolean;
	enableCompression: boolean;
	mcpServers: string[];
	disabledTools: string[];
	temperature?: number;
	top_p?: number;
	frequency_penalty?: number;
	presence_penalty?: number;
	response_format?: string;
	reasoning_effort?: 'low' | 'medium' | 'high';
	seed?: number;
	createdAt: number;
	updatedAt: number;
}

export class MyDatabase extends Dexie {
	chats!: Table<Chat>;
	messages!: Table<Message>;
	knowledge!: Table<Knowledge>;
	profiles!: Table<Profile>;

	constructor() {
		super('ChatDB');
		this.version(6).stores({
			chats: '++id, title, createdAt',
			messages: '++id, chatId, role, createdAt',
			knowledge: '++id, content, createdAt, updatedAt',
			profiles: '++id, name, createdAt'
		}).upgrade(tx => {
			// Optional: handle data migration if needed
		});
	}
}

export const db = new MyDatabase();
