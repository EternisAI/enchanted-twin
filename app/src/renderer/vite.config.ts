import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  root: __dirname,
  build: {
    outDir: '../../dist/renderer',
    emptyOutDir: true
  },
  plugins: [react(), tailwindcss()]
})
