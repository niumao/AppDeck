import { goto } from '$app/navigation';
import { base } from '$app/paths';

const API_BASE = '/api/v1';

async function request(method: string, path: string, opts: {
	body?: unknown;
	params?: Record<string, string | number | undefined>;
	responseType?: 'blob';
	timeout?: number;
} = {}): Promise<any> {
	const token = localStorage.getItem('admin_token');
	const headers: Record<string, string> = {};
	if (token) headers['Authorization'] = `Bearer ${token}`;

	let url = `${API_BASE}${path}`;
	if (opts.params) {
		const sp = new URLSearchParams();
		for (const [k, v] of Object.entries(opts.params)) {
			if (v !== undefined && v !== '') sp.set(k, String(v));
		}
		const qs = sp.toString();
		if (qs) url += `?${qs}`;
	}

	const init: RequestInit = { method, headers };
	if (opts.body instanceof FormData) {
		init.body = opts.body;
	} else if (opts.body !== undefined) {
		headers['Content-Type'] = 'application/json';
		init.body = JSON.stringify(opts.body);
	}

	const controller = new AbortController();
	const timeout = opts.timeout || 30000;
	const timer = setTimeout(() => controller.abort(), timeout);
	init.signal = controller.signal;

	let res: Response;
	try {
		res = await fetch(url, init);
	} finally {
		clearTimeout(timer);
	}

	if (res.status === 401) {
		localStorage.removeItem('admin_token');
		localStorage.removeItem('admin_user');
		goto(`${base}/login`);
		throw new Error('Unauthorized');
	}

	if (opts.responseType === 'blob') {
		if (!res.ok) throw new Error(`Request failed: ${res.status}`);
		return res.blob();
	}

	const json = await res.json();

	// Unwrap {code, data, message} envelope
	if (typeof json.code !== 'undefined') {
		if (json.code === 0) return json.data;
		throw new Error(json.message || 'Request failed');
	}

	if (!res.ok) {
		throw new Error(json.message || json.error || `Request failed: ${res.status}`);
	}

	return json;
}

const get = (path: string, params?: Record<string, any>) => request('GET', path, { params });
const post = (path: string, body?: unknown, opts?: { timeout?: number }) => request('POST', path, { body, ...opts });
const put = (path: string, body?: unknown) => request('PUT', path, { body });
const del = (path: string) => request('DELETE', path);
const getBlob = (path: string) => request('GET', path, { responseType: 'blob' });
const postForm = (path: string, form: FormData, opts?: { timeout?: number }) =>
	request('POST', path, { body: form, timeout: opts?.timeout });

// Auth API
export const authAPI = {
	login: (data: { username: string; password: string }) => post('/admin/login', data),
	logout: () => post('/admin/logout'),
	me: () => get('/admin/me')
};

// Apps API
export const appsAPI = {
	getAll: (params?: Record<string, any>) => get('/admin/apps', params),
	getById: (id: string) => get(`/admin/apps/${id}`),
	create: (data: any) => post('/admin/apps', data),
	update: (id: string, data: any) => put(`/admin/apps/${id}`, data),
	delete: (id: string) => del(`/admin/apps/${id}`),
	reorder: (ids: number[]) => put('/admin/apps/reorder', { ids }),
	getDeleted: () => get('/admin/apps/deleted'),
	recover: (id: string) => post(`/admin/apps/${id}/recover`),
	batchDelist: (ids: string[]) => post('/admin/apps/batch-delist', { ids })
};

// Categories API
export const categoriesAPI = {
	getAll: (params?: Record<string, any>) => get('/admin/categories', params),
	getById: (id: number) => get(`/admin/categories/${id}`),
	create: (data: any) => post('/admin/categories', data),
	update: (id: number, data: any) => put(`/admin/categories/${id}`, data),
	delete: (id: number) => del(`/admin/categories/${id}`),
	reorder: (ids: number[]) => put('/admin/categories/reorder', { ids })
};

// Users API
export const usersAPI = {
	getAll: (params?: Record<string, any>) => get('/admin/users', params),
	getById: (id: number) => get(`/admin/users/${id}`),
	adjustBalance: (id: number, data: any) => post(`/admin/users/${id}/balance`, data),
	updateStatus: (id: number, data: any) => put(`/admin/users/${id}/status`, data),
	getBalanceLogs: (id: number, params?: Record<string, any>) => get(`/admin/users/${id}/balance-logs`, params),
	adjustTenantBalance: (id: number, data: any) => post(`/admin/users/${id}/tenant-balance`, data)
};

// Billing Plans API
export const billingPlansAPI = {
	getAll: () => get('/admin/plans'),
	getById: (id: number) => get(`/admin/plans/${id}`),
	create: (data: any) => post('/admin/plans', data),
	update: (id: number, data: any) => put(`/admin/plans/${id}`, data),
	delete: (id: number) => del(`/admin/plans/${id}`)
};

