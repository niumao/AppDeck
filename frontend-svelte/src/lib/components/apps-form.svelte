<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { t } from '$lib/i18n';
	import { appsAPI, categoriesAPI } from '$lib/api';
	import { searchSteamGames, getSteamGameDetails } from '$lib/services/steamSearch';
	import { searchRetroarchGames } from '$lib/services/retroarchSearch';
	import { toast } from 'svelte-sonner';
	import { goto } from '$app/navigation';
	import { base } from '$app/paths';
	import { Search, Loader2, X } from '@lucide/svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Badge } from '$lib/components/ui/badge';
	import { Switch } from '$lib/components/ui/switch';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import * as Card from '$lib/components/ui/card';

	interface Props {
		appId?: string;
	}

	let { appId }: Props = $props();
	let isEdit = $derived(Boolean(appId));

	const RETROARCH_CORES = [
		{ value: 'snes9x', label: 'SNES (snes9x)' },
		{ value: 'mgba', label: 'GBA (mGBA)' },
		{ value: 'nestopia', label: 'NES (Nestopia)' },
		{ value: 'genesis_plus_gx', label: 'Genesis/MD (Genesis Plus GX)' },
		{ value: 'pcsx_rearmed', label: 'PS1 (PCSX ReARMed)' },
		{ value: 'ppsspp', label: 'PSP (PPSSPP)' },
		{ value: 'mupen64plus_next', label: 'N64 (Mupen64Plus)' },
		{ value: 'fbneo', label: 'Arcade (FBNeo)' },
		{ value: 'gambatte', label: 'GB/GBC (Gambatte)' },
		{ value: 'fbalpha2012_neogeo', label: 'Arcade/Neo Geo (FB Alpha 2012)' },
		{ value: 'picodrive', label: 'Sega 32X (PicoDrive)' },
		{ value: 'mednafen_pce_fast', label: 'PC Engine (Mednafen PCE Fast)' },
		{ value: 'mednafen_ngp', label: 'Neo Geo Pocket (Mednafen NGP)' },
		{ value: 'melonds', label: 'Nintendo DS (melonDS)' },
		{ value: 'fceumm', label: 'NES (FCEUmm)' }
	];

	const DIR_TO_CORE: Record<string, string> = {
		'FC': 'nestopia',
		'SFC': 'snes9x',
		'ARCADE': 'fbalpha2012_neogeo',
		'sega-md': 'genesis_plus_gx',
		'Sega-32X': 'picodrive',
		'PCE': 'mednafen_pce_fast',
		'NGPC': 'mednafen_ngp',
		'NDS': 'melonds',
	};

	function detectCoreFromRom(romPath: string): string | null {
		const dir = romPath.split('/')[0];
		return dir ? (DIR_TO_CORE[dir] ?? null) : null;
	}

	interface Category {
		id: string;
		name: string;
	}

	interface SteamResult {
		appid: number;
		name: string;
		thumbnail?: string;
	}

	interface RetroarchResult {
		name: string;
		boxartUrl?: string;
		snapUrl?: string;
		thumbnail?: string;
	}

	let categories: Category[] = $state([]);
	let loading = $state(false);

	// Form fields
	let name = $state('');
	let description = $state('');
	let launchParam = $state('');
	let categoryIds: string[] = $state([]);
	let order = $state(100);
	let launchType = $state('moonlight');
	let retroarchCore = $state('');
	let retroarchRom = $state('');
	let titleId = $state('');

	// Steam search
	let steamResults: SteamResult[] = $state([]);
	let steamSearchLoading = $state(false);
	let steamSearchInput = $state('');
	let steamDropdownOpen = $state(false);
	let steamSearchTimer: ReturnType<typeof setTimeout> | null = null;

	// RetroArch search
	let retroarchResults: RetroarchResult[] = $state([]);
	let retroarchSearchLoading = $state(false);
	let retroarchSearchInput = $state('');
	let retroarchDropdownOpen = $state(false);
	let retroarchSearchTimer: ReturnType<typeof setTimeout> | null = null;

	onMount(() => {
		loadCategories();
		if (appId) loadApp();
	});

	onDestroy(() => {
		if (steamSearchTimer) clearTimeout(steamSearchTimer);
		if (retroarchSearchTimer) clearTimeout(retroarchSearchTimer);
	});

	async function loadCategories() {
		try {
			const data = await categoriesAPI.getAll();
			categories = data || [];
		} catch (error: any) {
			toast.error(t('common.loadFailed') + ': ' + error.message);
		}
	}

	async function loadApp() {
		try {
			const app = await appsAPI.getById(appId!);
			name = app.name || '';
			description = app.description || '';
			launchParam = app.launchParam || '';
			categoryIds = app.categoryIds || [];
			order = app.order ?? 100;
			launchType = app.launchType || 'moonlight';
			retroarchCore = app.retroarchCore || '';
			retroarchRom = app.retroarchRom || '';
			titleId = app.titleId || '';
		} catch (error: any) {
			toast.error(t('common.loadFailed') + ': ' + error.message);
		}
	}

	// Steam search with debounce
	function handleSteamSearchInput(value: string) {
		steamSearchInput = value;
		name = value;
		if (steamSearchTimer) clearTimeout(steamSearchTimer);
		if (value.length < 2) {
			steamResults = [];
			steamDropdownOpen = false;
			return;
		}
		steamSearchTimer = setTimeout(async () => {
			steamSearchLoading = true;
			try {
				const results = await searchSteamGames(value);
				steamResults = results;
				steamDropdownOpen = results.length > 0;
			} catch (error: any) {
				toast.error('Steam search failed: ' + error.message);
				steamResults = [];
			} finally {
				steamSearchLoading = false;
			}
		}, 300);
	}

	async function handleSteamGameSelect(game: SteamResult) {
		steamDropdownOpen = false;
		if (!game?.appid) return;
		loading = true;
		try {
			const details = await getSteamGameDetails(game.appid);
			name = details.name;
			description = details.description;
			launchParam = details.launchParam;
			await handleAutoCreateCategories(details.genres);
			toast.success('Game details loaded successfully!');
		} catch (error: any) {
			toast.error('Failed to load game details: ' + error.message);
		} finally {
			loading = false;
		}
	}

	// RetroArch search with debounce
	function handleRetroarchSearchInput(value: string) {
		retroarchSearchInput = value;
		name = value;
		if (retroarchSearchTimer) clearTimeout(retroarchSearchTimer);
		if (value.length < 2 || !retroarchCore) {
			retroarchResults = [];
			retroarchDropdownOpen = false;
			return;
		}
		retroarchSearchTimer = setTimeout(async () => {
			retroarchSearchLoading = true;
			try {
				const results = await searchRetroarchGames(retroarchCore, value);
				retroarchResults = results;
				retroarchDropdownOpen = results.length > 0;
			} catch (error: any) {
				toast.error('RetroArch search failed: ' + error.message);
				retroarchResults = [];
			} finally {
				retroarchSearchLoading = false;
			}
		}, 300);
	}

	function handleRetroarchGameSelect(game: RetroarchResult) {
		retroarchDropdownOpen = false;
		if (!game?.name) return;
		name = game.name;
		toast.success(t('apps.retroarchGameLoaded'));
	}

	async function handleAutoCreateCategories(genres: string[]) {
		if (!genres || genres.length === 0) return;
		const newCategoryIds: string[] = [];
		for (const genreName of genres) {
			const existing = categories.find(
				(cat) => cat.name.toLowerCase() === genreName.toLowerCase()
			);
			if (existing) {
				newCategoryIds.push(existing.id);
			} else {
				try {
					const newCat = await categoriesAPI.create({ name: genreName });
					categories = [...categories, newCat];
					newCategoryIds.push(newCat.id);
					toast.info(`Created new category: ${genreName}`);
				} catch {
					toast.warning(`Failed to create category: ${genreName}`);
				}
			}
		}
		categoryIds = newCategoryIds;
	}

	function toggleCategory(catId: string) {
		if (categoryIds.includes(catId)) {
			categoryIds = categoryIds.filter((id) => id !== catId);
		} else {
			categoryIds = [...categoryIds, catId];
		}
	}

	async function handleSubmit() {
		if (!name.trim()) {
			toast.warning(t('apps.enterAppName'));
			return;
		}
		if (launchType === 'moonlight' && !launchParam.trim()) {
			toast.warning(t('apps.enterLaunchParam'));
			return;
		}
		if (launchType === 'retroarch') {
			if (!retroarchRom.trim()) {
				toast.warning(t('apps.enterRetroarchRom'));
				return;
			}
		}
		if (launchType === 'eden' && !launchParam.trim()) {
			toast.warning('Eden apps require ROM path');
			return;
		}
		if (launchType === 'eden' && titleId.trim() && !/^[0-9A-Fa-f]{16}$/.test(titleId.trim())) {
			toast.warning(t('apps.invalidTitleId'));
			return;
		}
		if (launchType === 'android' && !launchParam.trim()) {
			toast.warning('Android apps require package name');
			return;
		}
		if (categoryIds.length === 0) {
			toast.warning(t('apps.selectCategory'));
			return;
		}

		loading = true;
		try {
			const payload = {
				name,
				description,
				launchParam,
				categoryIds,
				order: Number(order),
				launchType,
				retroarchCore,
				retroarchRom,
				titleId
			};

			if (isEdit) {
				await appsAPI.update(appId!, payload);
				toast.success(t('common.updateSuccess'));
			} else {
				await appsAPI.create(payload);
				toast.success(t('common.createSuccess'));
			}
			setTimeout(() => {
				goto(`${base}/apps`);
			}, 1000);
		} catch (error: any) {
			toast.error(
				(isEdit ? t('common.updateFailed') : t('common.createFailed')) +
					': ' +
					(error.message || '')
			);
		} finally {
			loading = false;
		}
	}
