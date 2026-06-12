import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

export default defineConfig({
  plugins: [vue()],
  server: {
    host: '0.0.0.0',
    port: 5174,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:3100',
        changeOrigin: true,
        configure(proxy) {
          proxy.on('proxyReq', (proxyReq, req) => {
            const remoteAddress = req.socket.remoteAddress?.replace('::ffff:', '') ?? '';
            if (remoteAddress) {
              proxyReq.setHeader('X-Real-IP', remoteAddress);
              proxyReq.setHeader('X-Forwarded-For', remoteAddress);
            }
          });
        },
      },
    },
  },
});
