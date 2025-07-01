import { resolve } from 'path'
import { defineConfig, externalizeDepsPlugin, loadEnv } from 'electron-vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { TanStackRouterVite } from '@tanstack/router-plugin/vite'
import fs from 'fs'

function inlineEnvVars(prefix: string, raw: Record<string, string>) {
  return Object.fromEntries(
    Object.entries(raw)
      .filter(([key]) => key.startsWith(prefix))
      .map(([key, val]) => [`process.env.${key}`, JSON.stringify(val)])
  )
}

// Plugin to copy Python files to output
function copyPythonFilesPlugin() {
  return {
    name: 'copy-python-files',
    writeBundle() {
      const pythonSrcDir = resolve('src/main/python')
      const pythonOutDir = resolve('out/main/python')

      if (fs.existsSync(pythonSrcDir)) {
        fs.mkdirSync(pythonOutDir, { recursive: true })
        const files = fs.readdirSync(pythonSrcDir)

        for (const file of files) {
          if (file.endsWith('.py')) {
            const srcFile = resolve(pythonSrcDir, file)
            const outFile = resolve(pythonOutDir, file)
            fs.copyFileSync(srcFile, outFile)
            console.log(`Copied ${file} to output directory`)
          }
        }
      }
    }
  }
}

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  // console.log('[electron-vite] Loaded ENV:', env)
  return {
    main: {
      plugins: [externalizeDepsPlugin(), copyPythonFilesPlugin()],
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
        __APP_ENV__: JSON.stringify(env),
        // Define process.env object for renderer
        'process.env': JSON.stringify({
          NODE_ENV: process.env.NODE_ENV,
          ...Object.fromEntries(
            Object.entries(env).filter(([key]) => key.startsWith('NEXT_PUBLIC_'))
          )
        })
      },
      plugins: [
        TanStackRouterVite({ target: 'react', autoCodeSplitting: true }),
        react(),
        tailwindcss()
      ]
    }
  }
})
