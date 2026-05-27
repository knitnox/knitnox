import { db } from './db';

export const localTools = [
	{
		name: 'knowledge',
		description: 'Manage a persistent knowledge base of user facts and preferences that lasts across different chat sessions. USE GUIDELINES: 1. Use "search" ONLY when the user asks about something from the past or if you need specific historical context not in this chat. 2. Use "add" ONLY for important, long-term facts (preferences, names, life events). 3. NEVER store trivial chat history or conversational fluff. 4. Always search before adding to avoid duplicates.',
		parameters: {
			type: 'object',
			properties: {
				action: {
					type: 'string',
					enum: ['add', 'search', 'list', 'delete'],
					description: 'The action: "add" (significant facts only), "search" (targeted queries), "list" (overview), or "delete" (outdated info).'
				},
				content: {
					type: 'string',
					description: 'The fact to store (required for "add"). Be concise, factual, and omit conversational context.'
				},
				query: {
					type: 'string',
					description: 'The search term (required for "search"). Use specific keywords instead of full sentences.'
				},
				id: {
					type: 'number',
					description: 'The ID of the knowledge entry to delete (required for "delete").'
				},
				limit: {
					type: 'number',
					description: 'Max results (default: 5). Keep it small unless a broad overview is needed.'
				},
				offset: {
					type: 'number',
					description: 'Number of results to skip (default: 0).'
				}
			},
			required: ['action']
		},
		handler: async (args: { action: 'add' | 'search' | 'list' | 'delete', content?: string, query?: string, id?: number, limit?: number, offset?: number }) => {
			try {
				const limit = args.limit || 5;
				const offset = args.offset || 0;

				switch (args.action) {
					case 'add':
						if (!args.content) {
							return { error: 'Content is required for "add" action.' };
						}
						const newId = await db.knowledge.add({
							content: args.content,
							createdAt: Date.now(),
							updatedAt: Date.now()
						});
						return { success: true, message: `Knowledge added successfully with ID: ${newId}`, id: newId };

					case 'search':
						if (!args.query) {
							return { error: 'Query is required for "search" action.' };
						}
						
						const queryLower = args.query.toLowerCase().trim();
						const queryWords = queryLower.split(/\s+/).filter(word => word.length > 0);
						
						if (queryWords.length === 0) {
							return { 
								success: true, 
								results: [],
								pagination: { total: 0, limit, offset, hasMore: false }
							};
						}

						const filterFn = (item: any) => {
							const contentLower = (item.content || '').toLowerCase();
							return queryWords.every(word => contentLower.includes(word));
						};

						const filteredResults = await db.knowledge
							.orderBy('updatedAt')
							.reverse()
							.filter(filterFn)
							.offset(offset)
							.limit(limit)
							.toArray();
						
						const totalSearchCount = await db.knowledge
							.filter(filterFn)
							.count();

						return { 
							success: true, 
							results: filteredResults.map(r => ({ id: r.id, content: r.content, updatedAt: r.updatedAt })),
							pagination: { total: totalSearchCount, limit, offset, hasMore: totalSearchCount > offset + limit }
						};

					case 'list':
						const totalCount = await db.knowledge.count();
						const items = await db.knowledge
							.orderBy('updatedAt')
							.reverse()
							.offset(offset)
							.limit(limit)
							.toArray();

						return { 
							success: true, 
							count: items.length, 
							items: items.map(k => ({ id: k.id, content: k.content, updatedAt: k.updatedAt })),
							pagination: { total: totalCount, limit, offset, hasMore: totalCount > offset + limit }
						};

					case 'delete':
						if (args.id === undefined) {
							return { error: 'ID is required for "delete" action.' };
						}
						const exists = await db.knowledge.get(args.id);
						if (!exists) {
							return { error: `No knowledge entry found with ID: ${args.id}` };
						}
						await db.knowledge.delete(args.id);
						return { success: true, message: `Knowledge entry with ID ${args.id} deleted successfully.` };

					default:
						return { error: `Invalid action: ${args.action}` };
				}
			} catch (error: any) {
				console.error('Knowledge tool error:', error);
				return { error: `An error occurred while performing the ${args.action} action: ${error.message || 'Unknown error'}` };
			}
		}
	}
];
