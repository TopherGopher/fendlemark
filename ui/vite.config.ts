import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import dts from 'vite-plugin-dts'
import { fileURLToPath } from 'node:url'

export default defineConfig({
  plugins: [vue(), dts({ bundleTypes: true, tsconfigPath: './tsconfig.json' })],
  build: {
    lib: {
      entry: fileURLToPath(new URL('./src/index.ts', import.meta.url)),
      name: 'FendlemarkUI',
      fileName: 'fendlemark-ui',
      cssFileName: 'tokens',
    },
    rolldownOptions: {
      external: ['vue'],
      output: { globals: { vue: 'Vue' } },
    },
    cssCodeSplit: false,
  },
})
