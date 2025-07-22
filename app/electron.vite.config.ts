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

        // Function to recursively copy directories
        function copyRecursive(src: string, dest: string) {
          const stats = fs.statSync(src)

          if (stats.isDirectory()) {
            if (!fs.existsSync(dest)) {
              fs.mkdirSync(dest, { recursive: true })
            }
            const files = fs.readdirSync(src)
            for (const file of files) {
              copyRecursive(resolve(src, file), resolve(dest, file))
            }
          } else {
            fs.copyFileSync(src, dest)
            console.log(`Copied ${src} to ${dest}`)
          }
        }

        const items = fs.readdirSync(pythonSrcDir)

        for (const item of items) {
          const srcPath = resolve(pythonSrcDir, item)
          const destPath = resolve(pythonOutDir, item)

          if (fs.statSync(srcPath).isDirectory()) {
            // Copy entire directory structure
            copyRecursive(srcPath, destPath)
            console.log(`Copied directory ${item} to output directory`)
          } else if (item.endsWith('.py')) {
            // Copy individual Python files
            fs.copyFileSync(srcPath, destPath)
            console.log(`Copied ${item} to output directory`)
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
          '@renderer': resolve('src/renderer/src'),
          '@resources': resolve('resources')
        }
      },
      define: {
        __APP_ENV__: JSON.stringify(env)
      },
      plugins: [
        TanStackRouterVite({ target: 'react', autoCodeSplitting: true }),
        react(),
        tailwindcss()
      ],
      build: {
        rollupOptions: {
          input: {
            index: resolve(__dirname, 'src/renderer/index.html'),
            omnibar: resolve(__dirname, 'src/renderer/omnibar.html')
          }
        }
      }
    }
  }
})
