<script lang="ts">
	import { t } from '$lib/i18n';
	import { dataMigrationAPI } from '$lib/api';
	import { toast } from 'svelte-sonner';
	import { Download, Upload, CloudUpload, Loader2 } from '@lucide/svelte';
	import { Button } from '$lib/components/ui/button';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { Label } from '$lib/components/ui/label';
	import * as Card from '$lib/components/ui/card';
	import * as Table from '$lib/components/ui/table';

	const EXPORTABLE_TABLES = [
		'apps',
		'categories',
		'plan_sets',
		'billing_plans',
		'global_packages',
		'system_config',
		'settings',
		'moonlight_hosts',
		'versions'
	] as const;

	let selectedTables: string[] = $state([...EXPORTABLE_TABLES]);
	let exporting = $state(false);
	let importing = $state(false);
	let importFile: File | null = $state(null);
	let importResults: Record<string, { created: number; updated: number }> | null = $state(null);
	let fileInput: HTMLInputElement | undefined = $state();

	let allSelected = $derived(selectedTables.length === EXPORTABLE_TABLES.length);
	let someSelected = $derived(selectedTables.length > 0 && !allSelected);

	function handleToggleAll() {
		selectedTables = allSelected ? [] : [...EXPORTABLE_TABLES];
	}

	function handleToggleTable(table: string) {
		if (selectedTables.includes(table)) {
			selectedTables = selectedTables.filter((t) => t !== table);
		} else {
			selectedTables = [...selectedTables, table];
		}
	}

	async function handleExport() {
		if (selectedTables.length === 0) {
			toast.warning(t('dataMigration.noTablesSelected'));
			return;
		}
		exporting = true;
		try {
			const blob = await dataMigrationAPI.exportData(selectedTables);
			const url = URL.createObjectURL(blob);
			const a = document.createElement('a');
			a.href = url;
			const now = new Date();
			const ts = now.toISOString().replace(/[:.]/g, '-').slice(0, 19);
			a.download = `appdeck-export-${ts}.json`;
			a.click();
			URL.revokeObjectURL(url);
			toast.success(t('dataMigration.exportSuccess'));
		} catch (err: any) {
			toast.error(t('dataMigration.exportFailed') + ': ' + (err.message || ''));
		} finally {
			exporting = false;
		}
	}

	function handleFileSelect(e: Event) {
		const input = e.target as HTMLInputElement;
		const file = input.files?.[0];
		if (file) importFile = file;
	}

	async function handleImport() {
		if (!importFile) {
			toast.warning(t('dataMigration.noFileSelected'));
			return;
		}
		importing = true;
		importResults = null;
		try {
			const response = await dataMigrationAPI.importData(importFile);
			importResults = response.results;
			toast.success(t('dataMigration.importSuccess'));
		} catch (err: any) {
			toast.error(t('dataMigration.importFailed') + ': ' + (err.message || ''));
		} finally {
			importing = false;
		}
	}
</script>

<div class="space-y-6">
	<h2 class="text-2xl font-bold tracking-tight">{t('dataMigration.title')}</h2>

	<!-- Export Section -->
	<Card.Root>
		<Card.Header>
			<Card.Title class="flex items-center gap-2">
				<Download class="h-5 w-5" />
				{t('dataMigration.exportTitle')}
			</Card.Title>
			<Card.Description>
				{t('dataMigration.exportDesc')}
			</Card.Description>
		</Card.Header>
		<Card.Content class="space-y-4">
			<div class="flex items-center space-x-2">
				<Checkbox
					id="select-all"
					checked={allSelected}
					indeterminate={someSelected}
					onCheckedChange={() => handleToggleAll()}
				/>
				<Label for="select-all" class="font-semibold">{t('dataMigration.selectAll')}</Label>
			</div>

			<div class="border-t pt-3">
				<div class="grid grid-cols-1 gap-2 sm:grid-cols-2 md:grid-cols-3">
					{#each EXPORTABLE_TABLES as table (table)}
						<div class="flex items-center space-x-2">
							<Checkbox
								id="table-{table}"
								checked={selectedTables.includes(table)}
								onCheckedChange={() => handleToggleTable(table)}
							/>
							<Label for="table-{table}">{t(`dataMigration.table.${table}`)}</Label>
						</div>
					{/each}
				</div>
			</div>

			<Button onclick={handleExport} disabled={exporting}>
				{#if exporting}
					<Loader2 class="mr-2 h-4 w-4 animate-spin" />
					{t('dataMigration.exporting')}
				{:else}
					<Download class="mr-2 h-4 w-4" />
					{t('dataMigration.export')}
				{/if}
			</Button>
		</Card.Content>
	</Card.Root>

	<!-- Import Section -->
	<Card.Root>
		<Card.Header>
			<Card.Title class="flex items-center gap-2">
				<Upload class="h-5 w-5" />
				{t('dataMigration.importTitle')}
			</Card.Title>
			<Card.Description>
				{t('dataMigration.importDesc')}
			</Card.Description>
		</Card.Header>
		<Card.Content class="space-y-4">
			<div class="flex items-center gap-3">
				<Button variant="outline" onclick={() => fileInput?.click()}>
					<CloudUpload class="mr-2 h-4 w-4" />
					{t('dataMigration.selectFile')}
				</Button>
				<input
					bind:this={fileInput}
					type="file"
					accept=".json"
					class="hidden"
					onchange={handleFileSelect}
				/>
				{#if importFile}
					<span class="text-sm text-muted-foreground">{importFile.name}</span>
				{/if}
			</div>

			<Button variant="secondary" onclick={handleImport} disabled={importing || !importFile}>
				{#if importing}
					<Loader2 class="mr-2 h-4 w-4 animate-spin" />
					{t('dataMigration.importing')}
				{:else}
					<Upload class="mr-2 h-4 w-4" />
					{t('dataMigration.import')}
				{/if}
			</Button>

			{#if importResults}
				<div class="mt-4 max-w-lg rounded-md border">
					<Table.Root>
						<Table.Header>
							<Table.Row>
								<Table.Head>{t('dataMigration.resultTable')}</Table.Head>
								<Table.Head class="text-right">{t('dataMigration.resultCreated')}</Table.Head>
								<Table.Head class="text-right">{t('dataMigration.resultUpdated')}</Table.Head>
							</Table.Row>
						</Table.Header>
						<Table.Body>
							{#each Object.entries(importResults) as [table, counts] (table)}
								<Table.Row>
									<Table.Cell>{t(`dataMigration.table.${table}`) || table}</Table.Cell>
									<Table.Cell class="text-right">{counts.created}</Table.Cell>
									<Table.Cell class="text-right">{counts.updated}</Table.Cell>
								</Table.Row>
							{/each}
						</Table.Body>
					</Table.Root>
				</div>
			{/if}
		</Card.Content>
	</Card.Root>
</div>
