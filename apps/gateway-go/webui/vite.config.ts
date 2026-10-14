import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath } from 'node:url'

export default defineConfig({
  base: '/',
  plugins: [vue()],
  // Keep every caller on the source module, even if stale emitted JS is present.
  resolve: { alias: [
    { find: /^(?:\.\/|\.\.\/)confirm$/, replacement: fileURLToPath(new URL('./src/confirm.ts', import.meta.url)) },
    { find: /^(?:\.\/|\.\.\/)stores\/config$/, replacement: fileURLToPath(new URL('./src/stores/config.ts', import.meta.url)) },
  ] },
  build: {
    outDir: '../internal/web/frontend/vue-dist',
    emptyOutDir: true,
    assetsDir: 'assets',
  },
  server: {
    host: '0.0.0.0',
    port: 5175,
    proxy: {
      '/api': { target: 'http://127.0.0.1:8088', changeOrigin: true, ws: true, rewriteWsOrigin: true },
      '/login': 'http://127.0.0.1:8088',
      '/packet-monitor': 'http://127.0.0.1:8088',
    },
  },
})
