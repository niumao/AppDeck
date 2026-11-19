<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from '$lib/i18n';
	import { globalPackagesAPI, refundsAPI } from '$lib/api';
	import { toast } from 'svelte-sonner';
	import { Undo2, LoaderCircle, AlertTriangle } from '@lucide/svelte';
	import SelectTablePagination from '$lib/components/select-table-pagination.svelte';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Table from '$lib/components/ui/table';
	import * as Select from '$lib/components/ui/select';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Badge } from '$lib/components/ui/badge';
	import * as Alert from '$lib/components/ui/alert';

	// Order list
	let orders: any[] = $state([]);
	let total = $state(0);
	let loading = $state(true);

	// Filters
	let searchFilter = $state('');
	let statusFilter = $state('');
	let dateFrom = $state('');
	let dateTo = $state('');

	// Pagination
	let page = $state(0);
	let rowsPerPage = $state(20);

	// Detail dialog
	let detailOpen = $state(false);
	let detailOrder: any = $state(null);

	// Refund dialog
	let refundOpen = $state(false);
	let refundOrder: any = $state(null);
	let refundPreview: any = $state(null);
	let refundProcessing = $state(false);

	onMount(() => {
		loadOrders();
	});

	$effect(() => {
		// Track filter/pagination changes to reload
		// Access the values so Svelte tracks them
		searchFilter;
		statusFilter;
		dateFrom;
		dateTo;
		page;
		rowsPerPage;
		loadOrders();
	});

	async function loadOrders() {
		try {
			loading = true;
			const params: Record<string, any> = {
				page: page + 1,
				limit: rowsPerPage,
				search: searchFilter || undefined,
				status: statusFilter || undefined,
				date_from: dateFrom || undefined,
				date_to: dateTo || undefined
			};
			const data = (await globalPackagesAPI.getOrders(params)) || {};
			orders = data.items || [];
			total = data.total || 0;
		} catch (error) {
			console.error('Failed to load orders:', error);
			toast.error(t('orders.loadFailed'));
		} finally {
			loading = false;
		}
	}

	function handleRowClick(order: any) {
		detailOrder = order;
		detailOpen = true;
	}

	async function handleRefundClick(order: any, event: MouseEvent) {
		event.stopPropagation();
		try {
			const preview = await refundsAPI.preview(order.id);
			refundOrder = order;
			refundPreview = preview;
			refundProcessing = false;
			refundOpen = true;
		} catch (error: any) {
			console.error('Failed to load refund preview:', error);
			toast.error(error.message || 'Failed to load refund preview');
		}
	}

	async function handleConfirmRefund() {
		refundProcessing = true;
		try {
			await refundsAPI.process(refundOrder.id, {});
			toast.success(t('orders.refundSuccess'));
			refundOpen = false;
			refundOrder = null;
			refundPreview = null;
			refundProcessing = false;
			loadOrders();
		} catch (error: any) {
			console.error('Failed to process refund:', error);
			toast.error(error.message || t('orders.refundFailed'));
			refundProcessing = false;
		}
	}

	function handleFilterChange() {
		page = 0;
	}

	function formatCurrency(cents: number): string {
		return `\u00A5${(cents / 100).toFixed(2)}`;
	}

	function formatDate(dateStr: string | null | undefined): string {
		if (!dateStr) return '-';
		return new Date(dateStr).toLocaleString('zh-CN');
	}

	function getStatusLabel(status: number): string {
		const map: Record<number, string> = {
			1: t('orders.paid'),
			0: t('orders.pending'),
			2: t('orders.failed'),
			3: t('orders.refunded')
		};
		return map[status] || t('orders.unknown');
	}

	function getStatusVariant(status: number): 'default' | 'secondary' | 'destructive' | 'outline' {
		const map: Record<number, 'default' | 'secondary' | 'destructive' | 'outline'> = {
			1: 'default',
			0: 'outline',
			2: 'destructive',
			3: 'secondary'
		};
		return map[status] || 'secondary';
	}

	function getScopeLabel(scope: string): string {
		if (scope === 'global') return t('orders.global');
		if (scope === 'tenant') return t('orders.tenant');
		return 'Unknown';
	}

	function getScopeVariant(scope: string): 'default' | 'secondary' | 'outline' {
		if (scope === 'global') return 'outline';
		if (scope === 'tenant') return 'default';
		return 'secondary';
	}

	function getRefundStatusLabel(refundStatus: string): string {
		const map: Record<string, string> = {
			none: t('common.none'),
			partial: t('orders.partial'),
			full: t('orders.full')
		};
		return map[refundStatus] || t('common.none');
	}

	function getRefundStatusVariant(refundStatus: string): 'default' | 'secondary' | 'destructive' | 'outline' {
		const map: Record<string, 'default' | 'secondary' | 'destructive' | 'outline'> = {
			none: 'secondary',
			partial: 'outline',
			full: 'destructive'
		};
		return map[refundStatus] || 'secondary';
	}
