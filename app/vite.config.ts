import tailwindcss from '@tailwindcss/vite';
import devtools from 'solid-devtools/vite';
import { defineConfig, loadEnv } from 'vite';
import solidPlugin from 'vite-plugin-solid';

export default defineConfig(({ mode }) => {
  // Load local .env files
  const env = loadEnv(mode, process.cwd(), '');

  // This will print every available key to your Cloudflare Build Logs
  console.log("--- ALL PROCESS.ENV KEYS ---");
  console.log(Object.keys(process.env).join(', '));
  console.log("----------------------------");

  // This will print every available key to your Cloudflare Build Logs
  console.log("--- ALL ENV KEYS ---");
  console.log(Object.keys(env).join(', '));
  console.log("----------------------------");

  // Cloudflare injects variables into process.env during build.
  // We check the loaded env first, then fall back to the system process.env.
  const target = env.RELAY_API_URL || process.env.RELAY_API_URL;

  // Debugging (optional): This will show up in your Cloudflare build logs
  if (!target) {
    console.log("Current Mode:", mode);
    console.log("Available Env Keys:", Object.keys(env));
    throw new Error("RELAY_API_URL is not configured. Check Cloudflare Dashboard -> Settings -> Environment Variables.");
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