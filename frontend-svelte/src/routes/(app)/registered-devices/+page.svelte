<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from '$lib/i18n';
	import { registeredDevicesAPI, tenantsAPI } from '$lib/api';
	import { toast } from 'svelte-sonner';
	import { Plus, Pencil, Trash2, Check, X, Search, Clock } from '@lucide/svelte';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Table from '$lib/components/ui/table';
	import * as Tabs from '$lib/components/ui/tabs';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Badge } from '$lib/components/ui/badge';
	import { Textarea } from '$lib/components/ui/textarea';
	import { Switch } from '$lib/components/ui/switch';
	import ConfirmActionDialog from '$lib/components/confirm-action-dialog.svelte';
	import { runWithToast } from '$lib/action-feedback';

	// --- Device list state ---
	let devices: any[] = $state([]);
	let loading = $state(true);
	let statusFilter = $state('all');
	let searchQuery = $state('');
	let searchDebounce = $state('');
	let tenants: any[] = $state([]);

	// --- Add/Edit dialog state ---
	let dialogOpen = $state(false);
	let dialogMode: 'add' | 'edit' = $state('add');
	let currentDevice: any = $state(null);
	let formData = $state({
		machine_code: '',
		host_id: '',
		device_name: '',
		remark: '',
	});
	let formErrors: Record<string, string> = $state({});

	// --- Delete confirm dialog state ---
	let deleteDialogOpen = $state(false);
	let deviceToDelete: any = $state(null);

	// --- Schedule dialog state ---
	let scheduleDialogOpen = $state(false);
	let scheduleDevice: any = $state(null);
	let scheduleForm = $state({
		shutdown_enabled: false,
		shutdown_time: '23:00',
		poweron_enabled: false,
		poweron_time: '08:00',
	});

	// --- Debounce search (300ms) ---
	$effect(() => {
		const timer = setTimeout(() => {
			searchDebounce = searchQuery;
		}, 300);
		return () => clearTimeout(timer);
	});

	// --- Filtered devices (derived) ---
	let filteredDevices = $derived.by(() => {
		let result = devices;

		if (statusFilter !== 'all') {
			result = result.filter((device) => device.status === statusFilter);
		}

		if (searchDebounce.trim()) {
			const query = searchDebounce.toLowerCase();
			result = result.filter(
				(device) =>
					device.machine_code?.toLowerCase().includes(query) ||
					device.host_id?.toLowerCase().includes(query) ||
					device.device_name?.toLowerCase().includes(query) ||
					device.ip?.toLowerCase().includes(query)
			);
		}

		return result;
	});

	let pendingCount = $derived(devices.filter((d) => d.status === 'pending').length);

	// --- Load data ---
	onMount(() => {
		loadDevices();
		loadTenants();
	});

	async function loadDevices() {
		try {
			loading = true;
			const data = await registeredDevicesAPI.list();
			devices = data || [];
		} catch (error: any) {
			console.error('Failed to load registered devices:', error);
			toast.error(error.message || t('common.loadFailed'));
		} finally {
			loading = false;
		}
	}

	async function loadTenants() {
		try {
			const data = await tenantsAPI.getAllActive();
			tenants = data || [];
		} catch (error) {
			console.error('Failed to load tenants:', error);
		}
	}

	// --- Add/Edit dialog ---
	function openAddDialog() {
		dialogMode = 'add';
		currentDevice = null;
		formData = { machine_code: '', host_id: '', device_name: '', remark: '' };
		formErrors = {};
		dialogOpen = true;
	}

	function openEditDialog(device: any) {
		dialogMode = 'edit';
		currentDevice = device;
		formData = {
			machine_code: device.machine_code || '',
			host_id: device.host_id || '',
			device_name: device.device_name || '',
			remark: device.remark || '',
		};
		formErrors = {};
		dialogOpen = true;
	}

	function closeDialog() {
		dialogOpen = false;
		currentDevice = null;
		formData = { machine_code: '', host_id: '', device_name: '', remark: '' };
		formErrors = {};
	}

	function validateForm(): boolean {
		const errors: Record<string, string> = {};

		if (dialogMode === 'add') {
			if (!formData.machine_code || formData.machine_code.trim() === '') {
				errors.machine_code = t('regDevices.machineCodeRequired');
			}
		}

		formErrors = errors;
		return Object.keys(errors).length === 0;
	}

	async function handleSave() {
		if (!validateForm()) return;

		try {
			if (dialogMode === 'edit') {
				const data = {
					device_name: formData.device_name.trim(),
					remark: formData.remark.trim(),
				};
				await registeredDevicesAPI.update(currentDevice.id, data);
				toast.success(t('common.updateSuccess'));
			} else {
				const data = {
					machine_code: formData.machine_code.trim(),
					host_id: formData.host_id.trim(),
					device_name: formData.device_name.trim(),
					remark: formData.remark.trim(),
				};
				await registeredDevicesAPI.create(data);
				toast.success(t('common.createSuccess'));
			}
			closeDialog();
			loadDevices();
		} catch (error: any) {
			console.error('Failed to save device:', error);
			toast.error(error.message || t('common.saveFailed'));
		}
	}

	// --- Approve / Reject ---
	async function handleApprove(device: any) {
		try {
			await registeredDevicesAPI.approve(device.id);
			toast.success(t('common.updateSuccess'));
			loadDevices();
		} catch (error: any) {
			console.error('Failed to approve device:', error);
			toast.error(error.message || t('common.updateFailed'));
		}
	}

	async function handleReject(device: any) {
		try {
			await registeredDevicesAPI.reject(device.id);
			toast.success(t('common.updateSuccess'));
			loadDevices();
		} catch (error: any) {
			console.error('Failed to reject device:', error);
			toast.error(error.message || t('common.updateFailed'));
		}
	}

	// --- Delete ---
	function openDeleteDialog(device: any) {
		deviceToDelete = device;
		deleteDialogOpen = true;
	}

	async function handleDelete() {
		if (!deviceToDelete) return;
		const ok = await runWithToast(() => registeredDevicesAPI.delete(deviceToDelete.id), {
			successMessage: t('common.deleteSuccess'),
			errorFallback: t('common.deleteFailed'),
			logLabel: 'Failed to delete device:',
		});
		if (ok) {
			deleteDialogOpen = false;
			deviceToDelete = null;
			loadDevices();
		}
	}

	// --- Tenant assignment ---
	async function handleAssignTenant(deviceId: number, tenantId: string) {
		try {
			await registeredDevicesAPI.assignTenant(deviceId, {
				tenant_id: tenantId || null,
			});
			toast.success(t('common.updateSuccess'));
			loadDevices();
		} catch (error: any) {
			toast.error(error.message || t('common.updateFailed'));
		}
	}

	// --- Schedule dialog ---
	function openScheduleDialog(device: any) {
		scheduleDevice = device;
		scheduleForm = {
			shutdown_enabled: device.shutdown_enabled || false,
			shutdown_time: device.shutdown_time || '23:00',
			poweron_enabled: device.poweron_enabled || false,
			poweron_time: device.poweron_time || '08:00',
		};
		scheduleDialogOpen = true;
	}

	function hasMinTimeGap(t1: string, t2: string, minMinutes: number): boolean {
		const [h1, m1] = t1.split(':').map(Number);
		const [h2, m2] = t2.split(':').map(Number);
		let diff = Math.abs((h1 * 60 + m1) - (h2 * 60 + m2));
		if (diff > 720) diff = 1440 - diff;
		return diff >= minMinutes;
	}

	async function handleSaveSchedule() {
		if (!scheduleDevice) return;
		if (scheduleForm.shutdown_enabled && scheduleForm.poweron_enabled) {
			if (!hasMinTimeGap(scheduleForm.shutdown_time, scheduleForm.poweron_time, 10)) {
				toast.error(t('regDevices.scheduleGapError'));
				return;
			}
		}
		try {
			await registeredDevicesAPI.updateSchedule(scheduleDevice.id, scheduleForm);
			toast.success(t('common.updateSuccess'));
			scheduleDialogOpen = false;
			scheduleDevice = null;
			loadDevices();
		} catch (error: any) {
			console.error('Failed to update schedule:', error);
			toast.error(error.message || t('common.updateFailed'));
		}
	}

	async function handleToggleBypassCharge(device: any) {
		try {
			await registeredDevicesAPI.updateBypassCharge(device.id, {
				bypass_charge_enabled: !device.bypass_charge_enabled,
			});
			toast.success(t('common.updateSuccess'));
			loadDevices();
		} catch (error: any) {
			console.error('Failed to toggle bypass charge:', error);
			toast.error(error.message || t('common.updateFailed'));
		}
	}

	// --- Helpers ---
	function getStatusBadge(status: string): { label: string; variant: 'default' | 'secondary' | 'destructive' | 'outline' } {
		const statusConfig: Record<string, { label: string; variant: 'default' | 'secondary' | 'destructive' | 'outline' }> = {
			pending: { label: t('regDevices.pending'), variant: 'outline' },
			approved: { label: t('regDevices.approved'), variant: 'default' },
			rejected: { label: t('regDevices.rejected'), variant: 'destructive' },
		};
		return statusConfig[status] || statusConfig.pending;
	}

	function formatDateTime(dateString: string | null | undefined): string {
		if (!dateString) return '\u2014';
		return new Date(dateString).toLocaleString();
	}