</script>

<div class="space-y-4">
	<h2 class="text-2xl font-bold tracking-tight">{t('orders.title')}</h2>

	<!-- Filters -->
	<div class="rounded-lg border bg-card p-4">
		<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
			<div>
				<label class="mb-1.5 block text-sm font-medium text-muted-foreground">{t('common.search')}</label>
				<Input
					type="text"
					placeholder={t('orders.searchPlaceholder')}
					bind:value={searchFilter}
					oninput={handleFilterChange}
				/>
			</div>
			<div>
				<label class="mb-1.5 block text-sm font-medium text-muted-foreground">{t('orders.statusLabel')}</label>
				<Select.Root type="single" bind:value={statusFilter} onValueChange={() => handleFilterChange()}>
					<Select.Trigger class="w-full">
						{statusFilter === '' ? t('common.all') : statusFilter === '0' ? t('orders.pending') : statusFilter === '1' ? t('orders.paid') : statusFilter === '2' ? t('orders.failed') : t('orders.refunded')}
					</Select.Trigger>
					<Select.Content>
						<Select.Item value="">{t('common.all')}</Select.Item>
						<Select.Item value="0">{t('orders.pending')}</Select.Item>
						<Select.Item value="1">{t('orders.paid')}</Select.Item>
						<Select.Item value="2">{t('orders.failed')}</Select.Item>
						<Select.Item value="3">{t('orders.refunded')}</Select.Item>
					</Select.Content>
				</Select.Root>
			</div>
			<div>
				<label class="mb-1.5 block text-sm font-medium text-muted-foreground">{t('orders.startDate')}</label>
				<Input
					type="date"
					bind:value={dateFrom}
					onchange={handleFilterChange}
				/>
			</div>
			<div>
				<label class="mb-1.5 block text-sm font-medium text-muted-foreground">{t('orders.endDate')}</label>
				<Input
					type="date"
					bind:value={dateTo}
					onchange={handleFilterChange}
				/>
			</div>
		</div>
	</div>

	<!-- Orders Table -->
	<div class="rounded-lg border">
		<Table.Root>
			<Table.Header>
				<Table.Row>
					<Table.Head>{t('orders.orderNo')}</Table.Head>
					<Table.Head>{t('common.user')}</Table.Head>
					<Table.Head>{t('orders.package')}</Table.Head>
					<Table.Head>{t('orders.amount')}</Table.Head>
					<Table.Head>{t('orders.scope')}</Table.Head>
					<Table.Head>{t('orders.refundStatus')}</Table.Head>
					<Table.Head>{t('orders.refundedAmount')}</Table.Head>
					<Table.Head>{t('common.status')}</Table.Head>
					<Table.Head>{t('common.createdAt')}</Table.Head>
					<Table.Head class="text-right">{t('common.actions')}</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#if loading}
					<Table.Row>
						<Table.Cell colspan={10} class="text-center py-8">
							<LoaderCircle class="mx-auto h-6 w-6 animate-spin text-muted-foreground" />
						</Table.Cell>
					</Table.Row>
				{:else if orders.length === 0}
					<Table.Row>
						<Table.Cell colspan={10} class="text-center py-8 text-muted-foreground">
							{t('orders.noOrders')}
						</Table.Cell>
					</Table.Row>
				{:else}
					{#each orders as order (order.id)}
						<Table.Row
							class="cursor-pointer"
							onclick={() => handleRowClick(order)}
						>
							<Table.Cell>
								<span class="font-mono text-sm">{order.order_no}</span>
							</Table.Cell>
							<Table.Cell>
								<div class="flex items-center gap-2">
									{#if order.user_avatar_url}
										<img
											src={order.user_avatar_url}
											alt={order.user_nickname}
											class="h-6 w-6 rounded-full object-cover"
										/>
									{:else}
										<div class="flex h-6 w-6 items-center justify-center rounded-full bg-muted text-xs font-medium">
											{(order.user_nickname || 'U').charAt(0)}
										</div>
									{/if}
									<span class="text-sm">
										{order.user_nickname || `${t('common.user')}${order.user_id}`}
									</span>
									{#if order.user_nickname && order.user_status === 0}
										<Badge variant="destructive" class="text-[0.65rem] h-4 px-1">{t('common.disabled')}</Badge>
									{/if}
								</div>
							</Table.Cell>
							<Table.Cell>{order.package_name || '-'}</Table.Cell>
							<Table.Cell>{formatCurrency(order.amount_cents)}</Table.Cell>
							<Table.Cell>
								<Badge variant={getScopeVariant(order.scope)}>{getScopeLabel(order.scope)}</Badge>
							</Table.Cell>
							<Table.Cell>
								<Badge variant={getRefundStatusVariant(order.refund_status)}>{getRefundStatusLabel(order.refund_status)}</Badge>
							</Table.Cell>
							<Table.Cell>{formatCurrency(order.refunded_cents || 0)}</Table.Cell>
							<Table.Cell>
								<Badge variant={getStatusVariant(order.status)}>{getStatusLabel(order.status)}</Badge>
							</Table.Cell>
							<Table.Cell class="text-sm">{formatDate(order.created_at)}</Table.Cell>
							<Table.Cell class="text-right">
								<Button
									variant="ghost"
									size="icon-sm"
									disabled={order.refund_status === 'full' || order.status !== 1}
									onclick={(e: MouseEvent) => handleRefundClick(order, e)}
									title="Refund"
								>
									<Undo2 class="h-4 w-4 text-orange-500" />
								</Button>
							</Table.Cell>
						</Table.Row>
					{/each}
				{/if}
			</Table.Body>
		</Table.Root>

		<!-- Pagination -->
		<SelectTablePagination bind:page bind:rowsPerPage {total} rowsPerPageOptions={[10, 20, 50, 100]} />
	</div>
</div>

<!-- Order Detail Dialog -->
<Dialog.Root bind:open={detailOpen}>
	<Dialog.Content class="sm:max-w-lg">
		<Dialog.Header>
			<Dialog.Title>{t('orders.orderDetail')}</Dialog.Title>
		</Dialog.Header>
		{#if detailOrder}
			<div class="grid gap-4 pt-2">
				<div>
					<p class="text-sm text-muted-foreground">{t('orders.orderNo')}</p>
					<p class="font-mono">{detailOrder.order_no}</p>
				</div>
				<div>
					<p class="text-sm text-muted-foreground">{t('orders.wxTransactionId')}</p>
					<p class="font-mono">{detailOrder.wx_transaction_id || '-'}</p>
				</div>
				<div>
					<p class="text-sm text-muted-foreground">{t('common.user')}</p>
					<div class="mt-1 flex items-center gap-2">
						{#if detailOrder.user_avatar_url}
							<img
								src={detailOrder.user_avatar_url}
								alt={detailOrder.user_nickname}
								class="h-8 w-8 rounded-full object-cover"
							/>
						{:else}
							<div class="flex h-8 w-8 items-center justify-center rounded-full bg-muted text-sm font-medium">
								{(detailOrder.user_nickname || 'U').charAt(0)}
							</div>
						{/if}
						<span>{detailOrder.user_nickname || `${t('common.user')}${detailOrder.user_id}`}</span>
						{#if detailOrder.user_nickname && detailOrder.user_status === 0}
							<Badge variant="destructive">{t('common.disabled')}</Badge>
						{/if}
					</div>
				</div>
				<div>
					<p class="text-sm text-muted-foreground">{t('orders.package')}</p>
					<p>{detailOrder.package_name || '-'}</p>
				</div>
				<div class="grid grid-cols-2 gap-4">
					<div>
						<p class="text-sm text-muted-foreground">{t('orders.amount')}</p>
						<p>{formatCurrency(detailOrder.amount_cents)}</p>
					</div>
					<div>
						<p class="text-sm text-muted-foreground">{t('orders.bonus')}</p>
						<p>{formatCurrency(detailOrder.bonus_cents)}</p>
					</div>
				</div>
				<div>
					<p class="text-sm text-muted-foreground">{t('orders.creditedAmount')}</p>
					<p>{formatCurrency((detailOrder.amount_cents || 0) + (detailOrder.bonus_cents || 0))}</p>
				</div>
				<div>
					<p class="text-sm text-muted-foreground">{t('common.status')}</p>
					<Badge variant={getStatusVariant(detailOrder.status)}>{getStatusLabel(detailOrder.status)}</Badge>
				</div>
				<div class="grid grid-cols-2 gap-4">
					<div>
						<p class="text-sm text-muted-foreground">{t('common.createdAt')}</p>
						<p class="text-sm">{formatDate(detailOrder.created_at)}</p>
					</div>
					<div>
						<p class="text-sm text-muted-foreground">{t('orders.paidAt')}</p>
						<p class="text-sm">{formatDate(detailOrder.paid_at)}</p>
					</div>
				</div>
			</div>
		{/if}
		<Dialog.Footer>
			<Button variant="outline" onclick={() => (detailOpen = false)}>{t('common.close')}</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<!-- Refund Dialog -->
<Dialog.Root bind:open={refundOpen} onOpenChange={(open) => { if (!open && !refundProcessing) { refundOpen = false; refundOrder = null; refundPreview = null; refundProcessing = false; } }}>
	<Dialog.Content class="sm:max-w-lg">
		<Dialog.Header>
			<Dialog.Title>{t('orders.refundOrder')}</Dialog.Title>
		</Dialog.Header>
		{#if refundPreview && refundOrder}
			<div class="grid gap-4 pt-2">
				<div>
					<p class="text-sm text-muted-foreground">{t('orders.orderNo')}</p>
					<p class="font-mono">{refundOrder.order_no}</p>
				</div>
				<div class="grid grid-cols-2 gap-4">
					<div>
						<p class="text-sm text-muted-foreground">{t('orders.originalPrice')}</p>
						<p>{formatCurrency(refundPreview.original_amount_cents)}</p>
					</div>
					<div>
						<p class="text-sm text-muted-foreground">{t('orders.consumedValue')}</p>
						<p>{formatCurrency(refundPreview.consumed_cents)}</p>
					</div>
				</div>
				<div>
					<p class="text-sm text-muted-foreground">{t('orders.refundableAmount')}</p>
					<p class="text-2xl font-semibold text-primary">{formatCurrency(refundPreview.refundable_cents)}</p>
				</div>
				{#if refundPreview.refundable_cents === 0}
					<Alert.Root>
						<AlertTriangle class="h-4 w-4" />
						<Alert.Description>{t('orders.noRefundable')}</Alert.Description>
					</Alert.Root>
				{/if}
			</div>
		{/if}
		<Dialog.Footer>
			<Button variant="outline" disabled={refundProcessing} onclick={() => { refundOpen = false; refundOrder = null; refundPreview = null; refundProcessing = false; }}>
				{t('common.cancel')}
			</Button>
			<Button
				variant="destructive"
				disabled={refundProcessing || (refundPreview && refundPreview.refundable_cents === 0)}
				onclick={handleConfirmRefund}
			>
				{#if refundProcessing}
					<LoaderCircle class="mr-2 h-4 w-4 animate-spin" />
				{/if}
				{refundProcessing ? t('orders.processing') : t('orders.confirmRefund')}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
