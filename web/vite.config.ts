import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

const BACKEND = "http://localhost:8080"

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
  ],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    proxy: {
      // Proxy all /v1/* API calls to the Go backend during development.
      // This avoids CORS issues and "<!doctype" JSON parse errors when
      // components use relative fetch("/v1/...") calls.
      "/v1": {
        target: BACKEND,
        changeOrigin: true,
      },
    },
  },
})
