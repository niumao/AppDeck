<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from '$lib/i18n';
	import { planSetsAPI } from '$lib/api';
	import { toast } from 'svelte-sonner';
	import { Plus, Pencil, Trash2, ChevronDown, ChevronUp, ArchiveRestore, LoaderCircle } from '@lucide/svelte';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Table from '$lib/components/ui/table';
	import * as Card from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Badge } from '$lib/components/ui/badge';
	import { Switch } from '$lib/components/ui/switch';

	// Plan sets list
	let planSets: any[] = $state([]);
	let loading = $state(true);
	let expandedSet: number | null = $state(null);

	// Show deleted
	let showDeleted = $state(false);
	let deletedSets: any[] = $state([]);

	// Summary stats
	let stats = $derived({
		total: planSets.length,
		active: planSets.filter((s) => s.status === 1).length,
		defaultSet: planSets.find((s) => s.is_default === 1)
	});

	// Plan Set dialog
	let setDialogOpen = $state(false);
	let setDialogMode: 'create' | 'edit' = $state('create');
	let editPlanSet: any = $state(null);
	let setFormName = $state('');
	let setFormDescription = $state('');
	let setFormIsDefault = $state(false);

	// Plan dialog
	let planDialogOpen = $state(false);
	let planDialogMode: 'create' | 'edit' = $state('create');
	let editPlan: any = $state(null);
	let planDialogSetId: number | null = $state(null);
	let planFormName = $state('');
	let planFormType = $state('rate');
	let planFormRateCents = $state(5000);
	let planFormPriceCents = $state(0);
	let planFormGivesMinutes = $state(0);
	let planFormUnit = $state('minute');
	let planFormDescription = $state('');
	let planFormStatus = $state(1);

	onMount(() => {
		loadPlanSets();
	});

	async function loadPlanSets() {
		try {
			loading = true;
			const response = await planSetsAPI.getAll();
			planSets = response?.plan_sets || [];
		} catch (error: any) {
			console.error('Failed to load plan sets:', error);
			toast.error(error.message || t('common.loadFailed'));
		} finally {
			loading = false;
		}
	}

	async function loadDeletedSets() {
		try {
			const data = await planSetsAPI.getDeleted();
			deletedSets = data || [];
		} catch (error: any) {
			toast.error(error.message || t('common.loadFailed'));
		}
	}

	function toggleExpand(setId: number) {
		expandedSet = expandedSet === setId ? null : setId;
	}

	// --- Plan Set CRUD ---

	function openSetDialog(mode: 'create' | 'edit', planSet: any = null) {
		setDialogMode = mode;
		editPlanSet = planSet;
		if (mode === 'edit' && planSet) {
			setFormName = planSet.name || '';
			setFormDescription = planSet.description || '';
			setFormIsDefault = planSet.is_default === 1;
		} else {
			setFormName = '';
			setFormDescription = '';
			setFormIsDefault = false;
		}
		setDialogOpen = true;
	}

	function closeSetDialog() {
		setDialogOpen = false;
		editPlanSet = null;
		setFormName = '';
		setFormDescription = '';
		setFormIsDefault = false;
	}

	async function handleSavePlanSet() {
		if (!setFormName.trim()) {
			toast.error(t('common.name') + ' ' + t('common.required'));
			return;
		}

		try {
			const payload = {
				name: setFormName.trim(),
				description: setFormDescription.trim(),
				is_default: setFormIsDefault ? 1 : 0
			};

			if (setDialogMode === 'create') {
				await planSetsAPI.create(payload);
				toast.success(t('common.createSuccess'));
			} else {
				await planSetsAPI.update(editPlanSet.id, payload);
				toast.success(t('common.updateSuccess'));
			}
			closeSetDialog();
			loadPlanSets();
		} catch (error: any) {
			console.error('Failed to save plan set:', error);
			toast.error(error.message || t('common.saveFailed'));
		}
	}

	async function handleTogglePlanSetStatus(planSet: any) {
		try {
			const newStatus = planSet.status === 1 ? 0 : 1;
			await planSetsAPI.updateStatus(planSet.id, { status: newStatus });
			toast.success(t('common.updateSuccess'));
			loadPlanSets();
		} catch (error: any) {
			console.error('Failed to update plan set status:', error);
			toast.error(error.message || t('common.updateFailed'));
		}
	}

	async function handleDeletePlanSet(planSet: any) {
		if (!window.confirm(t('common.confirmDelete') + '?')) return;
		try {
			await planSetsAPI.delete(planSet.id);
			toast.success(t('common.deleteSuccess'));
			loadPlanSets();
		} catch (error: any) {
			toast.error(error.message || t('common.deleteFailed'));
		}
	}

	async function handleRecoverPlanSet(id: number) {
		try {
			await planSetsAPI.recover(id);
			toast.success(t('common.recoverSuccess'));
			loadDeletedSets();
			loadPlanSets();
		} catch (error: any) {
			toast.error(error.message || t('common.recoverFailed'));
		}
	}

	// --- Standard / Global Rate assignment ---

	async function handleSetStandardPlan(planSetId: number, planId: string) {
		try {
			await planSetsAPI.update(planSetId, { standard_plan_id: planId || null });
			toast.success(t('common.updateSuccess'));
			loadPlanSets();
		} catch (error: any) {
			console.error('Failed to set standard plan:', error);
			toast.error(error.message || t('common.updateFailed'));
		}
	}

	async function handleSetGlobalRatePlan(planSetId: number, planId: string) {
		try {
			await planSetsAPI.update(planSetId, { global_rate_plan_id: planId || null });
			toast.success(t('common.updateSuccess'));
			loadPlanSets();
		} catch (error: any) {
			console.error('Failed to set global rate plan:', error);
			toast.error(error.message || t('common.updateFailed'));
		}
	}

	// --- Plan CRUD ---

	function openPlanDialog(mode: 'create' | 'edit', planSetId: number, plan: any = null) {
		planDialogMode = mode;
		planDialogSetId = planSetId;
		editPlan = plan;
		if (mode === 'edit' && plan) {
			planFormName = plan.name || '';
			planFormType = plan.type || 'rate';
			planFormRateCents = plan.rate_cents || 0;
			planFormPriceCents = plan.price_cents || 0;
			planFormGivesMinutes = plan.gives_minutes || 0;
			planFormUnit = plan.unit || 'minute';
			planFormDescription = plan.description || '';
			planFormStatus = plan.status ?? 1;
		} else {
			planFormName = '';
			planFormType = 'rate';
			planFormRateCents = 5000;
			planFormPriceCents = 0;
			planFormGivesMinutes = 0;
			planFormUnit = 'minute';
			planFormDescription = '';
			planFormStatus = 1;
		}
		planDialogOpen = true;
	}

	function closePlanDialog() {
		planDialogOpen = false;
		editPlan = null;
		planDialogSetId = null;
		planFormName = '';
		planFormType = 'rate';
		planFormRateCents = 5000;
		planFormPriceCents = 0;
		planFormGivesMinutes = 0;
		planFormUnit = 'minute';
		planFormDescription = '';
		planFormStatus = 1;
	}

	async function handleSavePlan() {
		if (!planFormName.trim()) {
			toast.error(t('common.name') + ' ' + t('common.required'));
			return;
		}

		try {
			const payload: Record<string, any> = {
				name: planFormName.trim(),
				type: planFormType,
				unit: planFormUnit,
				description: planFormDescription.trim(),
				status: planFormStatus
			};

			if (planFormType === 'rate') {
				payload.rate_cents = planFormRateCents;
			} else {
				payload.price_cents = planFormPriceCents;
				payload.gives_minutes = planFormGivesMinutes;
			}

			if (planDialogMode === 'create') {
				await planSetsAPI.addPlan(planDialogSetId!, payload);
				toast.success(t('common.createSuccess'));
			} else {
				await planSetsAPI.updatePlan(editPlan.id, payload);
				toast.success(t('common.updateSuccess'));
			}
			closePlanDialog();
			loadPlanSets();
		} catch (error: any) {
			console.error('Failed to save plan:', error);
			toast.error(error.message || t('common.saveFailed'));
		}
	}

	async function handleTogglePlanStatus(plan: any) {
		try {
			const newStatus = plan.status === 1 ? 0 : 1;
			await planSetsAPI.updatePlanStatus(plan.id, { status: newStatus });
			toast.success(t('common.updateSuccess'));
			loadPlanSets();
		} catch (error: any) {
			console.error('Failed to update plan status:', error);
			toast.error(error.message || t('common.updateFailed'));
		}
	}

	async function handleDeletePlan(planId: number) {
		if (!window.confirm(t('common.confirmDelete') + '?')) return;
		try {
			await planSetsAPI.deletePlan(planId);
			toast.success(t('common.deleteSuccess'));
			loadPlanSets();
		} catch (error: any) {
			console.error('Failed to delete plan:', error);
			toast.error(error.message || t('common.deleteFailed'));
		}
	}

	// --- Helpers ---

	function formatCurrency(cents: number): string {
		return `\u00A5${(cents / 100).toFixed(2)}`;
	}

	function formatCentsToYuan(cents: number): string {
		return (cents / 100).toFixed(2);
	}

	function formatYuanToCents(yuan: string): number {
		return Math.round(parseFloat(yuan) * 100);
	}
