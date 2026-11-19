<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from '$lib/i18n';
	import { usersAPI, tenantsAPI } from '$lib/api';
	import { toast } from 'svelte-sonner';
	import { Search, DollarSign, Clock, History } from '@lucide/svelte';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Table from '$lib/components/ui/table';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Badge } from '$lib/components/ui/badge';
	import { Switch } from '$lib/components/ui/switch';
	import TablePagination from '$lib/components/table-pagination.svelte';

	// --- User list state ---
	let users: any[] = $state([]);
	let loading = $state(true);
	let page = $state(0);
	let rowsPerPage = $state(20);
	let total = $state(0);
	let keyword = $state('');
	let searchInput = $state('');

	// --- Balance adjustment dialog ---
	let balanceDialogOpen = $state(false);
	let balanceUser: any = $state(null);
	let balanceAmount = $state(1);
	let balanceRemark = $state('');

	// --- Tenant time balance adjustment dialog ---
	let tenantDialogOpen = $state(false);
	let tenantDialogUser: any = $state(null);
	let tenantDialogTenants: any[] = $state([]);
	let tenantDialogSelectedTenantId = $state('');
	let tenantDialogMinutes = $state(30);
	let tenantDialogRemark = $state('');

	// --- Balance logs dialog ---
	let logsDialogOpen = $state(false);
	let logsUser: any = $state(null);
	let logs: any[] = $state([]);
	let logsLoading = $state(false);
	let logsTotal = $state(0);
	let logsPage = $state(0);
	let logsRowsPerPage = $state(20);

	// --- Load users ---
	async function loadUsers() {
		try {
			loading = true;
			const data = await usersAPI.getAll({
				page: page + 1,
				limit: rowsPerPage,
				search: keyword || undefined,
			});
			users = data.items || data.users || [];
			total = data.total || 0;
		} catch (error) {
			console.error('Failed to load users:', error);
			toast.error(t('users.loadFailed'));
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		// Re-fetch whenever page, rowsPerPage, or keyword changes
		page; rowsPerPage; keyword;
		loadUsers();
	});

	function handleSearch() {
		page = 0;
		keyword = searchInput;
	}

	function handleKeyPress(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			handleSearch();
		}
	}

	// --- Balance adjustment ---
	function openBalanceDialog(user: any) {
		balanceUser = user;
		balanceAmount = 1;
		balanceRemark = '';
		balanceDialogOpen = true;
	}

	async function handleAdjustBalance() {
		if (!balanceUser) return;
		try {
			const amountCents = Math.abs(Math.round(balanceAmount * 100));
			const adjustmentType = balanceAmount > 0 ? 'add' : 'deduct';

			if (adjustmentType === 'deduct' && amountCents > balanceUser.balance_cents) {
				toast.error(t('users.deductExceed'));
				return;
			}

			await usersAPI.adjustBalance(balanceUser.id, {
				adjustment_type: adjustmentType,
				amount_cents: amountCents,
				remark: balanceRemark,
			});
			toast.success(t('users.adjustSuccess'));
			balanceDialogOpen = false;
			loadUsers();
		} catch (error) {
			console.error('Failed to adjust balance:', error);
			toast.error(t('users.adjustFailed'));
		}
	}

	// --- Tenant time balance adjustment ---
	async function openTenantDialog(user: any) {
		tenantDialogUser = user;
		tenantDialogMinutes = 30;
		tenantDialogRemark = '';
		tenantDialogSelectedTenantId = '';
		tenantDialogOpen = true;
		try {
			const data = await tenantsAPI.getAllActive();
			tenantDialogTenants = data || [];
			if (tenantDialogTenants.length > 0) {
				tenantDialogSelectedTenantId = String(tenantDialogTenants[0].id);
			}
		} catch {
			tenantDialogTenants = [];
		}
	}

	function getCurrentTenantMinutes(): number {
		if (!tenantDialogUser || !tenantDialogSelectedTenantId) return 0;
		const balances = tenantDialogUser.tenant_balances || [];
		const tb = balances.find((b: any) => String(b.tenant_id) === tenantDialogSelectedTenantId);
		return tb?.balance_minutes || 0;
	}

	async function handleAdjustTenantBalance() {
		if (!tenantDialogUser || !tenantDialogSelectedTenantId) return;
		try {
			await usersAPI.adjustTenantBalance(tenantDialogUser.id, {
				tenant_id: tenantDialogSelectedTenantId,
				minutes: tenantDialogMinutes,
				remark: tenantDialogRemark,
			});
			toast.success(t('users.tenantBalanceAdjustSuccess'));
			tenantDialogOpen = false;
			loadUsers();
		} catch (error) {
			console.error('Failed to adjust tenant balance:', error);
			toast.error(t('users.tenantBalanceAdjustFailed'));
		}
	}

	// --- Balance logs ---
	async function openLogsDialog(user: any) {
		logsUser = user;
		logs = [];
		logsTotal = 0;
		logsPage = 0;
		logsDialogOpen = true;
		await loadBalanceLogs(user.id, 0, logsRowsPerPage);
	}

	async function loadBalanceLogs(userId: number, p: number, limit: number) {
		try {
			logsLoading = true;
			const data = await usersAPI.getBalanceLogs(userId, {
				page: p + 1,
				limit,
			});
			logs = data?.items || [];
			logsTotal = data?.total || 0;
		} catch (error) {
			console.error('Failed to load balance logs:', error);
			toast.error(t('users.logLoadFailed'));
		} finally {
			logsLoading = false;
		}
	}

	// --- Toggle user status ---
	async function handleToggleStatus(user: any) {
		try {
			const newStatus = user.status === 1 ? 0 : 1;
			await usersAPI.updateStatus(user.id, { status: newStatus });
			toast.success(newStatus === 1 ? t('users.statusEnabled') : t('users.statusDisabled'));
			loadUsers();
		} catch (error) {
			console.error('Failed to toggle user status:', error);
			toast.error(t('users.statusFailed'));
		}
	}

	// --- Helpers ---
	function formatDate(dateStr: string | null | undefined): string {
		if (!dateStr) return '-';
		return new Date(dateStr).toLocaleString('zh-CN');
	}

	function formatMinutes(minutes: number | null | undefined): string {
		if (!minutes || minutes <= 0) return '-';
		const h = Math.floor(minutes / 60);
		const m = minutes % 60;
		if (h > 0 && m > 0) return `${h}h ${m}min`;
		if (h > 0) return `${h}h`;
		return `${m}min`;
	}

	function getStatusBadge(user: any): { label: string; variant: 'default' | 'secondary' | 'destructive' | 'outline' } {
		if (user.status === 0) {
			return { label: t('common.disabled'), variant: 'destructive' };
		}
		if (user.balance_cents > 0) {
			return { label: t('users.hasBalance'), variant: 'default' };
		}
		return { label: t('users.noBalance'), variant: 'secondary' };
	}

	function getLogTypeBadge(type: string): { label: string; variant: 'default' | 'secondary' | 'destructive' | 'outline' } {
		const typeMap: Record<string, { label: string; variant: 'default' | 'secondary' | 'destructive' | 'outline' }> = {
			recharge: { label: t('users.typeRecharge'), variant: 'default' },
			billing: { label: t('users.typeBilling'), variant: 'destructive' },
			admin: { label: t('users.typeAdmin'), variant: 'secondary' },
			bonus: { label: t('users.typeBonus'), variant: 'default' },
			refund: { label: t('users.typeRefund'), variant: 'outline' },
		};
		return typeMap[type] || { label: type, variant: 'secondary' };
	}

