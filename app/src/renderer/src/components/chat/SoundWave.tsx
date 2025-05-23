import { useRef, useEffect, useState } from 'react'
import { useFrame } from '@react-three/fiber'
import * as THREE from 'three'

interface SoundWaveProps {
  audioSource?: MediaElementAudioSourceNode
  size?: number
  color?: string
}

export function SoundWave({ audioSource, size = 1, color = '#4f46e5' }: SoundWaveProps) {
  const meshRef = useRef<THREE.Mesh>(null)
  const [audioData, setAudioData] = useState<Float32Array>(new Float32Array(128))
  const analyserRef = useRef<AnalyserNode | null>(null)

  useEffect(() => {
    if (!audioSource) return

    const analyser = audioSource.context.createAnalyser()
    analyser.fftSize = 256
    audioSource.connect(analyser)
    analyserRef.current = analyser

    const dataArray = new Float32Array(analyser.frequencyBinCount)
    setAudioData(dataArray)

    return () => {
      analyser.disconnect()
    }
  }, [audioSource])

  useFrame(() => {
    if (!meshRef.current || !analyserRef.current) return

    analyserRef.current.getFloatTimeDomainData(audioData)

    const geometry = meshRef.current.geometry as THREE.BufferGeometry
    const positions = geometry.attributes.position.array as Float32Array

    for (let i = 0; i < positions.length / 3; i++) {
      const audioIndex = Math.floor((i / (positions.length / 3)) * audioData.length)
      const amplitude = audioData[audioIndex] * size

      positions[i * 3 + 1] = amplitude
    }

    geometry.attributes.position.needsUpdate = true
  })

  return (
    <group>
      <mesh ref={meshRef}>
        <planeGeometry args={[2, 1, 128, 1]} />
        <meshStandardMaterial
          color={color}
          side={THREE.DoubleSide}
          transparent
          opacity={0.8}
          metalness={0.5}
          roughness={0.2}
        />
      </mesh>
    </group>
  )
}
