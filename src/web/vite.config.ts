import { defineConfig } from 'vite'
import type { Plugin } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'node:path'
import { readFile } from 'node:fs/promises'

function mswWorkerDevPlugin(): Plugin {
  const workerPath = path.resolve(import.meta.dirname, './node_modules/msw/lib/mockServiceWorker.js')
  return {
    name: 'msw-worker-dev-only',
    apply: 'serve',
    configureServer(server) {
      server.middlewares.use(async (request, response, next) => {
        if (request.url?.split('?')[0] !== '/mockServiceWorker.js') {
          next()
          return
        }
        try {
          const worker = await readFile(workerPath)
          response.statusCode = 200
          response.setHeader('Content-Type', 'text/javascript; charset=utf-8')
          response.setHeader('Cache-Control', 'no-store')
          response.end(worker)
        } catch (error) {
          next(error)
        }
      })
    },
  }
}

export default defineConfig({
  plugins: [mswWorkerDevPlugin(), react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(import.meta.dirname, './src'),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://127.0.0.1:9095',
      '/healthz': 'http://127.0.0.1:9095',
      '/readyz': 'http://127.0.0.1:9095',
    },
  },
})
