import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function getMockFrequencyData(): Uint8Array {
  const arraySize = 128
  const freqData = new Uint8Array(arraySize)

  const time = Date.now() * 0.001

  for (let i = 0; i < arraySize; i++) {
    const lowFreq = Math.sin(time * 2 + i * 0.1) * 40 + 60
    const midFreq = Math.sin(time * 3 + i * 0.05) * 30 + 80
    const highFreq = Math.sin(time * 1.5 + i * 0.15) * 20 + 40

    let amplitude = 0
    if (i < arraySize * 0.3) {
      amplitude = lowFreq
    } else if (i < arraySize * 0.7) {
      amplitude = midFreq
    } else {
      amplitude = highFreq
    }
    amplitude += (Math.random() - 0.5) * 15
    amplitude *= 0.8 + 0.2 * Math.sin(time * 0.5)
    freqData[i] = Math.max(0, Math.min(255, Math.round(amplitude)))
  }

  return freqData
}
