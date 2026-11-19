export type ToastType = 'success' | 'error' | 'warning' | 'info';

interface Toast {
	id: number;
	message: string;
	type: ToastType;
}

let toasts = $state<Toast[]>([]);
let nextId = 0;

export function getToasts(): Toast[] {
	return toasts;
}

export function toast(message: string, type: ToastType = 'success') {
	const id = nextId++;
	toasts = [...toasts, { id, message, type }];
	setTimeout(() => {
		toasts = toasts.filter((t) => t.id !== id);
	}, 3000);
}

export function dismissToast(id: number) {
	toasts = toasts.filter((t) => t.id !== id);
}
