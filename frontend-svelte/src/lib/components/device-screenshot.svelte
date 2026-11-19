<script lang="ts">
	import { t } from '$lib/i18n';
	import { devicesAPI } from '$lib/api';
	import { toast } from 'svelte-sonner';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Tabs from '$lib/components/ui/tabs';
	import { Button } from '$lib/components/ui/button';
	import { Loader2, RefreshCw } from '@lucide/svelte';

	let {
		deviceId = '',
		deviceName = '',
		open = $bindable(false),
	}: {
		deviceId: string;
		deviceName: string;
		open: boolean;
	} = $props();

	let activeDisplay = $state('phone');
	let loading = $state(false);
	let imageBase64 = $state('');
	let imageWidth = $state(0);
	let imageHeight = $state(0);
	let errorMessage = $state('');

	async function captureScreen() {
		if (!deviceId || loading) return;
		loading = true;
		imageBase64 = '';
		errorMessage = '';

		try {
			const data = await devicesAPI.screenshot(deviceId, activeDisplay);
			imageBase64 = data.image_base64;
			imageWidth = data.width || 0;
			imageHeight = data.height || 0;
		} catch (e: any) {
			errorMessage = e.message || t('devices.screenshotFailed');
			toast.error(errorMessage);
		} finally {
			loading = false;
		}
	}

	function handleTabChange(value: string) {
		activeDisplay = value;
		imageBase64 = '';
		errorMessage = '';
	}

	$effect(() => {
		if (!open) {
			imageBase64 = '';
			errorMessage = '';
			activeDisplay = 'phone';
		}
	});
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-2xl max-h-[85vh] flex flex-col">
		<Dialog.Header>
			<Dialog.Title>{t('devices.screenshotTitle').replace('{device}', deviceName)}</Dialog.Title>
		</Dialog.Header>

		<Tabs.Root value={activeDisplay} onValueChange={handleTabChange}>
			<Tabs.List>
				<Tabs.Trigger value="phone">{t('devices.phoneScreen')}</Tabs.Trigger>
				<Tabs.Trigger value="external">{t('devices.externalDisplay')}</Tabs.Trigger>
			</Tabs.List>
		</Tabs.Root>

		<div class="flex-1 flex flex-col items-center justify-center min-h-[200px]">
			{#if loading}
				<div class="flex flex-col items-center gap-3 py-8">
					<Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
					<span class="text-sm text-muted-foreground">{t('devices.capturing')}</span>
				</div>
			{:else if errorMessage}
				<div class="flex flex-col items-center gap-3 py-8">
					<p class="text-sm text-destructive">{errorMessage}</p>
					<Button variant="outline" size="sm" onclick={captureScreen}>
						<RefreshCw class="h-4 w-4 mr-2" />
						{t('common.retry')}
					</Button>
				</div>
			{:else if imageBase64}
				<div class="flex flex-col items-center gap-2">
					<img
						src="data:image/jpeg;base64,{imageBase64}"
						alt="Screenshot"
						class="max-w-full max-h-[60vh] rounded-lg border"
					/>
					<div class="flex items-center gap-3">
						<span class="text-xs text-muted-foreground">{imageWidth} x {imageHeight}</span>
						<Button variant="outline" size="sm" onclick={captureScreen}>
							<RefreshCw class="h-4 w-4 mr-2" />
							{t('devices.recapture')}
						</Button>
					</div>
				</div>
			{:else}
				<div class="flex flex-col items-center gap-3 py-8">
					<Button variant="outline" size="sm" onclick={captureScreen}>
						<RefreshCw class="h-4 w-4 mr-2" />
						{t('devices.capture')}
					</Button>
				</div>
			{/if}
		</div>
	</Dialog.Content>
</Dialog.Root>
