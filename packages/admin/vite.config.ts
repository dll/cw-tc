import { fileURLToPath, URL } from 'node:url'
// 从 vitest/config 引 defineConfig，才能带上 test 段的类型
import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

// 管理端构建配置（对照 tsloms/packages/admin 的约定）。
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5174,
    // 本地开发把 /api 反代到本机后端（127.0.0.1:8071，与 deploy/README.md 一致）
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8071',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: false,
  },
  test: {
    environment: 'jsdom',
  },
})