// Devices API
export const devicesAPI = {
	getAll: (params?: Record<string, any>) => get('/admin/devices', params),
	disconnect: (id: number) => post(`/admin/devices/${id}/disconnect`),
	reboot: (id: number) => post(`/admin/devices/${id}/reboot`),
	poweroff: (id: number) => post(`/admin/devices/${id}/poweroff`),
	shell: (id: string, command: string) =>
		post(`/admin/devices/${id}/shell`, { command }, { timeout: 35000 }),
	shellHistory: (id: string, params?: Record<string, any>) =>
		get(`/admin/devices/${id}/shell-history`, params),
	screenshot: (id: string, display: string = 'phone') =>
		request('POST', `/admin/devices/${id}/screenshot`, {
			params: { display },
			timeout: 20000,
		}),
};

// Steam API
export const steamAPI = {
	search: (query: string) => get('/admin/steam/search', { q: query }),
	validate: (steamId: string) => post('/admin/steam/validate', { steam_id: steamId })
};

// User Clients API
export const userClientsAPI = {
	getAll: (params?: Record<string, any>) => get('/admin/user-clients', params),
	create: (data: any) => post('/admin/user-clients', data),
	update: (id: number, data: any) => put(`/admin/user-clients/${id}`, data),
	delete: (id: number) => del(`/admin/user-clients/${id}`)
};

// Screensaver API
export const screensaverAPI = {
	uploadVideo: (file: File) => {
		const form = new FormData();
		form.append('video', file);
		return postForm('/admin/screensaver/video', form, { timeout: 600000 });
	},
	getSettings: () => get('/admin/screensaver/settings'),
	updateSettings: (data: any) => put('/admin/screensaver/settings', data),
	getVideoUrl: () => `${API_BASE}/admin/screensaver/video`,
	// Video library
	listVideos: () => get('/admin/screensaver/videos'),
	uploadVideoToLibrary: (file: File) => {
		const form = new FormData();
		form.append('video', file);
		return postForm('/admin/screensaver/videos', form, { timeout: 600000 });
	},
	deleteVideo: (id: string) => del(`/admin/screensaver/videos/${id}`),
	getVideoFileUrl: (id: string) => `${API_BASE}/admin/screensaver/videos/${id}/file`,
	// Global default
	getDefaultVideo: () => get('/admin/screensaver/default-video'),
	setDefaultVideo: (videoId: string) => put('/admin/screensaver/default-video', { video_id: videoId }),
	// Tenant configs
	listTenantConfigs: () => get('/admin/screensaver/tenant-configs'),
	setTenantConfig: (tenantId: string, data: any) => put(`/admin/screensaver/tenant-configs/${tenantId}`, data),
	deleteTenantConfig: (tenantId: string) => del(`/admin/screensaver/tenant-configs/${tenantId}`)
};

// Config API
export const configAPI = {
	getMoonlightHost: () => get('/admin/config/moonlight_host'),
	updateMoonlightHost: (value: string) => put('/admin/config/moonlight_host', { value }),
	getMoonlightPort: () => get('/admin/config/moonlight_port'),
	updateMoonlightPort: (value: string) => put('/admin/config/moonlight_port', { value })
};

// Moonlight Hosts API
export const moonlightHostsAPI = {
	list: () => get('/admin/moonlight/hosts'),
	create: (data: any) => post('/admin/moonlight/hosts', data),
	update: (id: number, data: any) => put(`/admin/moonlight/hosts/${id}`, data),
	delete: (id: number) => del(`/admin/moonlight/hosts/${id}`),
	forceRelease: (id: number) => post(`/admin/moonlight/hosts/${id}/release`),
	triggerPair: (id: number) => post(`/admin/moonlight/hosts/${id}/pair`)
};

// Revenue API
export const revenueAPI = {
	getSummary: (params?: Record<string, any>) => get('/admin/revenue/summary', params)
};

// Registered Devices API
export const registeredDevicesAPI = {
	list: (params?: Record<string, any>) => get('/admin/registered-devices', params),
	create: (data: any) => post('/admin/registered-devices', data),
	approve: (id: number) => post(`/admin/registered-devices/${id}/approve`),
	reject: (id: number) => post(`/admin/registered-devices/${id}/reject`),
	update: (id: number, data: any) => put(`/admin/registered-devices/${id}`, data),
	delete: (id: number) => del(`/admin/registered-devices/${id}`),
	assignTenant: (id: number, data: any) => put(`/admin/registered-devices/${id}/assign-tenant`, data),
	updateSchedule: (id: number, data: any) => put(`/admin/registered-devices/${id}/schedule`, data),
	updateBypassCharge: (id: number, data: any) => put(`/admin/registered-devices/${id}/bypass-charge`, data)
};

