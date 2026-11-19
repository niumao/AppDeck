<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from '$lib/i18n';
	import { moonlightHostsAPI } from '$lib/api';
	import { toast } from 'svelte-sonner';
	import { Plus, Pencil, Trash2, LockOpen, Link, Wrench } from '@lucide/svelte';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Table from '$lib/components/ui/table';
	import * as Alert from '$lib/components/ui/alert';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Badge } from '$lib/components/ui/badge';
	import { Switch } from '$lib/components/ui/switch';
	import ConfirmActionDialog from '$lib/components/confirm-action-dialog.svelte';
	import { runWithToast } from '$lib/action-feedback';

	// --- Host list state ---
	let hosts: any[] = $state([]);
	let loading = $state(true);

	// --- Create/Edit dialog state ---
	let dialogOpen = $state(false);
	let dialogMode: 'add' | 'edit' = $state('add');
	let currentHost: any = $state(null);
	let formHost = $state('');
	let formPort = $state('0');
	let formEnabled = $state(true);
	let formSunshineUser = $state('');
	let formSunshinePass = $state('');
	let formErrors: Record<string, string> = $state({});

	// --- Delete confirm dialog state ---
	let deleteDialogOpen = $state(false);
	let hostToDelete: any = $state(null);

	// --- Force release confirm dialog state ---
	let releaseDialogOpen = $state(false);
	let hostToRelease: any = $state(null);

	// --- Pair confirm dialog state ---
	let pairDialogOpen = $state(false);
	let hostToPair: any = $state(null);

	// --- Repair confirm dialog state ---
	let repairDialogOpen = $state(false);
	let hostToRepair: any = $state(null);

	// --- Load hosts ---
	onMount(() => {
		loadHosts();
	});

	async function loadHosts() {
		try {
			loading = true;
			const data = await moonlightHostsAPI.list();
			hosts = data || [];
		} catch (error: any) {
			console.error('Failed to load Moonlight hosts:', error);
			toast.error(error.message || t('common.loadFailed'));
		} finally {
			loading = false;
		}
	}

	// --- Create/Edit dialog ---
	function handleAddClick() {
		dialogMode = 'add';
		currentHost = null;
		formHost = '';
		formPort = '0';
		formEnabled = true;
		formSunshineUser = '';
		formSunshinePass = '';
		formErrors = {};
		dialogOpen = true;
	}

	function handleEditClick(host: any) {
		dialogMode = 'edit';
		currentHost = host;
		formHost = host.host;
		formPort = String(host.port);
		formEnabled = host.enabled;
		formSunshineUser = host.sunshine_user || '';
		formSunshinePass = '';
		formErrors = {};
		dialogOpen = true;
	}

	function closeDialog() {
		dialogOpen = false;
		currentHost = null;
		formHost = '';
		formPort = '0';
		formEnabled = true;
		formSunshineUser = '';
		formSunshinePass = '';
		formErrors = {};
	}

	function validateForm(): boolean {
		const errors: Record<string, string> = {};

		// Validate host
		if (!formHost || formHost.trim() === '') {
			errors.host = t('moonlight.hostRequired');
		} else {
			const trimmedHost = formHost.trim();
			if (/^https?:\/\//i.test(trimmedHost)) {
				errors.host = 'Do not include http:// or https://';
			} else if (trimmedHost.endsWith('/')) {
				errors.host = 'Do not include trailing slash';
			} else {
				const ipv4Regex = /^(\d{1,3}\.){3}\d{1,3}$/;
				const hostnameRegex = /^[a-zA-Z0-9.-]+$/;

				if (ipv4Regex.test(trimmedHost)) {
					const parts = trimmedHost.split('.');
					for (const part of parts) {
						const num = parseInt(part, 10);
						if (num < 0 || num > 255) {
							errors.host = 'Invalid IP address (octets must be 0-255)';
							break;
						}
					}
				} else if (!hostnameRegex.test(trimmedHost)) {
					errors.host = 'Invalid IP address or hostname';
				}
			}
		}

		// Validate port
		const portNum = parseInt(formPort, 10);
		if (isNaN(portNum)) {
			errors.port = 'Port must be a number';
		} else if (portNum < 0 || portNum > 65535) {
			errors.port = 'Port must be between 0 and 65535';
		}

		formErrors = errors;
		return Object.keys(errors).length === 0;
	}

	async function handleSave() {
		if (!validateForm()) return;

		try {
			const data: any = {
				host: formHost.trim(),
				port: parseInt(formPort, 10)
			};
			if (formSunshineUser) {
				data.sunshine_user = formSunshineUser;
			}
			if (formSunshinePass) {
				data.sunshine_pass = formSunshinePass;
			}

			if (dialogMode === 'edit') {
				data.enabled = formEnabled;
				await moonlightHostsAPI.update(currentHost.id, data);
				toast.success(t('common.updateSuccess'));
			} else {
				await moonlightHostsAPI.create(data);
				toast.success(t('common.createSuccess'));
			}

			closeDialog();
			loadHosts();
		} catch (error: any) {
			console.error('Failed to save host:', error);
			toast.error(error.message || t('common.saveFailed'));
		}
	}

	// --- Delete ---
	function handleDeleteClick(host: any) {
		hostToDelete = host;
		deleteDialogOpen = true;
	}

	async function handleDelete() {
		if (!hostToDelete) return;
		const ok = await runWithToast(() => moonlightHostsAPI.delete(hostToDelete.id), {
			successMessage: t('common.deleteSuccess'),
			errorFallback: t('common.deleteFailed'),
			logLabel: 'Failed to delete host:',
		});
		if (ok) {
			deleteDialogOpen = false;
			hostToDelete = null;
			loadHosts();
		}
	}

	// --- Force release ---
	function handleReleaseClick(host: any) {
		hostToRelease = host;
		releaseDialogOpen = true;
	}

	async function handleRelease() {
		if (!hostToRelease) return;
		const ok = await runWithToast(() => moonlightHostsAPI.forceRelease(hostToRelease.id), {
			successMessage: t('common.updateSuccess'),
			errorFallback: t('common.updateFailed'),
			logLabel: 'Failed to release host:',
		});
		if (ok) {
			releaseDialogOpen = false;
			hostToRelease = null;
			loadHosts();
		}
	}

	// --- Pair ---
	function handlePairClick(host: any) {
		hostToPair = host;
		pairDialogOpen = true;
	}

	async function handlePair() {
		if (!hostToPair) return;
		const ok = await runWithToast(
			async () => {
				try {
					await moonlightHostsAPI.triggerPair(hostToPair.id);
				} catch (error: any) {
					const msg =
						error.response?.data?.message === 'MISSING_SUNSHINE_CREDENTIALS'
							? t('moonlight.missingCredentials')
							: error.response?.data?.message || t('moonlight.pairFailed');
					throw new Error(msg);
				}
			},
			{
				successMessage: t('moonlight.pairInitiated'),
				errorFallback: t('moonlight.pairFailed'),
				logLabel: 'Failed to trigger pairing:',
			}
		);
		if (ok) {
			pairDialogOpen = false;
			hostToPair = null;
			setTimeout(loadHosts, 3000);
		}
	}

	// --- Repair (re-pair) ---
	function handleRepairClick(host: any) {
		hostToRepair = host;
		repairDialogOpen = true;
	}

	async function handleRepair() {
		if (!hostToRepair) return;
		const ok = await runWithToast(
			async () => {
				try {
					await moonlightHostsAPI.triggerPair(hostToRepair.id);
				} catch (error: any) {
					const msg =
						error.response?.data?.message === 'MISSING_SUNSHINE_CREDENTIALS'
							? t('moonlight.missingCredentials')
							: error.response?.data?.message || t('moonlight.repairFailed');
					throw new Error(msg);
				}
			},
			{
				successMessage: t('moonlight.repairInitiated'),
				errorFallback: t('moonlight.repairFailed'),
				logLabel: 'Failed to trigger repair pairing:',
			}
		);
		if (ok) {
			repairDialogOpen = false;
			hostToRepair = null;
			setTimeout(loadHosts, 3000);
		}
	}

	// --- Helpers ---
	function isHostOccupied(host: any): boolean {
		return !!host?.occupied_by_device_id;
	}

	function getStatusBadge(host: any): { label: string; variant: 'default' | 'secondary' | 'destructive' | 'outline' } {
		if (!host.enabled) {
			return { label: t('common.disabled'), variant: 'secondary' };
		}
		if (host.occupied_by_device_id) {
			return { label: t('moonlight.occupied'), variant: 'destructive' };
		}
		return { label: t('moonlight.available'), variant: 'default' };
	}

	function getPairedBadge(host: any): { label: string; variant: 'default' | 'secondary' | 'destructive' | 'outline' } {
		if (host.paired) {
			return { label: t('moonlight.paired'), variant: 'outline' };
		}
		return { label: t('moonlight.unpaired'), variant: 'outline' };
	}
