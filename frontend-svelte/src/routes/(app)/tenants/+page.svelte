<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from '$lib/i18n';
	import { tenantsAPI, planSetsAPI } from '$lib/api';
	import { toast } from 'svelte-sonner';
	import { Plus, Pencil, Trash2, KeyRound, Search } from '@lucide/svelte';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Table from '$lib/components/ui/table';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Switch } from '$lib/components/ui/switch';
	import ConfirmActionDialog from '$lib/components/confirm-action-dialog.svelte';
	import TablePagination from '$lib/components/table-pagination.svelte';
	import { runWithToast } from '$lib/action-feedback';


	// --- Tenant list state ---
	let tenants: any[] = $state([]);
	let total = $state(0);
	let loading = $state(true);
	let page = $state(0);
	let rowsPerPage = $state(10);
	let searchQuery = $state('');
	let searchDebounce = $state('');
	let activePlanSets: any[] = $state([]);

	// --- Create/Edit/Password dialog state ---
	let dialogOpen = $state(false);
	let dialogMode: 'add' | 'edit' | 'password' = $state('add');
	let currentTenant: any = $state(null);
	let formUsername = $state('');
	let formPassword = $state('');
	let formDisplayName = $state('');
	let formPhone = $state('');
	let formErrors: Record<string, string> = $state({});

	// --- Delete confirmation dialog ---
	let deleteDialogOpen = $state(false);
	let tenantToDelete: any = $state(null);


	// --- Debounce search (300ms) ---
	let debounceTimer: ReturnType<typeof setTimeout> | undefined;
	$effect(() => {
		searchQuery;
		clearTimeout(debounceTimer);
		debounceTimer = setTimeout(() => {
			searchDebounce = searchQuery;
		}, 300);
		return () => clearTimeout(debounceTimer);
	});

	// --- Reload when search/page/rowsPerPage change ---
	$effect(() => {
		searchDebounce;
		page;
		rowsPerPage;
		loadTenants();
	});

	onMount(() => {
		loadActivePlanSets();
	});

	// --- Data loading ---
	async function loadTenants() {
		try {
			loading = true;
			const params: Record<string, any> = {
				page: page + 1,
				limit: rowsPerPage
			};
			if (searchDebounce.trim()) {
				params.search = searchDebounce.trim();
			}
			const data = await tenantsAPI.getAll(params);
			tenants = data.tenants || [];
			total = data.total || 0;
		} catch (error: any) {
			console.error('Failed to load tenants:', error);
			toast.error(error.message || t('common.loadFailed'));
		} finally {
			loading = false;
		}
	}

	async function loadActivePlanSets() {
		try {
			const data = await planSetsAPI.getActive();
			activePlanSets = data || [];
		} catch (error) {
			console.error('Failed to load active plan sets:', error);
		}
	}

	// --- Add / Edit / Password dialog ---
	function handleAddClick() {
		dialogMode = 'add';
		currentTenant = null;
		formUsername = '';
		formPassword = '';
		formDisplayName = '';
		formPhone = '';
		formErrors = {};
		dialogOpen = true;
	}

	function handleEditClick(tenant: any) {
		dialogMode = 'edit';
		currentTenant = tenant;
		formUsername = tenant.username || '';
		formPassword = '';
		formDisplayName = tenant.display_name || '';
		formPhone = tenant.phone || '';
		formErrors = {};
		dialogOpen = true;
	}

	function handlePasswordClick(tenant: any) {
		dialogMode = 'password';
		currentTenant = tenant;
		formUsername = '';
		formPassword = '';
		formDisplayName = '';
		formPhone = '';
		formErrors = {};
		dialogOpen = true;
	}

	function handleDialogClose() {
		dialogOpen = false;
		currentTenant = null;
		formUsername = '';
		formPassword = '';
		formDisplayName = '';
		formPhone = '';
		formErrors = {};
	}

	function validateForm(): boolean {
		const errors: Record<string, string> = {};

		if (dialogMode === 'add') {
			if (!formUsername || formUsername.trim() === '') {
				errors.username = t('tenants.usernameRequired');
			}
			if (!formPassword || formPassword.trim() === '') {
				errors.password = t('tenants.passwordRequired');
			}
		} else if (dialogMode === 'password') {
			if (!formPassword || formPassword.trim() === '') {
				errors.password = t('tenants.passwordRequired');
			}
		}

		formErrors = errors;
		return Object.keys(errors).length === 0;
	}

	async function handleSave() {
		if (!validateForm()) return;

		try {
			if (dialogMode === 'edit') {
				await tenantsAPI.update(currentTenant.id, {
					display_name: formDisplayName.trim(),
					phone: formPhone.trim()
				});
				toast.success(t('common.updateSuccess'));
			} else if (dialogMode === 'password') {
				await tenantsAPI.updatePassword(currentTenant.id, {
					password: formPassword.trim()
				});
				toast.success(t('common.updateSuccess'));
			} else {
				await tenantsAPI.create({
					username: formUsername.trim(),
					password: formPassword.trim(),
					display_name: formDisplayName.trim(),
					phone: formPhone.trim()
				});
				toast.success(t('common.createSuccess'));
			}
			handleDialogClose();
			loadTenants();
		} catch (error: any) {
			console.error('Failed to save tenant:', error);
			toast.error(error.message || t('common.saveFailed'));
		}
	}

	// --- Delete ---
	function handleDeleteClick(tenant: any) {
		tenantToDelete = tenant;
		deleteDialogOpen = true;
	}

	async function handleDelete() {
		if (!tenantToDelete) return;
		const ok = await runWithToast(() => tenantsAPI.delete(tenantToDelete.id), {
			successMessage: t('common.deleteSuccess'),
			errorFallback: t('common.deleteFailed'),
			logLabel: 'Failed to delete tenant:',
		});
		if (ok) {
			deleteDialogOpen = false;
			tenantToDelete = null;
			loadTenants();
		}
	}

	// --- Status toggle ---
	async function handleStatusToggle(tenant: any) {
		try {
			const newStatus = tenant.status === 1 ? 0 : 1;
			await tenantsAPI.updateStatus(tenant.id, { status: newStatus });
			toast.success(t('common.updateSuccess'));
			loadTenants();
		} catch (error: any) {
			console.error('Failed to update tenant status:', error);
			toast.error(error.message || t('common.updateFailed'));
		}
	}

	// --- Plan set change ---
	async function handlePlanSetChange(tenantId: string, planSetId: string) {
		try {
			await tenantsAPI.updateBilling(tenantId, {
				plan_set_id: planSetId || ''
			});
			tenants = tenants.map(t =>
				t.id === tenantId ? { ...t, plan_set_id: planSetId || null } : t
			);
			toast.success(t('common.updateSuccess'));
		} catch (error: any) {
			console.error('Failed to update plan set:', error);
			toast.error(error.message || t('common.updateFailed'));
			loadTenants();
		}
	}

	// --- Global enabled toggle ---
	async function handleGlobalEnabledChange(tenantId: string, enabled: boolean) {
		try {
			await tenantsAPI.updateBilling(tenantId, { global_enabled: enabled ? 1 : 0 });
			toast.success(t('common.updateSuccess'));
			loadTenants();
		} catch (error: any) {
			console.error('Failed to update global billing:', error);
			toast.error(error.message || t('common.updateFailed'));
		}
	}

	// --- Free play toggle ---
	async function handleFreePlayChange(tenantId: string, enabled: boolean) {
		try {
			await tenantsAPI.updateBilling(tenantId, { free_play: enabled ? 1 : 0 });
			toast.success(t('common.updateSuccess'));
			loadTenants();
		} catch (error: any) {
			console.error('Failed to update free play:', error);
			toast.error(error.message || t('common.updateFailed'));
		}
	}

	// --- Helpers ---
	function formatDateTime(dateStr: string | null | undefined): string {
		if (!dateStr) return '\u2014';
		return new Date(dateStr).toLocaleString();
	}
