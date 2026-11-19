<script lang="ts">
	import { t } from '$lib/i18n';
	import { devicesAPI } from '$lib/api';
	import { toast } from 'svelte-sonner';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Button } from '$lib/components/ui/button';
	import { Loader2 } from '@lucide/svelte';

	interface ShellEntry {
		command: string;
		stdout: string;
		stderr: string;
		exit_code: number;
		status?: string;
		created_at?: string;
	}

	let {
		deviceId = '',
		deviceName = '',
		open = $bindable(false),
	}: {
		deviceId: string;
		deviceName: string;
		open: boolean;
	} = $props();

	let history: ShellEntry[] = $state([]);
	let inputValue = $state('');
	let executing = $state(false);
	let loadingHistory = $state(false);
	let scrollContainer: HTMLDivElement | undefined = $state(undefined);
	let commandStack: string[] = $state([]);
	let historyIndex = $state(-1);

	function scrollToBottom() {
		if (scrollContainer) {
			requestAnimationFrame(() => {
				if (scrollContainer) {
					scrollContainer.scrollTop = scrollContainer.scrollHeight;
				}
			});
		}
	}

	async function loadHistory() {
		if (!deviceId) return;
		loadingHistory = true;
		try {
			const data = await devicesAPI.shellHistory(deviceId, { limit: 50, offset: 0 });
			const items: ShellEntry[] = Array.isArray(data.items) ? data.items : [];
			// Reverse so oldest is first (API returns DESC)
			history = items.reverse();
			scrollToBottom();
		} catch (e: any) {
			console.error('Failed to load shell history:', e);
		} finally {
			loadingHistory = false;
		}
	}

	async function executeCommand() {
		const cmd = inputValue.trim();
		if (!cmd || executing) return;

		commandStack = [...commandStack, cmd];
		historyIndex = -1;
		inputValue = '';
		executing = true;

		try {
			const data = await devicesAPI.shell(deviceId, cmd);
			history = [...history, {
				command: cmd,
				stdout: data.stdout || '',
				stderr: data.stderr || '',
				exit_code: data.exit_code ?? -1,
				status: data.status || 'completed',
				created_at: data.created_at,
			}];
		} catch (e: any) {
			history = [...history, {
				command: cmd,
				stdout: '',
				stderr: e.message || 'Request failed',
				exit_code: -1,
				status: 'error',
			}];
		} finally {
			executing = false;
			scrollToBottom();
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault();
			executeCommand();
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			if (commandStack.length === 0) return;
			if (historyIndex === -1) {
				historyIndex = commandStack.length - 1;
			} else if (historyIndex > 0) {
				historyIndex--;
			}
			inputValue = commandStack[historyIndex];
		} else if (e.key === 'ArrowDown') {
			e.preventDefault();
			if (historyIndex === -1) return;
			if (historyIndex < commandStack.length - 1) {
				historyIndex++;
				inputValue = commandStack[historyIndex];
			} else {
				historyIndex = -1;
				inputValue = '';
			}
		}
	}

	// Load history when dialog opens
	$effect(() => {
		if (open && deviceId) {
			loadHistory();
		}
		if (!open) {
			history = [];
			inputValue = '';
			historyIndex = -1;
		}
	});
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-3xl max-h-[85vh] flex flex-col p-0">
		<Dialog.Header class="px-4 pt-4 pb-2">
			<Dialog.Title>{t('devices.shellTitle').replace('{device}', deviceName)}</Dialog.Title>
		</Dialog.Header>

		<div class="flex-1 flex flex-col min-h-0 px-4 pb-4">
			<!-- Terminal output area -->
			<div
				bind:this={scrollContainer}
				class="flex-1 min-h-0 overflow-y-auto bg-zinc-900 rounded-t-lg p-3 font-mono text-sm leading-relaxed"
				style="max-height: 60vh;"
			>
				{#if loadingHistory}
					<div class="text-zinc-500 text-center py-4">{t('common.loading')}</div>
				{:else if history.length === 0}
					<div class="text-zinc-500 text-center py-4">{t('devices.shellPlaceholder')}</div>
				{:else}
					{#each history as entry}
						<div class="mb-3">
							<!-- Command line -->
							<div class="text-green-400">
								<span class="text-green-600 select-none">$ </span>{entry.command}
							</div>
							<!-- stdout -->
							{#if entry.stdout}
								<pre class="text-zinc-200 whitespace-pre-wrap break-all mt-0.5">{entry.stdout}</pre>
							{/if}
							<!-- stderr -->
							{#if entry.stderr}
								<pre class="text-red-400 whitespace-pre-wrap break-all mt-0.5">{entry.stderr}</pre>
							{/if}
							<!-- Exit code badge -->
							{#if entry.status === 'timeout'}
								<span class="inline-block mt-1 px-1.5 py-0.5 text-xs rounded bg-yellow-900 text-yellow-300">
									{t('devices.shellTimeout')}
								</span>
							{:else if entry.exit_code !== 0}
								<span class="inline-block mt-1 px-1.5 py-0.5 text-xs rounded bg-red-900 text-red-300">
									exit {entry.exit_code}
								</span>
							{/if}
						</div>
					{/each}
				{/if}

				{#if executing}
					<div class="flex items-center gap-2 text-zinc-400">
						<Loader2 class="h-3 w-3 animate-spin" />
						<span>{t('devices.shellExecuting')}</span>
					</div>
				{/if}
			</div>

			<!-- Input area -->
			<div class="flex items-center bg-zinc-800 rounded-b-lg border-t border-zinc-700 px-3 py-2">
				<span class="text-green-500 font-mono mr-2 select-none">$</span>
				<input
					type="text"
					class="flex-1 bg-transparent text-zinc-100 font-mono text-sm outline-none placeholder:text-zinc-600"
					placeholder={t('devices.shellPlaceholder')}
					bind:value={inputValue}
					onkeydown={handleKeydown}
					disabled={executing}
					autofocus
				/>
				<Button
					variant="ghost"
					size="sm"
					class="text-zinc-400 hover:text-zinc-200 ml-2"
					onclick={executeCommand}
					disabled={executing || !inputValue.trim()}
				>
					{#if executing}
						<Loader2 class="h-4 w-4 animate-spin" />
					{:else}
						Run
					{/if}
				</Button>
			</div>
		</div>
	</Dialog.Content>
</Dialog.Root>
