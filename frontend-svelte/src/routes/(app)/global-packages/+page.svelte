<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from '$lib/i18n';
	import { globalPackagesAPI } from '$lib/api';
	import { toast } from 'svelte-sonner';
	import { Plus, Pencil, Trash2, RotateCcw, LoaderCircle } from '@lucide/svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Badge } from '$lib/components/ui/badge';
	import { Switch } from '$lib/components/ui/switch';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Table from '$lib/components/ui/table';
	import ConfirmActionDialog from '$lib/components/confirm-action-dialog.svelte';
	import SelectTablePagination from '$lib/components/select-table-pagination.svelte';
	import { runWithToast } from '$lib/action-feedback';

	interface Package {
		id: number;
		name: string;
		price_cents: number;
		real_cents: number;
		bonus_cents: number;
		sort_order: number;
		status: number;
		created_at: string;
		deleted_at?: string;
	}

	// Package list
	let packages: Package[] = $state([]);
	let total = $state(0);
	let loading = $state(true);

	// Pagination
	let page = $state(0);
	let rowsPerPage = $state(10);

	// Show deleted
	let showDeleted = $state(false);
	let deletedPackages: Package[] = $state([]);

	// Create/Edit dialog
	let dialogOpen = $state(false);
	let dialogMode: 'create' | 'edit' = $state('create');
	let editPackage: Package | null = $state(null);

	// Form state (yuan for display, converted to cents for API)
	let formName = $state('');
	let formPriceYuan = $state('10.00');
	let formRealAmountYuan = $state('10.00');
	let formBonusYuan = $state('0.00');
	let formSortOrder = $state(100);
	let formStatus = $state(1);

	// Delete confirm dialog
	let deleteDialogOpen = $state(false);
	let deletePackage: Package | null = $state(null);

	// Permanent delete confirm dialog
	let permanentDeleteDialogOpen = $state(false);
	let permanentDeletePackage: Package | null = $state(null);

	onMount(() => {
		loadPackages();
	});

	$effect(() => {
		page;
		rowsPerPage;
		loadPackages();
	});

	async function loadPackages() {
		try {
			loading = true;
			const data = await globalPackagesAPI.getAll({ page: page + 1, limit: rowsPerPage });
			packages = data?.packages || [];
			total = data?.total || 0;
		} catch (error: any) {
			toast.error(error.message || t('common.loadFailed'));
		} finally {
			loading = false;
		}
	}

	async function loadDeletedPackages() {
		try {
			const data = await globalPackagesAPI.getDeleted();
			deletedPackages = data || [];
		} catch (error: any) {
			toast.error(error.message || t('common.loadFailed'));
		}
	}

	function openCreateDialog() {
		dialogMode = 'create';
		editPackage = null;
		formName = '';
		formPriceYuan = '10.00';
		formRealAmountYuan = '10.00';
		formBonusYuan = '0.00';
		formSortOrder = 100;
		formStatus = 1;
		dialogOpen = true;
	}

	function openEditDialog(pkg: Package) {
		dialogMode = 'edit';
		editPackage = pkg;
		formName = pkg.name;
		formPriceYuan = ((pkg.price_cents || 0) / 100).toFixed(2);
		formRealAmountYuan = ((pkg.real_cents || 0) / 100).toFixed(2);
		formBonusYuan = ((pkg.bonus_cents || 0) / 100).toFixed(2);
		formSortOrder = pkg.sort_order ?? 100;
		formStatus = pkg.status ?? 1;
		dialogOpen = true;
	}

	function closeDialog() {
		dialogOpen = false;
		editPackage = null;
		formName = '';
		formPriceYuan = '10.00';
		formRealAmountYuan = '10.00';
		formBonusYuan = '0.00';
		formSortOrder = 100;
		formStatus = 1;
	}

	async function handleSubmit() {
		if (!formName.trim()) {
			toast.error(t('globalPkg.nameRequired'));
			return;
		}

		try {
			const apiData = {
				name: formName.trim(),
				price_cents: Math.round(parseFloat(formPriceYuan) * 100),
				real_cents: Math.round(parseFloat(formRealAmountYuan) * 100),
				bonus_cents: Math.round(parseFloat(formBonusYuan) * 100),
				sort_order: formSortOrder,
				status: formStatus
			};

			if (dialogMode === 'create') {
				await globalPackagesAPI.create(apiData);
				toast.success(t('common.createSuccess'));
			} else {
				await globalPackagesAPI.update(editPackage!.id, apiData);
				toast.success(t('common.updateSuccess'));
			}
			closeDialog();
			loadPackages();
		} catch (error: any) {
			toast.error(
				error.message ||
					(dialogMode === 'create' ? t('common.createFailed') : t('common.updateFailed'))
			);
		}
	}

	async function handleToggleStatus(pkg: Package) {
		try {
			const newStatus = pkg.status === 1 ? 0 : 1;
			await globalPackagesAPI.updateStatus(pkg.id, { status: newStatus });
			toast.success(newStatus === 1 ? t('globalPkg.activated') : t('globalPkg.deactivated'));
			loadPackages();
		} catch (error: any) {
			toast.error(error.message || t('common.updateFailed'));
		}
	}

	function openDeleteDialog(pkg: Package) {
		deletePackage = pkg;
		deleteDialogOpen = true;
	}

	async function handleDelete() {
		if (!deletePackage) return;
		const ok = await runWithToast(() => globalPackagesAPI.delete(deletePackage!.id), {
			successMessage: t('common.deleteSuccess'),
			errorFallback: t('common.deleteFailed'),
		});
		if (ok) {
			deleteDialogOpen = false;
			deletePackage = null;
			loadPackages();
		}
	}

	async function handleRecover(id: number) {
		try {
			await globalPackagesAPI.recover(id);
			toast.success(t('common.recoverSuccess'));
			loadDeletedPackages();
			loadPackages();
		} catch (error: any) {
			toast.error(error.message || t('common.recoverFailed'));
		}
	}

	function openPermanentDeleteDialog(pkg: Package) {
		permanentDeletePackage = pkg;
		permanentDeleteDialogOpen = true;
	}

	async function handlePermanentDelete() {
		if (!permanentDeletePackage) return;
		const ok = await runWithToast(
			() => globalPackagesAPI.permanentDelete(permanentDeletePackage!.id),
			{
				successMessage: t('common.deleteSuccess'),
				errorFallback: t('common.deleteFailed'),
			}
		);
		if (ok) {
			permanentDeleteDialogOpen = false;
			permanentDeletePackage = null;
			loadDeletedPackages();
		}
	}

	function handleShowDeletedToggle(checked: boolean) {
		showDeleted = checked;
		if (checked) loadDeletedPackages();
	}

	function formatCents(cents: number): string {
		return `\u00A5${((cents || 0) / 100).toFixed(2)}`;
	}

	function formatDate(dateStr: string | null | undefined): string {
		if (!dateStr) return '-';
		return new Date(dateStr).toLocaleString('zh-CN');
	}
