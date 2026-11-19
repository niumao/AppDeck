<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from '$lib/i18n';
	import { notebookAPI } from '$lib/api';
	import { toast } from 'svelte-sonner';
	import { Textarea } from '$lib/components/ui/textarea';

	let content = $state('');
	let loading = $state(true);
	let saveState: 'idle' | 'saving' | 'saved' = $state('idle');
	let saveDebounce: ReturnType<typeof setTimeout> | undefined;

	onMount(() => {
		loadNote();
	});

	async function loadNote() {
		try {
			const data = await notebookAPI.get();
			content = data.content ?? '';
		} catch (e: any) {
			toast.error(e.message || 'Failed to load notebook');
		} finally {
			loading = false;
		}
	}

	function handleInput() {
		clearTimeout(saveDebounce);
		saveDebounce = setTimeout(saveNote, 800);
	}

	async function saveNote() {
		saveState = 'saving';
		try {
			await notebookAPI.save(content);
			saveState = 'saved';
		} catch (e: any) {
			toast.error(e.message || 'Failed to save notebook');
			saveState = 'idle';
		}
	}
</script>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<h2 class="text-2xl font-bold tracking-tight">{t('notebook.title')}</h2>
		{#if saveState === 'saving'}
			<span class="text-sm text-muted-foreground">{t('notebook.saving')}</span>
		{:else if saveState === 'saved'}
			<span class="text-sm text-muted-foreground">{t('notebook.saved')}</span>
		{/if}
	</div>

	{#if !loading}
		<Textarea
			bind:value={content}
			oninput={handleInput}
			placeholder={t('notebook.placeholder')}
			class="min-h-[70vh] resize-none font-mono text-sm"
		/>
	{/if}
</div>