</script>

<!-- Header -->
<div class="flex items-center justify-between mb-6">
	<h2 class="text-2xl font-semibold">{t('users.title')}</h2>
	<div class="flex items-center gap-2">
		<div class="relative">
			<Input
				placeholder={t('users.searchPlaceholder')}
				bind:value={searchInput}
				onkeydown={handleKeyPress}
				class="w-64 pr-9"
			/>
			<button
				class="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
				onclick={handleSearch}
			>
				<Search class="h-4 w-4" />
			</button>
		</div>
	</div>
</div>

<!-- Users Table -->
<div class="rounded-lg border">
	<Table.Root>
		<Table.Header>
			<Table.Row>
				<Table.Head>{t('users.user')}</Table.Head>
				<Table.Head>{t('users.enableDisable')}</Table.Head>
				<Table.Head>{t('common.status')}</Table.Head>
				<Table.Head>{t('users.balance')}</Table.Head>
				<Table.Head>{t('users.timeRemaining')}</Table.Head>
				<Table.Head>{t('users.totalRecharge')}</Table.Head>
				<Table.Head>{t('users.registeredAt')}</Table.Head>
				<Table.Head>{t('users.lastLogin')}</Table.Head>
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
			{:else if users.length === 0}
				<Table.Row>
					<Table.Cell colspan={9} class="text-center py-8 text-muted-foreground">
						{t('users.noUsers')}
					</Table.Cell>
				</Table.Row>
			{:else}
				{#each users as user (user.id)}
					{@const status = getStatusBadge(user)}
					<Table.Row>
						<Table.Cell>
							<div class="flex items-center gap-2">
								{#if user.avatar_url}
									<img src={user.avatar_url} alt="" class="h-8 w-8 rounded-full object-cover" />
								{:else}
									<div class="flex h-8 w-8 items-center justify-center rounded-full bg-muted text-sm font-medium">
										{user.nickname?.[0] || '?'}
									</div>
								{/if}
								<span class="text-sm">{user.nickname || t('users.notSet')}</span>
							</div>
						</Table.Cell>
						<Table.Cell>
							<Switch
								checked={user.status === 1}
								onCheckedChange={() => handleToggleStatus(user)}
								title={user.status === 1 ? t('users.clickToDisable') : t('users.clickToEnable')}
							/>
						</Table.Cell>
						<Table.Cell>
							<Badge variant={status.variant}>{status.label}</Badge>
						</Table.Cell>
						<Table.Cell>{'\u00A5'}{((user.balance_cents || 0) / 100).toFixed(2)}</Table.Cell>
						<Table.Cell>
							{#if user.tenant_balances && user.tenant_balances.length > 0}
								{#each user.tenant_balances as tb}
									<div class="whitespace-nowrap text-sm">
										<span class="text-muted-foreground">{tb.tenant_name || tb.tenant_id?.slice(0, 6)}:</span>
										{formatMinutes(tb.balance_minutes)}
									</div>
								{/each}
							{:else}
								-
							{/if}
						</Table.Cell>
						<Table.Cell>{'\u00A5'}{((user.total_recharge_amount || 0) / 100).toFixed(2)}</Table.Cell>
						<Table.Cell class="text-sm">{formatDate(user.created_at)}</Table.Cell>
						<Table.Cell class="text-sm">{formatDate(user.last_login_at)}</Table.Cell>
						<Table.Cell class="text-right">
							<div class="flex items-center justify-end gap-1">
								<Button
									variant="ghost"
									size="icon-sm"
									onclick={() => openBalanceDialog(user)}
									title={t('users.adjustBalance')}
								>
									<DollarSign class="h-4 w-4" />
								</Button>
								<Button
									variant="ghost"
									size="icon-sm"
									onclick={() => openTenantDialog(user)}
									title={t('users.adjustTenantBalance')}
								>
									<Clock class="h-4 w-4" />
								</Button>
								<Button
									variant="ghost"
									size="icon-sm"
									onclick={() => openLogsDialog(user)}
									title={t('users.viewLogs')}
								>
									<History class="h-4 w-4" />
								</Button>
							</div>
						</Table.Cell>
					</Table.Row>
				{/each}
			{/if}
		</Table.Body>
	</Table.Root>

	<!-- Pagination -->
	<TablePagination bind:page bind:rowsPerPage {total} />
</div>

<!-- Balance Adjustment Dialog -->
<Dialog.Root bind:open={balanceDialogOpen}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>{t('users.adjustBalanceTitle')}</Dialog.Title>
		</Dialog.Header>

		{#if balanceUser}
			<div class="space-y-4 pt-2">
				<!-- User info -->
				<div class="flex items-center gap-2">
					{#if balanceUser.avatar_url}
						<img src={balanceUser.avatar_url} alt="" class="h-10 w-10 rounded-full object-cover" />
					{:else}
						<div class="flex h-10 w-10 items-center justify-center rounded-full bg-muted text-sm font-medium">
							{balanceUser.nickname?.[0] || '?'}
						</div>
					{/if}
					<span class="font-medium">{balanceUser.nickname}</span>
				</div>

				<!-- Current balance -->
				<p class="text-sm text-muted-foreground">
					{t('users.currentBalance')}: {'\u00A5'}{((balanceUser.balance_cents || 0) / 100).toFixed(2)}
				</p>

				<!-- Amount input with +/- buttons -->
				<div class="flex items-center gap-2">
					<Button
						variant="outline"
						size="icon-sm"
						onclick={() => (balanceAmount = balanceAmount - 1)}
					>
						-
					</Button>
					<Input
						type="number"
						bind:value={balanceAmount}
						class="w-36 text-center"
					/>
					<Button
						variant="outline"
						size="icon-sm"
						onclick={() => (balanceAmount = balanceAmount + 1)}
					>
						+
					</Button>
				</div>
				<p class="text-xs text-muted-foreground">{t('users.adjustHelp')}</p>

				<!-- Quick-add buttons -->
				<div class="flex flex-wrap gap-2">
					{#each [1, 5, 10, 50] as amt}
						<button
							class="rounded-full border px-3 py-1 text-sm transition-colors
								{balanceAmount === amt ? 'bg-primary text-primary-foreground border-primary' : 'hover:bg-accent'}"
							onclick={() => (balanceAmount = amt)}
						>
							{t(`users.quickAdd${amt}`)}
						</button>
					{/each}
				</div>

				<!-- Remark -->
				<Input
					placeholder={t('users.remarkPlaceholder')}
					bind:value={balanceRemark}
				/>
			</div>

			<Dialog.Footer class="mt-4">
				<Button variant="outline" onclick={() => (balanceDialogOpen = false)}>
					{t('common.cancel')}
				</Button>
				<Button disabled={balanceAmount === 0} onclick={handleAdjustBalance}>
					{t('users.confirmAdjust')}
				</Button>
			</Dialog.Footer>
		{/if}
	</Dialog.Content>
</Dialog.Root>

<!-- Tenant Time Balance Adjustment Dialog -->
<Dialog.Root bind:open={tenantDialogOpen}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>{t('users.adjustTenantBalance')}</Dialog.Title>
		</Dialog.Header>

		{#if tenantDialogUser}
			<div class="space-y-4 pt-2">
				<!-- User info -->
				<div class="flex items-center gap-2">
					{#if tenantDialogUser.avatar_url}
						<img src={tenantDialogUser.avatar_url} alt="" class="h-10 w-10 rounded-full object-cover" />
					{:else}
						<div class="flex h-10 w-10 items-center justify-center rounded-full bg-muted text-sm font-medium">
							{tenantDialogUser.nickname?.[0] || '?'}
						</div>
					{/if}
					<span class="font-medium">{tenantDialogUser.nickname}</span>
				</div>

				<!-- Tenant selector -->
				<div>
					<label class="mb-1 block text-sm font-medium">{t('users.tenant')}</label>
					<select
						class="w-full rounded-lg border bg-transparent px-2.5 py-1.5 text-sm"
						bind:value={tenantDialogSelectedTenantId}
					>
						{#each tenantDialogTenants as ten}
							<option value={String(ten.id)}>{ten.display_name || ten.username}</option>
						{/each}
					</select>
				</div>

				<!-- Current time remaining -->
				<p class="text-sm text-muted-foreground">
					{t('users.currentTimeRemaining')}: {formatMinutes(getCurrentTenantMinutes())}
				</p>

				<!-- Minutes input -->
				<div>
					<label class="mb-1 block text-sm font-medium">{t('users.timeMinutes')}</label>
					<Input type="number" bind:value={tenantDialogMinutes} class="w-36" />
				</div>

				<!-- Quick-add buttons -->
				<div class="flex flex-wrap gap-2">
					{#each [[30, 'users.quickAdd30min'], [60, 'users.quickAdd1h'], [120, 'users.quickAdd2h'], [300, 'users.quickAdd5h']] as [mins, key]}
						<button
							class="rounded-full border px-3 py-1 text-sm transition-colors
								{tenantDialogMinutes === mins ? 'bg-primary text-primary-foreground border-primary' : 'hover:bg-accent'}"
							onclick={() => (tenantDialogMinutes = mins as number)}
						>
							{t(key as string)}
						</button>
					{/each}
				</div>

				<!-- Remark -->
				<Input
					placeholder={t('users.remarkPlaceholder')}
					bind:value={tenantDialogRemark}
				/>
			</div>

			<Dialog.Footer class="mt-4">
				<Button variant="outline" onclick={() => (tenantDialogOpen = false)}>
					{t('common.cancel')}
				</Button>
				<Button
					disabled={!tenantDialogSelectedTenantId || tenantDialogMinutes === 0}
					onclick={handleAdjustTenantBalance}
				>
					{t('users.confirmAdjust')}
				</Button>
			</Dialog.Footer>
		{/if}
	</Dialog.Content>
</Dialog.Root>

<!-- Balance History Logs Dialog -->
<Dialog.Root bind:open={logsDialogOpen}>
	<Dialog.Content class="sm:max-w-3xl">
		<Dialog.Header>
			<Dialog.Title>{t('users.balanceHistory')} - {logsUser?.nickname || ''}</Dialog.Title>
		</Dialog.Header>

		<div class="max-h-[60vh] overflow-auto">
			<Table.Root>
				<Table.Header>
					<Table.Row>
						<Table.Head>{t('users.time')}</Table.Head>
						<Table.Head>{t('users.type')}</Table.Head>
						<Table.Head>{t('users.changeAmount')}</Table.Head>
						<Table.Head>{t('users.before')}</Table.Head>
						<Table.Head>{t('users.after')}</Table.Head>
						<Table.Head>{t('users.remark')}</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#if logsLoading}
						<Table.Row>
							<Table.Cell colspan={6} class="text-center py-8 text-muted-foreground">
								{t('common.loading')}
							</Table.Cell>
						</Table.Row>
					{:else if logs.length === 0}
						<Table.Row>
							<Table.Cell colspan={6} class="text-center py-8 text-muted-foreground">
								{t('users.noLogs')}
							</Table.Cell>
						</Table.Row>
					{:else}
						{#each logs as log (log.id)}
							{@const typeBadge = getLogTypeBadge(log.type)}
							<Table.Row>
								<Table.Cell class="text-sm whitespace-nowrap">{formatDate(log.created_at)}</Table.Cell>
								<Table.Cell>
									<Badge variant={typeBadge.variant}>{typeBadge.label}</Badge>
								</Table.Cell>
								<Table.Cell>
									<span class={log.amount_cents > 0 ? 'text-green-600' : 'text-red-600'}>
										{log.amount_cents > 0 ? '+' : ''}{'\u00A5'}{(log.amount_cents / 100).toFixed(2)}
									</span>
								</Table.Cell>
								<Table.Cell>{'\u00A5'}{(log.balance_before_cents / 100).toFixed(2)}</Table.Cell>
								<Table.Cell>{'\u00A5'}{(log.balance_after_cents / 100).toFixed(2)}</Table.Cell>
								<Table.Cell>
									<span class="block max-w-[200px] truncate text-sm">{log.remark || '-'}</span>
								</Table.Cell>
							</Table.Row>
						{/each}
					{/if}
				</Table.Body>
			</Table.Root>
		</div>

		<!-- Logs pagination -->
		<TablePagination
			bind:page={logsPage}
			bind:rowsPerPage={logsRowsPerPage}
			total={logsTotal}
			onChange={() => {
				if (logsUser) loadBalanceLogs(logsUser.id, logsPage, logsRowsPerPage);
			}}
		/>

		<Dialog.Footer class="mt-2">
			<Button variant="outline" onclick={() => (logsDialogOpen = false)}>
				{t('common.close')}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
