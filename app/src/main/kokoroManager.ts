import log from 'electron-log/main'
import { DependencyProgress, KokoroBootstrap } from './pythonManager'

let kokoro: KokoroBootstrap | null = null

export function startKokoro(mainWindow: Electron.BrowserWindow) {
  // Check if Kokoro should be started based on environment variable
  const startKokoroEnv = process.env.START_KOKORO
  if (startKokoroEnv === 'FALSE' || startKokoroEnv === 'false') {
    log.info('[Kokoro] Kokoro startup disabled by START_KOKORO environment variable')
    return null
  }

  log.info('[Kokoro] Starting Kokoro TTS service')
  
  const kokoroProgress = (data: DependencyProgress) => {
    if (mainWindow) {
      log.info(`[Kokoro] Emitting launch-progress: ${data.progress}, Status: ${data.status}`)
      mainWindow.webContents.send('launch-progress', data)
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

export async function getKokoroState(): Promise<DependencyProgress> {
  if (!kokoro) {
    return {
      dependency: 'Kokoro',
      progress: 0,
      status: 'Not started'
    }
  }
  return kokoro.getLatestProgress()
}
