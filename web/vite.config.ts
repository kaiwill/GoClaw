import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue(), tailwindcss()],
  server: {
    port: 5173,
    host: '0.0.0.0',
    allowedHosts: ['9da1-240e-456-ff30-1b1d-c01e-6c46-64ba-9b04.ngrok-free.app'],
    proxy: {
      '/api': {
        target: 'http://localhost:4096',
        changeOrigin: true,
        secure: false,
        ws: true,
      },
    },
  },
})