</script>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<h2 class="text-2xl font-bold tracking-tight">{t('globalPkg.title')}</h2>
		<div class="flex items-center gap-4">
			<div class="flex items-center gap-2">
				<Switch
					checked={showDeleted}
					onCheckedChange={handleShowDeletedToggle}
					size="sm"
				/>
				<span class="text-sm text-muted-foreground">{t('common.showDeleted')}</span>
			</div>
			<Button onclick={openCreateDialog}>
				<Plus class="mr-2 h-4 w-4" />
				{t('globalPkg.add')}
			</Button>
		</div>
	</div>

	<!-- Active Packages Table -->
	<div class="rounded-lg border">
		<Table.Root>
			<Table.Header>
				<Table.Row>
					<Table.Head>{t('globalPkg.id')}</Table.Head>
					<Table.Head>{t('common.name')}</Table.Head>
					<Table.Head>{t('globalPkg.price')}</Table.Head>
					<Table.Head>{t('globalPkg.realAmount')}</Table.Head>
					<Table.Head>{t('globalPkg.bonus')}</Table.Head>
					<Table.Head>{t('common.sortOrder')}</Table.Head>
					<Table.Head>{t('common.status')}</Table.Head>
					<Table.Head>{t('common.createdAt')}</Table.Head>
					<Table.Head class="text-right">{t('common.actions')}</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#if loading}
					<Table.Row>
						<Table.Cell colspan={9} class="py-8 text-center">
							<LoaderCircle class="mx-auto h-6 w-6 animate-spin text-muted-foreground" />
						</Table.Cell>
					</Table.Row>
				{:else if packages.length === 0}
					<Table.Row>
						<Table.Cell colspan={9} class="py-8 text-center text-muted-foreground">
							{t('globalPkg.noPackages')}
						</Table.Cell>
					</Table.Row>
				{:else}
					{#each packages as pkg (pkg.id)}
						<Table.Row>
							<Table.Cell>{pkg.id}</Table.Cell>
							<Table.Cell>{pkg.name}</Table.Cell>
							<Table.Cell>{formatCents(pkg.price_cents)}</Table.Cell>
							<Table.Cell>{formatCents(pkg.real_cents)}</Table.Cell>
							<Table.Cell>{formatCents(pkg.bonus_cents)}</Table.Cell>
							<Table.Cell>{pkg.sort_order}</Table.Cell>
							<Table.Cell>
								<div class="flex items-center gap-2">
									<Badge variant={pkg.status === 1 ? 'default' : 'secondary'}>
										{pkg.status === 1 ? t('common.active') : t('common.inactive')}
									</Badge>
									<Switch
										checked={pkg.status === 1}
										onCheckedChange={() => handleToggleStatus(pkg)}
										size="sm"
									/>
								</div>
							</Table.Cell>
							<Table.Cell class="text-sm">{formatDate(pkg.created_at)}</Table.Cell>
							<Table.Cell class="text-right">
								<Button
									variant="ghost"
									size="icon"
									onclick={() => openEditDialog(pkg)}
								>
									<Pencil class="h-4 w-4" />
								</Button>
								{#if pkg.status === 0}
									<Button
										variant="ghost"
										size="icon"
										onclick={() => openDeleteDialog(pkg)}
									>
										<Trash2 class="h-4 w-4 text-destructive" />
									</Button>
								{/if}
							</Table.Cell>
						</Table.Row>
					{/each}
				{/if}
			</Table.Body>
		</Table.Root>

		<!-- Pagination -->
		<SelectTablePagination bind:page bind:rowsPerPage {total} />
	</div>

	<!-- Deleted Packages Section -->
	{#if showDeleted}
		<div class="space-y-2">
			<h3 class="text-lg font-semibold">
				{t('common.deleted')} {t('globalPkg.title')}
			</h3>
			<div class="rounded-lg border bg-muted/30">
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<Table.Head>{t('common.name')}</Table.Head>
							<Table.Head>{t('globalPkg.price')}</Table.Head>
							<Table.Head>{t('globalPkg.realAmount')}</Table.Head>
							<Table.Head>{t('globalPkg.bonus')}</Table.Head>
							<Table.Head>{t('common.deletedAt')}</Table.Head>
							<Table.Head class="text-right">{t('common.actions')}</Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#if deletedPackages.length === 0}
							<Table.Row>
								<Table.Cell colspan={6} class="py-8 text-center text-muted-foreground">
									{t('common.noData')}
								</Table.Cell>
							</Table.Row>
						{:else}
							{#each deletedPackages as pkg (pkg.id)}
								<Table.Row>
									<Table.Cell>{pkg.name}</Table.Cell>
									<Table.Cell>{formatCents(pkg.price_cents)}</Table.Cell>
									<Table.Cell>{formatCents(pkg.real_cents)}</Table.Cell>
									<Table.Cell>{formatCents(pkg.bonus_cents)}</Table.Cell>
									<Table.Cell class="text-sm">
										{pkg.deleted_at ? new Date(pkg.deleted_at).toLocaleString('zh-CN') : '-'}
									</Table.Cell>
									<Table.Cell class="text-right">
										<Button
											variant="ghost"
											size="icon"
											onclick={() => handleRecover(pkg.id)}
										>
											<RotateCcw class="h-4 w-4 text-green-600" />
										</Button>
										<Button
											variant="ghost"
											size="icon"
											onclick={() => openPermanentDeleteDialog(pkg)}
										>
											<Trash2 class="h-4 w-4 text-destructive" />
										</Button>
									</Table.Cell>
								</Table.Row>
							{/each}
						{/if}
					</Table.Body>
				</Table.Root>
			</div>
		</div>
	{/if}
</div>

<!-- Create/Edit Package Dialog -->
<Dialog.Root bind:open={dialogOpen}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>
				{dialogMode === 'create' ? t('globalPkg.addPackage') : t('globalPkg.editPackage')}
			</Dialog.Title>
			<Dialog.Description>
				{dialogMode === 'create' ? t('globalPkg.addPackage') : t('globalPkg.editPackage')}
			</Dialog.Description>
		</Dialog.Header>
		<div class="space-y-4 py-4">
			<div class="space-y-2">
				<Label for="pkg-name">{t('globalPkg.packageName')}</Label>
				<Input
					id="pkg-name"
					bind:value={formName}
					placeholder={t('globalPkg.packageNamePlaceholder')}
					autofocus
				/>
			</div>
			<div class="space-y-2">
				<Label for="pkg-price">{t('globalPkg.priceYuan')}</Label>
				<Input
					id="pkg-price"
					type="number"
					step="0.01"
					min="0"
					bind:value={formPriceYuan}
				/>
				<p class="text-sm text-muted-foreground">{t('globalPkg.priceHelp')}</p>
			</div>
			<div class="space-y-2">
				<Label for="pkg-real-amount">{t('globalPkg.realAmountYuan')}</Label>
				<Input
					id="pkg-real-amount"
					type="number"
					step="0.01"
					min="0"
					bind:value={formRealAmountYuan}
				/>
				<p class="text-sm text-muted-foreground">{t('globalPkg.realAmountHelp')}</p>
			</div>
			<div class="space-y-2">
				<Label for="pkg-bonus">{t('globalPkg.bonusYuan')}</Label>
				<Input
					id="pkg-bonus"
					type="number"
					step="0.01"
					min="0"
					bind:value={formBonusYuan}
				/>
				<p class="text-sm text-muted-foreground">{t('globalPkg.bonusHelp')}</p>
			</div>
			<div class="space-y-2">
				<Label for="pkg-sort-order">{t('common.sortOrder')}</Label>
				<Input
					id="pkg-sort-order"
					type="number"
					min="0"
					bind:value={formSortOrder}
				/>
				<p class="text-sm text-muted-foreground">{t('globalPkg.sortOrderHelp')}</p>
			</div>
			<div class="flex items-center gap-2">
				<Switch
					checked={formStatus === 1}
					onCheckedChange={(checked) => (formStatus = checked ? 1 : 0)}
				/>
				<Label>{t('common.active')}</Label>
			</div>
		</div>
		<Dialog.Footer>
			<Button variant="outline" onclick={closeDialog}>{t('common.cancel')}</Button>
			<Button onclick={handleSubmit} disabled={!formName.trim()}>
				{dialogMode === 'create' ? t('common.create') : t('common.save')}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<!-- Delete Confirm Dialog -->
<ConfirmActionDialog
	bind:open={deleteDialogOpen}
	title={t('common.confirmDelete')}
	description={`${t('common.confirmDelete')}?`}
	confirmLabel={t('common.delete')}
	confirmVariant="destructive"
	onConfirm={handleDelete}
	onCancel={() => (deletePackage = null)}
/>

<!-- Permanent Delete Confirm Dialog -->
<ConfirmActionDialog
	bind:open={permanentDeleteDialogOpen}
	title={t('common.confirmDelete')}
	description={t('globalPkg.permanentDeleteWarning')}
	confirmLabel={t('common.delete')}
	confirmVariant="destructive"
	onConfirm={handlePermanentDelete}
	onCancel={() => (permanentDeletePackage = null)}
/>
