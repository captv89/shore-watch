import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  // Recorded runs live in ../runs and are bundled at build time.
  server: { fs: { allow: ['..'] } },
  // Relative paths so the static build works from a Hugging Face Space subpath.
  base: './',
  // The Natural Earth coastline is one lazy-loaded chunk of about 550 kB (180 kB gzipped).
  build: { chunkSizeWarningLimit: 600 },
})