</script>

<!-- Header -->
<div class="flex items-center justify-between mb-6">
	<h2 class="text-2xl font-semibold">{t('moonlight.title')}</h2>
	<Button onclick={handleAddClick}>
		<Plus class="mr-2 h-4 w-4" />
		{t('moonlight.addHost')}
	</Button>
</div>

<!-- Hosts Table -->
<div class="rounded-md border">
	<Table.Root>
		<Table.Header>
			<Table.Row>
				<Table.Head>{t('moonlight.host')}</Table.Head>
				<Table.Head>{t('moonlight.port')}</Table.Head>
				<Table.Head>{t('common.enabled')}</Table.Head>
				<Table.Head>{t('common.status')}</Table.Head>
				<Table.Head>{t('moonlight.paired')}</Table.Head>
				<Table.Head>{t('moonlight.occupiedBy')}</Table.Head>
				<Table.Head class="text-right">{t('common.actions')}</Table.Head>
			</Table.Row>
		</Table.Header>
		<Table.Body>
			{#if loading}
				<Table.Row>
					<Table.Cell colspan={7} class="text-center py-8 text-muted-foreground">
						{t('common.loading')}
					</Table.Cell>
				</Table.Row>
			{:else if hosts.length === 0}
				<Table.Row>
					<Table.Cell colspan={7} class="text-center py-8 text-muted-foreground">
						{t('moonlight.noHosts')}
					</Table.Cell>
				</Table.Row>
			{:else}
				{#each hosts as host (host.id)}
					{@const status = getStatusBadge(host)}
					{@const paired = getPairedBadge(host)}
					<Table.Row>
						<Table.Cell>{host.host}</Table.Cell>
						<Table.Cell>
							{#if host.port === 0}
								<span class="text-muted-foreground">{t('common.default')}</span>
							{:else}
								{host.port}
							{/if}
						</Table.Cell>
						<Table.Cell>
							<Badge variant={host.enabled ? 'outline' : 'secondary'}>
								{host.enabled ? t('common.yes') : t('common.no')}
							</Badge>
						</Table.Cell>
						<Table.Cell>
							<Badge variant={status.variant}>{status.label}</Badge>
						</Table.Cell>
						<Table.Cell>
							<Badge variant={paired.variant}>
								{paired.label}
							</Badge>
						</Table.Cell>
						<Table.Cell>
							{#if host.occupied_by_device_id}
								<span
									class="font-mono text-sm"
									title={`Since: ${new Date(host.occupied_at).toLocaleString()}`}
								>
									{host.occupied_by_device_id.substring(0, 8)}...
								</span>
							{:else}
								<span class="text-muted-foreground">&mdash;</span>
							{/if}
						</Table.Cell>
						<Table.Cell class="text-right">
							<div class="flex items-center justify-end gap-1">
								<Button
									variant="ghost"
									size="icon"
									onclick={() => handleEditClick(host)}
									title={t('common.edit')}
								>
									<Pencil class="h-4 w-4" />
								</Button>
								{#if isHostOccupied(host)}
									<Button
										variant="ghost"
										size="icon"
										onclick={() => handleReleaseClick(host)}
										title={t('moonlight.forceRelease')}
									>
										<LockOpen class="h-4 w-4 text-yellow-600" />
									</Button>
								{/if}
								{#if !host.paired}
									<Button
										variant="ghost"
										size="icon"
										onclick={() => handlePairClick(host)}
										title={t('moonlight.pair')}
									>
										<Link class="h-4 w-4 text-blue-600" />
									</Button>
								{:else}
									<Button
										variant="ghost"
										size="icon"
										onclick={() => handleRepairClick(host)}
										title={t('moonlight.repair')}
									>
										<Wrench class="h-4 w-4 text-orange-600" />
									</Button>
								{/if}
								<Button
									variant="ghost"
									size="icon"
									onclick={() => handleDeleteClick(host)}
									disabled={isHostOccupied(host)}
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

<!-- Add/Edit Dialog -->
<Dialog.Root bind:open={dialogOpen}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>
				{dialogMode === 'add' ? t('moonlight.addMoonlightHost') : t('moonlight.editMoonlightHost')}
			</Dialog.Title>
		</Dialog.Header>

		<div class="space-y-4 py-4">
			<!-- Host -->
			<div class="space-y-2">
				<Label for="ml-host">{t('moonlight.host')}</Label>
				<Input
					id="ml-host"
					placeholder="192.168.1.100 or moonlight.local"
					bind:value={formHost}
					disabled={dialogMode === 'edit' && isHostOccupied(currentHost)}
				/>
				{#if formErrors.host}
					<p class="text-sm text-destructive">{formErrors.host}</p>
				{:else}
					<p class="text-sm text-muted-foreground">{t('moonlight.hostHelp')}</p>
				{/if}
			</div>

			<!-- Port -->
			<div class="space-y-2">
				<Label for="ml-port">{t('moonlight.port')}</Label>
				<Input
					id="ml-port"
					type="number"
					bind:value={formPort}
					min="0"
					max="65535"
					disabled={dialogMode === 'edit' && isHostOccupied(currentHost)}
				/>
				{#if formErrors.port}
					<p class="text-sm text-destructive">{formErrors.port}</p>
				{:else}
					<p class="text-sm text-muted-foreground">{t('moonlight.portHelp')}</p>
				{/if}
			</div>

			<!-- Sunshine User -->
			<div class="space-y-2">
				<Label for="ml-sunshine-user">{t('moonlight.sunshineUser')}</Label>
				<Input
					id="ml-sunshine-user"
					bind:value={formSunshineUser}
					autocomplete="off"
				/>
			</div>

			<!-- Sunshine Password -->
			<div class="space-y-2">
				<Label for="ml-sunshine-pass">{t('moonlight.sunshinePass')}</Label>
				<Input
					id="ml-sunshine-pass"
					type="password"
					bind:value={formSunshinePass}
					autocomplete="off"
					placeholder={dialogMode === 'edit' && currentHost?.has_sunshine_pass ? '••••••••' : ''}
				/>
				<p class="text-sm text-muted-foreground">
					{#if dialogMode === 'edit' && currentHost?.has_sunshine_pass && !formSunshinePass}
						{t('moonlight.sunshinePassKept')}
					{:else}
						{t('moonlight.sunshineCredHelp')}
					{/if}
				</p>
			</div>

			<!-- Edit-only: occupied warning + enabled switch -->
			{#if dialogMode === 'edit'}
				{#if isHostOccupied(currentHost)}
					<Alert.Root variant="destructive">
						<Alert.Description>{t('moonlight.occupiedWarning')}</Alert.Description>
					</Alert.Root>
				{/if}
				<div class="flex items-center gap-3">
					<Switch
						id="ml-enabled"
						checked={formEnabled}
						onCheckedChange={(v) => (formEnabled = v)}
					/>
					<Label for="ml-enabled">{t('common.enabled')}</Label>
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
	title={t('moonlight.deleteHost')}
	confirmLabel={t('common.delete')}
	confirmVariant="destructive"
	onConfirm={handleDelete}
	onCancel={() => (hostToDelete = null)}
>
	{#snippet children()}
		{#if hostToDelete}
			<p>
				{t('moonlight.deleteConfirm')} <strong>{hostToDelete.host}:{hostToDelete.port}</strong>?
			</p>
		{/if}
	{/snippet}
</ConfirmActionDialog>

<!-- Force Release Confirmation Dialog -->
<ConfirmActionDialog
	bind:open={releaseDialogOpen}
	title={t('moonlight.forceRelease')}
	confirmLabel={t('moonlight.forceRelease')}
	confirmVariant="destructive"
	onConfirm={handleRelease}
	onCancel={() => (hostToRelease = null)}
>
	{#snippet children()}
		{#if hostToRelease}
			<div class="space-y-4">
				<p>
					{t('moonlight.releaseConfirm')} <strong>{hostToRelease.host}:{hostToRelease.port}</strong>?
				</p>
				<Alert.Root variant="destructive">
					<Alert.Description>
						{t('moonlight.releaseWarning')}
						<strong>{hostToRelease.occupied_by_device_id}</strong>
						{t('moonlight.releaseWarning2')}
					</Alert.Description>
				</Alert.Root>
			</div>
		{/if}
	{/snippet}
</ConfirmActionDialog>

<!-- Pair Confirmation Dialog -->
<ConfirmActionDialog
	bind:open={pairDialogOpen}
	title={t('moonlight.pairHost')}
	confirmLabel={t('moonlight.pair')}
	confirmVariant="default"
	onConfirm={handlePair}
	onCancel={() => (hostToPair = null)}
>
	{#snippet children()}
		{#if hostToPair}
			<p>
				{t('moonlight.pairConfirm')} <strong>{hostToPair.host}:{hostToPair.port}</strong>?
			</p>
		{/if}
	{/snippet}
</ConfirmActionDialog>

<!-- Repair Confirmation Dialog -->
<ConfirmActionDialog
	bind:open={repairDialogOpen}
	title={t('moonlight.repairHost')}
	confirmLabel={t('moonlight.repair')}
	confirmVariant="default"
	onConfirm={handleRepair}
	onCancel={() => (hostToRepair = null)}
>
	{#snippet children()}
		{#if hostToRepair}
			<div class="space-y-4">
				<p>
					{t('moonlight.repairConfirm')} <strong>{hostToRepair.host}:{hostToRepair.port}</strong>?
				</p>
				<Alert.Root>
					<Alert.Description>{t('moonlight.repairWarning')}</Alert.Description>
				</Alert.Root>
			</div>
		{/if}
	{/snippet}
</ConfirmActionDialog>
