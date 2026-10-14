import { defineConfig, loadEnv } from 'vite';
import vue from '@vitejs/plugin-vue';

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), 'VITE_');
  const admin = env.VITE_PORTAL_MODE === 'admin';
  return {
    plugins: [vue()],
    build: { outDir: admin ? 'dist/admin' : 'dist/tenant', emptyOutDir: true },
    server: {
      host: '0.0.0.0',
      port: admin ? 5176 : 5174,
      proxy: {
        '/api': {
          target: 'http://127.0.0.1:3100',
          changeOrigin: true,
          ws: true,
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
  };
});
