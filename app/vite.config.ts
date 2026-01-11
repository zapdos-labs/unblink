import tailwindcss from '@tailwindcss/vite';
import devtools from 'solid-devtools/vite';
import { defineConfig, loadEnv } from 'vite';
import solidPlugin from 'vite-plugin-solid';

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '');

  // Cloudflare injects variables into process.env during build.
  // We check the loaded env first, then fall back to the system process.env.
  const target = env.RELAY_API_URL;
  if (!target) {
    const is_cf = env.WORKERS_CI;
    const msg = is_cf
      ? 'Set this manually at Cloudflare Dashboard > Settings > Build > Build variables and secrets.'
      : 'Create a .env file with RELAY_API_URL var.';

    throw new Error(`RELAY_API_URL is not configured\n${msg}`);
  }

  return {
    plugins: [devtools(), solidPlugin(), tailwindcss()],
    server: {
      port: 3000,
      proxy: {
        '/relay': {
          target,
          changeOrigin: true,
          secure: false,
          rewrite: (path: string) => path.replace(/^\/relay/, ''),
        },
      },
    },
    build: {
      target: 'esnext',
    },
    appType: 'spa',
  };
});