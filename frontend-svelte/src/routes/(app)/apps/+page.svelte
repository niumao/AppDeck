<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from '$lib/i18n';
	import { appsAPI, categoriesAPI } from '$lib/api';
	import { toast } from 'svelte-sonner';
	import { goto } from '$app/navigation';
	import { base } from '$app/paths';
	import { Plus, Pencil, PackageMinus, PackagePlus, Search } from '@lucide/svelte';
	import { Input } from '$lib/components/ui/input';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import { Switch } from '$lib/components/ui/switch';
	import { Label } from '$lib/components/ui/label';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import * as Table from '$lib/components/ui/table';
	import * as Select from '$lib/components/ui/select';
	import * as Tabs from '$lib/components/ui/tabs';
	import ConfirmActionDialog from '$lib/components/confirm-action-dialog.svelte';
	import SelectTablePagination from '$lib/components/select-table-pagination.svelte';
	import { runWithToast } from '$lib/action-feedback';

	interface App {
		id: string;
		name: string;
		description: string;
		launchParam: string;
		launchType: string;
		titleId?: string;
		categoryIds: string[];
		clickCount: number;
		order: number;
		deletedAt?: string;
	}

	interface Category {
		id: string;
		name: string;
	}

	const LAUNCH_TYPE_LABELS: Record<string, string> = {
		retroarch: 'RetroArch',
		eden: 'Eden',
		android: 'Android'
	};

	function getLaunchTypeLabel(launchType: string): string {
		return LAUNCH_TYPE_LABELS[launchType] ?? 'Moonlight';
	}

	let apps: App[] = $state([]);
	let categories: Category[] = $state([]);
	let loading = $state(true);
	let filterCategory = $state('');
	let activeTab = $state('eden');
	let searchKeyword = $state('');
	let showDelisted = $state(false);
	let delistedApps: App[] = $state([]);

	// Batch selection state
	let selectedIds: Set<string> = $state(new Set());

	// Pagination state
	let page = $state(0);
	let rowsPerPage = $state(20);

	// Delist dialog state
	let delistDialogOpen = $state(false);
	let delistApp: App | null = $state(null);

	// Batch delist dialog
	let batchDelistDialogOpen = $state(false);

	let appsByOtherFilters = $derived(
		apps.filter((app) => {
			if (searchKeyword && !app.name?.toLowerCase().includes(searchKeyword.toLowerCase())) return false;
			if (filterCategory && !(app.categoryIds && app.categoryIds.includes(filterCategory))) return false;
			return true;
		})
	);

	let tabCounts = $derived({
		moonlight: appsByOtherFilters.filter((a) => a.launchType === 'moonlight').length,
		retroarch: appsByOtherFilters.filter((a) => a.launchType === 'retroarch').length,
		eden: appsByOtherFilters.filter((a) => a.launchType === 'eden').length,
		android: appsByOtherFilters.filter((a) => a.launchType === 'android').length
	});

	let filteredApps = $derived(appsByOtherFilters.filter((app) => app.launchType === activeTab));

	let filteredDelistedApps = $derived(
		delistedApps.filter((app) => {
			if (searchKeyword && !app.name?.toLowerCase().includes(searchKeyword.toLowerCase())) return false;
			if (app.launchType !== activeTab) return false;
			return true;
		})
	);

	// Derived: paginated apps (from filteredApps)
	let paginatedApps = $derived(filteredApps.slice(page * rowsPerPage, (page + 1) * rowsPerPage));

	// Check states for header checkbox
	let allOnPageSelected = $derived(
		paginatedApps.length > 0 && paginatedApps.every((app) => selectedIds.has(String(app.id)))
	);
	let someOnPageSelected = $derived(
		paginatedApps.some((app) => selectedIds.has(String(app.id))) && !allOnPageSelected
	);

	onMount(() => {
		loadData();
	});

	// Reset page when filter changes
	$effect(() => {
		filterCategory;
		activeTab;
		searchKeyword;
		page = 0;
	});

	// Clear batch selection when switching launch-type tabs
	$effect(() => {
		activeTab;
		selectedIds = new Set();
	});

	async function loadData() {
		selectedIds = new Set();
		try {
			loading = true;
			const [catsData, appsData] = await Promise.all([
				categoriesAPI.getAll(),
				appsAPI.getAll()
			]);
			categories = catsData || [];
			apps = appsData || [];
		} catch (error: any) {
			toast.error(t('common.loadFailed') + ': ' + error.message);
		} finally {
			loading = false;
		}
	}

	async function loadDelistedApps() {
		try {
			const data = await appsAPI.getDeleted();
			delistedApps = data || [];
		} catch (error: any) {
			toast.error(t('common.loadFailed') + ': ' + error.message);
		}
	}

	function getCategoryNames(categoryIds: string[]): string[] {
		if (!categoryIds || categoryIds.length === 0) return [];
		return categoryIds.map((id) => {
			const category = categories.find((c) => c.id === id);
			return category ? category.name : id;
		});
	}

	function openDelistDialog(app: App) {
		delistApp = app;
		delistDialogOpen = true;
	}

	async function handleDelist() {
		if (!delistApp) return;

		const ok = await runWithToast(() => appsAPI.delete(delistApp!.id), {
			successMessage: t('apps.delistSuccess'),
			errorFallback: t('apps.delistFailed'),
			logLabel: 'Failed to delist app:',
		});
		if (ok) {
			delistDialogOpen = false;
			delistApp = null;
			loadData();
			if (showDelisted) loadDelistedApps();
		}
	}

	async function handleRelist(id: string) {
		try {
			await appsAPI.recover(id);
			toast.success(t('apps.relistSuccess'));
			loadDelistedApps();
			loadData();
		} catch (error: any) {
			toast.error(t('apps.relistFailed') + ': ' + error.message);
		}
	}

	function handleShowDelistedToggle(checked: boolean) {
		showDelisted = checked;
		if (checked) loadDelistedApps();
	}

	// Track last clicked index for Shift+click range selection
	let lastClickedIndex: number | null = $state(null);

	function toggleSelectAll() {
		if (allOnPageSelected) {
			const next = new Set(selectedIds);
			paginatedApps.forEach((app) => next.delete(String(app.id)));
			selectedIds = next;
		} else {
			const next = new Set(selectedIds);
			paginatedApps.forEach((app) => next.add(String(app.id)));
			selectedIds = next;
		}
		lastClickedIndex = null;
	}

	function toggleSelect(id: string, index: number, event?: MouseEvent) {
		const next = new Set(selectedIds);

		if (event?.shiftKey && lastClickedIndex !== null) {
			// Shift+click: select range between last clicked and current
			const start = Math.min(lastClickedIndex, index);
			const end = Math.max(lastClickedIndex, index);
			for (let i = start; i <= end; i++) {
				next.add(String(paginatedApps[i].id));
			}
		} else {
			// Normal click: toggle single item
			if (next.has(id)) {
				next.delete(id);
			} else {
				next.add(id);
			}
		}

		lastClickedIndex = index;
		selectedIds = next;
	}

	async function handleBatchDelist() {
		if (selectedIds.size === 0) return;
		const ok = await runWithToast(() => appsAPI.batchDelist(Array.from(selectedIds)), {
			successMessage: t('apps.delistSuccess'),
			errorFallback: t('apps.delistFailed'),
			logLabel: 'Failed to batch delist apps:',
		});
		if (ok) {
			batchDelistDialogOpen = false;
			selectedIds = new Set();
			loadData();
			if (showDelisted) loadDelistedApps();
		}
	}
