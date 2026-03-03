import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  base: '/dashboard/',
  build: {
    outDir: '../internal/dashboard/dist',
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    proxy: {
      '/dashboard/api': 'http://localhost:8081',
      '/dashboard/events': 'http://localhost:8081',
      '/dashboard/login': 'http://localhost:8081',
      '/dashboard/qr': 'http://localhost:8081',
    },
  },
})
