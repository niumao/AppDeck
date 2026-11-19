import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig, loadEnv } from 'vite';

export default defineConfig(({ mode }) => {
	const env = loadEnv(mode, process.cwd(), '');
	const backendUrl = env.VITE_BACKEND_URL || 'http://localhost:80';

	return {
		plugins: [tailwindcss(), sveltekit()],
		server: {
			host: '0.0.0.0',
			port: 3000,
			proxy: {
				'/api': {
					target: backendUrl,
					changeOrigin: true
				}
			}
		}
	};
});
