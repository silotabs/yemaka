import { defineConfig, type Plugin } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

function fullReloadOnFrontendChange(): Plugin {
  return {
    name: 'yemaka-full-page-reload',
    apply: 'serve',
    handleHotUpdate(context) {
      const file = context.file.replace(/\\/g, '/');
      const shouldReload =
        file.includes('/frontend/src/') ||
        file.endsWith('/frontend/index.html') ||
        file.endsWith('/frontend/tailwind.config.cjs') ||
        file.endsWith('/frontend/postcss.config.cjs');

      if (!shouldReload) {
        return;
      }

      context.server.ws.send({
        type: 'full-reload',
        path: '*'
      });
      return [];
    }
  };
}

export default defineConfig({
  plugins: [svelte(), fullReloadOnFrontendChange()],
  clearScreen: false,
  server: {
    host: '127.0.0.1',
    port: 5173,
    strictPort: true,
    hmr: {
      host: '127.0.0.1',
      port: 5173
    }
  }
});
