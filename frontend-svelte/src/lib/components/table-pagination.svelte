<script lang="ts">
	import { t } from '$lib/i18n';
	import { Button } from '$lib/components/ui/button';

	let {
		page = $bindable(),
		rowsPerPage = $bindable(),
		total,
		rowsPerPageOptions = [10, 20, 50],
		onChange,
	}: {
		/** 0-indexed current page */
		page: number;
		rowsPerPage: number;
		total: number;
		rowsPerPageOptions?: number[];
		/**
		 * Called after page or rowsPerPage changes. Only needed by callers that
		 * reload explicitly (e.g. `onclick={() => loadX()}` elsewhere in the page)
		 * rather than via their own `$effect` watching `page`/`rowsPerPage`.
		 */
		onChange?: () => void;
	} = $props();

	const totalPages = $derived(Math.ceil(total / rowsPerPage));
	const rangeFrom = $derived(total === 0 ? 0 : page * rowsPerPage + 1);
	const rangeTo = $derived(Math.min((page + 1) * rowsPerPage, total));

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

{#if total > 0}
	<div class="flex items-center justify-between border-t px-4 py-3">
		<div class="flex items-center gap-2 text-sm text-muted-foreground">
			<span>{t('common.rowsPerPage')}:</span>
			<select
				class="rounded border bg-transparent px-1 py-0.5 text-sm"
				value={rowsPerPage}
				onchange={(e) => {
					changeRowsPerPage(parseInt((e.currentTarget as HTMLSelectElement).value, 10));
				}}
			>
				{#each rowsPerPageOptions as option (option)}
					<option value={option}>{option}</option>
				{/each}
			</select>
			<span>{rangeFrom}-{rangeTo} / {total}</span>
		</div>
		<div class="flex items-center gap-1">
			<Button variant="outline" size="sm" disabled={page === 0} onclick={() => goToPage(page - 1)}>
				&laquo;
			</Button>
			<Button
				variant="outline"
				size="sm"
				disabled={page >= totalPages - 1}
				onclick={() => goToPage(page + 1)}
			>
				&raquo;
			</Button>
		</div>
	</div>
{/if}
