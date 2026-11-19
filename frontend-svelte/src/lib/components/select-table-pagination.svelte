<script lang="ts">
	import { t } from '$lib/i18n';
	import * as Select from '$lib/components/ui/select';
	import { Button } from '$lib/components/ui/button';
	import { ChevronLeft, ChevronRight } from '@lucide/svelte';

	let {
		page = $bindable(),
		rowsPerPage = $bindable(),
		total,
		rowsPerPageOptions = [10, 20, 50],
		showPageNumber = false,
		containerClass = 'border-t px-4 py-3',
		onChange,
	}: {
		/** 0-indexed current page */
		page: number;
		rowsPerPage: number;
		total: number;
		rowsPerPageOptions?: number[];
		/** Show a "page X / Y" indicator between the prev/next buttons. */
		showPageNumber?: boolean;
		containerClass?: string;
		/**
		 * Called after page or rowsPerPage changes. Only needed by callers that
		 * reload explicitly rather than via their own `$effect`/`$derived` watching
		 * `page`/`rowsPerPage`.
		 */
		onChange?: () => void;
	} = $props();

	const totalPages = $derived(Math.ceil(total / rowsPerPage));
	const displayFrom = $derived(total === 0 ? 0 : page * rowsPerPage + 1);
	const displayTo = $derived(Math.min((page + 1) * rowsPerPage, total));

	function goToPage(p: number) {
		page = p;
		onChange?.();
	}

	function changeRowsPerPage(n: number) {
		rowsPerPage = n;
		page = 0;
		onChange?.();
	}
</script>

<div class="flex items-center justify-between {containerClass}">
	<div class="flex items-center gap-2">
		<span class="text-sm text-muted-foreground">{t('common.rowsPerPage')}</span>
		<Select.Root
			type="single"
			value={String(rowsPerPage)}
			onValueChange={(v) => changeRowsPerPage(Number(v))}
		>
			<Select.Trigger class="w-[70px]">
				{rowsPerPage}
			</Select.Trigger>
			<Select.Content>
				{#each rowsPerPageOptions as option (option)}
					<Select.Item value={String(option)}>{option}</Select.Item>
				{/each}
			</Select.Content>
		</Select.Root>
	</div>
	<div class="flex items-center gap-4">
		<span class="text-sm text-muted-foreground">
			{displayFrom}-{displayTo} / {total}
		</span>
		<div class="flex items-center gap-1">
			<Button variant="outline" size="icon-sm" disabled={page === 0} onclick={() => goToPage(page - 1)}>
				<ChevronLeft class="h-4 w-4" />
			</Button>
			{#if showPageNumber}
				<span class="text-sm text-muted-foreground min-w-[3rem] text-center">
					{page + 1} / {totalPages || 1}
				</span>
			{/if}
			<Button
				variant="outline"
				size="icon-sm"
				disabled={page >= totalPages - 1}
				onclick={() => goToPage(page + 1)}
			>
				<ChevronRight class="h-4 w-4" />
			</Button>
		</div>
	</div>
</div>
