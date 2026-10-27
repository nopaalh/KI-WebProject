import { defineConfig, loadEnv } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), 'DEV_')

  return {
    plugins: [svelte()],
    server: {
      port: 5173,
      strictPort: true,
      watch: { usePolling: env.DEV_USE_POLLING === 'true' },
      proxy: {
        '/api': 'http://localhost:8080',
      },
    },
  }
})
