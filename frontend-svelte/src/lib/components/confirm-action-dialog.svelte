<script lang="ts">
	import type { Snippet } from 'svelte';
	import { t } from '$lib/i18n';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Alert from '$lib/components/ui/alert';
	import { Button, type ButtonVariant } from '$lib/components/ui/button';

	let {
		open = $bindable(false),
		title,
		description,
		warningText,
		warningTone = 'destructive',
		confirmLabel,
		confirmVariant = 'destructive',
		confirmClass,
		cancelLabel = t('common.cancel'),
		onConfirm,
		onCancel,
		children,
	}: {
		open?: boolean;
		title: string;
		description?: string;
		/** Text shown in the warning callout below the body content. Omit to skip the callout. */
		warningText?: string;
		/** 'destructive' renders a red callout; 'warning' renders the orange callout used for less-severe actions (e.g. reboot). */
		warningTone?: 'destructive' | 'warning';
		confirmLabel: string;
		confirmVariant?: ButtonVariant;
		/** Escape hatch for actions (like reboot) that use custom colors instead of a Button variant. */
		confirmClass?: string;
		cancelLabel?: string;
		onConfirm: () => void | Promise<void>;
		onCancel?: () => void;
		/** Body content describing what's about to be affected (e.g. a device/entity summary). */
		children?: Snippet;
	} = $props();

	function handleCancel() {
		open = false;
		onCancel?.();
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>{title}</Dialog.Title>
			{#if description}
				<Dialog.Description>{description}</Dialog.Description>
			{/if}
		</Dialog.Header>

		{@render children?.()}

		{#if warningText}
			<Alert.Root
				variant={warningTone === 'destructive' ? 'destructive' : undefined}
				class={warningTone === 'warning'
					? 'mt-2 border-orange-200 bg-orange-50 text-orange-800 dark:border-orange-900 dark:bg-orange-950 dark:text-orange-200'
					: 'mt-2'}
			>
				<Alert.Description>{warningText}</Alert.Description>
			</Alert.Root>
		{/if}

		<Dialog.Footer class="mt-4">
			<Button variant="outline" onclick={handleCancel}>
				{cancelLabel}
			</Button>
			<Button
				variant={confirmClass ? undefined : confirmVariant}
				class={confirmClass}
				onclick={onConfirm}
			>
				{confirmLabel}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
