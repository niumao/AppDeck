<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { t } from '$lib/i18n';
	import { devicesAPI } from '$lib/api';
	import { toast } from 'svelte-sonner';
	import {
		RefreshCw,
		Search,
		Unplug,
		ChevronDown,
		ChevronUp,
		Power,
		BatteryCharging,
		BatteryFull,
		BatteryWarning,
		Battery,
		Thermometer,
		Terminal,
		Camera,
	} from '@lucide/svelte';
	import * as Table from '$lib/components/ui/table';
	import * as Card from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Badge } from '$lib/components/ui/badge';
	import DeviceShell from '$lib/components/device-shell.svelte';
	import DeviceScreenshot from '$lib/components/device-screenshot.svelte';
	import ConfirmActionDialog from '$lib/components/confirm-action-dialog.svelte';
	import TablePagination from '$lib/components/table-pagination.svelte';
	import { runWithToast } from '$lib/action-feedback';

	// --- Device list state ---
	let devices: any[] = $state([]);
	let summary = $state({ total: 0, online: 0, in_session: 0, offline: 0 });
	let loading = $state(true);
	let refreshing = $state(false);

	// --- Filters ---
	let searchText = $state('');
	let stateFilter = $state('online');

	// --- Pagination ---
	let page = $state(0);
	let rowsPerPage = $state(20);
	let total = $state(0);

	// --- Expanded rows ---
	let expandedRows: Set<number> = $state(new Set());

	// --- Disconnect dialog ---
	let disconnectDialogOpen = $state(false);
	let disconnectDevice: any = $state(null);

	// --- Reboot dialog ---
	let rebootDialogOpen = $state(false);
	let rebootDevice: any = $state(null);

	// --- Poweroff dialog ---
	let poweroffDialogOpen = $state(false);
	let poweroffDevice: any = $state(null);

	// --- Shell dialog ---
	let shellOpen = $state(false);
	let shellDevice: any = $state(null);

	// --- Screenshot dialog ---
	let screenshotOpen = $state(false);
	let screenshotDevice: any = $state(null);

	// --- Live session duration tracking ---
	let liveSessionDurations: Record<number, number> = $state({});

	// --- Intervals ---
	let refreshInterval: ReturnType<typeof setInterval> | undefined;
	let durationInterval: ReturnType<typeof setInterval> | undefined;
	let searchDebounce: ReturnType<typeof setTimeout> | undefined;

	// --- Load devices ---
	async function loadDevices(isRefresh = false) {
		try {
			if (isRefresh) {
				refreshing = true;
			} else {
				loading = true;
			}

			const params: Record<string, any> = {
				page: page + 1,
				limit: rowsPerPage,
			};

			if (searchText.trim()) {
				params.search = searchText.trim();
			}

			if (stateFilter) {
				params.state = stateFilter;
			}

			const data = await devicesAPI.getAll(params);

			const items = Array.isArray(data.items) ? data.items : [];
			const summaryData =
				data.summary && typeof data.summary === 'object'
					? {
							total: Number(data.summary.total) || 0,
							online: Number(data.summary.online) || 0,
							in_session: Number(data.summary.in_session) || 0,
							offline: Number(data.summary.offline) || 0,
						}
					: { total: 0, online: 0, in_session: 0, offline: 0 };

			devices = items;
			summary = summaryData;
			total = data.total || 0;

			// Reset live durations from fresh API data
			const updated: Record<number, number> = {};
			for (const device of items) {
				if (device.state === 'logged_in' && device.session_duration_seconds !== undefined) {
					updated[device.id] = device.session_duration_seconds;
				}
			}
			liveSessionDurations = updated;
		} catch (error: any) {
			console.error('Failed to load devices:', error);
			toast.error(error.message || t('sessions.loadFailed'));
			devices = [];
			summary = { total: 0, online: 0, in_session: 0, offline: 0 };
		} finally {
			loading = false;
			refreshing = false;
		}
	}

	// --- Disconnect ---
	function openDisconnectDialog(device: any) {
		disconnectDevice = device;
		disconnectDialogOpen = true;
	}

	async function handleDisconnect() {
		if (!disconnectDevice) return;
		const ok = await runWithToast(() => devicesAPI.disconnect(disconnectDevice.id), {
			successMessage: t('devices.disconnected'),
			errorFallback: t('devices.disconnectFailed'),
			logLabel: 'Failed to disconnect device:',
		});
		if (ok) {
			disconnectDialogOpen = false;
			disconnectDevice = null;
			loadDevices();
		}
	}

	// --- Reboot ---
	function openRebootDialog(device: any) {
		rebootDevice = device;
		rebootDialogOpen = true;
	}

	async function handleReboot() {
		if (!rebootDevice) return;
		const ok = await runWithToast(() => devicesAPI.reboot(rebootDevice.id), {
			successMessage: t('devices.rebooted'),
			errorFallback: t('devices.rebootFailed'),
			logLabel: 'Failed to reboot device:',
		});
		if (ok) {
			rebootDialogOpen = false;
			rebootDevice = null;
			loadDevices();
		}
	}

	// --- Poweroff ---
	function openPoweroffDialog(device: any) {
		poweroffDevice = device;
		poweroffDialogOpen = true;
	}

	async function handlePoweroff() {
		if (!poweroffDevice) return;
		const ok = await runWithToast(() => devicesAPI.poweroff(poweroffDevice.id), {
			successMessage: t('devices.poweredOff'),
			errorFallback: t('devices.poweroffFailed'),
			logLabel: 'Failed to power off device:',
		});
		if (ok) {
			poweroffDialogOpen = false;
			poweroffDevice = null;
			loadDevices();
		}
	}

	// --- Shell ---
	function openShellDialog(device: any) {
		shellDevice = device;
		shellOpen = true;
	}

	// --- Screenshot ---
	function openScreenshotDialog(device: any) {
		screenshotDevice = device;
		screenshotOpen = true;
	}

	// --- Row expand/collapse ---
	function toggleRowExpand(deviceId: number) {
		const next = new Set(expandedRows);
		if (next.has(deviceId)) {
			next.delete(deviceId);
		} else {
			next.add(deviceId);
		}
		expandedRows = next;
	}

	// --- State badge ---
	function getStateBadge(state: string): {
		label: string;
		variant: 'default' | 'secondary' | 'destructive' | 'outline';
	} {
		const config: Record<
			string,
			{ label: string; variant: 'default' | 'secondary' | 'destructive' | 'outline' }
		> = {
			idle: { label: t('devices.idle'), variant: 'default' },
			logged_in: { label: t('devices.loggedIn'), variant: 'secondary' },
			offline: { label: t('devices.offline'), variant: 'outline' },
		};
		return config[state] || { label: state || '-', variant: 'outline' };
	}

	// --- Formatting helpers ---
	function formatRelativeTime(dateStr: string | null | undefined): string {
		if (!dateStr) return '-';
		const date = new Date(dateStr);
		const now = new Date();
		const diffMs = now.getTime() - date.getTime();
		const diffSec = Math.floor(diffMs / 1000);
		const diffMin = Math.floor(diffSec / 60);
		const diffHour = Math.floor(diffMin / 60);
		const diffDay = Math.floor(diffHour / 24);

		if (diffDay > 0) return t('devices.daysAgo').replace('{n}', String(diffDay));
		if (diffHour > 0)
			return t('devices.hoursAgo')
				.replace('{n}', String(diffHour))
				.replace('{m}', String(diffMin % 60));
		if (diffMin > 0) return t('devices.minutesAgo').replace('{n}', String(diffMin));
		return t('devices.secondsAgo').replace('{n}', String(diffSec));
	}

	function formatDuration(seconds: number | null | undefined): string {
		if (!seconds && seconds !== 0) return '-';
		const hours = Math.floor(seconds / 3600);
		const mins = Math.floor((seconds % 3600) / 60);
		const secs = seconds % 60;
		return `${String(hours).padStart(2, '0')}:${String(mins).padStart(2, '0')}:${String(secs).padStart(2, '0')}`;
	}

	function formatCurrency(cents: number): string {
		return `\u00A5${(cents / 100).toFixed(2)}`;
	}

	function getActivityDisplay(device: any): string {
		if (device.state === 'idle') return 'Idle';
		if (device.state === 'offline') return 'Offline';
		if (device.state === 'logged_in') {
			const durationSeconds =
				liveSessionDurations[device.id] !== undefined
					? liveSessionDurations[device.id]
					: device.session_duration_seconds;
			const duration = durationSeconds ? formatDuration(durationSeconds) : '00:00:00';
			return `Billing (${duration})`;
		}
		return '-';
	}

	function maskMachineCode(code: string | null | undefined): string {
		if (!code || code.length <= 6) return code || '-';
		return '...' + code.slice(-6);
	}

	function getBatteryIcon(device: any) {
		const level = device.battery_level;
		const state = device.charging_state;
		if (level == null || level < 0) return { icon: Battery, color: 'text-muted-foreground', text: '-' };

		let icon = Battery, color = 'text-green-600';
		if (state === 'charging') { icon = BatteryCharging; color = 'text-blue-500'; }
		else if (state === 'full') { icon = BatteryFull; color = 'text-green-600'; }
		else if (level <= 20) { icon = BatteryWarning; color = 'text-red-500'; }
		else if (level <= 50) { color = 'text-yellow-500'; }

		const label = state === 'charging' ? t('devices.charging')
			: state === 'full' ? t('devices.batteryFull') : '';
		return { icon, color, text: label ? `${level}% ${label}` : `${level}%` };
	}

	function getTemperatureDisplay(device: any): { text: string; color: string } {
		const temp = device.device_temp;
		if (temp == null || temp < 0) return { text: '-', color: 'text-muted-foreground' };
		const rounded = Math.round(temp);
		let color = 'text-green-600';
		if (rounded >= 55) color = 'text-red-500';
		else if (rounded >= 45) color = 'text-yellow-500';
		return { text: `${rounded}°C`, color };
	}

	// --- Search with debounce ---
	function handleSearchInput() {
		if (searchDebounce) clearTimeout(searchDebounce);
		searchDebounce = setTimeout(() => {
			page = 0;
			loadDevices();
		}, 300);
	}

	function handleStateFilterChange(e: Event) {
		stateFilter = (e.currentTarget as HTMLSelectElement).value;
		page = 0;
		loadDevices();
	}

	// --- Lifecycle ---
	onMount(() => {
		loadDevices();

		// Auto-refresh every 15 seconds
		refreshInterval = setInterval(() => {
			loadDevices(true);
		}, 15000);

		// Live session duration counter - updates every second
		durationInterval = setInterval(() => {
			const updated: Record<number, number> = {};
			let changed = false;
			for (const device of devices) {
				if (device.state === 'logged_in' && device.session_duration_seconds !== undefined) {
					const prev =
						liveSessionDurations[device.id] !== undefined
							? liveSessionDurations[device.id]
							: device.session_duration_seconds;
					updated[device.id] = prev + 1;
					changed = true;
				}
			}
			if (changed) {
				liveSessionDurations = updated;
			}
		}, 1000);
	});

	onDestroy(() => {
		if (refreshInterval) clearInterval(refreshInterval);
		if (durationInterval) clearInterval(durationInterval);
		if (searchDebounce) clearTimeout(searchDebounce);
	});
