<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { base } from '$app/paths';
	import { t, getLang, toggleLang } from '$lib/i18n';
	import { authAPI } from '$lib/api';
	import { toggleMode, mode } from 'mode-watcher';
	import { Button } from '$lib/components/ui/button';
	import { Separator } from '$lib/components/ui/separator';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import {
		LayoutGrid, Users, Settings, Monitor, Shield, Database,
		DollarSign, Tv, ArrowRightLeft, HardDrive, LogOut, Menu,
		ChevronLeft, Building2, Receipt, Gift, Tag, Sun, Moon,
		NotebookText, Save
	} from '@lucide/svelte';

	let { children } = $props();
	let adminName = $state('');
	let sidebarOpen = $state(true);
	let mobileOpen = $state(false);

	const menuGroups = $derived([
		{
			label: t('menu.content'),
			items: [
				{ text: t('menu.apps'), icon: LayoutGrid, path: '/apps' },
				{ text: t('menu.categories'), icon: Tag, path: '/categories' },
				{ text: t('menu.screensaver'), icon: Tv, path: '/screensaver' }
			]
		},
		{
			label: t('menu.users'),
			items: [
				{ text: t('menu.usersList'), icon: Users, path: '/users' },
				{ text: t('menu.tenants'), icon: Building2, path: '/tenants' }
			]
		},
		{
			label: t('menu.billing'),
			items: [
				{ text: t('menu.planSets'), icon: Settings, path: '/plan-sets' },
				{ text: t('menu.packages'), icon: Gift, path: '/global-packages' },
				{ text: t('menu.orders'), icon: Receipt, path: '/orders' },
				{ text: t('menu.revenue'), icon: DollarSign, path: '/revenue' }
			]
		},
		{
			label: t('menu.devices'),
			items: [
				{ text: t('menu.connectedDevices'), icon: Monitor, path: '/devices' },
				{ text: t('menu.deviceRegistration'), icon: Shield, path: '/registered-devices' },
				{ text: t('menu.moonlightHosts'), icon: Database, path: '/moonlight-hosts' }
			]
		},
		{
			label: t('menu.system'),
			items: [
				{ text: t('menu.dataMigration'), icon: ArrowRightLeft, path: '/data-migration' },
				{ text: t('menu.databaseBackup'), icon: HardDrive, path: '/database-backup' },
				{ text: t('menu.saveStorage'), icon: Save, path: '/save-storage' },
				{ text: t('menu.notebook'), icon: NotebookText, path: '/notebook' }
			]
		}
	]);

	function isActive(path: string): boolean {
		const current = page.url.pathname.replace(base, '');
		return current === path || current.startsWith(path + '/');
	}

	async function handleLogout() {
		try {
			await authAPI.logout();
		} catch {}
		localStorage.removeItem('admin_token');
		localStorage.removeItem('admin_user');
		goto(`${base}/login`);
	}

	onMount(async () => {
		const token = localStorage.getItem('admin_token');
		if (!token) {
			goto(`${base}/login`);
			return;
		}
		try {
			const data = await authAPI.me();
			adminName = data.display_name || data.username;
		} catch {
			goto(`${base}/login`);
		}
	});
</script>

<Tooltip.Provider>
<div class="flex h-screen overflow-hidden">
	<!-- Mobile overlay -->
	{#if mobileOpen}
		<div
			class="fixed inset-0 z-40 bg-black/50 md:hidden"
			onclick={() => (mobileOpen = false)}
			onkeydown={() => {}}
			role="button"
			tabindex="-1"
		></div>
	{/if}

	<!-- Sidebar -->
	<aside
		class="fixed z-50 flex h-full flex-col border-r bg-sidebar-background transition-all duration-200 md:relative md:z-auto
			{sidebarOpen ? 'w-60' : 'w-16'}
			{mobileOpen ? 'translate-x-0' : '-translate-x-full md:translate-x-0'}"
	>
		<div class="flex h-14 items-center justify-between border-b border-sidebar-border px-4">
			{#if sidebarOpen}
				<span class="text-sm font-bold tracking-tight text-primary">{t('app.sidebarTitle')}</span>
			{/if}
			<button class="hidden rounded p-1 hover:bg-accent md:block" onclick={() => (sidebarOpen = !sidebarOpen)}>
				{#if sidebarOpen}
					<ChevronLeft class="h-4 w-4" />
				{:else}
					<Menu class="h-4 w-4" />
				{/if}
			</button>
		</div>

		<nav class="flex-1 overflow-y-auto py-2">
			{#each menuGroups as group, gi}
				{#if gi > 0}
					<Separator class="my-1" />
				{/if}
				{#if sidebarOpen}
					<div class="px-4 py-1 text-[0.65rem] font-semibold uppercase tracking-wider text-muted-foreground">
						{group.label}
					</div>
				{/if}
				{#each group.items as item}
					{@const Icon = item.icon}
					{@const active = isActive(item.path)}
					<Tooltip.Root>
						<Tooltip.Trigger class="block w-full text-left">
							<a
								href="{base}{item.path}"
								class="mx-2 mb-0.5 flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors
									{active
										? 'bg-primary/12 text-primary font-medium'
										: 'text-muted-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground'}
									{sidebarOpen ? '' : 'justify-center'}"
								onclick={() => (mobileOpen = false)}
							>
								<Icon class="h-4 w-4 shrink-0" />
								{#if sidebarOpen}
									<span class="truncate">{item.text}</span>
								{/if}
							</a>
						</Tooltip.Trigger>
						{#if !sidebarOpen}
							<Tooltip.Content side="right">{item.text}</Tooltip.Content>
						{/if}
					</Tooltip.Root>
				{/each}
			{/each}
		</nav>
	</aside>

	<!-- Main content -->
	<div class="flex flex-1 flex-col overflow-hidden">
		<!-- Top bar -->
		<header class="flex h-14 items-center gap-4 border-b bg-background px-4 md:px-6">
			<button class="rounded p-1 hover:bg-accent md:hidden" onclick={() => (mobileOpen = !mobileOpen)}>
				<Menu class="h-5 w-5" />
			</button>
			<h1 class="flex-1 text-lg font-semibold">{t('app.title')}</h1>

			<button
				class="rounded-lg p-2 text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
				onclick={toggleMode}
				aria-label="Toggle theme"
			>
				{#if mode.current === 'light'}
					<Moon class="h-4 w-4" />
				{:else}
					<Sun class="h-4 w-4" />
				{/if}
			</button>

			<button
				class="rounded border px-2 py-1 text-xs hover:bg-accent"
				onclick={toggleLang}
			>
				{getLang() === 'cn' ? 'EN' : '中文'}
			</button>

			<span class="hidden text-sm text-muted-foreground md:inline">{adminName}</span>

			<Button variant="ghost" size="sm" onclick={handleLogout}>
				<LogOut class="mr-1 h-4 w-4" />
				<span class="hidden md:inline">{t('app.logout')}</span>
			</Button>
		</header>

		<!-- Page content -->
		<main class="flex-1 overflow-y-auto p-4 md:p-6">
			{@render children()}
		</main>
	</div>
</div>
</Tooltip.Provider>
