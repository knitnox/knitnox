import { type ResourceContent } from './db';
import { toast } from './toast.svelte';

let _pending = $state<ResourceContent[]>([]);

export const resourceContext = {
	get pending() {
		return _pending;
	},
	add(value: ResourceContent) {
		if (!_pending.find(r => r.uri === value.uri)) {
			_pending.push(value);
			toast.add(`Added ${value.name} to chat context`, 'success');
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