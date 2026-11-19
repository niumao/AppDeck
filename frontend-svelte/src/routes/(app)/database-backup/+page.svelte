<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from '$lib/i18n';
	import { backupAPI } from '$lib/api';
	import { toast } from 'svelte-sonner';
	import {
		DatabaseBackup,
		Download,
		Trash2,
		RotateCcw,
		Clock,
		Save,
		LoaderCircle,
		Upload
	} from '@lucide/svelte';
	import * as Card from '$lib/components/ui/card';
	import * as Table from '$lib/components/ui/table';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Switch } from '$lib/components/ui/switch';

	const INTERVAL_OPTIONS = [
		{ value: 6, labelKey: 'backup.every6h' },
		{ value: 12, labelKey: 'backup.every12h' },
		{ value: 24, labelKey: 'backup.daily' },
		{ value: 168, labelKey: 'backup.weekly' }
	];

	function formatBytes(bytes: number): string {
		if (bytes === 0) return '0 B';
		const k = 1024;
		const sizes = ['B', 'KB', 'MB', 'GB'];
		const i = Math.floor(Math.log(bytes) / Math.log(k));
		return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
	}

	// Backup list
	let backups: any[] = $state([]);
	let loading = $state(true);
	let creating = $state(false);
	let restoring = $state(false);
	let uploading = $state(false);
	let fileInput: HTMLInputElement;

	// Restore dialog
	let restoreOpen = $state(false);
	let restoreFilename = $state('');
	let restoreDate = $state('');
	let restoreConfirmText = $state('');

	// Delete dialog
	let deleteOpen = $state(false);
	let deleteFilename = $state('');
	let deleteConfirmText = $state('');
	let deleting = $state(false);

	// Schedule
	let scheduleEnabled = $state(false);
	let intervalHours = $state(24);
	let maxBackups = $state(20);
	let lastBackupAt: string | null = $state(null);
	let savingSchedule = $state(false);

	onMount(() => {
		fetchBackups();
		fetchSchedule();
	});

	async function fetchBackups() {
		try {
			const data = await backupAPI.list();
			backups = data || [];
		} catch {
			toast.error(t('common.loadFailed'));
		} finally {
			loading = false;
		}
	}

	async function fetchSchedule() {
		try {
			const data = await backupAPI.getSchedule();
			if (data) {
				scheduleEnabled = data.enabled;
				intervalHours = data.interval_hours;
				maxBackups = data.max_backups;
				lastBackupAt = data.last_backup_at;
			}
		} catch {
			// Schedule may not exist yet
		}
	}

	async function handleCreate() {
		creating = true;
		try {
			await backupAPI.create();
			toast.success(t('backup.backupSuccess'));
			fetchBackups();
			fetchSchedule();
		} catch (err: any) {
			toast.error(t('backup.backupFailed') + ': ' + (err.message || ''));
		} finally {
			creating = false;
		}
	}

	async function handleUpload(e: Event) {
		const input = e.target as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;
		if (!file.name.endsWith('.sql')) {
			toast.error(t('backup.invalidFileType'));
			input.value = '';
			return;
		}
		uploading = true;
		try {
			await backupAPI.upload(file);
			toast.success(t('backup.uploadSuccess'));
			fetchBackups();
		} catch (err: any) {
			toast.error(t('backup.uploadFailed') + ': ' + (err.message || ''));
		} finally {
			uploading = false;
			input.value = '';
		}
	}

	function handleRestoreClick(backup: any) {
		restoreFilename = backup.filename;
		restoreDate = new Date(backup.created_at).toLocaleString();
		restoreConfirmText = '';
		restoreOpen = true;
	}

	async function handleRestoreConfirm() {
		restoring = true;
		try {
			await backupAPI.restore(restoreFilename);
			restoreOpen = false;
			toast.success(t('backup.restoreSuccess'));
			setTimeout(() => window.location.reload(), 2000);
		} catch (err: any) {
			toast.error(t('backup.restoreFailed') + ': ' + (err.message || ''));
		} finally {
			restoring = false;
		}
	}

	function handleDeleteClick(filename: string) {
		deleteFilename = filename;
		deleteConfirmText = '';
		deleteOpen = true;
	}

	async function handleDeleteConfirm() {
		deleting = true;
		try {
			await backupAPI.delete(deleteFilename);
			deleteOpen = false;
			toast.success(t('backup.deleteSuccess'));
			fetchBackups();
		} catch (err: any) {
			toast.error(t('backup.deleteFailed') + ': ' + (err.message || ''));
		} finally {
			deleting = false;
		}
	}

	async function handleDownload(backup: any) {
		try {
			const blob = await backupAPI.download(backup.filename);
			const url = URL.createObjectURL(blob);
			const a = document.createElement('a');
			a.href = url;
			a.download = backup.filename;
			a.click();
			URL.revokeObjectURL(url);
		} catch {
			toast.error(t('backup.downloadFailed'));
		}
	}

	async function handleSaveSchedule() {
		savingSchedule = true;
		try {
			const data = await backupAPI.updateSchedule({
				enabled: scheduleEnabled,
				interval_hours: intervalHours,
				max_backups: maxBackups
			});
			if (data) {
				scheduleEnabled = data.enabled;
				intervalHours = data.interval_hours;
				maxBackups = data.max_backups;
				lastBackupAt = data.last_backup_at;
			}
			toast.success(t('backup.scheduleUpdated'));
		} catch (err: any) {
			toast.error(t('backup.saveFailed') + ': ' + (err.message || ''));
		} finally {
			savingSchedule = false;
		}
	}

	function handleMaxBackupsInput(e: Event) {
		const target = e.target as HTMLInputElement;
		const val = parseInt(target.value) || 1;
		maxBackups = Math.max(1, Math.min(100, val));
	}
