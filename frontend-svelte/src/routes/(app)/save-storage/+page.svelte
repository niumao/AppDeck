<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from '$lib/i18n';
	import { saveStorageAPI } from '$lib/api';
	import { toast } from 'svelte-sonner';
	import {
		HardDrive,
		Download,
		Upload,
		RotateCcw,
		Trash2,
		Save,
		LoaderCircle,
		DatabaseBackup
	} from '@lucide/svelte';
	import * as Card from '$lib/components/ui/card';
	import * as Table from '$lib/components/ui/table';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Switch } from '$lib/components/ui/switch';

	function formatBytes(bytes: number): string {
		if (!bytes) return '0 B';
		const k = 1024;
		const sizes = ['B', 'KB', 'MB', 'GB'];
		const i = Math.floor(Math.log(bytes) / Math.log(k));
		return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
	}

	// Storage config
	let syncEnabled = $state(true);
	let storageType = $state('local');
	let localPath = $state('./data/saves');
	let s3Bucket = $state('');
	let s3Region = $state('');
	let s3Prefix = $state('');
	let savingConfig = $state(false);

	// Current save data (informational; not per-user/per-game actions)
	let saves: any[] = $state([]);
	let loadingSaves = $state(true);

	// Backups (all users, all games in one archive -- like database-backup)
	let backups: any[] = $state([]);
	let loadingBackups = $state(true);
	let creating = $state(false);
	let uploading = $state(false);
	let backupFileInput: HTMLInputElement;

	// Restore dialog
	let restoreOpen = $state(false);
	let restoreFilename = $state('');
	let restoreDate = $state('');
	let restoreConfirmText = $state('');
	let restoring = $state(false);

	// Delete dialog
	let deleteOpen = $state(false);
	let deleteFilename = $state('');
	let deleteConfirmText = $state('');
	let deleting = $state(false);

	onMount(() => {
		fetchConfig();
		fetchSaves();
		fetchBackups();
	});

	async function fetchConfig() {
		try {
			const data = await saveStorageAPI.getConfig();
			if (data) {
				syncEnabled = data.enabled;
				storageType = data.storageType;
				localPath = data.localPath;
				s3Bucket = data.s3Bucket || '';
				s3Region = data.s3Region || '';
				s3Prefix = data.s3Prefix || '';
			}
		} catch {
			// Config not created yet, defaults stand
		}
	}

	async function fetchSaves() {
		try {
			const data = await saveStorageAPI.list();
			saves = data || [];
		} catch {
			toast.error(t('saveStorage.loadFailed'));
		} finally {
			loadingSaves = false;
		}
	}

	async function fetchBackups() {
		try {
			const data = await saveStorageAPI.listBackups();
			backups = data || [];
		} catch {
			toast.error(t('saveStorage.backupLoadFailed'));
		} finally {
			loadingBackups = false;
		}
	}

	async function handleSaveConfig() {
		savingConfig = true;
		try {
			const data = await saveStorageAPI.updateConfig({
				enabled: syncEnabled,
				storageType,
				localPath,
				s3Bucket,
				s3Region,
				s3Prefix
			});
			if (data) {
				syncEnabled = data.enabled;
				storageType = data.storageType;
				localPath = data.localPath;
				s3Bucket = data.s3Bucket || '';
				s3Region = data.s3Region || '';
				s3Prefix = data.s3Prefix || '';
			}
			toast.success(t('saveStorage.configSaved'));
		} catch (err: any) {
			toast.error(t('saveStorage.configSaveFailed') + ': ' + (err.message || ''));
		} finally {
			savingConfig = false;
		}
	}

	async function handleCreateBackup() {
		creating = true;
		try {
			await saveStorageAPI.createBackup();
			toast.success(t('saveStorage.backupSuccess'));
			fetchBackups();
		} catch (err: any) {
			toast.error(t('saveStorage.backupFailed') + ': ' + (err.message || ''));
		} finally {
			creating = false;
		}
	}

	async function handleUploadBackup(e: Event) {
		const input = e.target as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;
		if (!file.name.endsWith('.zip')) {
			toast.error(t('saveStorage.invalidFileType'));
			input.value = '';
			return;
		}
		uploading = true;
		try {
			await saveStorageAPI.uploadBackup(file);
			toast.success(t('saveStorage.uploadSuccess'));
			fetchBackups();
		} catch (err: any) {
			toast.error(t('saveStorage.uploadFailed') + ': ' + (err.message || ''));
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
			await saveStorageAPI.restoreBackup(restoreFilename);
			restoreOpen = false;
			toast.success(t('saveStorage.restoreSuccess'));
			fetchSaves();
		} catch (err: any) {
			toast.error(t('saveStorage.restoreFailed') + ': ' + (err.message || ''));
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
			await saveStorageAPI.deleteBackup(deleteFilename);
			deleteOpen = false;
			toast.success(t('saveStorage.deleteSuccess'));
			fetchBackups();
		} catch (err: any) {
			toast.error(t('saveStorage.deleteFailed') + ': ' + (err.message || ''));
		} finally {
			deleting = false;
		}
	}

	async function handleDownloadBackup(backup: any) {
		try {
			const blob = await saveStorageAPI.downloadBackup(backup.filename);
			const url = URL.createObjectURL(blob);
			const a = document.createElement('a');
			a.href = url;
			a.download = backup.filename;
			a.click();
			URL.revokeObjectURL(url);
		} catch {
			toast.error(t('saveStorage.downloadFailed'));
		}
	}
