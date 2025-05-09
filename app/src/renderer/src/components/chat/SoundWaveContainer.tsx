import { Canvas } from '@react-three/fiber'
import { OrbitControls } from '@react-three/drei'
import { SoundWave } from './SoundWave'
import { useRef, useEffect, useState } from 'react'
import * as THREE from 'three'

interface SoundWaveContainerProps {
  audioUrl?: string
  size?: number
  color?: string
}

export function SoundWaveContainer({ audioUrl, size = 1, color }: SoundWaveContainerProps) {
  const [audioSource, setAudioSource] = useState<MediaElementAudioSourceNode | null>(null)
  const audioContextRef = useRef<AudioContext | null>(null)

  useEffect(() => {
    if (!audioUrl) return

    const setupAudio = async () => {
      try {
        const audioContext = new AudioContext()
        audioContextRef.current = audioContext

        const response = await fetch(audioUrl)
        const arrayBuffer = await response.arrayBuffer()
        const audioBuffer = await audioContext.decodeAudioData(arrayBuffer)

        const source = audioContext.createBufferSource()
        source.buffer = audioBuffer
        source.loop = true

        // Create a gain node to control the audio output
        const gainNode = audioContext.createGain()
        source.connect(gainNode)
        gainNode.connect(audioContext.destination)

        // Create an analyzer node for visualization
        const analyser = audioContext.createAnalyser()
        gainNode.connect(analyser)

        // Create a media source for visualization
        const mediaSource = audioContext.createMediaElementSource(new Audio())
        setAudioSource(mediaSource)

        source.start(0)
      } catch (error) {
        console.error('Error setting up audio:', error)
      }
    }

    setupAudio()

    return () => {
      if (audioContextRef.current) {
        audioContextRef.current.close()
      }
    }
  }, [audioUrl])

  return (
    <div className="w-full h-48">
      <Canvas
        camera={{ position: [0, 0, 2], fov: 50 }}
        gl={{ alpha: true, antialias: true }}
        style={{ background: 'transparent' }}
      >
        <ambientLight intensity={0.5} />
        <pointLight position={[10, 10, 10]} />
        <SoundWave audioSource={audioSource || undefined} size={size} color={color} />
        <OrbitControls enableZoom={false} enablePan={false} />
      </Canvas>
    </div>
  )
}
