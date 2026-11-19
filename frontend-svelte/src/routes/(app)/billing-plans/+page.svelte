<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from '$lib/i18n';
	import { billingPlansAPI } from '$lib/api';
	import { toast } from 'svelte-sonner';
	import { Plus, Pencil, Trash2 } from '@lucide/svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Badge } from '$lib/components/ui/badge';
	import { Switch } from '$lib/components/ui/switch';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Table from '$lib/components/ui/table';
	import ConfirmActionDialog from '$lib/components/confirm-action-dialog.svelte';
	import { runWithToast } from '$lib/action-feedback';

	interface BillingPlan {
		id: number;
		name: string;
		rate_cents: number;
		unit: string;
		description?: string;
		is_default: number;
		status: number;
		created_at: string;
	}

	let plans: BillingPlan[] = $state([]);
	let loading = $state(true);

	// Create/Edit dialog state
	let dialogOpen = $state(false);
	let dialogMode: 'create' | 'edit' = $state('create');
	let editPlan: BillingPlan | null = $state(null);

	// Form state
	let formName = $state('');
	let formRateYuan = $state('0.50');
	let formUnit = $state('minute');
	let formDescription = $state('');
	let formIsDefault = $state(false);
	let formStatus = $state(true);

	// Delete confirm dialog state
	let deleteDialogOpen = $state(false);
	let deletePlan: BillingPlan | null = $state(null);

	onMount(() => {
		loadData();
	});

	async function loadData() {
		try {
			loading = true;
			const data = await billingPlansAPI.getAll();
			plans = data || [];
		} catch (error: any) {
			toast.error(t('billingPlans.loadFailed'));
		} finally {
			loading = false;
		}
	}

	function formatRate(rateCents: number, unit: string): string {
		const yuan = (rateCents / 100).toFixed(2);
		return `¥${yuan}/${unit === 'minute' ? t('billingPlans.perMinute') : t('billingPlans.perHour')}`;
	}

	function formatDate(dateStr: string): string {
		if (!dateStr) return '-';
		return new Date(dateStr).toLocaleString('zh-CN');
	}

	function openCreateDialog() {
		dialogMode = 'create';
		editPlan = null;
		formName = '';
		formRateYuan = '0.50';
		formUnit = 'minute';
		formDescription = '';
		formIsDefault = false;
		formStatus = true;
		dialogOpen = true;
	}

	function openEditDialog(plan: BillingPlan) {
		dialogMode = 'edit';
		editPlan = plan;
		formName = plan.name;
		formRateYuan = ((plan.rate_cents || 0) / 100).toFixed(2);
		formUnit = plan.unit;
		formDescription = plan.description || '';
		formIsDefault = plan.is_default === 1;
		formStatus = plan.status === 1;
		dialogOpen = true;
	}

	function closeDialog() {
		dialogOpen = false;
		editPlan = null;
	}

	let formValid = $derived(formName.trim().length > 0 && parseFloat(formRateYuan) > 0);

	let ratePreview = $derived(
		formatRate(Math.round(parseFloat(formRateYuan || '0') * 100), formUnit)
	);

	async function handleSubmit() {
		if (!formName.trim()) {
			toast.error(t('billingPlans.enterPlanName'));
			return;
		}

		if (parseFloat(formRateYuan) <= 0) {
			toast.error(t('billingPlans.rateMustBePositive'));
			return;
		}

		const payload = {
			name: formName.trim(),
			rate_cents: Math.round(parseFloat(formRateYuan) * 100),
			unit: formUnit,
			description: formDescription.trim(),
			is_default: formIsDefault ? 1 : 0,
			status: formStatus ? 1 : 0,
		};

		try {
			if (dialogMode === 'create') {
				await billingPlansAPI.create(payload);
				toast.success(t('common.createSuccess'));
			} else {
				await billingPlansAPI.update(editPlan!.id, payload);
				toast.success(t('common.updateSuccess'));
			}
			closeDialog();
			loadData();
		} catch (error: any) {
			toast.error(
				(dialogMode === 'create' ? t('common.createFailed') : t('common.updateFailed')) +
					': ' +
					(error.message || '')
			);
		}
	}

	function openDeleteDialog(plan: BillingPlan) {
		deletePlan = plan;
		deleteDialogOpen = true;
	}

	async function handleDelete() {
		if (!deletePlan) return;

		const ok = await runWithToast(() => billingPlansAPI.delete(deletePlan!.id), {
			successMessage: t('common.deleteSuccess'),
			errorFallback: t('common.deleteFailed'),
			logLabel: 'Failed to delete billing plan:',
		});
		if (ok) {
			deleteDialogOpen = false;
			deletePlan = null;
			loadData();
		}
	}
