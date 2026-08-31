import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  base: '/agent/',
  plugins: [vue()],
  build: {
    outDir: '../../build/dist/agent',
    emptyOutDir: true,
  },
  test: {
    environment: 'jsdom',
  },
})