</script>

<div class="space-y-4">
	<!-- Header -->
	<div class="flex items-center justify-between">
		<h2 class="text-2xl font-bold tracking-tight">{t('planSets.title')}</h2>
		<div class="flex items-center gap-3">
			<label class="flex items-center gap-2 text-sm">
				<Switch
					checked={showDeleted}
					onCheckedChange={(v) => {
						showDeleted = v;
						if (v) loadDeletedSets();
					}}
					size="sm"
				/>
				{t('common.showDeleted')}
			</label>
			<Button onclick={() => openSetDialog('create')}>
				<Plus class="mr-2 h-4 w-4" />
				{t('planSets.addPlanSet')}
			</Button>
		</div>
	</div>

	<!-- Summary Cards -->
	<div class="grid grid-cols-3 gap-4">
		<Card.Root>
			<Card.Content class="pt-6">
				<p class="text-sm text-muted-foreground">{t('planSets.totalPlanSets')}</p>
				<p class="text-3xl font-bold">{stats.total}</p>
			</Card.Content>
		</Card.Root>
		<Card.Root>
			<Card.Content class="pt-6">
				<p class="text-sm text-muted-foreground">{t('planSets.activePlanSets')}</p>
				<p class="text-3xl font-bold">{stats.active}</p>
			</Card.Content>
		</Card.Root>
		<Card.Root>
			<Card.Content class="pt-6">
				<p class="text-sm text-muted-foreground">{t('planSets.defaultPlanSet')}</p>
				<p class="text-lg font-semibold">{stats.defaultSet?.name || t('common.none')}</p>
			</Card.Content>
		</Card.Root>
	</div>

	<!-- Plan Sets Table -->
	<div class="rounded-md border">
		<Table.Root>
			<Table.Header>
				<Table.Row>
					<Table.Head class="w-12"></Table.Head>
					<Table.Head>{t('common.name')}</Table.Head>
					<Table.Head>{t('common.description')}</Table.Head>
					<Table.Head>{t('common.default')}</Table.Head>
					<Table.Head>{t('planSets.standardPlan')}</Table.Head>
					<Table.Head>{t('planSets.globalRatePlan')}</Table.Head>
					<Table.Head>{t('planSets.plansCount')}</Table.Head>
					<Table.Head>{t('common.status')}</Table.Head>
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
				{:else if planSets.length === 0}
					<Table.Row>
						<Table.Cell colspan={9} class="py-8 text-center text-muted-foreground">
							{t('planSets.noPlanSets')}
						</Table.Cell>
					</Table.Row>
				{:else}
					{#each planSets as planSet (planSet.id)}
						<!-- Plan Set Row -->
						<Table.Row>
							<Table.Cell>
								<Button variant="ghost" size="icon-sm" onclick={() => toggleExpand(planSet.id)}>
									{#if expandedSet === planSet.id}
										<ChevronUp class="h-4 w-4" />
									{:else}
										<ChevronDown class="h-4 w-4" />
									{/if}
								</Button>
							</Table.Cell>
							<Table.Cell>
								<span class="font-medium">{planSet.name}</span>
							</Table.Cell>
							<Table.Cell>
								<span class="max-w-[200px] truncate block">{planSet.description || '\u2014'}</span>
							</Table.Cell>
							<Table.Cell>
								{#if planSet.is_default === 1}
									<Badge>{t('common.default')}</Badge>
								{:else}
									<Badge variant="outline">{t('common.no')}</Badge>
								{/if}
							</Table.Cell>
							<Table.Cell>
								{planSet.standard_plan?.name || '\u2014'}
							</Table.Cell>
							<Table.Cell>
								{planSet.global_rate_plan?.name || '\u2014'}
							</Table.Cell>
							<Table.Cell>
								<Badge variant="secondary">{planSet.plans?.length || 0}</Badge>
							</Table.Cell>
							<Table.Cell>
								<div class="flex items-center gap-2">
									<Badge variant={planSet.status === 1 ? 'default' : 'outline'}>
										{planSet.status === 1 ? t('common.active') : t('common.inactive')}
									</Badge>
									<Switch
										checked={planSet.status === 1}
										onCheckedChange={() => handleTogglePlanSetStatus(planSet)}
										size="sm"
									/>
								</div>
							</Table.Cell>
							<Table.Cell class="text-right">
								<Button variant="ghost" size="icon-sm" onclick={() => openSetDialog('edit', planSet)}>
									<Pencil class="h-4 w-4" />
								</Button>
								{#if planSet.is_default !== 1}
									<Button variant="ghost" size="icon-sm" onclick={() => handleDeletePlanSet(planSet)}>
										<Trash2 class="h-4 w-4 text-destructive" />
									</Button>
								{/if}
							</Table.Cell>
						</Table.Row>

						<!-- Expanded Plans Section -->
						{#if expandedSet === planSet.id}
							<Table.Row>
								<Table.Cell colspan={9} class="bg-muted/50 p-0">
									<div class="p-4">
										<div class="mb-3 flex flex-wrap items-center justify-between gap-3">
											<h3 class="text-base font-semibold">{t('planSets.plansInSet')}</h3>
											<div class="flex flex-wrap items-center gap-3">
												<!-- Set Standard Plan -->
												<div class="flex items-center gap-2">
													<Label class="text-sm whitespace-nowrap">{t('planSets.setAsStandard')}</Label>
													<select
														class="h-8 rounded-md border border-input bg-background px-2 text-sm"
														value={planSet.standard_plan_id || ''}
														onchange={(e) => handleSetStandardPlan(planSet.id, (e.target as HTMLSelectElement).value)}
													>
														<option value="">{t('common.none')}</option>
														{#each planSet.plans || [] as p (p.id)}
															<option value={p.id}>{p.name}</option>
														{/each}
													</select>
												</div>
												<!-- Set Global Rate Plan -->
												<div class="flex items-center gap-2">
													<Label class="text-sm whitespace-nowrap">{t('planSets.setAsGlobalRate')}</Label>
													<select
														class="h-8 rounded-md border border-input bg-background px-2 text-sm"
														value={planSet.global_rate_plan_id || ''}
														onchange={(e) => handleSetGlobalRatePlan(planSet.id, (e.target as HTMLSelectElement).value)}
													>
														<option value="">{t('common.none')}</option>
														{#each (planSet.plans || []).filter((p: any) => p.type === 'rate') as p (p.id)}
															<option value={p.id}>{p.name}</option>
														{/each}
													</select>
												</div>
												<Button size="sm" onclick={() => openPlanDialog('create', planSet.id)}>
													<Plus class="mr-1 h-4 w-4" />
													{t('planSets.addPlan')}
												</Button>
											</div>
										</div>

										<!-- Plans sub-table -->
										<div class="rounded-md border bg-background">
											<Table.Root>
												<Table.Header>
													<Table.Row>
														<Table.Head>{t('common.name')}</Table.Head>
														<Table.Head>{t('planSets.type')}</Table.Head>
														<Table.Head>{t('planSets.rate')}</Table.Head>
														<Table.Head>{t('planSets.price')}</Table.Head>
														<Table.Head>{t('planSets.givesMinutes')}</Table.Head>
														<Table.Head>{t('planSets.unit')}</Table.Head>
														<Table.Head>{t('common.status')}</Table.Head>
														<Table.Head class="text-right">{t('common.actions')}</Table.Head>
													</Table.Row>
												</Table.Header>
												<Table.Body>
													{#if !planSet.plans || planSet.plans.length === 0}
														<Table.Row>
															<Table.Cell colspan={8} class="py-6 text-center text-muted-foreground">
																{t('planSets.noPlans')}
															</Table.Cell>
														</Table.Row>
													{:else}
														{#each planSet.plans as plan (plan.id)}
															<Table.Row>
																<Table.Cell>{plan.name}</Table.Cell>
																<Table.Cell>
																	<Badge variant="secondary">{plan.type}</Badge>
																</Table.Cell>
																<Table.Cell>
																	{plan.type === 'rate' ? formatCurrency(plan.rate_cents) : '\u2014'}
																</Table.Cell>
																<Table.Cell>
																	{plan.type === 'package' ? formatCurrency(plan.price_cents) : '\u2014'}
																</Table.Cell>
																<Table.Cell>
																	{plan.type === 'package' ? plan.gives_minutes : '\u2014'}
																</Table.Cell>
																<Table.Cell>{plan.unit}</Table.Cell>
																<Table.Cell>
																	<div class="flex items-center gap-2">
																		<Badge variant={plan.status === 1 ? 'default' : 'outline'}>
																			{plan.status === 1 ? t('common.active') : t('common.inactive')}
																		</Badge>
																		<Switch
																			checked={plan.status === 1}
																			onCheckedChange={() => handleTogglePlanStatus(plan)}
																			size="sm"
																		/>
																	</div>
																</Table.Cell>
																<Table.Cell class="text-right">
																	<Button
																		variant="ghost"
																		size="icon-sm"
																		onclick={() => openPlanDialog('edit', planSet.id, plan)}
																	>
																		<Pencil class="h-4 w-4" />
																	</Button>
																	<Button
																		variant="ghost"
																		size="icon-sm"
																		onclick={() => handleDeletePlan(plan.id)}
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
								</Table.Cell>
							</Table.Row>
						{/if}
					{/each}
				{/if}
			</Table.Body>
		</Table.Root>
	</div>

	<!-- Deleted Plan Sets -->
	{#if showDeleted}
		<div class="space-y-2">
			<h3 class="text-lg font-semibold">{t('common.deleted')} {t('planSets.title')}</h3>
			<div class="rounded-md border bg-muted/30">
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<Table.Head>{t('common.name')}</Table.Head>
							<Table.Head>{t('common.description')}</Table.Head>
							<Table.Head>{t('common.deletedAt')}</Table.Head>
							<Table.Head class="text-right">{t('common.actions')}</Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#if deletedSets.length === 0}
							<Table.Row>
								<Table.Cell colspan={4} class="py-6 text-center text-muted-foreground">
									{t('common.noData')}
								</Table.Cell>
							</Table.Row>
						{:else}
							{#each deletedSets as planSet (planSet.id)}
								<Table.Row>
									<Table.Cell>{planSet.name}</Table.Cell>
									<Table.Cell>{planSet.description || '\u2014'}</Table.Cell>
									<Table.Cell>
										{planSet.deleted_at ? new Date(planSet.deleted_at).toLocaleString() : '-'}
									</Table.Cell>
									<Table.Cell class="text-right">
										<Button
											variant="ghost"
											size="icon-sm"
											onclick={() => handleRecoverPlanSet(planSet.id)}
										>
											<ArchiveRestore class="h-4 w-4 text-green-600" />
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

<!-- Plan Set Create/Edit Dialog -->
<Dialog.Root bind:open={setDialogOpen}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>
				{setDialogMode === 'create' ? t('planSets.addPlanSet') : t('planSets.editPlanSet')}
			</Dialog.Title>
		</Dialog.Header>
		<div class="space-y-4 py-4">
			<div class="space-y-2">
				<Label for="set-name">{t('common.name')}</Label>
				<Input id="set-name" bind:value={setFormName} autofocus />
			</div>
			<div class="space-y-2">
				<Label for="set-description">{t('common.description')}</Label>
				<textarea
					id="set-description"
					class="flex min-h-[80px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
					bind:value={setFormDescription}
					rows={3}
				></textarea>
			</div>
			<label class="flex items-center gap-2">
				<Switch bind:checked={setFormIsDefault} />
				<span class="text-sm">{t('planSets.setAsDefault')}</span>
			</label>
		</div>
		<Dialog.Footer>
			<Button variant="outline" onclick={closeSetDialog}>{t('common.cancel')}</Button>
			<Button onclick={handleSavePlanSet}>{t('common.save')}</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<!-- Plan Create/Edit Dialog -->
<Dialog.Root bind:open={planDialogOpen}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>
				{planDialogMode === 'create' ? t('planSets.addPlan') : t('planSets.editPlan')}
			</Dialog.Title>
		</Dialog.Header>
		<div class="space-y-4 py-4">
			<div class="space-y-2">
				<Label for="plan-name">{t('common.name')}</Label>
				<Input id="plan-name" bind:value={planFormName} autofocus />
			</div>

			<!-- Type selection -->
			<div class="space-y-2">
				<Label>{t('planSets.type')}</Label>
				<div class="flex gap-4">
					<label class="flex items-center gap-2">
						<input type="radio" name="plan-type" value="rate" bind:group={planFormType} class="accent-primary" />
						<span class="text-sm">{t('planSets.rate')}</span>
					</label>
					<label class="flex items-center gap-2">
						<input type="radio" name="plan-type" value="package" bind:group={planFormType} class="accent-primary" />
						<span class="text-sm">{t('planSets.price')}</span>
					</label>
				</div>
			</div>

			<!-- Rate field (rate type) -->
			{#if planFormType === 'rate'}
				<div class="space-y-2">
					<Label for="plan-rate">{t('planSets.rateYuan')}</Label>
					<Input
						id="plan-rate"
						type="number"
						step="0.01"
						min="0"
						value={formatCentsToYuan(planFormRateCents)}
						oninput={(e) => { planFormRateCents = formatYuanToCents((e.target as HTMLInputElement).value); }}
					/>
					<p class="text-xs text-muted-foreground">{t('common.current')}: {formatCurrency(planFormRateCents)}</p>
				</div>
			{/if}

			<!-- Price + gives_minutes fields (package type) -->
			{#if planFormType === 'package'}
				<div class="space-y-2">
					<Label for="plan-price">{t('planSets.priceYuan')}</Label>
					<Input
						id="plan-price"
						type="number"
						step="0.01"
						min="0"
						value={formatCentsToYuan(planFormPriceCents)}
						oninput={(e) => { planFormPriceCents = formatYuanToCents((e.target as HTMLInputElement).value); }}
					/>
					<p class="text-xs text-muted-foreground">{t('common.current')}: {formatCurrency(planFormPriceCents)}</p>
				</div>
				<div class="space-y-2">
					<Label for="plan-gives-minutes">{t('planSets.givesMinutes')}</Label>
					<Input
						id="plan-gives-minutes"
						type="number"
						min="0"
						bind:value={planFormGivesMinutes}
					/>
				</div>
			{/if}

			<!-- Unit -->
			<div class="space-y-2">
				<Label for="plan-unit">{t('planSets.unit')}</Label>
				<select
					id="plan-unit"
					class="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
					bind:value={planFormUnit}
				>
					<option value="minute">{t('planSets.minute')}</option>
					<option value="hour">{t('planSets.hour')}</option>
					<option value="day">{t('planSets.day')}</option>
				</select>
			</div>

			<!-- Description -->
			<div class="space-y-2">
				<Label for="plan-description">{t('common.description')}</Label>
				<textarea
					id="plan-description"
					class="flex min-h-[60px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
					bind:value={planFormDescription}
					rows={2}
				></textarea>
			</div>

			<!-- Status -->
			<label class="flex items-center gap-2">
				<Switch
					checked={planFormStatus === 1}
					onCheckedChange={(v) => { planFormStatus = v ? 1 : 0; }}
				/>
				<span class="text-sm">{t('common.active')}</span>
			</label>
		</div>
		<Dialog.Footer>
			<Button variant="outline" onclick={closePlanDialog}>{t('common.cancel')}</Button>
			<Button onclick={handleSavePlan}>{t('common.save')}</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
