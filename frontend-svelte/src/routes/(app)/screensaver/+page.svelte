<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from '$lib/i18n';
	import { screensaverAPI, tenantsAPI } from '$lib/api';
	import { toast } from 'svelte-sonner';
	import { Upload, Star, Trash2, Copy, Plus, Pencil } from '@lucide/svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Switch } from '$lib/components/ui/switch';
	import * as Card from '$lib/components/ui/card';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Select from '$lib/components/ui/select';
	import ConfirmActionDialog from '$lib/components/confirm-action-dialog.svelte';
	import { runWithToast } from '$lib/action-feedback';

	// Types
	interface Video {
		id: string;
		filename: string;
		original_name: string;
		content_type: string;
		size: number;
		hash: string;
		created_at: string;
	}

	interface TenantConfig {
		id: string;
		tenant_id: string;
		video_id: string;
		start_date: string;
		end_date: string;
		start_time: string;
		end_time: string;
		tenant?: { id: string; username: string; display_name: string };
		video?: Video;
	}

	interface TenantOption {
		id: string;
		username: string;
		display_name: string;
	}

	// Settings state
	let loading = $state(true);
	let enabled = $state(true);
	let timeout = $state(5);
	let saving = $state(false);

	// Video library state
	let videos: Video[] = $state([]);
	let defaultVideoId = $state('');
	let uploading = $state(false);
	let uploadProgress = $state(0);
	let uploadFilename = $state('');
	let fileInput: HTMLInputElement;

	// Tenant configs state
	let tenantConfigs: TenantConfig[] = $state([]);
	let tenants: TenantOption[] = $state([]);

	// Dialog state
	let dialogOpen = $state(false);
	let dialogMode: 'add' | 'edit' = $state('add');
	let editTenantId = $state('');
	let formVideoId = $state('');
	let formStartDate = $state('');
	let formEndDate = $state('');
	let formStartTime = $state('');
	let formEndTime = $state('');

	// Delete confirm state
	let deleteDialogOpen = $state(false);
	let deleteVideoId = $state('');

	// Delete tenant config confirm state
	let deleteTenantDialogOpen = $state(false);
	let deleteTenantId = $state('');

	let timeoutInvalid = $derived(timeout < 1 || timeout > 60);

	onMount(() => {
		loadAll();

		const handleKeyDown = (e: KeyboardEvent) => {
			if ((e.ctrlKey || e.metaKey) && e.key === 's') {
				e.preventDefault();
				handleSaveSettings();
			}
		};

		const handleBeforeUnload = (e: BeforeUnloadEvent) => {
			if (uploading) {
				e.preventDefault();
				e.returnValue = 'Upload in progress. Are you sure you want to leave?';
				return e.returnValue;
			}
		};

		window.addEventListener('keydown', handleKeyDown);
		window.addEventListener('beforeunload', handleBeforeUnload);

		return () => {
			window.removeEventListener('keydown', handleKeyDown);
			window.removeEventListener('beforeunload', handleBeforeUnload);
		};
	});

	async function loadAll() {
		try {
			loading = true;
			const [settingsData, videosData, configsData, tenantsData] = await Promise.all([
				screensaverAPI.getSettings(),
				screensaverAPI.listVideos(),
				screensaverAPI.listTenantConfigs(),
				tenantsAPI.getAllActive()
			]);
			enabled = settingsData?.enabled ?? true;
			timeout = settingsData?.timeout ?? 5;
			videos = videosData?.videos || [];
			defaultVideoId = videosData?.default_video_id || '';
			tenantConfigs = configsData || [];
			tenants = tenantsData || [];
		} catch (err: any) {
			toast.error(t('screensaver.loadFailed'));
		} finally {
			loading = false;
		}
	}

	// Settings handlers
	async function handleSaveSettings() {
		const timeoutValue = parseInt(String(timeout));
		if (isNaN(timeoutValue) || timeoutValue < 1 || timeoutValue > 60) {
			toast.error('Must be between 1 and 60 minutes');
			return;
		}
		try {
			saving = true;
			await screensaverAPI.updateSettings({ enabled, timeout: timeoutValue });
			toast.success(t('common.updateSuccess'));
		} catch (err: any) {
			toast.error(err.message || t('common.saveFailed'));
		} finally {
			saving = false;
		}
	}

	// Video library handlers
	function handleUploadClick() {
		fileInput?.click();
	}

	async function handleFileSelect(event: Event) {
		const input = event.target as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;
		input.value = '';

		if (!file.type.match(/^video\/(mp4|x-matroska)$/) && !file.name.match(/\.(mp4|mkv)$/i)) {
			toast.error(t('screensaver.invalidFormat'));
			return;
		}
		const maxSize = 200 * 1024 * 1024;
		if (file.size > maxSize) {
			toast.error(t('screensaver.fileTooLarge'));
			return;
		}

		try {
			uploading = true;
			uploadProgress = 0;
			uploadFilename = file.name;

			await new Promise<void>((resolve, reject) => {
				const xhr = new XMLHttpRequest();
				const token = localStorage.getItem('admin_token');

				xhr.upload.addEventListener('progress', (e) => {
					if (e.lengthComputable) {
						uploadProgress = Math.round((e.loaded * 100) / e.total);
					}
				});
				xhr.addEventListener('load', () => {
					if (xhr.status >= 200 && xhr.status < 300) {
						resolve();
					} else {
						try {
							const resp = JSON.parse(xhr.responseText);
							reject(new Error(resp.message || 'Upload failed'));
						} catch {
							reject(new Error('Upload failed'));
						}
					}
				});
				xhr.addEventListener('error', () => reject(new Error('Upload failed')));
				xhr.addEventListener('abort', () => reject(new Error('Upload cancelled')));

				const form = new FormData();
				form.append('video', file);
				xhr.open('POST', '/api/v1/admin/screensaver/videos');
				if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`);
				xhr.send(form);
			});

			toast.success('Video uploaded');
			// Reload video list
			const videosData = await screensaverAPI.listVideos();
			videos = videosData?.videos || [];
			defaultVideoId = videosData?.default_video_id || '';
		} catch (err: any) {
			toast.error(err.message || 'Upload failed');
		} finally {
			uploading = false;
			uploadProgress = 0;
			uploadFilename = '';
		}
	}

	async function handleSetDefault(videoId: string) {
		try {
			await screensaverAPI.setDefaultVideo(videoId);
			defaultVideoId = videoId;
			toast.success(t('common.updateSuccess'));
		} catch (err: any) {
			toast.error(err.message || t('common.updateFailed'));
		}
	}

	function confirmDeleteVideo(id: string) {
		deleteVideoId = id;
		deleteDialogOpen = true;
	}

	async function handleDeleteVideo() {
		const ok = await runWithToast(
			async () => {
				await screensaverAPI.deleteVideo(deleteVideoId);
				videos = videos.filter(v => v.id !== deleteVideoId);
				if (defaultVideoId === deleteVideoId) defaultVideoId = '';
			},
			{
				successMessage: t('common.deleteSuccess'),
				errorFallback: t('screensaver.deleteVideoFailed'),
			}
		);
		if (ok) {
			deleteDialogOpen = false;
		}
	}

	async function copyLink(videoId: string) {
		const url = `${window.location.origin}${screensaverAPI.getVideoFileUrl(videoId)}`;
		try {
			await navigator.clipboard.writeText(url);
			toast.success(t('screensaver.linkCopied'));
		} catch {
			// Fallback
			const ta = document.createElement('textarea');
			ta.value = url;
			document.body.appendChild(ta);
			ta.select();
			document.execCommand('copy');
			document.body.removeChild(ta);
			toast.success(t('screensaver.linkCopied'));
		}
	}

	function formatFileSize(bytes: number): string {
		if (bytes < 1024) return bytes + ' B';
		if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
		return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
	}

	function formatDate(dateStr: string): string {
		if (!dateStr) return '';
		return new Date(dateStr).toLocaleDateString();
	}

	// Tenant config handlers
	function openAddDialog() {
		dialogMode = 'add';
		editTenantId = '';
		formVideoId = '';
		formStartDate = '';
		formEndDate = '';
		formStartTime = '';
		formEndTime = '';
		dialogOpen = true;
	}

	function openEditDialog(config: TenantConfig) {
		dialogMode = 'edit';
		editTenantId = config.tenant_id;
		formVideoId = config.video_id;
		formStartDate = config.start_date || '';
		formEndDate = config.end_date || '';
		formStartTime = config.start_time || '';
		formEndTime = config.end_time || '';
		dialogOpen = true;
	}

	async function handleSaveTenantConfig() {
		if (!editTenantId || !formVideoId) {
			toast.error('Please select tenant and video');
			return;
		}
		try {
			await screensaverAPI.setTenantConfig(editTenantId, {
				video_id: formVideoId,
				start_date: formStartDate,
				end_date: formEndDate,
				start_time: formStartTime,
				end_time: formEndTime
			});
			dialogOpen = false;
			toast.success(t('common.updateSuccess'));
			const configsData = await screensaverAPI.listTenantConfigs();
			tenantConfigs = configsData || [];
		} catch (err: any) {
			toast.error(err.message || t('common.saveFailed'));
		}
	}

	function confirmDeleteTenantConfig(tenantId: string) {
		deleteTenantId = tenantId;
		deleteTenantDialogOpen = true;
	}

	async function handleDeleteTenantConfig() {
		const ok = await runWithToast(
			async () => {
				await screensaverAPI.deleteTenantConfig(deleteTenantId);
				tenantConfigs = tenantConfigs.filter(c => c.tenant_id !== deleteTenantId);
			},
			{
				successMessage: t('common.deleteSuccess'),
				errorFallback: t('common.deleteFailed'),
			}
		);
		if (ok) {
			deleteTenantDialogOpen = false;
		}
	}

	// Available tenants for add dialog (exclude already configured ones)
	let availableTenants = $derived(
		dialogMode === 'add'
			? tenants.filter(t => !tenantConfigs.some(c => c.tenant_id === t.id))
			: tenants
	);

	function formatSchedule(config: TenantConfig): string {
		const parts: string[] = [];
		if (config.start_date || config.end_date) {
			parts.push(`${config.start_date || '...'} ~ ${config.end_date || '...'}`);
		}
		if (config.start_time || config.end_time) {
			parts.push(`${config.start_time || '00:00'} - ${config.end_time || '23:59'}`);
		}
		return parts.length > 0 ? parts.join(', ') : t('screensaver.alwaysActive');
	}
</script>

{#if loading}
	<div class="flex h-[60vh] items-center justify-center">
		<div class="text-muted-foreground">{t('common.loading')}</div>
	</div>
{:else}
	<div class="space-y-6">
		<h2 class="text-2xl font-bold tracking-tight">{t('screensaver.title')}</h2>

		<!-- Info Alert -->
		<div class="rounded-lg border bg-blue-50 p-4 text-sm text-blue-800 dark:bg-blue-950 dark:text-blue-200">
			{t('screensaver.info')}
		</div>

		<!-- Tips -->
		<div class="rounded-lg border bg-amber-50 p-4 text-sm text-amber-800 dark:bg-amber-950 dark:text-amber-200">
			{t('screensaver.tips')}
		</div>

		<!-- Section 1: Settings Card -->
		<Card.Root>
			<Card.Header>
				<Card.Title>{t('screensaver.settings')}</Card.Title>
			</Card.Header>
			<Card.Content class="space-y-6">
				<div class="flex items-center space-x-3">
					<Switch bind:checked={enabled} />
					<Label>{t('screensaver.enableScreensaver')}</Label>
				</div>
				<div class="max-w-[300px] space-y-2">
					<Label for="idle-timeout">{t('screensaver.idleTimeout')}</Label>
					<div class="flex items-center gap-2">
						<Input id="idle-timeout" type="number" bind:value={timeout} min={1} max={60} step={1} disabled={!enabled} class={timeoutInvalid ? 'border-destructive' : ''} />
						<span class="text-sm text-muted-foreground whitespace-nowrap">{t('screensaver.minutes')}</span>
					</div>
					<p class="text-sm text-muted-foreground">{t('screensaver.timeoutHelp')}</p>
				</div>
				<div>
					<Button onclick={handleSaveSettings} disabled={timeoutInvalid || saving}>
						{saving ? t('common.saving') : t('screensaver.saveSettings')}
					</Button>
				</div>
			</Card.Content>
		</Card.Root>

		<!-- Section 2: Video Library Card -->
		<Card.Root>
			<Card.Header class="flex flex-row items-center justify-between">
				<Card.Title>{t('screensaver.videoLibrary')}</Card.Title>
				<div>
					<input bind:this={fileInput} type="file" accept=".mp4,.mkv,video/mp4,video/x-matroska" class="hidden" onchange={handleFileSelect} disabled={uploading} />
					<Button onclick={handleUploadClick} disabled={uploading} size="sm">
						<Upload class="mr-2 h-4 w-4" />
						{t('screensaver.uploadToLibrary')}
					</Button>
				</div>
			</Card.Header>
			<Card.Content>
				<!-- Upload Progress -->
				{#if uploading}
					<div class="mb-4 space-y-2">
						<p class="text-sm">{uploadFilename} - {uploadProgress}%</p>
						<div class="h-2 w-full overflow-hidden rounded-full bg-muted">
							<div class="h-full rounded-full bg-primary transition-all duration-300" style="width: {uploadProgress}%"></div>
						</div>
					</div>
				{/if}

				{#if videos.length === 0}
					<p class="text-sm text-muted-foreground py-8 text-center">{t('screensaver.noVideos')}</p>
				{:else}
					<div class="overflow-x-auto">
						<table class="w-full text-sm">
							<thead>
								<tr class="border-b">
									<th class="py-2 px-3 text-left font-medium">{t('screensaver.fileName')}</th>
									<th class="py-2 px-3 text-left font-medium">{t('screensaver.fileSize')}</th>
									<th class="py-2 px-3 text-left font-medium">{t('screensaver.uploadDate')}</th>
									<th class="py-2 px-3 text-left font-medium">{t('screensaver.link')}</th>
									<th class="py-2 px-3 text-right font-medium">{t('common.actions')}</th>
								</tr>
							</thead>
							<tbody>
								{#each videos as video (video.id)}
									<tr class="border-b hover:bg-muted/50">
										<td class="py-2 px-3">
											<div class="flex items-center gap-2">
												<span class="truncate max-w-[200px]">{video.original_name}</span>
												{#if video.id === defaultVideoId}
													<span class="inline-flex items-center rounded-full bg-yellow-100 px-2 py-0.5 text-xs font-medium text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200">
														<Star class="mr-1 h-3 w-3" />{t('screensaver.isDefault')}
													</span>
												{/if}
											</div>
										</td>
										<td class="py-2 px-3 text-muted-foreground">{formatFileSize(video.size)}</td>
										<td class="py-2 px-3 text-muted-foreground">{formatDate(video.created_at)}</td>
										<td class="py-2 px-3">
											<Button variant="ghost" size="sm" onclick={() => copyLink(video.id)}>
												<Copy class="mr-1 h-3 w-3" />{t('screensaver.copyLink')}
											</Button>
										</td>
										<td class="py-2 px-3 text-right">
											<div class="flex items-center justify-end gap-1">
												{#if video.id !== defaultVideoId}
													<Button variant="ghost" size="sm" onclick={() => handleSetDefault(video.id)} title={t('screensaver.setAsDefault')}>
														<Star class="h-4 w-4" />
													</Button>
												{/if}
												<Button variant="ghost" size="sm" onclick={() => confirmDeleteVideo(video.id)} title={t('common.delete')}>
													<Trash2 class="h-4 w-4 text-destructive" />
												</Button>
											</div>
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{/if}
				<p class="mt-2 text-xs text-muted-foreground">{t('screensaver.videoHint')}</p>
			</Card.Content>
		</Card.Root>

		<!-- Section 3: Tenant Configs Card -->
		<Card.Root>
			<Card.Header class="flex flex-row items-center justify-between">
				<Card.Title>{t('screensaver.tenantConfigs')}</Card.Title>
				<Button onclick={openAddDialog} size="sm" disabled={videos.length === 0}>
					<Plus class="mr-2 h-4 w-4" />
					{t('screensaver.addTenantConfig')}
				</Button>
			</Card.Header>
			<Card.Content>
				{#if tenantConfigs.length === 0}
					<p class="text-sm text-muted-foreground py-8 text-center">{t('screensaver.noTenantConfigs')}</p>
				{:else}
					<div class="overflow-x-auto">
						<table class="w-full text-sm">
							<thead>
								<tr class="border-b">
									<th class="py-2 px-3 text-left font-medium">{t('screensaver.tenant')}</th>
									<th class="py-2 px-3 text-left font-medium">{t('screensaver.videoName')}</th>
									<th class="py-2 px-3 text-left font-medium">{t('screensaver.schedule')}</th>
									<th class="py-2 px-3 text-right font-medium">{t('common.actions')}</th>
								</tr>
							</thead>
							<tbody>
								{#each tenantConfigs as config (config.id)}
									<tr class="border-b hover:bg-muted/50">
										<td class="py-2 px-3">{config.tenant?.display_name || config.tenant?.username || config.tenant_id}</td>
										<td class="py-2 px-3">{config.video?.original_name || config.video_id}</td>
										<td class="py-2 px-3 text-muted-foreground">{formatSchedule(config)}</td>
										<td class="py-2 px-3 text-right">
											<div class="flex items-center justify-end gap-1">
												<Button variant="ghost" size="sm" onclick={() => openEditDialog(config)}>
													<Pencil class="h-4 w-4" />
												</Button>
												<Button variant="ghost" size="sm" onclick={() => confirmDeleteTenantConfig(config.tenant_id)}>
													<Trash2 class="h-4 w-4 text-destructive" />
												</Button>
											</div>
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{/if}
			</Card.Content>
		</Card.Root>
	</div>

	<!-- Delete Video Dialog -->
	<ConfirmActionDialog
		bind:open={deleteDialogOpen}
		title={t('screensaver.deleteVideo')}
		description={t('screensaver.deleteVideoConfirm')}
		confirmLabel={t('common.delete')}
		confirmVariant="destructive"
		onConfirm={handleDeleteVideo}
	/>

	<!-- Delete Tenant Config Dialog -->
	<ConfirmActionDialog
		bind:open={deleteTenantDialogOpen}
		title={t('common.confirmDelete')}
		confirmLabel={t('common.delete')}
		confirmVariant="destructive"
		onConfirm={handleDeleteTenantConfig}
	/>

	<!-- Add/Edit Tenant Config Dialog -->
	<Dialog.Root bind:open={dialogOpen}>
		<Dialog.Content class="max-w-md">
			<Dialog.Header>
				<Dialog.Title>
					{dialogMode === 'add' ? t('screensaver.addTenantConfig') : t('screensaver.editTenantConfig')}
				</Dialog.Title>
			</Dialog.Header>
			<div class="space-y-4 py-4">
				<!-- Tenant select -->
				{#if dialogMode === 'add'}
					<div class="space-y-2">
						<Label>{t('screensaver.selectTenant')}</Label>
						<select bind:value={editTenantId} class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm">
							<option value="">{t('screensaver.selectTenant')}</option>
							{#each availableTenants as tenant (tenant.id)}
								<option value={tenant.id}>{tenant.display_name || tenant.username}</option>
							{/each}
						</select>
					</div>
				{:else}
					<div class="space-y-2">
						<Label>{t('screensaver.tenant')}</Label>
						<p class="text-sm text-muted-foreground">
							{tenants.find(t => t.id === editTenantId)?.display_name || tenants.find(t => t.id === editTenantId)?.username || editTenantId}
						</p>
					</div>
				{/if}

				<!-- Video select -->
				<div class="space-y-2">
					<Label>{t('screensaver.selectVideo')}</Label>
					<select bind:value={formVideoId} class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm">
						<option value="">{t('screensaver.selectVideo')}</option>
						{#each videos as video (video.id)}
							<option value={video.id}>{video.original_name}</option>
						{/each}
					</select>
				</div>

				<!-- Date range -->
				<div class="space-y-2">
					<Label>{t('screensaver.dateRange')}</Label>
					<div class="grid grid-cols-2 gap-2">
						<Input type="date" bind:value={formStartDate} placeholder={t('screensaver.startDate')} />
						<Input type="date" bind:value={formEndDate} placeholder={t('screensaver.endDate')} />
					</div>
					<p class="text-xs text-muted-foreground">{t('screensaver.alwaysActive')}</p>
				</div>

				<!-- Time range -->
				<div class="space-y-2">
					<Label>{t('screensaver.timeRange')}</Label>
					<div class="grid grid-cols-2 gap-2">
						<Input type="time" bind:value={formStartTime} placeholder={t('screensaver.startTime')} />
						<Input type="time" bind:value={formEndTime} placeholder={t('screensaver.endTime')} />
					</div>
					<p class="text-xs text-muted-foreground">{t('screensaver.allDay')}</p>
				</div>
			</div>
			<Dialog.Footer>
				<Button variant="outline" onclick={() => (dialogOpen = false)}>{t('common.cancel')}</Button>
				<Button onclick={handleSaveTenantConfig} disabled={!editTenantId || !formVideoId}>
					{t('common.save')}
				</Button>
			</Dialog.Footer>
		</Dialog.Content>
	</Dialog.Root>
{/if}
