import { toast } from 'svelte-sonner';

/**
 * Runs an async action and reports the result via toast, replacing the
 * try/success-toast/catch/error-toast block that used to be hand-copied
 * around every create/update/delete call. Returns whether the action
 * succeeded, so callers can decide whether to close a dialog or refresh a list.
 */
export async function runWithToast(
	action: () => Promise<void>,
	options: { successMessage?: string; errorFallback: string; logLabel?: string }
): Promise<boolean> {
	try {
		await action();
		if (options.successMessage) toast.success(options.successMessage);
		return true;
	} catch (error: any) {
		console.error(options.logLabel || options.errorFallback, error);
		toast.error(error?.message || options.errorFallback);
		return false;
	}
}
