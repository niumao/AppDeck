<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from '$lib/i18n';
	import { userClientsAPI, usersAPI, billingPlansAPI, steamAPI } from '$lib/api';
	import { toast } from 'svelte-sonner';
	import { Plus, Pencil, Trash2, CheckCircle, Loader2 } from '@lucide/svelte';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Table from '$lib/components/ui/table';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Badge } from '$lib/components/ui/badge';
	import ConfirmActionDialog from '$lib/components/confirm-action-dialog.svelte';
	import { runWithToast } from '$lib/action-feedback';

	// --- User-client list state ---
	let clients: any[] = $state([]);
	let loading = $state(true);

	// --- Reference data ---
	let users: any[] = $state([]);
	let plans: any[] = $state([]);

	// --- Create/Edit dialog ---
	let dialogOpen = $state(false);
	let dialogMode: 'create' | 'edit' = $state('create');
	let dialogClient: any = $state(null);

	// --- Form state ---
	let formUserId = $state('');
	let formPlanId = $state('');
	let formMachineId = $state('');
	let formSteamId = $state('');

	// --- Steam validation state ---
	let steamValidating = $state(false);
	let steamValidated = $state(false);
	let steamValid = $state(false);
	let steamMessage = $state('');

	// --- Delete dialog ---
	let deleteDialogOpen = $state(false);
	let deleteClient: any = $state(null);

	// --- Derived: disable submit when form is incomplete ---
	let submitDisabled = $derived(
		!formUserId ||
		!formPlanId ||
		!formMachineId.trim() ||
		(formSteamId.trim() !== '' && !steamValid)
	);

	// --- Load data ---
	async function loadClients() {
		try {
			loading = true;
			const data = await userClientsAPI.getAll();
			clients = data || [];
		} catch (error) {
			console.error('Failed to load user clients:', error);
			toast.error(t('userClients.loadFailed'));
		} finally {
			loading = false;
		}
	}

	async function loadUsers() {
		try {
			const data = await usersAPI.getAll();
			users = data?.items || data || [];
		} catch (error) {
			console.error('Failed to load users:', error);
		}
	}

	async function loadPlans() {
		try {
			const data = await billingPlansAPI.getAll();
			plans = data || [];
		} catch (error) {
			console.error('Failed to load plans:', error);
		}
	}

	onMount(() => {
		loadClients();
		loadUsers();
		loadPlans();
	});

	// --- Helpers ---
	function formatDate(dateStr: string | null | undefined): string {
		if (!dateStr) return '-';
		return new Date(dateStr).toLocaleString('zh-CN');
	}

	function getUserName(userId: any): string {
		const user = users.find((u) => u.id === userId);
		return user?.nickname || user?.openid || `User ${userId}`;
	}

	function getPlanName(planId: any): string {
		const plan = plans.find((p) => p.id === planId);
		return plan?.name || `Plan ${planId}`;
	}

	// --- Reset steam validation ---
	function resetSteamValidation() {
		steamValidating = false;
		steamValidated = false;
		steamValid = false;
		steamMessage = '';
	}

	// --- Open create/edit dialog ---
	function openDialog(mode: 'create' | 'edit', client: any = null) {
		dialogMode = mode;
		dialogClient = client;
		resetSteamValidation();

		if (mode === 'edit' && client) {
			formUserId = client.user_id;
			formPlanId = client.plan_id;
			formMachineId = client.machine_id;
			formSteamId = client.steam_id || '';
		} else {
			formUserId = '';
			formPlanId = '';
			formMachineId = '';
			formSteamId = '';
		}

		dialogOpen = true;
	}

	function closeDialog() {
		dialogOpen = false;
		dialogClient = null;
		formUserId = '';
		formPlanId = '';
		formMachineId = '';
		formSteamId = '';
		resetSteamValidation();
	}

	// --- Steam ID validation ---
	async function handleValidateSteamId() {
		if (!formSteamId.trim()) {
			toast.warning(t('userClients.pleaseEnterSteamId'));
			return;
		}

		try {
			steamValidating = true;
			steamValidated = false;
			steamValid = false;
			steamMessage = '';

			const data = await steamAPI.validate(formSteamId);

			if (data?.valid) {
				steamValidating = false;
				steamValidated = true;
				steamValid = true;
				steamMessage = t('userClients.steamIdValid');
				toast.success(t('userClients.steamIdValid'));
			} else {
				steamValidating = false;
				steamValidated = true;
				steamValid = false;
				steamMessage = data?.message || t('userClients.steamIdInvalid');
				toast.error(data?.message || t('userClients.steamIdInvalid'));
			}
		} catch (error: any) {
			console.error('Failed to validate Steam ID:', error);
			steamValidating = false;
			steamValidated = true;
			steamValid = false;
			steamMessage = error?.response?.data?.message || t('userClients.steamIdValidateFailed');
			toast.error(error?.response?.data?.message || t('userClients.steamIdValidateFailed'));
		}
	}

	// --- Submit create/edit ---
	async function handleSubmit() {
		if (!formUserId) {
			toast.error(t('userClients.pleaseSelectUser'));
			return;
		}
		if (!formPlanId) {
			toast.error(t('userClients.pleaseSelectPlan'));
			return;
		}
		if (!formMachineId.trim()) {
			toast.error(t('userClients.pleaseEnterMachineId'));
			return;
		}
		if (formSteamId.trim() && !steamValid) {
			toast.warning(t('userClients.pleaseValidateSteamId'));
			return;
		}

		try {
			const payload = {
				user_id: formUserId,
				plan_id: formPlanId,
				machine_id: formMachineId.trim(),
				steam_id: formSteamId.trim() || null,
			};

			if (dialogMode === 'create') {
				await userClientsAPI.create(payload);
				toast.success(t('common.createSuccess'));
			} else {
				await userClientsAPI.update(dialogClient.id, payload);
				toast.success(t('common.updateSuccess'));
			}
			closeDialog();
			loadClients();
		} catch (error: any) {
			console.error('Failed to save user client:', error);
			toast.error(
				error?.response?.data?.message ||
					(dialogMode === 'create' ? t('common.createFailed') : t('common.updateFailed'))
			);
		}
	}

	// --- Delete ---
	function openDeleteDialog(client: any) {
		deleteClient = client;
		deleteDialogOpen = true;
	}

	async function handleDelete() {
		if (!deleteClient) return;
		const ok = await runWithToast(() => userClientsAPI.delete(deleteClient.id), {
			successMessage: t('common.deleteSuccess'),
			errorFallback: t('common.deleteFailed'),
			logLabel: 'Failed to delete user client:',
		});
		if (ok) {
			deleteDialogOpen = false;
			deleteClient = null;
			loadClients();
		}
	}