</script>

<!-- Header -->
<div class="flex items-center justify-between mb-6">
	<h2 class="text-2xl font-semibold">{t('devices.title')}</h2>
	<div class="flex items-center gap-2">
		<span class="text-sm text-muted-foreground">{t('devices.autoRefresh')}</span>
		<Button
			variant="outline"
			size="sm"
			onclick={() => loadDevices(true)}
			disabled={refreshing}
		>
			<RefreshCw class="mr-2 h-4 w-4 {refreshing ? 'animate-spin' : ''}" />
			{t('common.refresh')}
		</Button>
	</div>
</div>

<!-- Summary Cards -->
<div class="grid grid-cols-4 gap-2 sm:gap-4 mb-6">
	<Card.Root>
		<Card.Content class="p-3 sm:pt-6 sm:px-6 sm:pb-6">
			<p class="text-xs sm:text-sm text-muted-foreground truncate">{t('devices.totalDevices')}</p>
			<p class="text-xl sm:text-3xl font-bold">{summary.total}</p>
		</Card.Content>
	</Card.Root>

	<Card.Root class="border-green-200 bg-green-50 dark:border-green-900 dark:bg-green-950">
		<Card.Content class="p-3 sm:pt-6 sm:px-6 sm:pb-6">
			<p class="text-xs sm:text-sm text-muted-foreground truncate">{t('devices.onlineDevices')}</p>
			<p class="text-xl sm:text-3xl font-bold">{summary.online}</p>
		</Card.Content>
	</Card.Root>

	<Card.Root class="border-blue-200 bg-blue-50 dark:border-blue-900 dark:bg-blue-950">
		<Card.Content class="p-3 sm:pt-6 sm:px-6 sm:pb-6">
			<p class="text-xs sm:text-sm text-muted-foreground truncate">{t('devices.inSession')}</p>
			<p class="text-xl sm:text-3xl font-bold">{summary.in_session}</p>
		</Card.Content>
	</Card.Root>

	<Card.Root>
		<Card.Content class="p-3 sm:pt-6 sm:px-6 sm:pb-6">
			<p class="text-xs sm:text-sm text-muted-foreground truncate">{t('devices.offlineDevices')}</p>
			<p class="text-xl sm:text-3xl font-bold">{summary.offline}</p>
		</Card.Content>
	</Card.Root>
