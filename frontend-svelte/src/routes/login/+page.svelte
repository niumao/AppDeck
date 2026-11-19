<script lang="ts">
	import { goto } from '$app/navigation';
	import { base } from '$app/paths';
	import { t } from '$lib/i18n';
	import { authAPI } from '$lib/api';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Alert from '$lib/components/ui/alert';

	let username = $state('');
	let password = $state('');
	let error = $state('');
	let loading = $state(false);

	async function handleSubmit(e: Event) {
		e.preventDefault();
		error = '';
		loading = true;
		try {
			const data = await authAPI.login({ username, password });
			localStorage.setItem('admin_token', data.token);
			localStorage.setItem('admin_user', JSON.stringify(data.admin));
			goto(`${base}/apps`);
		} catch (err: any) {
			error = err.message || t('login.failed');
		} finally {
			loading = false;
		}
	}
</script>

<div class="login-space relative flex min-h-screen items-center justify-center overflow-hidden">
	<!-- Deep space background -->
	<div class="absolute inset-0 bg-[#050b1a]"></div>

	<!-- Star layers -->
	<div class="stars-sm absolute inset-0"></div>
	<div class="stars-md absolute inset-0"></div>
	<div class="stars-lg absolute inset-0"></div>

	<!-- Nebula glow -->
	<div class="absolute top-[-20%] left-[-10%] h-[600px] w-[600px] rounded-full bg-[#1a0a3e]/40 blur-[120px]"></div>
	<div class="absolute bottom-[-20%] right-[-10%] h-[500px] w-[500px] rounded-full bg-[#0a1a3e]/50 blur-[100px]"></div>
	<div class="nebula-drift absolute top-[30%] right-[20%] h-[300px] w-[400px] rounded-full bg-[#1e0640]/30 blur-[80px]"></div>

	<!-- Shooting stars -->
	<div class="shooting-star shooting-star-1"></div>
	<div class="shooting-star shooting-star-2"></div>
	<div class="shooting-star shooting-star-3"></div>

	<!-- Login card -->
	<div class="relative z-10 w-full max-w-[420px] px-4">
		<div class="rounded-2xl border border-white/[0.08] bg-white/[0.04] p-8 shadow-2xl shadow-black/50 backdrop-blur-xl">
			<!-- Header -->
			<div class="mb-8 text-center">
				<div class="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-blue-500/10 ring-1 ring-blue-400/20">
					<svg class="h-7 w-7 text-blue-400" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="7" height="7" x="3" y="3" rx="1"/><rect width="7" height="7" x="14" y="3" rx="1"/><rect width="7" height="7" x="14" y="14" rx="1"/><rect width="7" height="7" x="3" y="14" rx="1"/></svg>
				</div>
				<h1 class="text-2xl font-bold tracking-tight text-gray-100">{t('login.title')}</h1>
				<p class="mt-1.5 text-sm text-gray-400">{t('login.subtitle')}</p>
			</div>

			{#if error}
				<Alert.Root variant="destructive" class="mb-5">
					<Alert.Description>{error}</Alert.Description>
				</Alert.Root>
			{/if}

			<!-- Form -->
			<form onsubmit={handleSubmit} class="space-y-5">
				<div class="space-y-2">
					<Label for="username" class="text-sm font-medium text-gray-300">{t('login.username')}</Label>
					<Input
						id="username"
						bind:value={username}
						autocomplete="username"
						autofocus
						required
						class="h-11 rounded-xl border-white/[0.08] bg-white/[0.05] text-gray-100 placeholder:text-gray-500 transition-all focus:border-blue-500/40 focus:bg-white/[0.08]"
					/>
				</div>
				<div class="space-y-2">
					<Label for="password" class="text-sm font-medium text-gray-300">{t('login.password')}</Label>
					<Input
						id="password"
						type="password"
						bind:value={password}
						autocomplete="current-password"
						required
						class="h-11 rounded-xl border-white/[0.08] bg-white/[0.05] text-gray-100 placeholder:text-gray-500 transition-all focus:border-blue-500/40 focus:bg-white/[0.08]"
					/>
				</div>
				<Button type="submit" class="mt-2 h-11 w-full rounded-xl bg-blue-600 font-semibold text-white shadow-lg shadow-blue-500/20 transition-all hover:bg-blue-500 hover:shadow-xl hover:shadow-blue-500/30" disabled={loading}>
					{loading ? t('login.submitting') : t('login.submit')}
				</Button>
			</form>
		</div>

		<!-- Footer -->
		<p class="mt-6 text-center text-xs text-gray-500/60">AppDeck Admin</p>
	</div>
</div>

<style>
	/* ── Star field layers ─────────────────────────────── */
	.stars-sm,
	.stars-md,
	.stars-lg {
		background-repeat: repeat;
		pointer-events: none;
	}

	/* Small stars - dense, subtle */
	.stars-sm {
		background-image:
			radial-gradient(1px 1px at 50px 80px, #fff8, transparent),
			radial-gradient(1px 1px at 180px 30px, #fff6, transparent),
			radial-gradient(1px 1px at 90px 190px, #fff5, transparent),
			radial-gradient(1px 1px at 250px 120px, #fff7, transparent),
			radial-gradient(1px 1px at 30px 260px, #fff5, transparent),
			radial-gradient(1px 1px at 310px 200px, #fff6, transparent),
			radial-gradient(1px 1px at 140px 280px, #fff4, transparent),
			radial-gradient(1px 1px at 370px 60px, #fff6, transparent),
			radial-gradient(1px 1px at 400px 300px, #fff5, transparent),
			radial-gradient(1px 1px at 220px 340px, #fff7, transparent),
			radial-gradient(1px 1px at 460px 150px, #fff5, transparent),
			radial-gradient(1px 1px at 500px 250px, #fff4, transparent),
			radial-gradient(1px 1px at 80px 380px, #fff6, transparent),
			radial-gradient(1px 1px at 340px 400px, #fff5, transparent),
			radial-gradient(1px 1px at 560px 80px, #fff6, transparent),
			radial-gradient(1px 1px at 600px 350px, #fff5, transparent);
		background-size: 650px 450px;
		animation: drift-slow 120s linear infinite;
	}

	/* Medium stars */
	.stars-md {
		background-image:
			radial-gradient(1.5px 1.5px at 120px 60px, #fffb, transparent),
			radial-gradient(1.5px 1.5px at 350px 180px, #fff9, transparent),
			radial-gradient(1.5px 1.5px at 60px 300px, #fffa, transparent),
			radial-gradient(1.5px 1.5px at 480px 100px, #fff8, transparent),
			radial-gradient(1.5px 1.5px at 200px 400px, #fffb, transparent),
			radial-gradient(1.5px 1.5px at 550px 280px, #fff9, transparent),
			radial-gradient(1.5px 1.5px at 300px 350px, #fffa, transparent),
			radial-gradient(1.5px 1.5px at 420px 440px, #fff8, transparent),
			radial-gradient(1.5px 1.5px at 150px 160px, #fff9, transparent),
			radial-gradient(1.5px 1.5px at 500px 40px, #fffb, transparent);
		background-size: 600px 500px;
		animation: twinkle-md 4s ease-in-out infinite alternate, drift-med 90s linear infinite;
	}

	/* Large bright stars */
	.stars-lg {
		background-image:
			radial-gradient(2px 2px at 200px 150px, #fff, transparent),
			radial-gradient(2.5px 2.5px at 450px 320px, #e8e0ff, transparent),
			radial-gradient(2px 2px at 100px 420px, #cce0ff, transparent),
			radial-gradient(2.5px 2.5px at 550px 80px, #fff, transparent),
			radial-gradient(2px 2px at 320px 250px, #ffe8d0, transparent),
			radial-gradient(2px 2px at 50px 50px, #e0e8ff, transparent),
			radial-gradient(2.5px 2.5px at 400px 450px, #fff, transparent);
		background-size: 650px 520px;
		animation: twinkle-lg 3s ease-in-out infinite alternate;
	}

	/* ── Shooting stars ────────────────────────────────── */
	.shooting-star {
		position: absolute;
		width: 120px;
		height: 1.5px;
		background: linear-gradient(90deg, #fff0, #fffc 30%, #fff 80%, #fff0);
		border-radius: 50px;
		opacity: 0;
		z-index: 1;
	}

	.shooting-star-1 {
		top: 15%;
		left: 10%;
		transform: rotate(-30deg);
		animation: shoot 6s ease-in-out 2s infinite;
	}
	.shooting-star-2 {
		top: 35%;
		left: 60%;
		width: 80px;
		transform: rotate(-25deg);
		animation: shoot 8s ease-in-out 5s infinite;
	}
	.shooting-star-3 {
		top: 55%;
		left: 30%;
		width: 100px;
		transform: rotate(-35deg);
		animation: shoot 10s ease-in-out 0s infinite;
	}

	/* ── Nebula drift ──────────────────────────────────── */
	.nebula-drift {
		animation: nebula 20s ease-in-out infinite alternate;
	}

	/* ── Keyframes ─────────────────────────────────────── */
	@keyframes drift-slow {
		from { transform: translateY(0); }
		to   { transform: translateY(-450px); }
	}
	@keyframes drift-med {
		from { transform: translateX(0); }
		to   { transform: translateX(-600px); }
	}
	@keyframes twinkle-md {
		0%   { opacity: 0.6; }
		100% { opacity: 1; }
	}
	@keyframes twinkle-lg {
		0%   { opacity: 0.7; }
		50%  { opacity: 1; }
		100% { opacity: 0.8; }
	}
	@keyframes shoot {
		0%   { opacity: 0; transform: rotate(-30deg) translateX(0); }
		2%   { opacity: 1; }
		5%   { opacity: 0; transform: rotate(-30deg) translateX(350px); }
		100% { opacity: 0; }
	}
	@keyframes nebula {
		0%   { transform: translate(0, 0) scale(1); opacity: 0.3; }
		100% { transform: translate(-40px, 30px) scale(1.15); opacity: 0.45; }
	}
</style>
