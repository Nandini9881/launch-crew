/// <reference types="vitest/config" />
import { writeFileSync } from 'node:fs'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const outDir = '../internal/httpserver/dist'

// emptyOutDir deletes the committed .gitkeep; put it back after each build.
const keepGitkeep = {
  name: 'keep-gitkeep',
  closeBundle() {
    writeFileSync(`${outDir}/.gitkeep`, '')
  },
}

export default defineConfig({
  plugins: [react(), keepGitkeep],
  build: { outDir, emptyOutDir: true },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/auth': 'http://localhost:8080',
      '/github': 'http://localhost:8080',
    },
  },
  test: { globals: true, environment: 'jsdom', setupFiles: ['./src/setup.ts'] },
})
