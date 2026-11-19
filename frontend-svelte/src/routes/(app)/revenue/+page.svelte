<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from '$lib/i18n';
	import { revenueAPI } from '$lib/api';
	import { toast } from 'svelte-sonner';
	import { DollarSign, TrendingUp, Users, Gift, RefreshCw, LoaderCircle } from '@lucide/svelte';
	import * as Card from '$lib/components/ui/card';
	import * as Table from '$lib/components/ui/table';
	import * as Tabs from '$lib/components/ui/tabs';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import SelectTablePagination from '$lib/components/select-table-pagination.svelte';

	const defaultSummary = {
		total_revenue_cents: 0,
		total_sessions: 0,
		total_duration_minutes: 0,
		active_users_count: 0,
		global_package_sales_cents: 0,
		tenant_package_sales_cents: 0,
		total_refunds_cents: 0,
		net_revenue_cents: 0,
		total_bonus_cents: 0,
		breakdown: {
			items: [] as any[],
			total: 0,
			page: 1,
			limit: 20
		}
	};

	let loading = $state(true);
	let currentTab = $state('overview');
	let period = $state('daily');
	let dateFrom = $state('');
	let dateTo = $state('');
	let summary: typeof defaultSummary = $state({ ...defaultSummary });

	// Pagination (0-indexed internally)
	let page = $state(0);
	let rowsPerPage = $state(20);

	onMount(() => {
		const today = new Date();
		const thirtyDaysAgo = new Date(today);
		thirtyDaysAgo.setDate(today.getDate() - 30);

		dateTo = formatDateForInput(today);
		dateFrom = formatDateForInput(thirtyDaysAgo);
	});

	// Reload overview when dates change and overview tab is active
	$effect(() => {
		if (dateFrom && dateTo && currentTab === 'overview') {
			loadRevenueSummary();
		}
	});

	// Reload breakdown when filters/pagination change and details tab is active
	$effect(() => {
		if (dateFrom && dateTo && currentTab === 'details') {
			// Access reactive deps so Svelte tracks them
			period;
			page;
			rowsPerPage;
			loadRevenueBreakdown();
		}
	});

	function formatDateForInput(date: Date): string {
		return date.toISOString().split('T')[0];
	}

	async function loadRevenueSummary() {
		try {
			loading = true;
			const data = await revenueAPI.getSummary({
				date_from: dateFrom,
				date_to: dateTo
			});
			summary = data || { ...defaultSummary };
		} catch (error: any) {
			console.error('Failed to load revenue summary:', error);
			toast.error(error.message || t('revenue.loadFailed'));
		} finally {
			loading = false;
		}
	}

	async function loadRevenueBreakdown() {
		try {
			loading = true;
			const data = await revenueAPI.getSummary({
				period,
				date_from: dateFrom,
				date_to: dateTo,
				page: page + 1,
				limit: rowsPerPage
			});
			summary = data || { ...defaultSummary };
		} catch (error: any) {
			console.error('Failed to load revenue breakdown:', error);
			toast.error(error.message || t('revenue.loadFailed'));
		} finally {
			loading = false;
		}
	}

	function handleRefresh() {
		if (currentTab === 'overview') {
			loadRevenueSummary();
		} else {
			loadRevenueBreakdown();
		}
	}

	function handlePeriodChange(newPeriod: string) {
		period = newPeriod;
		page = 0;
	}

	function formatCurrency(cents: number): string {
		return `\u00A5${(cents / 100).toFixed(2)}`;
	}

	function formatDuration(minutes: number): string {
		if (minutes < 60) {
			return t('revenue.minutes', { n: minutes });
		}
		const hours = Math.floor(minutes / 60);
		const mins = minutes % 60;
		return t('revenue.hoursMinutes', { h: hours, m: mins });
	}

	function formatPeriodRange(periodStart: string, periodEnd: string): string {
		const start = new Date(periodStart);
		const end = new Date(periodEnd);
		if (period === 'daily') {
			return start.toLocaleDateString('zh-CN', {
				month: 'short',
				day: 'numeric',
				weekday: 'short'
			});
		} else if (period === 'weekly') {
			return `${start.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' })} - ${end.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' })}`;
		} else {
			return start.toLocaleDateString('zh-CN', { year: 'numeric', month: 'long' });
		}
	}

	const summaryCards = $derived([
		{
			title: t('revenue.globalPackageSales'),
			value: formatCurrency(summary.global_package_sales_cents || 0),
			icon: DollarSign,
			color: 'text-blue-500'
		},
		{
			title: t('revenue.tenantPackageSales'),
			value: formatCurrency(summary.tenant_package_sales_cents || 0),
			icon: DollarSign,
			color: 'text-green-500'
		},
		{
			title: t('revenue.totalRefunds'),
			value: formatCurrency(summary.total_refunds_cents || 0),
			icon: TrendingUp,
			color: 'text-red-500'
		},
		{
			title: t('revenue.netRevenue'),
			value: formatCurrency(summary.net_revenue_cents || 0),
			icon: DollarSign,
			color: 'text-purple-500'
		},
		{
			title: t('revenue.totalBonusIssued'),
			value: formatCurrency(summary.total_bonus_cents || 0),
			icon: Gift,
			color: 'text-orange-500'
		}
	]);
</script>