</div>

<!-- Toolbar: Search + State Filter -->
<div class="flex flex-col sm:flex-row items-start sm:items-center gap-3 mb-4">
	<div class="relative w-full sm:w-72">
		<Input
			placeholder={t('devices.searchPlaceholder')}
			bind:value={searchText}
			oninput={handleSearchInput}
			class="pr-9"
		/>
		<span class="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground">
			<Search class="h-4 w-4" />
		</span>
	</div>
	<select
		class="rounded-lg border bg-transparent px-2.5 py-1.5 text-sm w-full sm:w-48"
		value={stateFilter}
		onchange={handleStateFilterChange}
	>
		<option value="">{t('common.all')}</option>
		<option value="online">{t('devices.online')}</option>
		<option value="idle">{t('devices.idle')}</option>
		<option value="logged_in">{t('devices.loggedIn')}</option>
		<option value="offline">{t('devices.offline')}</option>
	</select>
</div>

<!-- Devices Table -->
<div class="rounded-lg border">
	<Table.Root>
		<Table.Header>
			<Table.Row>
				<Table.Head class="w-10"></Table.Head>
				<Table.Head>{t('devices.machineName')}</Table.Head>
				<Table.Head>{t('common.status')}</Table.Head>
				<Table.Head>{t('devices.battery')}</Table.Head>
				<Table.Head>{t('devices.temperature')}</Table.Head>
				<Table.Head>{t('common.user')}</Table.Head>
				<Table.Head>{t('devices.ipAddress')}</Table.Head>
				<Table.Head>{t('devices.connectedDuration')}</Table.Head>
				<Table.Head>{t('devices.lastSeen')}</Table.Head>
				<Table.Head>{t('devices.activity')}</Table.Head>
				<Table.Head>{t('devices.currentApp')}</Table.Head>
				<Table.Head class="text-right">{t('common.actions')}</Table.Head>
			</Table.Row>
		</Table.Header>
		<Table.Body>
			{#if loading}
				<Table.Row>
					<Table.Cell colspan={12} class="text-center py-8 text-muted-foreground">
						{t('common.loading')}
					</Table.Cell>
				</Table.Row>
			{:else if devices.length === 0}
				<Table.Row>
					<Table.Cell colspan={12} class="text-center py-8 text-muted-foreground">
						{t('devices.noDevices')}
					</Table.Cell>
				</Table.Row>
			{:else}
				{#each devices as device (device.id)}
					{@const badge = getStateBadge(device.state)}
					{@const bat = getBatteryIcon(device)}
					{@const tempInfo = getTemperatureDisplay(device)}
					<!-- Main row -->
					<Table.Row class="cursor-pointer" onclick={() => toggleRowExpand(device.id)}>
						<Table.Cell>
							<button
								class="p-1 rounded hover:bg-accent"
								onclick={(e) => {
									e.stopPropagation();
									toggleRowExpand(device.id);
								}}
							>
								{#if expandedRows.has(device.id)}
									<ChevronUp class="h-4 w-4" />
								{:else}
									<ChevronDown class="h-4 w-4" />
								{/if}
							</button>
						</Table.Cell>
						<Table.Cell class="font-medium">{device.name || '-'}</Table.Cell>
						<Table.Cell>
							<Badge variant={badge.variant}>{badge.label}</Badge>
						</Table.Cell>
						<Table.Cell class="text-sm">
							<span class="inline-flex items-center gap-1 {bat.color}">
								<svelte:component this={bat.icon} class="h-4 w-4" />
								{bat.text}
							</span>
						</Table.Cell>
						<Table.Cell class="text-sm">
							<span class="inline-flex items-center gap-1 {tempInfo.color}">
								<Thermometer class="h-4 w-4" />
								{tempInfo.text}
							</span>
						</Table.Cell>
						<Table.Cell>{device.user_nickname || '-'}</Table.Cell>
						<Table.Cell class="text-sm">{device.ip || '-'}</Table.Cell>
						<Table.Cell class="text-sm" title={device.connected_since ? new Date(device.connected_since).toISOString() : ''}>
							{formatRelativeTime(device.connected_since)}
						</Table.Cell>
						<Table.Cell class="text-sm" title={device.last_seen ? new Date(device.last_seen).toISOString() : ''}>
							{device.last_seen ? formatRelativeTime(device.last_seen) : '-'}
						</Table.Cell>
						<Table.Cell class="text-sm">{getActivityDisplay(device)}</Table.Cell>
						<Table.Cell class="text-sm">{device.current_app_name || '—'}</Table.Cell>
						<Table.Cell class="text-right">
							{#if device.state !== 'offline'}
								<Button
									variant="ghost"
									size="icon-sm"
									class="text-emerald-600 hover:text-emerald-600"
									onclick={(e) => {
										e.stopPropagation();
										openShellDialog(device);
									}}
									title={t('devices.shell')}
								>
									<Terminal class="h-4 w-4" />
								</Button>
								<Button
									variant="ghost"
									size="icon-sm"
									class="text-blue-600 hover:text-blue-600"
									onclick={(e) => {
										e.stopPropagation();
										openScreenshotDialog(device);
									}}
									title={t('devices.screenshot')}
								>
									<Camera class="h-4 w-4" />
								</Button>
								<Button
									variant="ghost"
									size="icon-sm"
									class="text-destructive hover:text-destructive"
									onclick={(e) => {
										e.stopPropagation();
										openDisconnectDialog(device);
									}}
									title={t('devices.disconnect')}
								>
									<Unplug class="h-4 w-4" />
								</Button>
								<Button
									variant="ghost"
									size="icon-sm"
									class="text-orange-600 hover:text-orange-600"
									onclick={(e) => {
										e.stopPropagation();
										openRebootDialog(device);
									}}
									title={t('devices.reboot')}
								>
									<RefreshCw class="h-4 w-4" />
								</Button>
								<Button
									variant="ghost"
									size="icon-sm"
									class="text-destructive hover:text-destructive"
									onclick={(e) => {
										e.stopPropagation();
										openPoweroffDialog(device);
									}}
									title={t('devices.poweroff')}
								>
									<Power class="h-4 w-4" />
								</Button>
							{/if}
						</Table.Cell>
					</Table.Row>

					<!-- Expanded detail row -->
					{#if expandedRows.has(device.id)}
						<Table.Row>
							<Table.Cell colspan={12} class="bg-muted/50 p-0">
								<div class="p-4">
									<h4 class="text-sm font-semibold mb-3">{t('devices.deviceDetails')}</h4>
									<div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-4">
										<div>
											<p class="text-xs text-muted-foreground">{t('devices.machineName')}</p>
											<p class="text-sm">{device.name || '-'}</p>
										</div>
										<div>
											<p class="text-xs text-muted-foreground">{t('devices.machineCode')}</p>
											<p class="text-sm">{maskMachineCode(device.machine_code)}</p>
										</div>
										<div>
											<p class="text-xs text-muted-foreground">{t('devices.hostId')}</p>
											<p class="text-sm">{device.host_id || '-'}</p>
										</div>
										<div>
											<p class="text-xs text-muted-foreground">{t('devices.ipAddress')}</p>
											<p class="text-sm">{device.ip || '-'}</p>
										</div>
										<div>
											<p class="text-xs text-muted-foreground">{t('devices.connectedSince')}</p>
											<p class="text-sm">
												{device.connected_since
													? new Date(device.connected_since).toLocaleString('zh-CN')
													: '-'}
											</p>
										</div>
										<div>
											<p class="text-xs text-muted-foreground">{t('common.status')}</p>
											<p class="text-sm">
												<Badge variant={badge.variant}>{badge.label}</Badge>
											</p>
										</div>
										<div>
											<p class="text-xs text-muted-foreground">{t('devices.battery')}</p>
											<p class="text-sm">
												<span class="inline-flex items-center gap-1 {bat.color}">
													<svelte:component this={bat.icon} class="h-4 w-4" />
													{bat.text}
												</span>
											</p>
										</div>
										<div>
											<p class="text-xs text-muted-foreground">{t('devices.temperature')}</p>
											<p class="text-sm">
												<span class="inline-flex items-center gap-1 {tempInfo.color}">
													<Thermometer class="h-4 w-4" />
													{tempInfo.text}
												</span>
											</p>
										</div>
										<div>
											<p class="text-xs text-muted-foreground">{t('devices.boundUser')}</p>
											<p class="text-sm">{device.user_nickname || '-'}</p>
										</div>

										{#if device.state === 'logged_in'}
											<div>
												<p class="text-xs text-muted-foreground">{t('devices.sessionDuration')}</p>
												<p class="text-sm">
													{formatDuration(
														liveSessionDurations[device.id] !== undefined
															? liveSessionDurations[device.id]
															: device.session_duration_seconds
													)}
												</p>
											</div>
											<div>
												<p class="text-xs text-muted-foreground">{t('devices.billingRate')}</p>
												<p class="text-sm">
													{#if device.plan_rate_cents && device.plan_unit}
														{formatCurrency(device.plan_rate_cents)}{device.plan_unit === 'hour'
															? t('devices.perHour')
															: t('devices.perMinute')}
													{:else}
														-
													{/if}
												</p>
											</div>
											<div>
												<p class="text-xs text-muted-foreground">{t('devices.currentCost')}</p>
												<p class="text-sm">
													{device.current_cost_cents !== undefined
														? formatCurrency(device.current_cost_cents)
														: '-'}
												</p>
											</div>
										{/if}
									</div>
								</div>
							</Table.Cell>
						</Table.Row>
					{/if}
				{/each}
			{/if}
		</Table.Body>
	</Table.Root>

	<!-- Pagination -->
	<TablePagination bind:page bind:rowsPerPage {total} onChange={loadDevices} />
</div>

<!-- Disconnect Confirm Dialog -->
<ConfirmActionDialog
	bind:open={disconnectDialogOpen}
	title={t('devices.disconnectDevice')}
	description={t('devices.disconnectWarning')}
	warningText={t('devices.disconnectWarning')}
	warningTone="destructive"
	confirmLabel={t('devices.disconnect')}
	confirmVariant="destructive"
	onConfirm={handleDisconnect}
	onCancel={() => (disconnectDevice = null)}
>
	{#snippet children()}
		{#if disconnectDevice}
			<div class="space-y-2 py-2">
				<p class="text-sm">
					<span class="font-medium">{t('common.device')}:</span>
					{disconnectDevice.name || '-'}
				</p>
				<p class="text-sm">
					<span class="font-medium">{t('common.status')}:</span>
					{disconnectDevice.state || '-'}
				</p>
				<p class="text-sm">
					<span class="font-medium">{t('devices.boundUser')}:</span>
					{disconnectDevice.user_nickname || t('devices.noBound')}
				</p>
			</div>
		{/if}
	{/snippet}
</ConfirmActionDialog>

<!-- Reboot Confirm Dialog -->
<ConfirmActionDialog
	bind:open={rebootDialogOpen}
	title={t('devices.rebootDevice')}
	description={t('devices.rebootWarning')}
	warningText={t('devices.rebootWarning')}
	warningTone="warning"
	confirmLabel={t('devices.reboot')}
	confirmClass="bg-orange-600 hover:bg-orange-700 text-white"
	onConfirm={handleReboot}
	onCancel={() => (rebootDevice = null)}
>
	{#snippet children()}
		{#if rebootDevice}
			<div class="space-y-2 py-2">
				<p class="text-sm">
					<span class="font-medium">{t('common.device')}:</span>
					{rebootDevice.name || '-'}
				</p>
				<p class="text-sm">
					<span class="font-medium">{t('common.status')}:</span>
					{rebootDevice.state || '-'}
				</p>
				<p class="text-sm">
					<span class="font-medium">{t('devices.boundUser')}:</span>
					{rebootDevice.user_nickname || t('devices.noBound')}
				</p>
			</div>
		{/if}
	{/snippet}
</ConfirmActionDialog>

<!-- Poweroff Confirm Dialog -->
<ConfirmActionDialog
	bind:open={poweroffDialogOpen}
	title={t('devices.poweroffDevice')}
	description={t('devices.poweroffWarning')}
	warningText={t('devices.poweroffWarning')}
	warningTone="destructive"
	confirmLabel={t('devices.poweroff')}
	confirmVariant="destructive"
	onConfirm={handlePoweroff}
	onCancel={() => (poweroffDevice = null)}
>
	{#snippet children()}
		{#if poweroffDevice}
			<div class="space-y-2 py-2">
				<p class="text-sm">
					<span class="font-medium">{t('common.device')}:</span>
					{poweroffDevice.name || '-'}
				</p>
				<p class="text-sm">
					<span class="font-medium">{t('common.status')}:</span>
					{poweroffDevice.state || '-'}
				</p>
				<p class="text-sm">
					<span class="font-medium">{t('devices.boundUser')}:</span>
					{poweroffDevice.user_nickname || t('devices.noBound')}
				</p>
			</div>
		{/if}
	{/snippet}
</ConfirmActionDialog>

<!-- Shell Dialog -->
{#if shellDevice}
	<DeviceShell
		deviceId={shellDevice.id}
		deviceName={shellDevice.name || shellDevice.machine_code || ''}
		bind:open={shellOpen}
	/>
{/if}

<!-- Screenshot Dialog -->
{#if screenshotDevice}
	<DeviceScreenshot
		deviceId={screenshotDevice.id}
		deviceName={screenshotDevice.name || screenshotDevice.machine_code || ''}
		bind:open={screenshotOpen}
	/>
{/if}
