import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import dts from 'vite-plugin-dts'
import { fileURLToPath } from 'node:url'

export default defineConfig({
  plugins: [vue(), dts({ rollupTypes: true, tsconfigPath: './tsconfig.json' })],
  build: {
    lib: {
      entry: fileURLToPath(new URL('./src/index.ts', import.meta.url)),
      name: 'FendlemarkUI',
      fileName: 'fendlemark-ui',
    },
    rollupOptions: {
      external: ['vue'],
      output: { globals: { vue: 'Vue' }, assetFileNames: (a) => (a.name === 'style.css' ? 'tokens.css' : a.name ?? 'asset') },
    },
    cssCodeSplit: false,
  },
})
