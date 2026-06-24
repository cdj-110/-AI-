import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import { ElementPlusResolver } from 'unplugin-vue-components/resolvers'
import monacoEditorPlugin from 'vite-plugin-monaco-editor'

export default defineConfig({
  plugins: [
    vue(),
    AutoImport({
      imports: ['vue', 'vue-router', 'pinia', '@vueuse/core'],
      dts: './src/type/auto-imports.d.ts',
      resolvers: [ElementPlusResolver()],
      exclude: [/[\\/]maotuDist[\\/]/],
      // dirs:["./src/utils"] //自动导入自己封装的函数
    }),
    Components({
      resolvers: [ElementPlusResolver()],
    }),
    monacoEditorPlugin({}),
  ],
  css: {
    preprocessorOptions: {
      scss: {
        api: 'modern-compiler'
      },
    },
  },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    },
    extensions: ['.js', '.ts', '.jsx', '.tsx', '.json', '.vue', '.es']
  },
  server: {
    open: false,
    host: "0.0.0.0",
    port: 2024,
    proxy: {
      "^/api": {
        // target: "http://39.106.71.228:8000",
        // target: "http://192.168.1.101:8000",
        target: "http://192.168.1.201:8000",
        ws: true,
        changeOrigin: true,
        rewrite: (path) => path.replace("/api/", "")
      }
    }
  },
  build: {
    outDir: 'dist',
  }
})
