import log from 'electron-log/main'
import { KokoroBootstrap } from './pythonManager'

let kokoro: KokoroBootstrap | null = null

export function startKokoro(mainWindow: Electron.BrowserWindow) {
  const kokoroProgress = (progress: number, status?: string) => {
    if (mainWindow) {
      log.info(`[Kokoro] Emitting launch-progress: ${progress}, Status: ${status}`)
      mainWindow.webContents.send('launch-progress', { progress, status })
    }
  }

  kokoro = new KokoroBootstrap(kokoroProgress)

  try {
    kokoro.setup()
  } catch (error) {
    console.error('Failed to setup Python environment:', error)
  }

  return kokoro
}

export async function cleanupKokoro() {
  if (kokoro) {
    log.info('Cleaning up Kokoro TTS server...')
    await kokoro.cleanup()
    kokoro = null
  }
}