</script>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<h2 class="text-2xl font-bold tracking-tight">{t('billingPlans.title')}</h2>
		<Button onclick={openCreateDialog}>
			<Plus class="mr-2 h-4 w-4" />
			{t('billingPlans.addPlan')}
		</Button>
	</div>

	<div class="rounded-md border">
		<Table.Root>
			<Table.Header>
				<Table.Row>
					<Table.Head>ID</Table.Head>
					<Table.Head>{t('billingPlans.planName')}</Table.Head>
					<Table.Head>{t('billingPlans.billingRate')}</Table.Head>
					<Table.Head>{t('common.description')}</Table.Head>
					<Table.Head>{t('billingPlans.defaultPlan')}</Table.Head>
					<Table.Head>{t('common.status')}</Table.Head>
					<Table.Head>{t('common.createdAt')}</Table.Head>
					<Table.Head class="text-right">{t('common.actions')}</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#if loading}
					<Table.Row>
						<Table.Cell colspan={8} class="text-center">
							{t('common.loading')}
						</Table.Cell>
					</Table.Row>
				{:else if plans.length === 0}
					<Table.Row>
						<Table.Cell colspan={8} class="text-center">
							{t('billingPlans.noPlan')}
						</Table.Cell>
					</Table.Row>
				{:else}
					{#each plans as plan (plan.id)}
						<Table.Row>
							<Table.Cell>{plan.id}</Table.Cell>
							<Table.Cell>{plan.name}</Table.Cell>
							<Table.Cell>{formatRate(plan.rate_cents, plan.unit)}</Table.Cell>
							<Table.Cell>
								<span class="block max-w-[300px] truncate">
									{plan.description || '-'}
								</span>
							</Table.Cell>
							<Table.Cell>
								{#if plan.is_default === 1}
									<Badge variant="default">{t('common.yes')}</Badge>
								{:else}
									<Badge variant="outline">{t('common.no')}</Badge>
								{/if}
							</Table.Cell>
							<Table.Cell>
								{#if plan.status === 1}
									<Badge variant="default">{t('common.enabled')}</Badge>
								{:else}
									<Badge variant="secondary">{t('common.disabled')}</Badge>
								{/if}
							</Table.Cell>
							<Table.Cell>{formatDate(plan.created_at)}</Table.Cell>
							<Table.Cell class="text-right">
								<Button
									variant="ghost"
									size="icon"
									onclick={() => openEditDialog(plan)}
								>
									<Pencil class="h-4 w-4" />
								</Button>
								<Button
									variant="ghost"
									size="icon"
									onclick={() => openDeleteDialog(plan)}
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

	<p class="text-sm text-muted-foreground">{t('billingPlans.hint')}</p>
</div>

<!-- Create/Edit Dialog -->
<Dialog.Root bind:open={dialogOpen}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>
				{dialogMode === 'create' ? t('billingPlans.addTitle') : t('billingPlans.editTitle')}
			</Dialog.Title>
			<Dialog.Description>
				{dialogMode === 'create' ? t('billingPlans.addTitle') : t('billingPlans.editTitle')}
			</Dialog.Description>
		</Dialog.Header>
		<div class="space-y-4 py-4">
			<div class="space-y-2">
				<Label for="plan-name">{t('billingPlans.planName')}</Label>
				<Input
					id="plan-name"
					bind:value={formName}
					maxlength={100}
					placeholder={t('billingPlans.planNamePlaceholder')}
					autofocus
				/>
			</div>

			<div class="space-y-2">
				<Label for="plan-rate">{t('billingPlans.rateYuan')}</Label>
				<Input
					id="plan-rate"
					type="number"
					bind:value={formRateYuan}
					step="0.01"
					min="0"
				/>
				<p class="text-sm text-muted-foreground">= {ratePreview}</p>
			</div>

			<div class="space-y-2">
				<Label for="plan-unit">{t('billingPlans.billingUnit')}</Label>
				<select
					id="plan-unit"
					bind:value={formUnit}
					class="border-input bg-background ring-offset-background placeholder:text-muted-foreground focus-visible:ring-ring flex h-9 w-full rounded-md border px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1"
				>
					<option value="minute">{t('billingPlans.perMinute')}</option>
					<option value="hour">{t('billingPlans.perHour')}</option>
				</select>
			</div>

			<div class="space-y-2">
				<Label for="plan-description">{t('common.description')}</Label>
				<textarea
					id="plan-description"
					bind:value={formDescription}
					maxlength={500}
					rows={3}
					class="border-input bg-background ring-offset-background placeholder:text-muted-foreground focus-visible:ring-ring flex min-h-[60px] w-full rounded-md border px-3 py-2 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1"
				></textarea>
				<p class="text-sm text-muted-foreground">{formDescription.length}/500</p>
			</div>

			<div class="flex items-center justify-between">
				<Label for="plan-status">{t('billingPlans.enable')}</Label>
				<Switch id="plan-status" bind:checked={formStatus} />
			</div>

			<div class="flex items-center gap-2">
				<Checkbox id="plan-default" bind:checked={formIsDefault} />
				<Label for="plan-default">{t('billingPlans.setDefault')}</Label>
			</div>
		</div>
		<Dialog.Footer>
			<Button variant="outline" onclick={closeDialog}>{t('common.cancel')}</Button>
			<Button onclick={handleSubmit} disabled={!formValid}>
				{dialogMode === 'create' ? t('common.create') : t('common.save')}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<!-- Delete Confirm Dialog -->
<ConfirmActionDialog
	bind:open={deleteDialogOpen}
	title={t('common.confirmDelete')}
	description={t('billingPlans.deleteConfirm').replace('{name}', deletePlan?.name || '')}
	confirmLabel={t('common.delete')}
	confirmVariant="destructive"
	onConfirm={handleDelete}
	onCancel={() => (deletePlan = null)}
/>