</script>

<div class="space-y-4">
	<h2 class="text-2xl font-bold tracking-tight">
		{isEdit ? t('apps.editApp') : t('apps.addApp')}
	</h2>

	<Card.Root>
		<Card.Content class="pt-6">
			<form
				class="space-y-4"
				onsubmit={(e) => {
					e.preventDefault();
					handleSubmit();
				}}
			>
				<!-- Launch Type -->
				<div class="space-y-2">
					<Label>{t('apps.launchType')}</Label>
					<select
						class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
						bind:value={launchType}
						onchange={() => {
							steamResults = [];
							retroarchResults = [];
						}}
					>
						<option value="moonlight">{t('apps.launchTypeMoonlight')}</option>
						<option value="retroarch">{t('apps.launchTypeRetroarch')}</option>
						<option value="eden">Eden (Switch)</option>
						<option value="android">Android</option>
					</select>
				</div>

				<!-- Name field: Steam autocomplete for moonlight -->
				{#if launchType === 'moonlight'}
					<div class="relative space-y-2">
						<Label>{t('apps.appName')} *</Label>
						<div class="relative">
							<Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
							<Input
								class="pl-9 pr-8"
								value={name}
								placeholder={t('apps.searchPlaceholder')}
								oninput={(e) => handleSteamSearchInput(e.currentTarget.value)}
								onfocus={() => {
									if (steamResults.length > 0) steamDropdownOpen = true;
								}}
							/>
							{#if steamSearchLoading}
								<Loader2 class="absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 animate-spin text-muted-foreground" />
							{/if}
						</div>
						{#if steamDropdownOpen && steamResults.length > 0}
							<!-- svelte-ignore a11y_no_static_element_interactions -->
							<div
								class="absolute z-50 mt-1 w-full rounded-md border bg-popover shadow-md max-h-60 overflow-y-auto"
								onmousedown={(e) => e.preventDefault()}
							>
								{#each steamResults as result (result.appid)}
									<button
										type="button"
										class="flex w-full items-center gap-2 px-3 py-2 text-sm hover:bg-accent text-left"
										onclick={() => handleSteamGameSelect(result)}
									>
										{#if result.thumbnail}
											<img
												src={result.thumbnail}
												alt={result.name}
												class="h-10 w-10 rounded object-cover"
											/>
										{/if}
										<span>{result.name}</span>
									</button>
								{/each}
							</div>
						{/if}
					</div>
				{:else if launchType === 'retroarch'}
					<!-- RetroArch core selector (optional — auto-detected from device playlist) -->
					<div class="space-y-2">
						<Label>{t('apps.retroarchCore')}</Label>
						<select
							class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
							bind:value={retroarchCore}
							onchange={() => {
								retroarchResults = [];
							}}
						>
							<option value="">{t('apps.retroarchSelectCoreFirst')}</option>
							{#each RETROARCH_CORES as core (core.value)}
								<option value={core.value}>{core.label}</option>
							{/each}
						</select>
						<p class="text-xs text-muted-foreground">Auto-detected from RetroArch playlist on device. Only set manually if needed.</p>
					</div>

					<!-- RetroArch ROM path -->
					<div class="space-y-2">
						<Label>{t('apps.retroarchRom')} *</Label>
						<Input
							bind:value={retroarchRom}
							placeholder={t('apps.retroarchRomPlaceholder')}
							oninput={(e) => {
								const detected = detectCoreFromRom(e.currentTarget.value);
								if (detected) retroarchCore = detected;
							}}
						/>
					</div>

					<!-- RetroArch game search for name + art -->
					<div class="relative space-y-2">
						<Label>{t('apps.appName')} *</Label>
						<div class="relative">
							<Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
							<Input
								class="pl-9 pr-8"
								value={name}
								placeholder={retroarchCore ? t('apps.retroarchSearchPlaceholder') : t('apps.retroarchSelectCoreFirst')}
								oninput={(e) => handleRetroarchSearchInput(e.currentTarget.value)}
								onfocus={() => {
									if (retroarchResults.length > 0) retroarchDropdownOpen = true;
								}}
							/>
							{#if retroarchSearchLoading}
								<Loader2 class="absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 animate-spin text-muted-foreground" />
							{/if}
						</div>
						{#if retroarchDropdownOpen && retroarchResults.length > 0}
							<!-- svelte-ignore a11y_no_static_element_interactions -->
							<div
								class="absolute z-50 mt-1 w-full rounded-md border bg-popover shadow-md max-h-60 overflow-y-auto"
								onmousedown={(e) => e.preventDefault()}
							>
								{#each retroarchResults as result (result.name)}
									<button
										type="button"
										class="flex w-full items-center gap-2 px-3 py-2 text-sm hover:bg-accent text-left"
										onclick={() => handleRetroarchGameSelect(result)}
									>
										{#if result.thumbnail}
											<img
												src={result.thumbnail}
												alt={result.name}
												class="h-10 w-10 rounded object-cover"
												onerror={(e) => { (e.currentTarget as HTMLImageElement).style.display = 'none'; }}
											/>
										{/if}
										<span>{result.name}</span>
									</button>
								{/each}
							</div>
						{/if}
					</div>

					<!-- Launch param (optional for retroarch) -->
					<div class="space-y-2">
						<Label>{t('apps.launchParamLabel')}</Label>
						<Input bind:value={launchParam} placeholder="app-launch-param" />
					</div>
				{:else if launchType === 'eden'}
					<!-- Eden: plain name + ROM path -->
					<div class="space-y-2">
						<Label>{t('apps.appName')} *</Label>
						<Input bind:value={name} />
					</div>
					<div class="space-y-2">
						<Label>ROM Path *</Label>
						<Input
							bind:value={launchParam}
							placeholder="/path/to/game.nsp"
						/>
						<p class="text-xs text-muted-foreground">Path to the ROM file on the device</p>
					</div>
					<div class="space-y-2">
						<Label>{t('apps.titleId')}</Label>
						<Input
							bind:value={titleId}
							placeholder="0100ABC00A0B0000"
						/>
						<p class="text-xs text-muted-foreground">Optional. 16-character hex Switch title ID.</p>
					</div>
				{:else if launchType === 'android'}
					<!-- Android: app name + package name -->
					<div class="space-y-2">
						<Label>{t('apps.appName')} *</Label>
						<Input bind:value={name} />
					</div>
					<div class="space-y-2">
						<Label>Package Name *</Label>
						<Input
							bind:value={launchParam}
							placeholder="com.example.app"
						/>
						<p class="text-xs text-muted-foreground">Android package name (e.g. com.tencent.tmgp.cod)</p>
					</div>
				{/if}

				<!-- Description -->
				<div class="space-y-2">
					<Label>{t('apps.appDescription')}</Label>
					<textarea
						class="flex min-h-[80px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
						rows="3"
						bind:value={description}
					></textarea>
				</div>


				<!-- Moonlight launch param -->
				{#if launchType === 'moonlight'}
					<div class="space-y-2">
						<Label>{t('apps.launchParamLabel')} *</Label>
						<Input bind:value={launchParam} placeholder="app-launch-param" />
					</div>
				{/if}

				<!-- Sort Order -->
				<div class="space-y-2">
					<Label>{t('common.sortOrder')}</Label>
					<Input type="number" bind:value={order} />
					<p class="text-xs text-muted-foreground">{t('common.sortHelp')}</p>
				</div>

				<!-- Categories (multi-select with checkboxes) -->
				<div class="space-y-2">
					<Label>{t('apps.categoriesMulti')} *</Label>
					{#if categoryIds.length > 0}
						<div class="flex flex-wrap gap-1 mb-2">
							{#each categoryIds as catId (catId)}
								{@const cat = categories.find((c) => c.id === catId)}
								{#if cat}
									<Badge variant="secondary" class="gap-1">
										{cat.name}
										<button
											type="button"
											class="ml-1 hover:text-destructive"
											onclick={() => toggleCategory(catId)}
										>
											<X class="h-3 w-3" />
										</button>
									</Badge>
								{/if}
							{/each}
						</div>
					{/if}
					<div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-2 rounded-md border p-3 max-h-48 overflow-y-auto">
						{#each categories as category (category.id)}
							<label class="flex items-center gap-2 text-sm cursor-pointer">
								<Checkbox
									checked={categoryIds.includes(category.id)}
									onCheckedChange={() => toggleCategory(category.id)}
								/>
								{category.name}
							</label>
						{/each}
					</div>
				</div>

				<!-- Actions -->
				<div class="flex gap-2 pt-4">
					<Button type="submit" disabled={loading}>
						{#if loading}
							<Loader2 class="mr-2 h-4 w-4 animate-spin" />
						{/if}
						{loading ? t('common.saving') : t('common.save')}
					</Button>
					<Button
						type="button"
						variant="outline"
						disabled={loading}
						onclick={() => goto(`${base}/apps`)}
					>
						{t('common.cancel')}
					</Button>
				</div>
			</form>
		</Card.Content>
	</Card.Root>
</div>

<!-- Close dropdowns on outside click -->
<svelte:window
	onclick={() => {
		steamDropdownOpen = false;
		retroarchDropdownOpen = false;
	}}
/>