</script>

<!-- Header -->
<div class="flex items-center justify-between mb-6">
	<h2 class="text-2xl font-semibold">{t('tenants.title')}</h2>
	<Button onclick={handleAddClick}>
		<Plus class="h-4 w-4 mr-2" />
		{t('tenants.addTenant')}
	</Button>
</div>

<!-- Search Bar -->
<div class="relative mb-4">
	<Search class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
	<Input
		placeholder={t('tenants.searchPlaceholder')}
		bind:value={searchQuery}
		class="pl-9"
	/>
</div>

<!-- Tenants Table -->
<div class="rounded-lg border">
	<Table.Root>
		<Table.Header>
			<Table.Row>
				<Table.Head>{t('tenants.username')}</Table.Head>
				<Table.Head>{t('tenants.displayName')}</Table.Head>
				<Table.Head>{t('tenants.phone')}</Table.Head>
				<Table.Head>{t('tenants.planSet')}</Table.Head>
				<Table.Head>{t('tenants.globalEnabled')}</Table.Head>
				<Table.Head>{t('tenants.freePlay')}</Table.Head>
				<Table.Head>{t('common.status')}</Table.Head>
				<Table.Head>{t('common.createdAt')}</Table.Head>
				<Table.Head class="text-right">{t('common.actions')}</Table.Head>
			</Table.Row>
		</Table.Header>
		<Table.Body>
			{#if loading}
				<Table.Row>
					<Table.Cell colspan={9} class="text-center py-8 text-muted-foreground">
						{t('common.loading')}
					</Table.Cell>
				</Table.Row>
			{:else if tenants.length === 0}
				<Table.Row>
					<Table.Cell colspan={9} class="text-center py-8 text-muted-foreground">
						{searchDebounce.trim() ? t('tenants.noMatch') : t('tenants.noTenants')}
					</Table.Cell>
				</Table.Row>
			{:else}
				{#each tenants as tenant (tenant.id)}
					<Table.Row>
						<Table.Cell>
							<span class="font-medium">{tenant.username}</span>
						</Table.Cell>
						<Table.Cell>
							{#if tenant.display_name}
								{tenant.display_name}
							{:else}
								<span class="text-muted-foreground">&mdash;</span>
							{/if}
						</Table.Cell>
						<Table.Cell>
							{#if tenant.phone}
								{tenant.phone}
							{:else}
								<span class="text-muted-foreground">&mdash;</span>
							{/if}
						</Table.Cell>
						<Table.Cell>
							<select
								class="rounded-lg border bg-transparent px-2.5 py-1.5 text-sm min-w-[150px]"
								value={tenant.plan_set_id || ''}
								onchange={(e) => handlePlanSetChange(tenant.id, e.currentTarget.value)}
							>
								<option value="">{t('common.default')}</option>
								{#each activePlanSets as planSet (planSet.id)}
									<option value={planSet.id}>{planSet.name}</option>
								{/each}
							</select>
						</Table.Cell>
						<Table.Cell>
							<Switch
								checked={tenant.global_enabled === 1}
								onCheckedChange={(checked) => handleGlobalEnabledChange(tenant.id, checked)}
								size="sm"
							/>
						</Table.Cell>
						<Table.Cell>
							<Switch
								checked={tenant.free_play === 1}
								onCheckedChange={(checked) => handleFreePlayChange(tenant.id, checked)}
								size="sm"
							/>
						</Table.Cell>
						<Table.Cell>
							<Switch
								checked={tenant.status === 1}
								onCheckedChange={() => handleStatusToggle(tenant)}
								size="sm"
							/>
						</Table.Cell>
						<Table.Cell>
							<span class="text-sm">{formatDateTime(tenant.created_at)}</span>
						</Table.Cell>
						<Table.Cell class="text-right">
							<div class="flex items-center justify-end gap-1">
								<Button
									variant="ghost"
									size="icon-sm"
									onclick={() => handleEditClick(tenant)}
									title={t('common.edit')}
								>
									<Pencil class="h-4 w-4" />
								</Button>
								<Button
									variant="ghost"
									size="icon-sm"
									onclick={() => handlePasswordClick(tenant)}
									title={t('tenants.resetPassword')}
								>
									<KeyRound class="h-4 w-4" />
								</Button>
								<Button
									variant="ghost"
									size="icon-sm"
									onclick={() => handleDeleteClick(tenant)}
									title={t('common.delete')}
									class="text-destructive hover:text-destructive"
								>
									<Trash2 class="h-4 w-4" />
								</Button>
							</div>
						</Table.Cell>
					</Table.Row>
				{/each}
			{/if}
		</Table.Body>
	</Table.Root>

	<!-- Pagination -->
	<TablePagination bind:page bind:rowsPerPage {total} rowsPerPageOptions={[5, 10, 25, 50]} />
</div>

<!-- Add / Edit / Password Dialog -->
<Dialog.Root bind:open={dialogOpen}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>
				{#if dialogMode === 'add'}
					{t('tenants.addTenant')}
				{:else if dialogMode === 'edit'}
					{t('tenants.editTenant')}
				{:else}
					{t('tenants.resetPassword')}
				{/if}
			</Dialog.Title>
		</Dialog.Header>

		<div class="space-y-4 pt-2">
			{#if dialogMode === 'add'}
				<!-- Username -->
				<div class="space-y-2">
					<Label>{t('tenants.username')} *</Label>
					<Input
						placeholder="e.g., tenant001"
						bind:value={formUsername}
					/>
					{#if formErrors.username}
						<p class="text-sm text-destructive">{formErrors.username}</p>
					{:else}
						<p class="text-sm text-muted-foreground">{t('tenants.usernameHelp')}</p>
					{/if}
				</div>
				<!-- Password -->
				<div class="space-y-2">
					<Label>{t('login.password')} *</Label>
					<Input
						type="password"
						placeholder={t('tenants.passwordHelp')}
						bind:value={formPassword}
					/>
					{#if formErrors.password}
						<p class="text-sm text-destructive">{formErrors.password}</p>
					{:else}
						<p class="text-sm text-muted-foreground">{t('tenants.passwordHelp')}</p>
					{/if}
				</div>
				<!-- Display Name -->
				<div class="space-y-2">
					<Label>{t('tenants.displayName')}</Label>
					<Input
						placeholder="e.g., Acme Corporation"
						bind:value={formDisplayName}
					/>
					<p class="text-sm text-muted-foreground">{t('tenants.displayNameHelp')}</p>
				</div>
				<!-- Phone -->
				<div class="space-y-2">
					<Label>{t('tenants.phone')}</Label>
					<Input
						placeholder="e.g., +1234567890"
						bind:value={formPhone}
					/>
					<p class="text-sm text-muted-foreground">{t('tenants.phoneHelp')}</p>
				</div>
			{/if}

			{#if dialogMode === 'edit'}
				<!-- Display Name -->
				<div class="space-y-2">
					<Label>{t('tenants.displayName')}</Label>
					<Input
						placeholder="e.g., Acme Corporation"
						bind:value={formDisplayName}
					/>
					<p class="text-sm text-muted-foreground">{t('tenants.displayNameHelp')}</p>
				</div>
				<!-- Phone -->
				<div class="space-y-2">
					<Label>{t('tenants.phone')}</Label>
					<Input
						placeholder="e.g., +1234567890"
						bind:value={formPhone}
					/>
					<p class="text-sm text-muted-foreground">{t('tenants.phoneHelp')}</p>
				</div>
			{/if}

			{#if dialogMode === 'password'}
				<!-- New Password -->
				<div class="space-y-2">
					<Label>{t('tenants.newPassword')} *</Label>
					<Input
						type="password"
						placeholder={t('tenants.newPasswordHelp')}
						bind:value={formPassword}
					/>
					{#if formErrors.password}
						<p class="text-sm text-destructive">{formErrors.password}</p>
					{:else}
						<p class="text-sm text-muted-foreground">{t('tenants.newPasswordHelp')}</p>
					{/if}
				</div>
			{/if}
		</div>

		<Dialog.Footer class="mt-4">
			<Button variant="outline" onclick={handleDialogClose}>
				{t('common.cancel')}
			</Button>
			<Button onclick={handleSave}>
				{t('common.save')}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<!-- Delete Confirmation Dialog -->
<ConfirmActionDialog
	bind:open={deleteDialogOpen}
	title={t('tenants.deleteTenant')}
	confirmLabel={t('common.delete')}
	confirmVariant="destructive"
	onConfirm={handleDelete}
	onCancel={() => (tenantToDelete = null)}
>
	{#snippet children()}
		{#if tenantToDelete}
			<p class="py-2">
				{t('tenants.deleteConfirm')}
				<strong>{tenantToDelete.display_name || tenantToDelete.username}</strong>?
			</p>
		{/if}
	{/snippet}
</ConfirmActionDialog>
