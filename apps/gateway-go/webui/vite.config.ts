import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  base: '/',
  plugins: [vue()],
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
