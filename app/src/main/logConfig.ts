import Logger from 'electron-log'
import path from 'path'
import fs from 'fs'

function pruneOldLogs(dir: string, maxFiles = 10) {
  const files = fs
    .readdirSync(dir)
    .filter((f) => /^main\.\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}/.test(f))
    .map((f) => ({
      name: f,
      path: path.join(dir, f),
      mtime: fs.statSync(path.join(dir, f)).mtimeMs
    }))
    .sort((a, b) => b.mtime - a.mtime) // newest first

  const excess = files.slice(maxFiles)

  for (const file of excess) {
    try {
      fs.unlinkSync(file.path)
    } catch (err) {
      console.error('Failed to delete log:', file.name, err)
    }
  }
}

export function rotateLog(logFile: Logger.LogFile) {
  const dir = path.dirname(logFile.path)
  const timestamp = new Date().toISOString().replace(/[:.]/g, '-')
  const archiveName = `main.${timestamp}.log`
  const newPath = path.join(dir, archiveName)

  try {
    fs.renameSync(logFile.path, newPath)
    pruneOldLogs(dir, 10)

    return true
  } catch (err: unknown) {
    return false
  }
}