</script>

<div class="space-y-6">
	<h2 class="text-2xl font-bold tracking-tight">{t('backup.title')}</h2>

	<!-- Create Backup Card -->
	<Card.Root>
		<Card.Header>
			<Card.Title class="flex items-center gap-2">
				<DatabaseBackup class="h-5 w-5" />
				{t('backup.createBackup')}
			</Card.Title>
		</Card.Header>
		<Card.Content>
			<div class="flex gap-3">
				<Button onclick={handleCreate} disabled={creating}>
					{#if creating}
						<LoaderCircle class="mr-2 h-4 w-4 animate-spin" />
						{t('backup.creating')}
					{:else}
						<DatabaseBackup class="mr-2 h-4 w-4" />
						{t('backup.createBackup')}
					{/if}
				</Button>
				<input type="file" accept=".sql" class="hidden" bind:this={fileInput} onchange={handleUpload} />
				<Button variant="outline" onclick={() => fileInput.click()} disabled={uploading}>
					{#if uploading}
						<LoaderCircle class="mr-2 h-4 w-4 animate-spin" />
						{t('backup.uploading')}
					{:else}
						<Upload class="mr-2 h-4 w-4" />
						{t('backup.uploadBackup')}
					{/if}
				</Button>
			</div>
		</Card.Content>
	</Card.Root>

	<!-- Backup List Card -->
	<Card.Root>
		<Card.Header>
			<Card.Title>{t('backup.backupList')}</Card.Title>
		</Card.Header>
		<Card.Content>
			{#if loading}
				<div class="flex justify-center py-8">
					<LoaderCircle class="h-6 w-6 animate-spin text-muted-foreground" />
				</div>
			{:else if backups.length === 0}
				<p class="py-4 text-center text-muted-foreground">{t('backup.noBackups')}</p>
			{:else}
				<div class="rounded-lg border">
					<Table.Root>
						<Table.Header>
							<Table.Row>
								<Table.Head>{t('backup.filename')}</Table.Head>
								<Table.Head>{t('backup.size')}</Table.Head>
								<Table.Head>{t('backup.date')}</Table.Head>
								<Table.Head class="text-right">{t('common.actions')}</Table.Head>
							</Table.Row>
						</Table.Header>
						<Table.Body>
							{#each backups as backup (backup.filename)}
								<Table.Row>
									<Table.Cell>
										<span class="font-mono text-sm">{backup.filename}</span>
									</Table.Cell>
									<Table.Cell>{formatBytes(backup.size)}</Table.Cell>
									<Table.Cell>{new Date(backup.created_at).toLocaleString()}</Table.Cell>
									<Table.Cell class="text-right">
										<Button
											variant="ghost"
											size="icon-sm"
											title={t('backup.restore')}
											onclick={() => handleRestoreClick(backup)}
										>
											<RotateCcw class="h-4 w-4 text-orange-500" />
										</Button>
										<Button
											variant="ghost"
											size="icon-sm"
											title={t('backup.download')}
											onclick={() => handleDownload(backup)}
										>
											<Download class="h-4 w-4 text-blue-500" />
										</Button>
										<Button
											variant="ghost"
											size="icon-sm"
											title={t('backup.delete')}
											onclick={() => handleDeleteClick(backup.filename)}
										>
											<Trash2 class="h-4 w-4 text-red-500" />
										</Button>
									</Table.Cell>
								</Table.Row>
							{/each}
						</Table.Body>
					</Table.Root>
				</div>
			{/if}
		</Card.Content>
	</Card.Root>

	<!-- Schedule Settings Card -->
	<Card.Root>
		<Card.Header>
			<Card.Title class="flex items-center gap-2">
				<Clock class="h-5 w-5" />
				{t('backup.schedule')}
			</Card.Title>
		</Card.Header>
		<Card.Content class="space-y-4">
			<div class="flex items-center gap-3">
				<Switch bind:checked={scheduleEnabled} />
				<Label>{t('backup.enabled')}</Label>
			</div>

			<div class="flex flex-wrap gap-4">
				<div class="w-[200px]">
					<Label class="mb-1.5 block">{t('backup.interval')}</Label>
					<select
						class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
						bind:value={intervalHours}
					>
						{#each INTERVAL_OPTIONS as opt}
							<option value={opt.value}>{t(opt.labelKey)}</option>
						{/each}
					</select>
				</div>

				<div class="w-[200px]">
					<Label class="mb-1.5 block">{t('backup.maxBackups')}</Label>
					<Input
						type="number"
						min={1}
						max={100}
						value={maxBackups}
						oninput={handleMaxBackupsInput}
					/>
				</div>
			</div>

			<p class="text-sm text-muted-foreground">
				{t('backup.lastBackup')}: {lastBackupAt ? new Date(lastBackupAt).toLocaleString() : t('backup.never')}
			</p>

			<Button onclick={handleSaveSchedule} disabled={savingSchedule}>
				{#if savingSchedule}
					<LoaderCircle class="mr-2 h-4 w-4 animate-spin" />
					{t('common.saving')}
				{:else}
					<Save class="mr-2 h-4 w-4" />
					{t('common.save')}
				{/if}
			</Button>
		</Card.Content>
	</Card.Root>
</div>

<!-- Restore Confirmation Dialog -->
<Dialog.Root bind:open={restoreOpen}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>{t('backup.restoreConfirmTitle')}</Dialog.Title>
		</Dialog.Header>
		<div class="space-y-4 py-2">
			<p class="text-sm text-muted-foreground">
				{t('backup.restoreConfirmText').replace('{date}', restoreDate)}
			</p>
			<Input
				placeholder="RESTORE"
				bind:value={restoreConfirmText}
			/>
		</div>
		<Dialog.Footer>
			<Button variant="outline" disabled={restoring} onclick={() => (restoreOpen = false)}>
				{t('common.cancel')}
			</Button>
			<Button
				variant="destructive"
				disabled={restoreConfirmText !== 'RESTORE' || restoring}
				onclick={handleRestoreConfirm}
			>
				{#if restoring}
					<LoaderCircle class="mr-2 h-4 w-4 animate-spin" />
				{/if}
				{t('backup.restore')}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<!-- Delete Confirmation Dialog -->
<Dialog.Root bind:open={deleteOpen}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>{t('backup.deleteConfirmTitle')}</Dialog.Title>
		</Dialog.Header>
		<div class="space-y-4 py-2">
			<p class="text-sm text-muted-foreground">
				{t('backup.deleteConfirmText').replace('{filename}', deleteFilename)}
			</p>
			<Input
				placeholder="DELETE"
				bind:value={deleteConfirmText}
			/>
		</div>
		<Dialog.Footer>
			<Button variant="outline" disabled={deleting} onclick={() => (deleteOpen = false)}>
				{t('common.cancel')}
			</Button>
			<Button
				variant="destructive"
				disabled={deleteConfirmText !== 'DELETE' || deleting}
				onclick={handleDeleteConfirm}
			>
				{#if deleting}
					<LoaderCircle class="mr-2 h-4 w-4 animate-spin" />
				{/if}
				{t('backup.delete')}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
