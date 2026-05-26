export interface Toast {
	id: number;
	message: string;
	type: 'info' | 'success' | 'error';
	duration?: number;
}

class ToastStore {
	toasts = $state<Toast[]>([]);
	private nextId = 0;

	add(message: string, type: Toast['type'] = 'info', duration = 3000) {
		const id = this.nextId++;
		const toast: Toast = { id, message, type, duration };
		this.toasts.push(toast);

		if (duration > 0) {
			setTimeout(() => {
				this.remove(id);
			}, duration);
		}
		return id;
	}

	remove(id: number) {
		this.toasts = this.toasts.filter((t) => t.id !== id);
	}
}

export const toast = new ToastStore();