</script>

<!-- Header -->
<div class="flex items-center justify-between mb-6">
	<h2 class="text-2xl font-semibold">{t('userClients.title')}</h2>
	<Button onclick={() => openDialog('create')}>
		<Plus class="mr-2 h-4 w-4" />
		{t('userClients.addAssoc')}
	</Button>
</div>

<!-- User Clients Table -->
<div class="rounded-lg border">
	<Table.Root>
		<Table.Header>
			<Table.Row>
				<Table.Head>{t('userClients.id')}</Table.Head>
				<Table.Head>{t('common.user')}</Table.Head>
				<Table.Head>{t('userClients.machineId')}</Table.Head>
				<Table.Head>{t('userClients.billingPlan')}</Table.Head>
				<Table.Head>{t('userClients.steamId')}</Table.Head>
				<Table.Head>{t('common.createdAt')}</Table.Head>
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
			{:else if clients.length === 0}
				<Table.Row>
					<Table.Cell colspan={7} class="text-center py-8 text-muted-foreground">
						{t('userClients.noClients')}
					</Table.Cell>
				</Table.Row>
			{:else}
				{#each clients as client (client.id)}
					<Table.Row>
						<Table.Cell>{client.id}</Table.Cell>
						<Table.Cell>{getUserName(client.user_id)}</Table.Cell>
						<Table.Cell>
							<code class="text-sm">{client.machine_id}</code>
						</Table.Cell>
						<Table.Cell>{getPlanName(client.plan_id)}</Table.Cell>
						<Table.Cell>
							{#if client.steam_id}
								<code class="text-sm">{client.steam_id}</code>
							{:else}
								-
							{/if}
						</Table.Cell>
						<Table.Cell class="text-sm">{formatDate(client.created_at)}</Table.Cell>
						<Table.Cell class="text-right">
							<div class="flex items-center justify-end gap-1">
								<Button
									variant="ghost"
									size="icon-sm"
									onclick={() => openDialog('edit', client)}
									title={t('common.edit')}
								>
									<Pencil class="h-4 w-4" />
								</Button>
								<Button
									variant="ghost"
									size="icon-sm"
									class="text-destructive hover:text-destructive"
									onclick={() => openDeleteDialog(client)}
									title={t('common.delete')}
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
</div>

<!-- Create / Edit Dialog -->
<Dialog.Root bind:open={dialogOpen}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>
				{dialogMode === 'create' ? t('userClients.addTitle') : t('userClients.editTitle')}
			</Dialog.Title>
		</Dialog.Header>

		<div class="space-y-4 pt-2">
			<!-- User select -->
			<div class="space-y-2">
				<Label>{t('common.user')} *</Label>
				<select
					class="w-full rounded-lg border bg-transparent px-2.5 py-1.5 text-sm"
					bind:value={formUserId}
					disabled={dialogMode === 'edit'}
				>
					<option value="">{t('userClients.selectUser')}</option>
					{#each users as user}
						<option value={user.id}>
							{user.nickname || user.openid} (ID: {user.id})
						</option>
					{/each}
				</select>
			</div>

			<!-- Billing plan select -->
			<div class="space-y-2">
				<Label>{t('userClients.billingPlan')} *</Label>
				<select
					class="w-full rounded-lg border bg-transparent px-2.5 py-1.5 text-sm"
					bind:value={formPlanId}
				>
					<option value="">{t('userClients.selectPlan')}</option>
					{#each plans.filter((p) => p.status === 1) as plan}
						<option value={plan.id}>{plan.name}</option>
					{/each}
				</select>
			</div>

			<!-- Machine ID -->
			<div class="space-y-2">
				<Label>{t('userClients.machineId')} *</Label>
				<Input
					bind:value={formMachineId}
					placeholder={t('userClients.machineIdPlaceholder')}
					disabled={dialogMode === 'edit'}
				/>
			</div>

			<!-- Steam ID with validate button -->
			<div class="space-y-2">
				<Label>{t('userClients.steamIdOptional')}</Label>
				<div class="flex items-center gap-2">
					<Input
						bind:value={formSteamId}
						placeholder={t('userClients.steamIdPlaceholder')}
						class="flex-1"
						oninput={() => resetSteamValidation()}
					/>
					{#if formSteamId.trim()}
						<Button
							variant={steamValid ? 'outline' : 'secondary'}
							size="sm"
							onclick={handleValidateSteamId}
							disabled={steamValidating}
							class={steamValid ? 'text-green-600 border-green-600' : ''}
						>
							{#if steamValidating}
								<Loader2 class="mr-1 h-4 w-4 animate-spin" />
								{t('userClients.validating')}
							{:else if steamValid}
								<CheckCircle class="mr-1 h-4 w-4" />
								{t('userClients.validated')}
							{:else}
								{t('userClients.validate')}
							{/if}
						</Button>
					{/if}
				</div>
				{#if steamValidated && steamMessage}
					<p class="text-sm {steamValid ? 'text-green-600' : 'text-destructive'}">
						{steamMessage}
					</p>
				{:else}
					<p class="text-sm text-muted-foreground">
						{t('userClients.steamIdValidateHint')}
					</p>
				{/if}
			</div>
		</div>

		<Dialog.Footer class="mt-4">
			<Button variant="outline" onclick={closeDialog}>
				{t('common.cancel')}
			</Button>
			<Button disabled={submitDisabled} onclick={handleSubmit}>
				{dialogMode === 'create' ? t('common.create') : t('common.save')}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<!-- Delete Confirmation Dialog -->
<ConfirmActionDialog
	bind:open={deleteDialogOpen}
	title={t('common.confirmDelete')}
	confirmLabel={t('common.delete')}
	confirmVariant="destructive"
	onConfirm={handleDelete}
	onCancel={() => (deleteClient = null)}
>
	{#snippet children()}
		<p class="py-4 text-sm text-muted-foreground">
			{t('userClients.deleteConfirm').replace('{name}', deleteClient ? getUserName(deleteClient.user_id) : '')}
		</p>
	{/snippet}
</ConfirmActionDialog>
