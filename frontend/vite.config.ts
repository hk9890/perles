import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// API port for development proxy.
// Set VITE_API_PORT env var to match the port shown when perles starts.
// Example: VITE_API_PORT=60440 npm run dev
const apiPort = process.env.VITE_API_PORT || '19999'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    proxy: {
      '/api': `http://localhost:${apiPort}`
    }
  }
})