// Tenants API
export const tenantsAPI = {
	getAll: (params?: Record<string, any>) => get('/admin/tenants', params),
	getAllActive: () => get('/admin/tenants/active'),
	create: (data: any) => post('/admin/tenants', data),
	update: (id: string, data: any) => put(`/admin/tenants/${id}`, data),
	updatePassword: (id: string, data: any) => put(`/admin/tenants/${id}/password`, data),
	updateStatus: (id: string, data: any) => put(`/admin/tenants/${id}/status`, data),
	updateBilling: (id: string, data: any) => put(`/admin/tenants/${id}/billing`, data),
	delete: (id: string) => del(`/admin/tenants/${id}`)
};

// Plan Sets API
export const planSetsAPI = {
	getAll: (params?: Record<string, any>) => get('/admin/plan-sets', params),
	getActive: () => get('/admin/plan-sets/active'),
	getById: (id: number) => get(`/admin/plan-sets/${id}`),
	create: (data: any) => post('/admin/plan-sets', data),
	update: (id: number, data: any) => put(`/admin/plan-sets/${id}`, data),
	updateStatus: (id: number, data: any) => put(`/admin/plan-sets/${id}/status`, data),
	addPlan: (id: number, data: any) => post(`/admin/plan-sets/${id}/plans`, data),
	updatePlan: (id: number, data: any) => put(`/admin/plan-sets/plans/${id}`, data),
	updatePlanStatus: (id: number, data: any) => put(`/admin/plan-sets/plans/${id}/status`, data),
	deletePlan: (id: number) => del(`/admin/plan-sets/plans/${id}`),
	getDeleted: () => get('/admin/plan-sets/deleted'),
	delete: (id: number) => del(`/admin/plan-sets/${id}`),
	recover: (id: number) => post(`/admin/plan-sets/${id}/recover`)
};

// Global Packages API
export const globalPackagesAPI = {
	getAll: (params?: Record<string, any>) => get('/admin/global-packages', params),
	create: (data: any) => post('/admin/global-packages', data),
	update: (id: number, data: any) => put(`/admin/global-packages/${id}`, data),
	updateStatus: (id: number, data: any) => put(`/admin/global-packages/${id}/status`, data),
	getDeleted: () => get('/admin/global-packages/deleted'),
	delete: (id: number) => del(`/admin/global-packages/${id}`),
	recover: (id: number) => post(`/admin/global-packages/${id}/recover`),
	permanentDelete: (id: number) => del(`/admin/global-packages/${id}/permanent`),
	getOrders: (params?: Record<string, any>) => get('/admin/recharge-orders', params),
	getOrderById: (id: number) => get(`/admin/recharge-orders/${id}`)
};

// Refunds API
export const refundsAPI = {
	preview: (orderId: number) => get(`/admin/orders/${orderId}/refund-preview`),
	process: (orderId: number, data: any) => post(`/admin/orders/${orderId}/refund`, data)
};

// Data Migration API
export const dataMigrationAPI = {
	exportData: (tables: string[]) => request('POST', '/admin/data/export', {
		body: { tables },
		responseType: 'blob'
	}),
	importData: (file: File) => {
		const form = new FormData();
		form.append('file', file);
		return postForm('/admin/data/import', form, { timeout: 120000 });
	}
};

// Backup API
export const backupAPI = {
	list: () => get('/admin/backups'),
	create: () => post('/admin/backups'),
	upload: (file: File) => {
		const form = new FormData();
		form.append('file', file);
		return postForm('/admin/backups/upload', form);
	},
	restore: (filename: string) => post('/admin/backups/restore', { filename }),
	delete: (filename: string) => del(`/admin/backups/${encodeURIComponent(filename)}`),
	download: (filename: string) => getBlob(`/admin/backups/${encodeURIComponent(filename)}/download`),
	getSchedule: () => get('/admin/backups/schedule'),
	updateSchedule: (data: any) => put('/admin/backups/schedule', data)
};

// Save Storage API (Eden/Switch emulator game-save sync)
export const saveStorageAPI = {
	getConfig: () => get('/admin/save-storage'),
	updateConfig: (data: any) => put('/admin/save-storage', data),
	list: () => get('/admin/save-storage/saves'),
	// Backups are whole-store snapshots (all users, all games in one file),
	// same shape as backupAPI -- not scoped to a specific user or game.
	listBackups: () => get('/admin/save-storage/backups'),
	createBackup: () => post('/admin/save-storage/backups'),
	uploadBackup: (file: File) => {
		const form = new FormData();
		form.append('file', file);
		return postForm('/admin/save-storage/backups/upload', form);
	},
	restoreBackup: (filename: string) => post('/admin/save-storage/backups/restore', { filename }),
	deleteBackup: (filename: string) => del(`/admin/save-storage/backups/${encodeURIComponent(filename)}`),
	downloadBackup: (filename: string) =>
		getBlob(`/admin/save-storage/backups/${encodeURIComponent(filename)}/download`)
};

// Version API
export const versionAPI = {
	get: () => get('/version')
};

// Notebook API
export const notebookAPI = {
	get: () => get('/admin/notebook'),
	save: (content: string) => put('/admin/notebook', { content })
};