</script>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<h2 class="text-2xl font-bold tracking-tight">{t('apps.title')}</h2>
	</div>

	<div class="flex items-center justify-between">
		<div>
			{#if selectedIds.size > 0}
				<div class="flex items-center gap-2">
					<span class="text-sm text-muted-foreground">
						{t('apps.selectedCount').replace('{count}', String(selectedIds.size))}
					</span>
					<Button variant="destructive" size="sm" onclick={() => (batchDelistDialogOpen = true)}>
						<PackageMinus class="mr-2 h-4 w-4" />
						{t('apps.batchDelist')}
					</Button>
				</div>
			{/if}
		</div>
		<div class="flex items-center gap-4">
			<div class="flex items-center gap-2">
				<Switch
					checked={showDelisted}
					onCheckedChange={handleShowDelistedToggle}
					size="sm"
				/>
				<Label class="text-sm">{t('apps.showDelisted')}</Label>
			</div>
			<Button onclick={() => goto(`${base}/apps/new`)}>
				<Plus class="mr-2 h-4 w-4" />
				{t('apps.add')}
			</Button>
		</div>
	</div>

	<!-- Launch type tabs -->
	<Tabs.Root bind:value={activeTab}>
		<Tabs.List>
			<Tabs.Trigger value="eden">{getLaunchTypeLabel('eden')} ({tabCounts.eden})</Tabs.Trigger>
			<Tabs.Trigger value="retroarch">{getLaunchTypeLabel('retroarch')} ({tabCounts.retroarch})</Tabs.Trigger>
			<Tabs.Trigger value="android">{getLaunchTypeLabel('android')} ({tabCounts.android})</Tabs.Trigger>
			<Tabs.Trigger value="moonlight">{getLaunchTypeLabel('moonlight')} ({tabCounts.moonlight})</Tabs.Trigger>
		</Tabs.List>
	</Tabs.Root>

	<!-- Filters -->
	<div class="flex items-center gap-3">
		<div class="relative">
			<Input
				placeholder={t('apps.searchNamePlaceholder')}
				bind:value={searchKeyword}
				class="w-64 pr-9"
			/>
			<div class="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground">
				<Search class="h-4 w-4" />
			</div>
		</div>
		<Select.Root type="single" bind:value={filterCategory}>
			<Select.Trigger class="w-[200px]">
				{filterCategory
					? categories.find((c) => String(c.id) === filterCategory)?.name || filterCategory
					: t('apps.allCategories')}
			</Select.Trigger>
			<Select.Content>
				<Select.Item value="">{t('apps.allCategories')}</Select.Item>
				{#each categories as category (category.id)}
					<Select.Item value={String(category.id)}>{category.name}</Select.Item>
				{/each}
			</Select.Content>
		</Select.Root>
	</div>

	<!-- Apps table -->
	<div class="rounded-md border">
		<Table.Root>
			<Table.Header>
				<Table.Row>
					<Table.Head class="w-[40px]">
						<Checkbox
							checked={allOnPageSelected}
							indeterminate={someOnPageSelected}
							onCheckedChange={toggleSelectAll}
						/>
					</Table.Head>
					<Table.Head class="w-[14%]">{t('apps.appName')}</Table.Head>
					<Table.Head class="w-[16%]">{t('apps.launchParam')}</Table.Head>
					<Table.Head class="w-[10%]">{t('apps.launchType')}</Table.Head>
					<Table.Head class="w-[12%]">{t('apps.titleId')}</Table.Head>
					<Table.Head class="w-[14%]">{t('apps.description')}</Table.Head>
					<Table.Head class="w-[14%]">{t('apps.categories')}</Table.Head>
					<Table.Head class="w-[6%]">{t('apps.clickCount')}</Table.Head>
					<Table.Head class="w-[6%]">{t('apps.order')}</Table.Head>
					<Table.Head class="w-[10%] text-right">{t('common.actions')}</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#if loading}
					<Table.Row>
						<Table.Cell colspan={10} class="text-center">
							{t('common.loading')}
						</Table.Cell>
					</Table.Row>
				{:else if filteredApps.length === 0}
					<Table.Row>
						<Table.Cell colspan={10} class="text-center">
							{t('common.noData')}
						</Table.Cell>
					</Table.Row>
				{:else}
					{#each paginatedApps as app, idx (app.id)}
						<Table.Row>
							<Table.Cell onclick={(e: MouseEvent) => { e.stopPropagation(); toggleSelect(String(app.id), idx, e); }}>
								<Checkbox
									checked={selectedIds.has(String(app.id))}
								/>
							</Table.Cell>
							<Table.Cell class="max-w-0 truncate font-medium" title={app.name || ''}>
								{app.name}
							</Table.Cell>
							<Table.Cell class="max-w-0 truncate" title={app.launchParam || ''}>
								{app.launchParam}
							</Table.Cell>
							<Table.Cell>
								<Badge variant={app.launchType === 'moonlight' ? 'outline' : 'secondary'}>
									{getLaunchTypeLabel(app.launchType)}
								</Badge>
							</Table.Cell>
							<Table.Cell class="max-w-0 truncate" title={app.titleId || ''}>
								{app.titleId || '—'}
							</Table.Cell>
							<Table.Cell class="max-w-0 truncate" title={app.description || ''}>
								{app.description}
							</Table.Cell>
							<Table.Cell>
								<div class="flex gap-1 overflow-hidden">
									{#each getCategoryNames(app.categoryIds) as name}
										<Badge variant="secondary" class="shrink-0">{name}</Badge>
									{/each}
								</div>
							</Table.Cell>
							<Table.Cell>{app.clickCount}</Table.Cell>
							<Table.Cell>{app.order}</Table.Cell>
							<Table.Cell class="text-right whitespace-nowrap">
								<Button
									variant="ghost"
									size="icon"
									onclick={() => goto(`${base}/apps/${app.id}`)}
								>
									<Pencil class="h-4 w-4" />
								</Button>
								<Button
									variant="ghost"
									size="icon"
									onclick={() => openDelistDialog(app)}
								>
									<PackageMinus class="h-4 w-4 text-destructive" />
								</Button>
							</Table.Cell>
						</Table.Row>
					{/each}
				{/if}
			</Table.Body>
		</Table.Root>
	</div>

	<!-- Pagination -->
	<SelectTablePagination
		bind:page
		bind:rowsPerPage
		total={filteredApps.length}
		rowsPerPageOptions={[20, 50, 100]}
		showPageNumber
		containerClass="border rounded-md px-4 py-3"
	/>

	<!-- Deleted apps table -->
	{#if showDelisted}
		<div class="mt-6">
			<h3 class="text-lg font-semibold mb-2">
				{t('apps.delisted')} {t('apps.title')}
			</h3>
			<div class="rounded-md border bg-muted/30">
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<Table.Head>{t('apps.appName')}</Table.Head>
							<Table.Head>{t('apps.launchParam')}</Table.Head>
							<Table.Head>{t('apps.launchType')}</Table.Head>
							<Table.Head>{t('apps.titleId')}</Table.Head>
							<Table.Head>{t('apps.description')}</Table.Head>
							<Table.Head>{t('apps.categories')}</Table.Head>
							<Table.Head>{t('apps.delistedAt')}</Table.Head>
							<Table.Head class="text-right">{t('common.actions')}</Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#if filteredDelistedApps.length === 0}
							<Table.Row>
								<Table.Cell colspan={8} class="text-center">
									{t('common.noData')}
								</Table.Cell>
							</Table.Row>
						{:else}
							{#each filteredDelistedApps as app (app.id)}
								<Table.Row>
									<Table.Cell class="font-medium" title={app.name || ''}>
										{app.name}
									</Table.Cell>
									<Table.Cell title={app.launchParam || ''}>
										{app.launchParam}
									</Table.Cell>
									<Table.Cell>
										<Badge variant={app.launchType === 'moonlight' ? 'outline' : 'secondary'}>
											{getLaunchTypeLabel(app.launchType)}
										</Badge>
									</Table.Cell>
									<Table.Cell title={app.titleId || ''}>
										{app.titleId || '—'}
									</Table.Cell>
									<Table.Cell title={app.description || ''}>
										<span class="block max-w-[200px] truncate">{app.description}</span>
									</Table.Cell>
									<Table.Cell>
										<div class="flex gap-1 flex-wrap">
											{#each getCategoryNames(app.categoryIds) as name}
												<Badge variant="secondary">{name}</Badge>
											{/each}
										</div>
									</Table.Cell>
									<Table.Cell>
										{app.deletedAt ? new Date(app.deletedAt).toLocaleString() : '-'}
									</Table.Cell>
									<Table.Cell class="text-right">
										<Button
											variant="ghost"
											size="icon"
											onclick={() => handleRelist(app.id)}
											title={t('apps.relist')}
										>
											<PackagePlus class="h-4 w-4 text-green-600" />
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

<!-- Delist Confirm Dialog -->
<ConfirmActionDialog
	bind:open={delistDialogOpen}
	title={t('apps.delist')}
	description={t('apps.confirmDelistMsg').replace('{name}', delistApp?.name || '')}
	confirmLabel={t('apps.delist')}
	confirmVariant="destructive"
	onConfirm={handleDelist}
	onCancel={() => (delistApp = null)}
>
	{#snippet children()}
		<div class="rounded-md border border-yellow-200 bg-yellow-50 p-3 text-sm text-yellow-800 dark:border-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-200">
			{t('apps.delistWarning')}
		</div>
	{/snippet}
</ConfirmActionDialog>

<!-- Batch Delist Confirm Dialog -->
<ConfirmActionDialog
	bind:open={batchDelistDialogOpen}
	title={t('apps.batchDelist')}
	description={t('apps.confirmBatchDelistMsg').replace('{count}', String(selectedIds.size))}
	confirmLabel={t('common.confirm')}
	confirmVariant="destructive"
	onConfirm={handleBatchDelist}
>
	{#snippet children()}
		<div class="rounded-md border border-yellow-200 bg-yellow-50 p-3 text-sm text-yellow-800 dark:border-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-200">
			{t('apps.delistWarning')}
		</div>
	{/snippet}
</ConfirmActionDialog>