</script>

<!-- Header -->
<div class="flex items-center justify-between mb-6">
	<h2 class="text-2xl font-semibold">{t('regDevices.title')}</h2>
	<Button onclick={openAddDialog}>
		<Plus class="mr-2 h-4 w-4" />
		{t('regDevices.addDevice')}
	</Button>
</div>

<!-- Search Bar -->
<div class="relative mb-4">
	<Input
		placeholder={t('regDevices.searchPlaceholder')}
		bind:value={searchQuery}
		class="pl-9"
	/>
	<Search class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
</div>

<!-- Status Filter Tabs -->
<Tabs.Root bind:value={statusFilter} class="mb-4">
	<Tabs.List>
		<Tabs.Trigger value="all">{t('common.all')}</Tabs.Trigger>
		<Tabs.Trigger value="pending">
			{t('regDevices.pending')}
			{#if pendingCount > 0}
				<Badge variant="secondary" class="ml-1.5 px-1.5 py-0 text-xs">{pendingCount}</Badge>
			{/if}
		</Tabs.Trigger>
		<Tabs.Trigger value="approved">{t('regDevices.approved')}</Tabs.Trigger>
		<Tabs.Trigger value="rejected">{t('regDevices.rejected')}</Tabs.Trigger>
	</Tabs.List>
</Tabs.Root>

<!-- Device Table -->
<div class="rounded-lg border">
	<Table.Root>
		<Table.Header>
			<Table.Row>
				<Table.Head>{t('common.status')}</Table.Head>
				<Table.Head>{t('regDevices.deviceName')}</Table.Head>
				<Table.Head>{t('regDevices.machineCode')}</Table.Head>
				<Table.Head>{t('regDevices.hostId')}</Table.Head>
				<Table.Head>{t('users.tenant')}</Table.Head>
				<Table.Head>{t('regDevices.ip')}</Table.Head>
				<Table.Head>{t('regDevices.bypassCharge')}</Table.Head>
				<Table.Head>{t('regDevices.lastAttempt')}</Table.Head>
				<Table.Head>{t('common.createdAt')}</Table.Head>
				<Table.Head class="text-right">{t('common.actions')}</Table.Head>
			</Table.Row>
		</Table.Header>
		<Table.Body>
			{#if loading}
				<Table.Row>
					<Table.Cell colspan={10} class="text-center py-8 text-muted-foreground">
						{t('common.loading')}
					</Table.Cell>
				</Table.Row>
			{:else if filteredDevices.length === 0}
				<Table.Row>
					<Table.Cell colspan={10} class="text-center py-8 text-muted-foreground">
						{searchDebounce.trim() || statusFilter !== 'all'
							? t('regDevices.noMatch')
							: t('regDevices.noDevices')}
					</Table.Cell>
				</Table.Row>
			{:else}
				{#each filteredDevices as device (device.id)}
					{@const status = getStatusBadge(device.status)}
					<Table.Row>
						<Table.Cell>
							<Badge variant={status.variant}>{status.label}</Badge>
						</Table.Cell>
						<Table.Cell>
							{#if device.device_name}
								{device.device_name}
							{:else}
								<span class="text-muted-foreground">&mdash;</span>
							{/if}
						</Table.Cell>
						<Table.Cell>
							<span class="font-mono text-sm">{device.machine_code}</span>
						</Table.Cell>
						<Table.Cell>
							<span class="font-mono text-sm">{device.host_id}</span>
						</Table.Cell>
						<Table.Cell>
							<select
								class="w-full min-w-[120px] rounded-lg border bg-transparent px-2.5 py-1.5 text-sm"
								value={device.tenant_id || ''}
								onchange={(e) => handleAssignTenant(device.id, e.currentTarget.value)}
							>
								<option value="">{t('regDevices.unassigned')}</option>
								{#each tenants as tenant (tenant.id)}
									<option value={tenant.id}>
										{tenant.display_name || tenant.username}
									</option>
								{/each}
							</select>
						</Table.Cell>
						<Table.Cell>
							<span class="text-sm">{device.ip || '\u2014'}</span>
						</Table.Cell>
						<Table.Cell>
							<Switch
								checked={device.bypass_charge_enabled || false}
								onCheckedChange={() => handleToggleBypassCharge(device)}
							/>
						</Table.Cell>
						<Table.Cell>
							<span class="text-sm">{formatDateTime(device.last_attempt_at)}</span>
						</Table.Cell>
						<Table.Cell>
							<span class="text-sm">{formatDateTime(device.created_at)}</span>
						</Table.Cell>
						<Table.Cell class="text-right">
							<div class="flex items-center justify-end gap-1">
								{#if device.status === 'pending'}
									<Button
										variant="ghost"
										size="icon-sm"
										onclick={() => handleApprove(device)}
										title={t('regDevices.approved')}
										class="text-green-600 hover:text-green-700"
									>
										<Check class="h-4 w-4" />
									</Button>
									<Button
										variant="ghost"
										size="icon-sm"
										onclick={() => handleReject(device)}
										title={t('regDevices.rejected')}
										class="text-destructive hover:text-destructive"
									>
										<X class="h-4 w-4" />
									</Button>
								{/if}
								{#if device.status === 'rejected'}
									<Button
										variant="ghost"
										size="icon-sm"
										onclick={() => handleApprove(device)}
										title={t('regDevices.approved')}
										class="text-green-600 hover:text-green-700"
									>
										<Check class="h-4 w-4" />
									</Button>
								{/if}
								<Button
									variant="ghost"
									size="icon-sm"
									onclick={() => openScheduleDialog(device)}
									title={t('regDevices.schedule')}
								>
									<Clock class="h-4 w-4" />
								</Button>
								<Button
									variant="ghost"
									size="icon-sm"
									onclick={() => openEditDialog(device)}
									title={t('common.edit')}
								>
									<Pencil class="h-4 w-4" />
								</Button>
								<Button
									variant="ghost"
									size="icon-sm"
									onclick={() => openDeleteDialog(device)}
									title={t('common.delete')}
								>
									<Trash2 class="h-4 w-4 text-destructive" />
								</Button>
							</div>
						</Table.Cell>
					</Table.Row>
				{/each}
			{/if}
		</Table.Body>
	</Table.Root>
</div>

<!-- Add/Edit Device Dialog -->
<Dialog.Root bind:open={dialogOpen}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>
				{dialogMode === 'add' ? t('regDevices.addApproved') : t('regDevices.editDevice')}
			</Dialog.Title>
		</Dialog.Header>

		<div class="space-y-4 py-4">
			{#if dialogMode === 'add'}
				<div class="space-y-2">
					<Label for="machine-code">{t('regDevices.machineCode')} *</Label>
					<Input
						id="machine-code"
						placeholder="e.g., a1b2c3d4e5f6..."
						bind:value={formData.machine_code}
						class={formErrors.machine_code ? 'border-destructive' : ''}
					/>
					{#if formErrors.machine_code}
						<p class="text-sm text-destructive">{formErrors.machine_code}</p>
					{:else}
						<p class="text-sm text-muted-foreground">{t('regDevices.machineCodeHelp')}</p>
					{/if}
				</div>

				<div class="space-y-2">
					<Label for="host-id">{t('regDevices.hostId')}</Label>
					<Input
						id="host-id"
						placeholder="e.g., a1b2c3d4 (first 8 chars)"
						bind:value={formData.host_id}
					/>
					<p class="text-sm text-muted-foreground">{t('regDevices.hostIdHelp')}</p>
				</div>
			{/if}

			<div class="space-y-2">
				<Label for="device-name">{t('regDevices.deviceName')}</Label>
				<Input
					id="device-name"
					placeholder="e.g., Office Desktop"
					bind:value={formData.device_name}
				/>
				<p class="text-sm text-muted-foreground">{t('regDevices.deviceNameHelp')}</p>
			</div>

			<div class="space-y-2">
				<Label for="device-remark">{t('common.description')}</Label>
				<Textarea
					id="device-remark"
					placeholder="Admin notes..."
					bind:value={formData.remark}
					rows={3}
				/>
				<p class="text-sm text-muted-foreground">{t('regDevices.remarkHelp')}</p>
			</div>

			{#if dialogMode === 'add'}
				<div class="rounded-md border border-blue-200 bg-blue-50 p-3 text-sm text-blue-800 dark:border-blue-800 dark:bg-blue-950 dark:text-blue-200">
					{t('regDevices.autoApprovedNote')}
				</div>
			{/if}
		</div>

		<Dialog.Footer>
			<Button variant="outline" onclick={closeDialog}>{t('common.cancel')}</Button>
			<Button onclick={handleSave}>{t('common.save')}</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<!-- Delete Confirmation Dialog -->
<ConfirmActionDialog
	bind:open={deleteDialogOpen}
	title={t('regDevices.deleteDevice')}
	confirmLabel={t('common.delete')}
	confirmVariant="destructive"
	onConfirm={handleDelete}
	onCancel={() => (deviceToDelete = null)}
>
	{#snippet children()}
		{#if deviceToDelete}
			<p class="text-sm text-muted-foreground">
				{t('regDevices.deleteConfirm')}
				<strong>{deviceToDelete.device_name || deviceToDelete.machine_code}</strong>?
			</p>
		{/if}
	{/snippet}
</ConfirmActionDialog>

<!-- Schedule Dialog -->
<Dialog.Root bind:open={scheduleDialogOpen}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>{t('regDevices.scheduleTitle')}</Dialog.Title>
			{#if scheduleDevice}
				<Dialog.Description>
					{scheduleDevice.device_name || scheduleDevice.machine_code}
				</Dialog.Description>
			{/if}
		</Dialog.Header>

		<div class="space-y-4 py-4">
			<!-- Auto Shutdown -->
			<div class="space-y-3">
				<div class="flex items-center justify-between">
					<Label for="shutdown-enabled">{t('regDevices.autoShutdown')}</Label>
					<Switch bind:checked={scheduleForm.shutdown_enabled} id="shutdown-enabled" />
				</div>
				<div class="space-y-2">
					<Label for="shutdown-time" class={!scheduleForm.shutdown_enabled ? 'text-muted-foreground' : ''}>
						{t('regDevices.enabled')}
					</Label>
					<input
						type="time"
						id="shutdown-time"
						bind:value={scheduleForm.shutdown_time}
						disabled={!scheduleForm.shutdown_enabled}
						class="w-full rounded-lg border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
					/>
				</div>
			</div>

			<!-- Separator -->
			<div class="border-t"></div>

			<!-- Auto Power On -->
			<div class="space-y-3">
				<div class="flex items-center justify-between">
					<Label for="poweron-enabled">{t('regDevices.autoPowerOn')}</Label>
					<Switch bind:checked={scheduleForm.poweron_enabled} id="poweron-enabled" />
				</div>
				<div class="space-y-2">
					<Label for="poweron-time" class={!scheduleForm.poweron_enabled ? 'text-muted-foreground' : ''}>
						{t('regDevices.enabled')}
					</Label>
					<input
						type="time"
						id="poweron-time"
						bind:value={scheduleForm.poweron_time}
						disabled={!scheduleForm.poweron_enabled}
						class="w-full rounded-lg border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
					/>
				</div>
			</div>

			<!-- Info Note -->
			<div class="rounded-md border border-blue-200 bg-blue-50 p-3 text-sm text-blue-800 dark:border-blue-800 dark:bg-blue-950 dark:text-blue-200">
				{t('regDevices.scheduleNote')}
			</div>
		</div>

		<Dialog.Footer>
			<Button variant="outline" onclick={() => { scheduleDialogOpen = false; scheduleDevice = null; }}>
				{t('common.cancel')}
			</Button>
			<Button onclick={handleSaveSchedule}>{t('common.save')}</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
