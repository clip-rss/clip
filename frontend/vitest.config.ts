import { defineConfig } from 'vitest/config'

// 仅单元测试用配置，不引入 wails/tailwind 插件（避免在测试环境中执行）。
export default defineConfig({
  test: {
    environment: 'jsdom',
    globals: true,
    include: ['src/**/*.{test,spec}.{ts,tsx}'],
  },
})
