export const localTools = [
	{
		name: 'popup',
		description: 'Show a popup message to the user',
		parameters: {
			type: 'object',
			properties: {
				message: {
					type: 'string',
					description: 'The message to show in the popup'
				}
			},
			required: ['message']
		},
		handler: async (args: { message: string }) => {
			alert(args.message);
			return { success: true };
		}
	},
	{
		name: 'add_numbers',
		description: 'Add two numbers together',
		parameters: {
			type: 'object',
			properties: {
				a: {
					type: 'number',
					description: 'The first number'
				},
				b: {
					type: 'number',
					description: 'The second number'
				}
			},
			required: ['a', 'b']
		},
		handler: async (args: { a: number, b: number }) => {
			return { result: args.a + args.b };
		}
	},
	{
		name: 'subtract_numbers',
		description: 'Subtract one number from another',
		parameters: {
			type: 'object',
			properties: {
				a: {
					type: 'number',
					description: 'The number to subtract from'
				},
				b: {
					type: 'number',
					description: 'The number to subtract'
				}
			},
			required: ['a', 'b']
		},
		handler: async (args: { a: number, b: number }) => {
			return { result: args.a - args.b };
		}
	}
];
