<script lang="ts">
	import { page } from '$app/state';
	import AppsForm from '$lib/components/apps-form.svelte';

	let appId = $derived(page.params.id);
</script>

<AppsForm {appId} />