</script>

<div class="space-y-6">
	<h2 class="text-2xl font-bold tracking-tight">{t('saveStorage.title')}</h2>

	<!-- Storage Config Card -->
	<Card.Root>
		<Card.Header>
			<Card.Title class="flex items-center gap-2">
				<HardDrive class="h-5 w-5" />
				{t('saveStorage.storageConfig')}
			</Card.Title>
		</Card.Header>
		<Card.Content class="space-y-4">
			<div class="flex items-center gap-3">
				<Switch bind:checked={syncEnabled} />
				<Label>{t('saveStorage.enabled')}</Label>
			</div>

			<div class="w-[240px]">
				<Label class="mb-1.5 block">{t('saveStorage.storageType')}</Label>
				<select
					class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
					bind:value={storageType}
				>
					<option value="local">{t('saveStorage.typeLocal')}</option>
					<option value="s3">{t('saveStorage.typeS3')}</option>
				</select>
			</div>

			{#if storageType === 'local'}
				<div class="max-w-md">
					<Label class="mb-1.5 block">{t('saveStorage.localPath')}</Label>
					<Input bind:value={localPath} placeholder="./data/saves" />
				</div>
			{:else}
				<div class="grid max-w-2xl grid-cols-1 gap-4 sm:grid-cols-3">
					<div>
						<Label class="mb-1.5 block">{t('saveStorage.s3Bucket')}</Label>
						<Input bind:value={s3Bucket} disabled />
					</div>
					<div>
						<Label class="mb-1.5 block">{t('saveStorage.s3Region')}</Label>
						<Input bind:value={s3Region} disabled />
					</div>
					<div>
						<Label class="mb-1.5 block">{t('saveStorage.s3Prefix')}</Label>
						<Input bind:value={s3Prefix} disabled />
					</div>
				</div>
			{/if}

			<Button onclick={handleSaveConfig} disabled={savingConfig}>
				{#if savingConfig}
					<LoaderCircle class="mr-2 h-4 w-4 animate-spin" />
					{t('common.saving')}
				{:else}
					<Save class="mr-2 h-4 w-4" />
					{t('common.save')}
				{/if}
			</Button>
		</Card.Content>
	</Card.Root>

	<!-- Create/Upload Backup Card -->
	<Card.Root>
		<Card.Header>
			<Card.Title class="flex items-center gap-2">
				<DatabaseBackup class="h-5 w-5" />
				{t('saveStorage.createBackup')}
			</Card.Title>
		</Card.Header>
		<Card.Content>
			<div class="flex gap-3">
				<Button onclick={handleCreateBackup} disabled={creating}>
					{#if creating}
						<LoaderCircle class="mr-2 h-4 w-4 animate-spin" />
						{t('saveStorage.creating')}
					{:else}
						<DatabaseBackup class="mr-2 h-4 w-4" />
						{t('saveStorage.createBackup')}
					{/if}
				</Button>
				<input type="file" accept=".zip" class="hidden" bind:this={backupFileInput} onchange={handleUploadBackup} />
				<Button variant="outline" onclick={() => backupFileInput.click()} disabled={uploading}>
					{#if uploading}
						<LoaderCircle class="mr-2 h-4 w-4 animate-spin" />
						{t('saveStorage.uploading')}
					{:else}
						<Upload class="mr-2 h-4 w-4" />
						{t('saveStorage.uploadBackup')}
					{/if}
				</Button>
			</div>
		</Card.Content>
	</Card.Root>

	<!-- Backup List Card -->
	<Card.Root>
		<Card.Header>
			<Card.Title>{t('saveStorage.backupList')}</Card.Title>
		</Card.Header>
		<Card.Content>
			{#if loadingBackups}
				<div class="flex justify-center py-8">
					<LoaderCircle class="h-6 w-6 animate-spin text-muted-foreground" />
				</div>
			{:else if backups.length === 0}
				<p class="py-4 text-center text-muted-foreground">{t('saveStorage.noBackups')}</p>
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
											title={t('saveStorage.restore')}
											onclick={() => handleRestoreClick(backup)}
										>
											<RotateCcw class="h-4 w-4 text-orange-500" />
										</Button>
										<Button
											variant="ghost"
											size="icon-sm"
											title={t('saveStorage.download')}
											onclick={() => handleDownloadBackup(backup)}
										>
											<Download class="h-4 w-4 text-blue-500" />
										</Button>
										<Button
											variant="ghost"
											size="icon-sm"
											title={t('saveStorage.delete')}
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

	<!-- Current Save Data Card (informational) -->
	<Card.Root>
		<Card.Header>
			<Card.Title>{t('saveStorage.saveList')}</Card.Title>
		</Card.Header>
		<Card.Content>
			{#if loadingSaves}
				<div class="flex justify-center py-8">
					<LoaderCircle class="h-6 w-6 animate-spin text-muted-foreground" />
				</div>
			{:else if saves.length === 0}
				<p class="py-4 text-center text-muted-foreground">{t('saveStorage.noSaves')}</p>
			{:else}
				<div class="rounded-lg border">
					<Table.Root>
						<Table.Header>
							<Table.Row>
								<Table.Head>{t('saveStorage.user')}</Table.Head>
								<Table.Head>{t('saveStorage.size')}</Table.Head>
								<Table.Head>{t('saveStorage.updatedAt')}</Table.Head>
							</Table.Row>
						</Table.Header>
						<Table.Body>
							{#each saves as save (save.id)}
								<Table.Row>
									<Table.Cell>{save.userNickname || save.userId}</Table.Cell>
									<Table.Cell>{formatBytes(save.sizeBytes)}</Table.Cell>
									<Table.Cell>{new Date(save.updatedAt).toLocaleString()}</Table.Cell>
								</Table.Row>
							{/each}
						</Table.Body>
					</Table.Root>
				</div>
			{/if}
		</Card.Content>
	</Card.Root>
</div>

<!-- Restore Confirmation Dialog -->
<Dialog.Root bind:open={restoreOpen}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>{t('saveStorage.restoreConfirmTitle')}</Dialog.Title>
		</Dialog.Header>
		<div class="space-y-4 py-2">
			<p class="text-sm text-muted-foreground">
				{t('saveStorage.restoreConfirmText').replace('{date}', restoreDate)}
			</p>
			<Input placeholder="RESTORE" bind:value={restoreConfirmText} />
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
				{t('saveStorage.restore')}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<!-- Delete Confirmation Dialog -->
<Dialog.Root bind:open={deleteOpen}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>{t('saveStorage.deleteConfirmTitle')}</Dialog.Title>
		</Dialog.Header>
		<div class="space-y-4 py-2">
			<p class="text-sm text-muted-foreground">
				{t('saveStorage.deleteConfirmText').replace('{filename}', deleteFilename)}
			</p>
			<Input placeholder="DELETE" bind:value={deleteConfirmText} />
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
				{t('saveStorage.delete')}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
