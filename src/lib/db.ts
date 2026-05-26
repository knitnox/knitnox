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

export interface Message {
	id?: number;
	chatId: number;
	role: 'user' | 'assistant' | 'system' | 'tool';
	content: string;
	createdAt: number;
	attachments?: Attachment[];
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

export class MyDatabase extends Dexie {
	chats!: Table<Chat>;
	messages!: Table<Message>;
	knowledge!: Table<Knowledge>;

	constructor() {
		super('ChatDB');
		this.version(4).stores({
			chats: '++id, title, createdAt',
			messages: '++id, chatId, role, createdAt',
			knowledge: '++id, content, createdAt, updatedAt'
		}).upgrade(tx => {
			// Optional: handle data migration if needed
		});
	}
}

export const db = new MyDatabase();
