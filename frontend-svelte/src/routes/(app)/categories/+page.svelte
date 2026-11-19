<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from '$lib/i18n';
	import { categoriesAPI } from '$lib/api';
	import { toast } from 'svelte-sonner';
	import { Plus, Pencil, Trash2 } from '@lucide/svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Table from '$lib/components/ui/table';
	import ConfirmActionDialog from '$lib/components/confirm-action-dialog.svelte';
	import { runWithToast } from '$lib/action-feedback';

	interface Category {
		id: number;
		name: string;
		order?: number;
		appCount?: number;
		createdAt: string;
	}

	let categories: Category[] = $state([]);
	let loading = $state(true);

	// Create/Edit dialog state
	let dialogOpen = $state(false);
	let dialogMode: 'create' | 'edit' = $state('create');
	let editCategory: Category | null = $state(null);
	let formName = $state('');
	let formOrder = $state(100);

	// Delete confirm dialog state
	let deleteDialogOpen = $state(false);
	let deleteCategory: Category | null = $state(null);

	onMount(() => {
		loadData();
	});

	async function loadData() {
		try {
			loading = true;
			const data = await categoriesAPI.getAll();
			categories = data || [];
		} catch (error: any) {
			toast.error(t('common.loadFailed') + ': ' + error.message);
		} finally {
			loading = false;
		}
	}

	function openCreateDialog() {
		dialogMode = 'create';
		editCategory = null;
		formName = '';
		formOrder = 100;
		dialogOpen = true;
	}

	function openEditDialog(category: Category) {
		dialogMode = 'edit';
		editCategory = category;
		formName = category.name;
		formOrder = category.order ?? 100;
		dialogOpen = true;
	}

	function closeDialog() {
		dialogOpen = false;
		editCategory = null;
		formName = '';
		formOrder = 100;
	}

	async function handleSubmit() {
		if (!formName.trim()) {
			toast.error(t('categories.enterName'));
			return;
		}

		try {
			if (dialogMode === 'create') {
				await categoriesAPI.create({ name: formName, order: Number(formOrder) });
				toast.success(t('common.createSuccess'));
			} else {
				await categoriesAPI.update(editCategory!.id, { name: formName, order: Number(formOrder) });
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

	function openDeleteDialog(category: Category) {
		deleteCategory = category;
		deleteDialogOpen = true;
	}

	async function handleDelete() {
		if (!deleteCategory) return;

		const ok = await runWithToast(() => categoriesAPI.delete(deleteCategory!.id), {
			successMessage: t('common.deleteSuccess'),
			errorFallback: t('common.deleteFailed'),
			logLabel: 'Failed to delete category:',
		});
		if (ok) {
			deleteDialogOpen = false;
			deleteCategory = null;
			loadData();
		}
	}
</script>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<h2 class="text-2xl font-bold tracking-tight">{t('categories.title')}</h2>
		<Button onclick={openCreateDialog}>
			<Plus class="mr-2 h-4 w-4" />
			{t('categories.add')}
		</Button>
	</div>

	<div class="rounded-md border">
		<Table.Root>
			<Table.Header>
				<Table.Row>
					<Table.Head>{t('common.name')}</Table.Head>
					<Table.Head>{t('categories.appCount')}</Table.Head>
					<Table.Head>{t('common.sortOrder')}</Table.Head>
					<Table.Head>{t('common.createdAt')}</Table.Head>
					<Table.Head class="text-right">{t('common.actions')}</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#if loading}
					<Table.Row>
						<Table.Cell colspan={5} class="text-center">
							{t('common.loading')}
						</Table.Cell>
					</Table.Row>
				{:else if categories.length === 0}
					<Table.Row>
						<Table.Cell colspan={5} class="text-center">
							{t('common.noData')}
						</Table.Cell>
					</Table.Row>
				{:else}
					{#each categories as category (category.id)}
						<Table.Row>
							<Table.Cell>{category.name}</Table.Cell>
							<Table.Cell>{category.appCount || 0}</Table.Cell>
							<Table.Cell>{category.order}</Table.Cell>
							<Table.Cell>
								{new Date(category.createdAt).toLocaleString('zh-CN')}
							</Table.Cell>
							<Table.Cell class="text-right">
								<Button
									variant="ghost"
									size="icon"
									onclick={() => openEditDialog(category)}
								>
									<Pencil class="h-4 w-4" />
								</Button>
								<Button
									variant="ghost"
									size="icon"
									onclick={() => openDeleteDialog(category)}
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

<!-- Create/Edit Dialog -->
<Dialog.Root bind:open={dialogOpen}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>
				{dialogMode === 'create' ? t('categories.add') : t('categories.editCategory')}
			</Dialog.Title>
			<Dialog.Description>
				{dialogMode === 'create' ? t('categories.add') : t('categories.editCategory')}
			</Dialog.Description>
		</Dialog.Header>
		<div class="space-y-4 py-4">
			<div class="space-y-2">
				<Label for="category-name">{t('categories.categoryName')}</Label>
				<Input
					id="category-name"
					bind:value={formName}
					autofocus
				/>
			</div>
			<div class="space-y-2">
				<Label for="category-order">{t('common.sortOrder')}</Label>
				<Input
					id="category-order"
					type="number"
					bind:value={formOrder}
				/>
				<p class="text-sm text-muted-foreground">{t('common.sortHelp')}</p>
			</div>
		</div>
		<Dialog.Footer>
			<Button variant="outline" onclick={closeDialog}>{t('common.cancel')}</Button>
			<Button onclick={handleSubmit}>{t('common.save')}</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<!-- Delete Confirm Dialog -->
<ConfirmActionDialog
	bind:open={deleteDialogOpen}
	title={t('common.confirmDelete')}
	description={t('categories.confirmDeleteMsg').replace('{name}', deleteCategory?.name || '')}
	confirmLabel={t('common.delete')}
	confirmVariant="destructive"
	onConfirm={handleDelete}
	onCancel={() => (deleteCategory = null)}
/>
