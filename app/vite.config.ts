import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';
import solidPlugin from 'vite-plugin-solid';
import devtools from 'solid-devtools/vite';

export default defineConfig({
  plugins: [devtools(), solidPlugin(), tailwindcss()],
  server: {
    port: 3000,
    proxy: {
      '/relay': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
        secure: false,
        rewrite: (path: string) => path.replace(/^\/relay/, ''),
      },
    },
  },
  build: {
    target: 'esnext',
  },
  // SPA fallback - serve index.html for /node/* routes
  appType: 'spa',
});
