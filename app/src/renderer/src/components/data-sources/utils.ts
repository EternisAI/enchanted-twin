export const truncatePath = (path: string): string => {
  return path.length > 40 ? '...' + path.slice(-37) : path
}

// Estimate time based on source type and progress
export const estimateRemainingTime = (
  sourceName: string,
  progress: number,
  startTime?: Date,
  fileSize?: number
): string => {
  let baseMinutes: number

  if (sourceName === 'WhatsApp' && fileSize) {
    // For WhatsApp, estimate based on file size
    // Approximately 1 minute per 50MB
    const fileSizeMB = fileSize / (1024 * 1024)
    baseMinutes = Math.ceil(fileSizeMB / 50)
    // Minimum 2 minutes, maximum 30 minutes
    baseMinutes = Math.max(2, Math.min(30, baseMinutes))
  } else {
    // Base estimates in minutes for each source type
    const baseEstimates: Record<string, number> = {
      X: 5,
      ChatGPT: 3,
      WhatsApp: 10,
      Telegram: 8,
      Slack: 7,
      Gmail: 15
    }
    baseMinutes = baseEstimates[sourceName] || 5
  }

  if (!startTime || progress === 0) {
    return `~${baseMinutes} min`
  }

  const elapsedMs = Date.now() - startTime.getTime()
  const elapsedMinutes = elapsedMs / 60000

  if (progress > 0) {
    const totalEstimatedMinutes = elapsedMinutes / (progress / 100)
    const remainingMinutes = Math.max(0, totalEstimatedMinutes - elapsedMinutes)

    if (remainingMinutes < 1) {
      return 'Less than 1 min'
    } else if (remainingMinutes < 60) {
      return `~${Math.ceil(remainingMinutes)} min`
    } else {
      const hours = Math.floor(remainingMinutes / 60)
      const mins = Math.round(remainingMinutes % 60)
      return `~${hours}h ${mins}m`
    }
  }

  return `~${baseMinutes} min`
}
