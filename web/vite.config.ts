import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'
import tailwindcss from '@tailwindcss/vite'

// The browser reaches each service under /api/<service> (D-084); nginx strips the prefix in the
// container stack (T-034) and this proxy does the same in `pnpm dev`. http-proxy streams the
// response as it arrives, so Server-Sent Events are not buffered.
const services = {
  '/api/proposal': 'http://localhost:8081',
  '/api/notification': 'http://localhost:8082',
  '/api/analytics': 'http://localhost:8083',
}

const proxy = Object.fromEntries(
  Object.entries(services).map(([prefix, target]) => [
    prefix,
    { target, rewrite: (path: string) => path.slice(prefix.length) || '/' },
  ]),
)

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue(), vueDevTools(), tailwindcss()],
  server: { proxy },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
})