<div class="space-y-4">
	<!-- Header -->
	<div class="flex items-center justify-between">
		<h2 class="text-2xl font-bold tracking-tight">{t('revenue.title')}</h2>
		<Button onclick={handleRefresh} disabled={loading}>
			<RefreshCw class="mr-2 h-4 w-4" />
			{t('revenue.refreshData')}
		</Button>
	</div>

	<!-- Main Tabs: Overview / Details -->
	<Tabs.Root bind:value={currentTab}>
		<Tabs.List>
			<Tabs.Trigger value="overview">{t('revenue.overview')}</Tabs.Trigger>
			<Tabs.Trigger value="details">{t('revenue.details')}</Tabs.Trigger>
		</Tabs.List>

		<!-- Overview Tab -->
		<Tabs.Content value="overview">
			{#if loading}
				<div class="flex min-h-[400px] items-center justify-center">
					<LoaderCircle class="h-8 w-8 animate-spin text-muted-foreground" />
				</div>
			{:else}
				<!-- Date Range Filter -->
				<div class="rounded-lg border bg-card p-4 mb-4">
					<div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
						<div>
							<label class="mb-1.5 block text-sm font-medium text-muted-foreground"
								>{t('revenue.startDate')}</label
							>
							<Input type="date" bind:value={dateFrom} />
						</div>
						<div>
							<label class="mb-1.5 block text-sm font-medium text-muted-foreground"
								>{t('revenue.endDate')}</label
							>
							<Input type="date" bind:value={dateTo} />
						</div>
						<div class="flex items-end">
							<Button class="w-full" variant="outline" onclick={handleRefresh} disabled={loading}>
								{t('common.applyFilter')}
							</Button>
						</div>
					</div>
				</div>

				<!-- Summary Cards -->
				<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5">
					{#each summaryCards as card}
						<Card.Root>
							<Card.Content class="p-4">
								<div class="flex items-center justify-between">
									<div>
										<p class="text-sm text-muted-foreground">{card.title}</p>
										<p class="text-2xl font-bold">{card.value}</p>
									</div>
									<div class={card.color}>
										<card.icon class="h-8 w-8" />
									</div>
								</div>
							</Card.Content>
						</Card.Root>
					{/each}
				</div>
			{/if}
		</Tabs.Content>

		<!-- Details Tab -->
		<Tabs.Content value="details">
			{#if loading}
				<div class="flex min-h-[400px] items-center justify-center">
					<LoaderCircle class="h-8 w-8 animate-spin text-muted-foreground" />
				</div>
			{:else}
				<!-- Period Tabs + Date Filters -->
				<div class="rounded-lg border bg-card p-4 mb-4 space-y-4">
					<!-- Period sub-tabs -->
					<Tabs.Root value={period} onValueChange={(v) => handlePeriodChange(v)}>
						<Tabs.List>
							<Tabs.Trigger value="daily">{t('revenue.daily')}</Tabs.Trigger>
							<Tabs.Trigger value="weekly">{t('revenue.weekly')}</Tabs.Trigger>
							<Tabs.Trigger value="monthly">{t('revenue.monthly')}</Tabs.Trigger>
						</Tabs.List>
					</Tabs.Root>

					<!-- Date range -->
					<div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
						<div>
							<label class="mb-1.5 block text-sm font-medium text-muted-foreground"
								>{t('revenue.startDate')}</label
							>
							<Input type="date" bind:value={dateFrom} />
						</div>
						<div>
							<label class="mb-1.5 block text-sm font-medium text-muted-foreground"
								>{t('revenue.endDate')}</label
							>
							<Input type="date" bind:value={dateTo} />
						</div>
						<div class="flex items-end">
							<Button class="w-full" variant="outline" onclick={handleRefresh} disabled={loading}>
								{t('common.query')}
							</Button>
						</div>
					</div>
				</div>

				<!-- Breakdown Table -->
				<div class="rounded-lg border">
					<div class="border-b px-4 py-3">
						<h3 class="text-lg font-semibold">{t('revenue.periodDetails')}</h3>
					</div>

					<Table.Root>
						<Table.Header>
							<Table.Row>
								<Table.Head>{t('revenue.period')}</Table.Head>
								<Table.Head class="text-right">{t('revenue.income')}</Table.Head>
								<Table.Head class="text-right">{t('revenue.sessionCount')}</Table.Head>
								<Table.Head class="text-right">{t('revenue.totalDuration')}</Table.Head>
								<Table.Head class="text-right">{t('revenue.activeUsers')}</Table.Head>
							</Table.Row>
						</Table.Header>
						<Table.Body>
							{#if (summary.breakdown?.items || []).length === 0}
								<Table.Row>
									<Table.Cell colspan={5} class="py-8 text-center text-muted-foreground">
										{t('common.noData')}
									</Table.Cell>
								</Table.Row>
							{:else}
								{#each summary.breakdown.items as item, index (index)}
									<Table.Row>
										<Table.Cell>
											{formatPeriodRange(item.period_start, item.period_end)}
										</Table.Cell>
										<Table.Cell class="text-right">
											{formatCurrency(item.revenue_cents)}
										</Table.Cell>
										<Table.Cell class="text-right">
											{item.session_count.toLocaleString()}
										</Table.Cell>
										<Table.Cell class="text-right">
											{formatDuration(item.duration_minutes)}
										</Table.Cell>
										<Table.Cell class="text-right">
											{item.user_count.toLocaleString()}
										</Table.Cell>
									</Table.Row>
								{/each}
							{/if}
						</Table.Body>
					</Table.Root>

					<!-- Pagination -->
					<SelectTablePagination bind:page bind:rowsPerPage total={summary.breakdown?.total || 0} />
				</div>
			{/if}
		</Tabs.Content>
	</Tabs.Root>
</div>
