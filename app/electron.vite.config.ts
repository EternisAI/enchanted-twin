import { resolve } from 'path'
import { defineConfig, externalizeDepsPlugin, loadEnv } from 'electron-vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { TanStackRouterVite } from '@tanstack/router-plugin/vite'

function inlineEnvVars(prefix: string, raw: Record<string, string>) {
  return Object.fromEntries(
    Object.entries(raw)
      .filter(([key]) => key.startsWith(prefix))
      .map(([key, val]) => [`process.env.${key}`, JSON.stringify(val)])
  )
}

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  // console.log('[electron-vite] Loaded ENV:', env)
  return {
    main: {
      plugins: [externalizeDepsPlugin()],
      define: {
        ...inlineEnvVars('', env),
        __APP_ENV__: JSON.stringify(env)
      }
    },
    preload: {
      plugins: [externalizeDepsPlugin()]
    },
    renderer: {
      base: './',
      root: 'src/renderer',
      resolve: {
        alias: {
          '@renderer': resolve('src/renderer/src')
        }
      },
      define: {
        __APP_ENV__: JSON.stringify(env)
      },
      plugins: [
        TanStackRouterVite({ target: 'react', autoCodeSplitting: true }),
        react(),
        tailwindcss()
      ]
    }
  }
})
