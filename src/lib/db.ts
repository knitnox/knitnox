import Dexie, { type Table } from 'dexie';

export interface Chat {
	id?: number;
	title: string;
	createdAt: number;
	totalInputTokens?: number;
	totalOutputTokens?: number;
	lastInputTokens?: number;
	lastOutputTokens?: number;
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

export class MyDatabase extends Dexie {
	chats!: Table<Chat>;
	messages!: Table<Message>;

	constructor() {
		super('ChatDB');
		this.version(2).stores({
			chats: '++id, title, createdAt',
			messages: '++id, chatId, role, createdAt'
		}).upgrade(tx => {
			// Optional: handle data migration if needed
		});
	}
}

export const db = new MyDatabase();
